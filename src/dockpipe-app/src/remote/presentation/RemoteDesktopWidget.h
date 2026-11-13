#pragma once

#include "LauncherUiExport.h"
#include "RemoteCommand.h"
#include "WorkflowCatalogData.h"

#include <QJsonArray>
#include <QPointer>
#include <QWidget>
#include <functional>

class QJsonDocument;
class QComboBox;
class QCheckBox;
class QLabel;
class QPushButton;
class QTimer;
class QDialog;
class RemoteAccessMonitor;
class PackageDisplayWidget;

// Native desktop rendering and transport are supplied by a package display.
// This page freezes only discovery/selection metadata for an explicit connection.
class DOCKPIPE_LAUNCHER_UI_EXPORT RemoteDesktopWidget final : public QWidget {
    Q_OBJECT
public:
    explicit RemoteDesktopWidget(QWidget *parent = nullptr);
    ~RemoteDesktopWidget() override;
    void setResolvers(const QVector<ResolverMeta> &resolvers);
    void setAccessMonitor(RemoteAccessMonitor *monitor);
    void setPairedMachines(const QJsonArray &machines);
    void setCompactPresentation(bool enabled);

public slots:
    void refresh();
    void disconnectDesktop();

signals:
    void installProviderRequested();
    void providerSetupRequested(const QString &workflowId, const QString &surfaceId, const QString &actionId);

protected:
    void showEvent(QShowEvent *event) override;
    void hideEvent(QHideEvent *event) override;

private:
    void query(const QString &command, quint64 generation, std::function<void(const QJsonDocument &)> completion);
    void refreshMetadata(bool connecting);
    void applySelection();
    void updateActions();
    void connectDesktop(const QJsonObject &identity, const QString &mode);
    void openDisplay(const QJsonObject &identity, const QString &mode);
    QJsonObject selectedIdentity() const;
    QJsonObject selectedCatalog() const;
    const ResolverMeta *providerFor(const QJsonObject &catalog) const;
    bool accessUnlocked() const;
    void cancelRefresh();
    void showProviderSetup();
    void showDesktopSettings();
    void enterFullscreen();
    void exitFullscreen();
    void refreshDecoderStatus();

    QComboBox *m_machine;
    QComboBox *m_mode;
    QCheckBox *m_keepConnected;
    QLabel *m_status;
    QLabel *m_preferencesNotice;
    QLabel *m_decoderStatus;
    QPushButton *m_connect;
    QPushButton *m_disconnect;
    QPushButton *m_refresh;
    QPushButton *m_unlock;
    QPushButton *m_packages;
    QPushButton *m_retained;
    QPushButton *m_settings;
    QPushButton *m_fullscreen;
    PackageDisplayWidget *m_display;
    QTimer *m_timer;
    QTimer *m_decoderTimer;
    QPointer<RemoteAccessMonitor> m_access;
    QVector<ResolverMeta> m_resolvers;
    QJsonArray m_paired;
    QJsonArray m_catalogs;
    QString m_cli;
    QString m_stateDirectory;
    quint64 m_generation = 0;
    bool m_busy = false;
    bool m_connected = false;
    bool m_closing = false;
    bool m_automaticMode = true;
    QJsonObject m_displayCatalog;
    QJsonObject m_captureOptions;
    QJsonObject m_presentationOptions;
    QString m_decoderMode;
    QPointer<QDialog> m_fullscreenWindow;
    bool m_compactPresentation = false;
    RemoteCommand m_command;
};
