#pragma once

#include "LauncherUiExport.h"
#include <QWidget>
#include <QJsonObject>
#include <memory>

class GitDiffWidget;
class QLabel;
class QPushButton;
class ProjectGitClient;

// Read-only comparison with an immutable originating repository and Git path.
class DOCKPIPE_LAUNCHER_UI_EXPORT DashboardGitPreview final : public QWidget {
    Q_OBJECT
public:
    DashboardGitPreview(const QString &repository, const QString &path, bool staged, QWidget *parent = nullptr);
    DashboardGitPreview(const QString &repository, const QString &path, bool staged,
                        const QJsonObject &catalog, const QString &rootId, QWidget *parent = nullptr);
    ~DashboardGitPreview() override;
    QString rootPath() const { return m_root; }
    QString currentPath() const { return m_path; }
    bool isStaged() const { return m_staged; }
    void refresh();

protected:
    void hideEvent(QHideEvent *event) override;
    void showEvent(QShowEvent *event) override;

private:
    const QString m_root;
    const QString m_path;
    const bool m_staged;
    quint64 m_generation = 0;
    bool m_refreshOnIdle = false;
    std::unique_ptr<ProjectGitClient> m_client;
    GitDiffWidget *m_diff;
    QLabel *m_status;
    QPushButton *m_refresh;
};
