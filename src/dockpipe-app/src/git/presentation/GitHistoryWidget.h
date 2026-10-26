#pragma once

#include "ProjectGitClient.h"
#include "LauncherUiExport.h"

#include <QWidget>

class GitCommitQueueModel;
class QComboBox;
class QLabel;
class QLineEdit;
class QListView;
class QListWidget;
class QPushButton;
class SourceEditor;

class DOCKPIPE_LAUNCHER_UI_EXPORT GitHistoryWidget final : public QWidget {
    Q_OBJECT
public:
    explicit GitHistoryWidget(QWidget *parent = nullptr);
    void setSnapshot(const QString &workdir, const QJsonObject &snapshot);
    void setBusy(bool busy);
    void clearQueue();
    GitCommitQueueModel *queueModel() const { return m_queue; }

signals:
    void actionRequested(const QJsonObject &request);

private:
    void search(int offset);
    void loadCommit();
    void addSelected();
    void requestCherryPick();
    void requestRevert();
    void moveSelected(int direction);
    void updateControls();

    ProjectGitClient m_reader;
    QString m_workdir;
    QString m_branch;
    QString m_snapshotToken;
    QString m_pageQuery;
    QString m_pageMatch;
    QString m_pageRef;
    quint64 m_generation = 0;
    bool m_parentBusy = false;
    bool m_operationActive = false;
    bool m_hasMore = false;
    int m_offset = 0;
    static constexpr int pageSize = 50;
    QLineEdit *m_query;
    QComboBox *m_match;
    QComboBox *m_ref;
    QPushButton *m_search;
    QPushButton *m_previous;
    QPushButton *m_next;
    QPushButton *m_add;
    QPushButton *m_revert;
    QPushButton *m_pick;
    QPushButton *m_up;
    QPushButton *m_down;
    QPushButton *m_remove;
    QPushButton *m_reverse;
    QPushButton *m_clear;
    QLabel *m_status;
    QLabel *m_page;
    QLabel *m_target;
    QListWidget *m_commits;
    SourceEditor *m_detail;
    QListView *m_queueView;
    GitCommitQueueModel *m_queue;
};
