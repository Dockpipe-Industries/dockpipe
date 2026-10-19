#pragma once

#include "LauncherUiExport.h"
#include "LauncherWorkspacePackage.h"
#include "LauncherSettings.h"
#include "PackageCommand.h"
#include <QPluginLoader>
#include <QProcess>
#include <QTimer>

class DOCKPIPE_LAUNCHER_UI_EXPORT WorkspacePackageManager final : public QObject {
    Q_OBJECT
public:
    static WorkspacePackageManager &instance();
    explicit WorkspacePackageManager(QObject *parent = nullptr);
    ~WorkspacePackageManager() override;
    bool ready() const { return m_package != nullptr; }
    bool busy() const { return m_active || m_resolver.state() != QProcess::NotRunning || m_command.busy(); }
    void prepare(const LauncherSettings &settings, bool download = true);
    void cancel();
    LauncherWorkspaceSurface *createWorkspace(QWidget *parent);
    LauncherFileSurface *createFiles(const QString &root, const QString &directory, QWidget *parent);
signals:
    void prepared();
    void failed(const QString &message);
    void progress(const QString &message);
private:
    enum class Operation { Resolve, Catalog, Install };
    void resolve();
    bool load(const QString &assets);
    void catalog();
    void fail(const QString &error);
    QPluginLoader m_loader;
    LauncherWorkspacePackage *m_package = nullptr;
    QProcess m_resolver;
    QTimer m_deadline;
    PackageCommand m_command;
    LauncherSettings m_settings;
    QByteArray m_output;
    QString m_error;
    QString m_manifest;
    int m_remote = 0;
    bool m_active = false;
    bool m_download = true;
    bool m_installed = false;
    Operation m_operation = Operation::Resolve;
};
