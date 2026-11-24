#pragma once

#include "LauncherUiExport.h"
#include <QJsonObject>
#include <QWidget>
#include <QVector>

class QComboBox;
class QLabel;
class QPlainTextEdit;
class QPushButton;
class QSplitter;
class SourceEditor;

// Presents the CLI's bounded unified patch as aligned source excerpts. It never
// reads repository files: before/after content must belong to the same snapshot.
class DOCKPIPE_LAUNCHER_UI_EXPORT GitDiffWidget final : public QWidget {
    Q_OBJECT
public:
    explicit GitDiffWidget(QWidget *parent = nullptr);
    void setReview(const QJsonObject &review);
    void setCommitReview(const QJsonObject &review);
    void clear();
    QStringList changedFiles() const;
    void selectFile(int index);
    void setFileSelectorVisible(bool visible);

private:
    struct Row {
        QString before;
        QString after;
        int beforeLine = 0;
        int afterLine = 0;
        bool removed = false;
        bool added = false;
        bool separator = false;
    };
    struct File {
        QString path;
        QString beforePath;
        QString notice;
        QVector<Row> rows;
        bool binary = false;
    };
    void loadPatch(const QJsonObject &review, bool commit);
    void displayFile(int index);
    void refreshHighlights();
    void navigateChange(int direction);
    void updateNavigation();

    QVector<File> m_files;
    QVector<int> m_changes;
    QVector<Row> m_rows;
    int m_changeIndex = -1;
    bool m_commitReview = false;
    bool m_loading = false;
    QString m_noticeText;
    QString m_beforeText;
    QString m_afterText;
    QComboBox *m_filePicker;
    QPlainTextEdit *m_commitMessage;
    QLabel *m_notice;
    QLabel *m_empty;
    QLabel *m_counts;
    QLabel *m_position;
    QLabel *m_beforeLabel;
    QLabel *m_afterLabel;
    QPushButton *m_previous;
    QPushButton *m_next;
    QSplitter *m_splitter;
    SourceEditor *m_before = nullptr;
    SourceEditor *m_after = nullptr;
};
