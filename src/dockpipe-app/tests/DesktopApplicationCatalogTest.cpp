#include "DesktopApplicationCatalog.h"

#include <QDir>
#include <QFile>
#include <QTemporaryDir>
#include <QTest>

class DesktopApplicationCatalogTest final : public QObject {
    Q_OBJECT
private slots:
    void metadataAndRevalidation();
};

void DesktopApplicationCatalogTest::metadataAndRevalidation()
{
#ifndef Q_OS_LINUX
    QSKIP("Linux desktop metadata fixture");
#else
    QTemporaryDir temporary;
    QVERIFY(temporary.isValid());
    const QByteArray oldHome = qgetenv("XDG_DATA_HOME");
    const QByteArray oldDirs = qgetenv("XDG_DATA_DIRS");
    struct RestoreEnvironment {
        QByteArray home;
        QByteArray dirs;
        ~RestoreEnvironment()
        {
            qputenv("XDG_DATA_HOME", home);
            qputenv("XDG_DATA_DIRS", dirs);
        }
    } restore{oldHome, oldDirs};
    qputenv("XDG_DATA_HOME", temporary.path().toUtf8());
    qputenv("XDG_DATA_DIRS", (temporary.path() + "/system").toUtf8());
    QVERIFY(QDir().mkpath(temporary.path() + "/applications"));
    const QString path = temporary.path() + "/applications/fixture.desktop";
    QFile entry(path);
    QVERIFY(entry.open(QIODevice::WriteOnly));
    entry.write("[Desktop Entry]\nType=Application\nName=Fixture application\nIcon=fixture\nStartupWMClass=FixtureClass\nExec=never-executed-by-this-test\n");
    entry.close();
    QString error;
    auto applications = DesktopApplicationCatalog::enumerate(&error);
    QVERIFY2(error.isEmpty(), qPrintable(error));
    QCOMPARE(applications.size(), 1);
    QCOMPARE(applications.front().name, QString("Fixture application"));
    QCOMPARE(applications.front().iconName, QString("fixture"));
    QCOMPARE(applications.front().windowClass, QString("FixtureClass"));
    QVERIFY(applications.front().hasDeclaredWindowClass);
    QCOMPARE(applications.front().launchTarget, path);
    DesktopApplicationEntry unrelated = applications.front();
    unrelated.launchTarget = "/tmp/not-an-installed-application.desktop";
    QVERIFY(!DesktopApplicationCatalog::launch(unrelated, error));
    QVERIFY(error.contains("no longer"));
    QVERIFY(QFile::remove(path));
    QVERIFY(!DesktopApplicationCatalog::launch(applications.front(), error));
    QVERIFY(error.contains("no longer"));
    QVERIFY(entry.open(QIODevice::WriteOnly));
    entry.write("[Desktop Entry]\nType=Application\nName=Hidden\nHidden=true\n");
    entry.close();
    QVERIFY(DesktopApplicationCatalog::enumerate(&error).isEmpty());
    QVERIFY(error.isEmpty());
    for (int index = 0; index < 501; ++index) {
        QFile extra(temporary.path() + "/applications/extra" + QString::number(index) + ".desktop");
        QVERIFY(extra.open(QIODevice::WriteOnly));
        extra.write("[Desktop Entry]\nType=Application\nName=Extra\n");
    }
    QVERIFY(DesktopApplicationCatalog::enumerate(&error).isEmpty());
    QVERIFY(error.contains("500"));
#endif
}

QTEST_GUILESS_MAIN(DesktopApplicationCatalogTest)
#include "DesktopApplicationCatalogTest.moc"
