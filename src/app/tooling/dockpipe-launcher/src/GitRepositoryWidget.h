#pragma once

#include "LauncherUiExport.h"

#include "ProjectGitClient.h"
#include <QJsonObject>
#include <QWidget>

class QCheckBox;
class QLabel;
class QLineEdit;
class QListWidget;

class DOCKPIPE_LAUNCHER_UI_EXPORT GitRepositoryWidget final : public QWidget {
    Q_OBJECT
public:
    explicit GitRepositoryWidget(QWidget *parent = nullptr);
    void setSnapshot(const QString &workdir, const QJsonObject &snapshot);
    void setBusy(bool busy);

signals:
    void actionRequested(const QJsonObject &request);

private:
    void refresh();
    void renderBranches(const QJsonObject &catalog);
    void renderStashes(const QJsonObject &reply);
    void updateControls();
    void requestBranchAction(const QString &action);
    void requestStashAction(const QString &action);
    bool confirm(const QString &title, const QString &text);

    ProjectGitClient m_reader;
    QString m_workdir;
    QString m_snapshotToken;
    QString m_currentBranch;
    QString m_catalogNotice;
    quint64 m_generation = 0;
    bool m_busy = false;
    bool m_refreshPending = false;
    bool m_operationActive = false;
    QWidget *m_controls;
    QListWidget *m_branches;
    QListWidget *m_stashes;
    QLineEdit *m_branchName;
    QLineEdit *m_stashMessage;
    QCheckBox *m_includeUntracked;
    QLabel *m_status;
    QLabel *m_remotes;
};
