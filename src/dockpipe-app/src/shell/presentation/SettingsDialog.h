#pragma once

#include "LauncherSettings.h"

#include <QDialog>

class QLineEdit;
class QPlainTextEdit;
class QCheckBox;
class QTabWidget;
class RemoteSecurityWidget;

class SettingsDialog : public QDialog {
    Q_OBJECT
public:
    explicit SettingsDialog(const LauncherSettings &settings, QWidget *parent = nullptr);

    ~SettingsDialog() override;
    LauncherSettings updatedSettings() const;

private slots:
    void browseRepoRoot();
    void browseGlobalRoot();

private:
    static QStringList splitLines(const QString &text);
    static QString joinLines(const QStringList &values);

    QLineEdit *m_repoRootOverride = nullptr;
    QLineEdit *m_globalRootOverride = nullptr;
    QPlainTextEdit *m_extraWorkflowRoots = nullptr;
    QPlainTextEdit *m_packageRemotes = nullptr;
    LauncherSettings m_settings;
    bool m_installPending = false;
    QCheckBox *m_defaultDashboardVisible = nullptr;
    QCheckBox *m_tilingEnabled = nullptr;
    QCheckBox *m_cockpitEnabled = nullptr;
    QCheckBox *m_appControl = nullptr;
    QCheckBox *m_orbitalMotion = nullptr;
    QCheckBox *m_controller = nullptr;
    QCheckBox *m_showShip = nullptr;
    QCheckBox *m_voiceCommands = nullptr;
    QCheckBox *m_hardwareAcceleration = nullptr;
    QTabWidget *m_tabs = nullptr;
    RemoteSecurityWidget *m_remoteSecurity = nullptr;
};
