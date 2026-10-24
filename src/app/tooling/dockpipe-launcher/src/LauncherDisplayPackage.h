#pragma once

#include <QJsonObject>
#include <QWidget>
#include <functional>

// Native code is supplied by an installed package matching this host's Qt kit.
// Transport, rendering, input mapping and endpoint references stay in the package.
class LauncherDisplaySurface : public QWidget {
public:
    using QWidget::QWidget;
    virtual void setActive(bool active) = 0;
    // Idempotently release input and cancel/join workers. Never stop the resource.
    virtual void closeDisplay() = 0;
    // Emit callbacks on the Qt GUI thread, and cease them before closeDisplay returns.
    std::function<void(const QString &)> statusChanged;
    std::function<void(const QString &)> failed;
};

class LauncherDisplayPackage {
public:
    virtual ~LauncherDisplayPackage() = default;
    virtual void initialize(const QString &assetsDirectory) = 0;
    virtual LauncherDisplaySurface *createDisplay(const QJsonObject &options, QWidget *parent) = 0;
};
#define LauncherDisplayPackage_iid "com.dockpipe.launcher.DisplayPackage/1.0"
Q_DECLARE_INTERFACE(LauncherDisplayPackage, LauncherDisplayPackage_iid)
