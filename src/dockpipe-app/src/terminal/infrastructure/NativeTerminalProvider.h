#pragma once

#include <QObject>
#include <QByteArray>
#include <QJsonObject>
#include <memory>

// Private one-session transport for the Core-owned local terminal provider.
// All protocol traffic uses inherited stdin/stdout; no launch data is in argv.
class NativeTerminalProvider final : public QObject {
public:
    explicit NativeTerminalProvider(QObject *parent = nullptr);
    ~NativeTerminalProvider() override;
    void start();

private:
    struct State;
    std::unique_ptr<State> m_state;
    void poll();
    void receive(const QByteArray &line);
    void acknowledgeInput();
    bool send(const QJsonObject &frame, bool control = true);
    void fail(const QString &code, quint64 id = 0);
    void close();
    void complete(bool verified);
};
