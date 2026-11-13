#pragma once

#include <QJsonObject>
#include <QObject>
#include <QPointer>
#include <QTimer>

// Optional QObject pre-close negotiation. The published Display SDK remains
// responsible for synchronous cancellation and joining during final teardown.
class NativeDisplaySession final : public QObject {
    Q_OBJECT
public:
    explicit NativeDisplaySession(QObject *parent = nullptr);
    bool requestClose(QObject *surface);
    void cancel();

signals:
    void closeCompleted(const QJsonObject &result);

private slots:
    void acceptCompletion(const QJsonObject &result);

private:
    void finish(const QJsonObject &result);
    QPointer<QObject> m_surface;
    QJsonObject m_reference;
    QTimer m_deadline;
    bool m_pending = false;
};
