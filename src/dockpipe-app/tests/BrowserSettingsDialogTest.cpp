#include "BrowserPreferences.h"
#include "BrowserSettingsDialog.h"
#include "Theme.h"

#include <QApplication>
#include <QCheckBox>
#include <QComboBox>
#include <QDialogButtonBox>
#include <QFileInfo>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QScrollArea>
#include <QScrollBar>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QtTest>

class BrowserSettingsDialogTest final : public QObject {
    Q_OBJECT
private slots:
    void synchronizesSettingsBetweenWindows();
    void validatesAndSavesExactOriginRules();
    void captureSettings();
};

void BrowserSettingsDialogTest::synchronizesSettingsBetweenWindows()
{
    BrowserSettingsDialog first;
    BrowserSettingsDialog second;
    auto *firstFullUrl = first.findChild<QCheckBox *>(QStringLiteral("browserSettingsFullUrl"));
    auto *secondFullUrl = second.findChild<QCheckBox *>(QStringLiteral("browserSettingsFullUrl"));
    auto *history = first.findChild<QPushButton *>(QStringLiteral("browserSettingsHistory"));
    QVERIFY(firstFullUrl);
    QVERIFY(secondFullUrl);
    QVERIFY(history);
    QSignalSpy changes(BrowserPreferences::instance(), &BrowserPreferences::changed);
    const bool enabled = !firstFullUrl->isChecked();
    firstFullUrl->setChecked(enabled);
    QCOMPARE(BrowserPreferences::instance()->settings().showFullUrl, enabled);
    QCOMPARE(secondFullUrl->isChecked(), enabled);
    QCOMPARE(changes.count(), 1);
    QSignalSpy historyRequests(&first, &BrowserSettingsDialog::historyRequested);
    history->click();
    QCOMPARE(historyRequests.count(), 1);
}

void BrowserSettingsDialogTest::validatesAndSavesExactOriginRules()
{
    BrowserSettingsDialog settings;
    auto *add = settings.findChild<QPushButton *>(QStringLiteral("browserSettingsAddSiteRule"));
    QVERIFY(add);
    add->click();
    auto *editor = settings.findChild<QDialog *>(QStringLiteral("browserSettingsSiteRuleEditor"));
    QVERIFY(editor);
    auto *origin = editor->findChild<QLineEdit *>(QStringLiteral("browserSettingsRuleOrigin"));
    auto *policy = editor->findChild<QComboBox *>(QStringLiteral("browserSettingsRulePolicy"));
    auto *error = editor->findChild<QLabel *>(QStringLiteral("browserSettingsRuleError"));
    auto *buttons = editor->findChild<QDialogButtonBox *>();
    QVERIFY(origin);
    QVERIFY(policy);
    QVERIFY(error);
    QVERIFY(buttons);
    origin->setText(QStringLiteral("https://example.test/private/page"));
    buttons->button(QDialogButtonBox::Save)->click();
    QVERIFY(!error->text().isEmpty());
    QVERIFY(!BrowserPreferences::instance()->settings().sitePermissions.contains(QStringLiteral("https://example.test")));
    origin->setText(QStringLiteral("https://example.test:8443"));
    policy->setCurrentIndex(policy->findData(QStringLiteral("block")));
    buttons->button(QDialogButtonBox::Save)->click();
    QCOMPARE(BrowserPreferences::instance()->settings().sitePermissions
                 .value(QStringLiteral("https://example.test:8443")).value(QStringLiteral("camera")),
             QStringLiteral("block"));
}

void BrowserSettingsDialogTest::captureSettings()
{
    const QString path = qEnvironmentVariable("DOCKPIPE_BROWSER_SETTINGS_CAPTURE");
    if (path.isEmpty())
        return;
    BrowserSettingsDialog dialog;
    dialog.show();
    QTest::qWait(60);
    QVERIFY(dialog.grab().save(path));
    auto *scroll = dialog.findChild<QScrollArea *>(QStringLiteral("browserSettingsScroll"));
    QVERIFY(scroll);
    const QFileInfo file(path);
    scroll->verticalScrollBar()->setValue(scroll->verticalScrollBar()->maximum() / 2);
    QCoreApplication::processEvents();
    QVERIFY(dialog.grab().save(file.path() + "/" + file.completeBaseName() + "-middle.png"));
    scroll->verticalScrollBar()->setValue(scroll->verticalScrollBar()->maximum());
    QCoreApplication::processEvents();
    QVERIFY(dialog.grab().save(file.path() + "/" + file.completeBaseName() + "-bottom.png"));
}

int main(int argc, char **argv)
{
    QTemporaryDir storage;
    if (!storage.isValid())
        return 1;
    qputenv("XDG_CONFIG_HOME", storage.path().toUtf8());
    QApplication app(argc, argv);
    QCoreApplication::setOrganizationName(QStringLiteral("DockPipeSettingsDialogTest"));
    QCoreApplication::setApplicationName(QStringLiteral("DockPipeSettingsDialogTest"));
    applyDockpipeLauncherTheme(app);
    BrowserSettingsDialogTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "BrowserSettingsDialogTest.moc"
