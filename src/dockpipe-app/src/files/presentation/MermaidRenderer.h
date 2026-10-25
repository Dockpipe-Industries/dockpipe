#pragma once

#include "LauncherUiExport.h"
#include <QImage>
#include <QObject>
#include <functional>

// The shared UI has no browser dependency. Desktop hosts supply the offline
// Mermaid implementation through this owned, cancellable rendering boundary.
class DOCKPIPE_LAUNCHER_UI_EXPORT MermaidRenderer : public QObject {
    Q_OBJECT
public:
    using Factory = std::function<MermaidRenderer *(QObject *)>;
    explicit MermaidRenderer(QObject *parent = nullptr) : QObject(parent) {}
    virtual void render(const QString &source, bool dark) = 0;
    virtual void cancel() = 0;
    static void setFactory(Factory factory);
    static MermaidRenderer *create(QObject *parent);

signals:
    void finished(const QImage &image, const QString &error);
};
