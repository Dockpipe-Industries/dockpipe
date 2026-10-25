#pragma once

#include <QImage>
#include <QObject>
#include <QVariantMap>
#include <memory>

class QDBusMessage;

// One explicitly approved, view-only portal WINDOW stream. The dedicated bus
// connection and helper process live only until stop() or owner destruction.
class PortalWindowCapture final : public QObject {
    Q_OBJECT
public:
    explicit PortalWindowCapture(QObject *parent = nullptr);
    ~PortalWindowCapture() override;
    void start();
    void stop();
    void setPaused(bool paused);
signals:
    void frameReady(const QImage &frame);
    void statusChanged(const QString &message);
    void failed(const QString &message);
    void cancelled();
private slots:
    void response(uint result, const QVariantMap &values, const QDBusMessage &message);
    void sessionClosed(const QVariantMap &details);
private:
    struct State;
    std::unique_ptr<State> m_state;
    void request(const QString &method, const QVariantList &arguments, QVariantMap options);
    void openRemote(const QVariantMap &values);
    void startHelper();
    void readFrames();
    void fail(const QString &message);
};
