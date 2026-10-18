#pragma once

#include <QWidget>

class ContextStore;
class SessionManager;
class QComboBox;
class QLabel;
class QPlainTextEdit;

// A bounded tail of the existing launcher log, never another log store.
class DashboardLogWidget final : public QWidget {
    Q_OBJECT
public:
    DashboardLogWidget(SessionManager &sessions, ContextStore &store, QWidget *parent = nullptr);
    void setWorkdir(const QString &workdir);
    void refresh();

private:
    void loadTail();
    SessionManager &m_sessions;
    ContextStore &m_store;
    QString m_workdir;
    QString m_lastTail;
    QComboBox *m_runs;
    QLabel *m_status;
    QPlainTextEdit *m_log;
};
