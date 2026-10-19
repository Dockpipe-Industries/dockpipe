#pragma once

#include <QImage>
#include <QObject>
#include <memory>

// macOS 14+ system-owned single-window selection. Capture only; no input control.
class MacWindowCapture final : public QObject {
    Q_OBJECT
public:
    explicit MacWindowCapture(QObject *parent = nullptr);
    ~MacWindowCapture() override;
    void start();
    void stop();
    void setVisible(bool visible);
signals:
    void frameReady(const QImage &frame);
    void statusChanged(const QString &message);
    void failed(const QString &message);
    void cancelled();
private:
    struct State;
    std::unique_ptr<State> m_state;
};
