#include "WorkspaceMachineMenu.h"

#include <QAction>
#include <QMenu>
#include <QSignalSpy>
#include <QTest>

class WorkspaceMachineMenuTest final : public QObject {
    Q_OBJECT
private slots:
    void unavailableSelectionNeverBecomesLocal()
    {
        WorkspaceMachineMenu selector;
        const QJsonObject target{{"node", "worker"}, {"fingerprint", QString(64, 'a')},
                                 {"generation", QString(64, 'b')}};
        const QJsonObject root{{"id", "source"}, {"name", "Project"}, {"files_read", true}};
        selector.setAccessUnlocked(true);
        selector.setCatalog({QJsonObject{{"target", target}, {"roots", QJsonArray{root}}}});
        selector.setCurrent(target, QStringLiteral("source"));
        QVERIFY(selector.text().contains(QStringLiteral("Project")));
        selector.setCatalog({});
        QCOMPARE(selector.currentTarget(), target);
        QCOMPARE(selector.currentRoot(), QStringLiteral("source"));
        QVERIFY(selector.text().contains(QStringLiteral("Unavailable")));
        selector.setAccessUnlocked(false);
        QVERIFY(selector.text().contains(QStringLiteral("Locked")));
        QCOMPARE(selector.currentTarget(), target);
    }
    void requestDoesNotCommitBeforeDraftApproval()
    {
        WorkspaceMachineMenu selector;
        const QJsonObject target{{"node", "worker"}, {"fingerprint", QString(64, 'a')},
                                 {"generation", QString(64, 'b')}};
        selector.setAccessUnlocked(true);
        selector.setCatalog({QJsonObject{{"target", target}, {"roots", QJsonArray{
            QJsonObject{{"id", "source"}, {"name", "Project"}}}}}});
        QSignalSpy requested(&selector, &WorkspaceMachineMenu::contextRequested);
        QVERIFY(QMetaObject::invokeMethod(selector.menu(), "aboutToShow"));
        QMenu *machine = nullptr;
        for (auto *action : selector.menu()->actions()) {
            if (action->menu())
                machine = action->menu();
        }
        QVERIFY(machine);
        QCOMPARE(machine->actions().size(), 1);
        machine->actions().first()->trigger();
        QCOMPARE(requested.size(), 1);
        QCOMPARE(requested.first().at(0).toJsonObject(), target);
        QCOMPARE(requested.first().at(1).toString(), QStringLiteral("source"));
        QVERIFY(selector.currentTarget().isEmpty());
        selector.setCurrent(target, QStringLiteral("source"));
        QCOMPARE(selector.currentTarget(), target);
    }
};

QTEST_MAIN(WorkspaceMachineMenuTest)
#include "WorkspaceMachineMenuTest.moc"
