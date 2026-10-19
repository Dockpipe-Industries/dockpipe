#pragma once

#include "LauncherUiExport.h"

#include <QJsonObject>
#include <QWidget>

class QLabel;
class QListWidget;
class ProjectFilesWidget;
class QPushButton;

class DOCKPIPE_LAUNCHER_UI_EXPORT GitConflictWidget final : public QWidget {
    Q_OBJECT
public:
    explicit GitConflictWidget(QWidget *parent = nullptr);
    void setSnapshot(const QString &workdir, const QJsonObject &snapshot);
    void setBusy(bool busy);
    bool confirmLeave();
    bool hasUnsavedChanges() const;

signals:
    void refreshRequested();
    void actionRequested(const QJsonObject &request);

private:
    void openSelected();
    void resolveSelected(const QString &action);
    void updateControls();
    bool confirm(const QString &title, const QString &text);

    QString m_workdir;
    QString m_snapshotToken;
    QJsonObject m_operation;
    bool m_busy = false;
    bool m_reloadAfterResolution = false;
    QLabel *m_heading;
    QLabel *m_status;
    QListWidget *m_conflicts;
    ProjectFilesWidget *m_editor;
    QWidget *m_resolutionActions;
    QPushButton *m_open;
    QPushButton *m_continue;
    QPushButton *m_skip;
    QPushButton *m_abort;
};
