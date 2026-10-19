#pragma once

#include <QWidget>
#include <QJsonObject>
#include <QPointer>

class QAction;
class TerminalHub;
class QComboBox;
class QLabel;
class QPushButton;
class QWebEngineView;
class QTimer;

class TerminalWidget final : public QWidget {
    Q_OBJECT
public:
    enum class Mode { Managed, Dashboard };
    TerminalWidget(TerminalHub &hub, const QString &workdir, QWidget *parent = nullptr, Mode mode = Mode::Managed);
    ~TerminalWidget() override;
    void setWorkdir(const QString &workdir);
    QString sessionId() const { return m_sessionId; }

public slots:
    // Only the packaged terminal document receives this WebChannel object.
    void rendererReady();
    void terminalInput(const QString &text, int generation);
    void terminalSize(int columns, int rows, int generation);
    void outputRendered(int generation);

signals:
    void openFullViewRequested();
    void appearanceChanged(const QJsonObject &appearance);
    void renderOutput(const QString &base64, bool reset, int generation);

protected:
    void showEvent(QShowEvent *event) override;
    void hideEvent(QHideEvent *event) override;
    void paintEvent(QPaintEvent *event) override;

private:
    void refreshSessions();
    void selectSession(const QString &id);
    void ensureRenderer();
    void applyAppearance();
    void scheduleOutput();
    void flushOutput();
    void startDashboardSession();
    void showSettings();
    void showStatus(const QString &message, bool issue = true);

    TerminalHub &m_hub;
    QPointer<TerminalHub> m_hubGuard;
    const Mode m_mode;
    QString m_ownedSessionId;
    bool m_dashboardStarted = false;
    QAction *m_restart = nullptr;
    QString m_workdir;
    QString m_sessionId;
    QComboBox *m_sessions;
    QPushButton *m_close;
    QLabel *m_status;
    QLabel *m_transparencyHint;
    QWebEngineView *m_view = nullptr;
    QTimer *m_flushTimer;
    quint64 m_cursor = 0;
    int m_generation = 0;
    bool m_ready = false;
    bool m_inFlight = false;
    bool m_reset = true;
    bool m_desktopBackground = false;
};
