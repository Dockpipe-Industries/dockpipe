#pragma once

#include "WorkspaceFilesClient.h"
#include <QJsonObject>
#include <QObject>

class DashboardFilesWidget;
class QStandardItemModel;

// Private browsing state keeps the published dashboard widget layout stable.
class DashboardFilesController final : public QObject {
    Q_OBJECT
public:
    explicit DashboardFilesController(DashboardFilesWidget *widget);
    static DashboardFilesController *get(const DashboardFilesWidget *widget);
    bool setMachineContext(const QJsonObject &target, const QString &rootId,
                           const QString &localRoot, QString *error);
    void setScope(const QString &scope, const QString &projectPath);
    bool navigate(const QString &path);
    void showFilesystem();
    void clearView();
    void setRemoteDispatch(WorkspaceFilesClient::RemoteDispatch dispatch);
    QJsonObject machineTarget() const { return m_target; }
    QString remoteRootId() const { return m_remote ? m_rootId : QString(); }
    bool remote() const { return m_remote; }
    bool available() const { return m_valid && (!m_root.isEmpty() || m_drives); }
    bool drives() const { return m_drives; }
    QString root() const { return m_root; }

private:
    void selectRoot(const QString &root, const QString &directory);
    void requestDirectory(const QString &directory);
    void listingLoaded(const QString &relativePath);
    void listingFailed(const QString &relativePath, const QString &error);
    DashboardFilesWidget *m_widget;
    QJsonObject m_target;
    QString m_rootId;
    QString m_localRoot;
    QString m_root;
    QString m_appliedScope;
    QString m_pendingDirectory;
    QString m_pendingRelative;
    QStandardItemModel *m_driveModel;
    quint64 m_epoch = 0;
    bool m_remote = false;
    bool m_valid = false;
    bool m_drives = false;
};
