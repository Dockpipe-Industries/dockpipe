#pragma once

#include <QObject>
#include <QJsonObject>
#include <QString>

// Expose only terminal transport, not the surrounding QWidget's public API.
class TerminalBridge final : public QObject {
    Q_OBJECT
public:
    using QObject::QObject;

public slots:
    void rendererReady() { emit ready(); }
    void terminalInput(const QString &text, int generation) { emit input(text, generation); }
    void terminalSize(int columns, int rows, int generation) { emit sizeChanged(columns, rows, generation); }
    void outputRendered(int generation) { emit acknowledged(generation); }

signals:
    void ready();
    void input(const QString &text, int generation);
    void sizeChanged(int columns, int rows, int generation);
    void acknowledged(int generation);
    void appearanceChanged(const QJsonObject &appearance);
    void renderOutput(const QString &base64, bool reset, int generation);
};
