#pragma once
#include "GitReviewModel.h"
#include "GitReviewProgress.h"
#include "GitReviewWorld.h"
#include "ProjectGitClient.h"
#include <QWidget>
#include <QTimer>
#include <QElapsedTimer>

class WorkspaceFlightView;
class SourceEditor;
class ProjectFilesWidget;
class QStackedWidget;
class QListWidget;
class QComboBox;
class QLabel;
class QPushButton;
class QLineEdit;
class QPlainTextEdit;
class QTreeWidget;

class GitReviewWidget final : public QWidget {
    Q_OBJECT
public:
    explicit GitReviewWidget(bool hardware, QWidget *parent = nullptr);
    void setWorkdir(const QString &root);
    void refresh();
    void beginReview();
    void jumpToFile(int index, bool continueWalking = false);
    void inspectHunk(int index);
    void showHub();
    void showSummary();
    void returnToSpace();
    bool confirmLeave();
    bool busy() const { return m_client.busy(); }
    bool spatial() const;
    WorkspaceFlightView *scene() const { return m_scene; }
    const GitReviewModel &model() const { return m_model; }
    const GitReviewDiff &diff() const { return m_diff; }
    int currentFile() const { return m_current; }
    void setHardwareAcceleration(bool hardware);
    void goBack();
signals:
    void exitRequested();
protected:
    void showEvent(QShowEvent *event) override;
    void hideEvent(QHideEvent *event) override;
    bool eventFilter(QObject *object, QEvent *event) override;
private:
    void run(QJsonObject request, ProjectGitClient::Completion completion);
    void loadCurrent(bool focus);
    void publishSection(bool preservePosition = false, bool continueWalking = false);
    void updateFileList();
    void updateCode();
    void mutate(const QString &action);
    void saveNote();
    void openEditor();
    void updateButtons();
    void trackPosition();
    void prepareCodePreviews();
    void advanceCodePreviews();
    int reviewedCount() const;
    GitComparison comparison() const;
    ProjectGitClient m_client;
    QString m_workdir;
    GitReviewModel m_model;
    GitReviewDiff m_diff;
    GitReviewProgress m_progress;
    GitReviewWorld m_world;
    int m_current = -1;
    int m_first = 0;
    int m_hunk = 0;
    int m_context = 3;
    quint64 m_generation = 0;
    bool m_mutating = false;
    bool m_focusAfterLoad = false;
    QTimer m_positionTimer;
    ScrollingPreview m_beforePreview;
    ScrollingPreview m_afterPreview;
    QElapsedTimer m_previewClock;
    qint64 m_previewMilliseconds = 0;
    int m_beforeOffset = -1;
    int m_afterOffset = -1;
    int m_previewFile = -1;
    WorkspaceFlightView *m_scene;
    QStackedWidget *m_pages;
    QWidget *m_hub;
    QWidget *m_focus;
    QWidget *m_summary;
    ProjectFilesWidget *m_editor = nullptr;
    QListWidget *m_files;
    QComboBox *m_comparison;
    QLabel *m_status;
    QLabel *m_location;
    QLabel *m_briefing;
    QLabel *m_summaryText;
    QLabel *m_beforeLabel;
    QLabel *m_afterLabel;
    SourceEditor *m_before;
    SourceEditor *m_after;
    QPlainTextEdit *m_notes;
    QLineEdit *m_message;
    QTreeWidget *m_history;
    QTreeWidget *m_branches;
    QPushButton *m_reviewed;
    QPushButton *m_stage;
    QPushButton *m_unstage;
    QPushButton *m_discard;
    QPushButton *m_commit;
    QPushButton *m_refresh;
};
