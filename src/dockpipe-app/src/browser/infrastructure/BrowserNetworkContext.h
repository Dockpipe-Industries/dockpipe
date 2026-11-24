#pragma once

#include <QJsonObject>
#include <QWidget>
#include <memory>

class QProcess;
class QLabel;
class QTemporaryFile;
class QTimer;

// Owns both relay and browser helper. Losing either closes the embedded surface.
class BrowserNetworkContext final : public QWidget {
    Q_OBJECT
public:
    BrowserNetworkContext(const QJsonObject &target, const QString &storagePath,
                          QWidget *parent = nullptr);
    ~BrowserNetworkContext() override;
    void start();
    void setTarget(const QJsonObject &target);
    void stop(const QString &reason);
    void navigate(const QUrl &url);
    void setPresentation(bool chrome, bool canvas, bool viewport, bool suspended);

private:
    void relayOutput();
    void hostOutput();
    void launchHost(const QJsonObject &ready);
    void releaseProcesses();
    void writeCommand(const QJsonObject &command);
    QJsonObject m_target;
    QString m_storagePath;
    QUrl m_pendingUrl;
    QLabel *m_status;
    QWidget *m_container = nullptr;
    QProcess *m_relay = nullptr;
    QProcess *m_host = nullptr;
    QTimer *m_deadline;
    QByteArray m_relayBuffer;
    QByteArray m_hostBuffer;
    std::unique_ptr<QTemporaryFile> m_request;
    std::unique_ptr<QTemporaryFile> m_hostRequest;
    QJsonObject m_presentation{{"type", "presentation"}, {"chrome", true}, {"canvas", false},
        {"viewport", true}, {"suspended", false}};
    bool m_ready = false;
    bool m_stopping = false;
};
