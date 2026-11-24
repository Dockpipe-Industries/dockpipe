#include "NativeDisplaySession.h"

#include <QApplication>
#include <QElapsedTimer>
#include <QSignalSpy>
#include <QTest>

namespace {
QJsonObject reference(const QString &id)
{
    return {{"schema", "dockpipe.remote.desktop/v1"}, {"id", id},
        {"target", QJsonObject{{"node", "fixture"}, {"fingerprint", QString(64, 'a')}, {"generation", QString(64, 'b')}}},
        {"epoch", QString(64, 'c')}};
}
}

class DeferredSurface final : public QObject {
    Q_OBJECT
    Q_PROPERTY(int closeCompletionBudgetMs MEMBER budget)
    Q_PROPERTY(QJsonObject closeReference MEMBER prepared)
public:
    int budget = 500;
    QJsonObject prepared = reference(QString(64, 'd'));
    int requests = 0;
    Q_INVOKABLE void requestClose() { ++requests; }
    void complete(const QString &outcome, QJsonObject selected = {})
    {
        if (selected.isEmpty())
            selected = prepared;
        emit closeCompleted({{"schema", "dockpipe.desktop.close/v1"}, {"reference", selected}, {"outcome", outcome}});
    }
signals:
    void closeCompleted(const QJsonObject &result);
};

class NativeDisplaySessionTest final : public QObject {
    Q_OBJECT
private slots:
    void delayedCompletionKeepsEventLoopResponsive_data()
    {
        QTest::addColumn<QString>("outcome");
        QTest::newRow("verified-closed") << QString("closed");
        QTest::newRow("unknown") << QString("unknown");
    }
    void delayedCompletionKeepsEventLoopResponsive()
    {
        QFETCH(QString, outcome);
        DeferredSurface surface;
        NativeDisplaySession session;
        QSignalSpy completed(&session, &NativeDisplaySession::closeCompleted);
        int ticks = 0;
        QTimer timer;
        timer.setInterval(5);
        connect(&timer, &QTimer::timeout, this, [&] {
            if (++ticks == 6) {
                timer.stop();
                surface.complete(outcome);
            }
        });
        QElapsedTimer elapsed;
        elapsed.start();
        QVERIFY(session.requestClose(&surface));
        QVERIFY(elapsed.elapsed() < 100);
        QCOMPARE(surface.requests, 1);
        QCOMPARE(completed.size(), 0);
        timer.start();
        QTRY_COMPARE(completed.size(), 1);
        QCOMPARE(completed.first().first().toJsonObject().value("outcome").toString(), outcome);
        QCOMPARE(ticks, 6);
        surface.complete("closed");
        QCOMPARE(completed.size(), 1);
    }
    void foreignReferenceCannotFinishOriginalClose()
    {
        DeferredSurface surface;
        NativeDisplaySession session;
        QSignalSpy completed(&session, &NativeDisplaySession::closeCompleted);
        QVERIFY(session.requestClose(&surface));
        surface.complete("closed", reference(QString(64, 'e')));
        QCOMPARE(completed.size(), 0);
        surface.prepared = reference(QString(64, 'f'));
        surface.complete("closed");
        QCOMPARE(completed.size(), 0);
        surface.complete("closed", reference(QString(64, 'd')));
        QCOMPARE(completed.size(), 1);
    }
    void deadlineFinishesWithoutClosureProof()
    {
        DeferredSurface surface;
        surface.budget = 30;
        NativeDisplaySession session;
        QSignalSpy completed(&session, &NativeDisplaySession::closeCompleted);
        QVERIFY(session.requestClose(&surface));
        QTRY_COMPARE(completed.size(), 1);
        QVERIFY(completed.first().first().toJsonObject().isEmpty());
        surface.complete("closed");
        QCOMPARE(completed.size(), 1);
    }
    void cancelledContextIgnoresLateCompletion()
    {
        DeferredSurface original;
        DeferredSurface replacement;
        replacement.prepared = reference(QString(64, 'e'));
        NativeDisplaySession session;
        QSignalSpy completed(&session, &NativeDisplaySession::closeCompleted);
        QVERIFY(session.requestClose(&original));
        session.cancel();
        original.complete("closed");
        QCOMPARE(completed.size(), 0);
        QVERIFY(session.requestClose(&replacement));
        original.complete("closed", replacement.prepared);
        QCOMPARE(completed.size(), 0);
        replacement.complete("unknown");
        QCOMPARE(completed.size(), 1);
        QCOMPARE(completed.first().first().toJsonObject().value("reference").toObject(), replacement.prepared);
    }
    void destroyedSurfaceAndConsumerCancelSafely()
    {
        NativeDisplaySession session;
        QSignalSpy completed(&session, &NativeDisplaySession::closeCompleted);
        auto *surface = new DeferredSurface;
        QVERIFY(session.requestClose(surface));
        delete surface;
        QCOMPARE(completed.size(), 1);
        QVERIFY(completed.first().first().toJsonObject().isEmpty());
        DeferredSurface retained;
        auto *consumer = new NativeDisplaySession;
        QVERIFY(consumer->requestClose(&retained));
        delete consumer;
        retained.complete("closed");
    }
    void missingPreparedReferenceNeverProvesClosed()
    {
        DeferredSurface surface;
        surface.prepared = {};
        NativeDisplaySession session;
        QSignalSpy completed(&session, &NativeDisplaySession::closeCompleted);
        QVERIFY(session.requestClose(&surface));
        surface.complete("closed");
        QCOMPARE(completed.size(), 0);
        surface.complete("unknown");
        QCOMPARE(completed.size(), 1);
    }
    void optionalContractBoundsAndLegacyFallback()
    {
        NativeDisplaySession session;
        QObject legacy;
        QVERIFY(!session.requestClose(&legacy));
        DeferredSurface surface;
        for (int budget : {0, -1, 12001}) {
            surface.budget = budget;
            QVERIFY(!session.requestClose(&surface));
        }
        surface.budget = 12000;
        surface.prepared = {{"oversized", QString(17000, 'x')}};
        QVERIFY(!session.requestClose(&surface));
        QCOMPARE(surface.requests, 0);
    }
};

QTEST_MAIN(NativeDisplaySessionTest)
#include "NativeDisplaySessionTest.moc"
