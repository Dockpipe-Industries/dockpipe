#pragma once

#include "CockpitSettings.h"
#include <QJsonArray>
#include <QJsonObject>
#include <QWidget>
#include <functional>

// Versioned native UI boundary. Heavy implementation and resources are supplied
// by an installed resolver package, never linked into the base launcher.
class LauncherWorkspaceSurface : public QWidget {
public:
    using QWidget::QWidget;
    virtual void setWorkspace(const QString &root) = 0;
    virtual void applySettings(const CockpitSettings &settings) = 0;
    virtual bool confirmLeave() = 0;
    virtual void closePanel() = 0;
    virtual QString activePanel() const = 0;
    virtual QString resourceSection() const { return {}; }
    std::function<QWidget *(const QString &)> pageProvider;
    std::function<QJsonArray(const QString &)> destinationsProvider;
    std::function<QJsonObject(const QString &)> resourceSnapshot;
    std::function<void(const QString &, const QString &)> resourceSelected;
    std::function<bool(const QString &)> workflowOpener;
    std::function<void()> requestExit;
    std::function<void()> requestSettings;
    std::function<void(const QString &, bool)> panelActivity;
};

class LauncherFileSurface : public QWidget {
public:
    using QWidget::QWidget;
    virtual void applyCockpitSettings(const CockpitSettings &settings) = 0;
    virtual QWidget *flightWidget() const = 0;
    virtual void pauseFlight(bool paused) = 0;
    std::function<void()> requestClose;
    std::function<void(const QString &)> requestOpenFile;
    std::function<void(const QString &)> requestFlightFile;
    std::function<void()> flightWillClose;
    std::function<void()> rendererChanged;
    std::function<void(const QString &)> requestReveal;
};

class LauncherWorkspacePackage {
public:
    virtual ~LauncherWorkspacePackage() = default;
    virtual void initialize(const QString &assetsDirectory) = 0;
    virtual LauncherWorkspaceSurface *createWorkspace(QWidget *parent) = 0;
    virtual LauncherFileSurface *createFiles(const QString &root, const QString &directory, QWidget *parent) = 0;
};
#define LauncherWorkspacePackage_iid "com.dockpipe.launcher.WorkspacePackage/2.0"
Q_DECLARE_INTERFACE(LauncherWorkspacePackage, LauncherWorkspacePackage_iid)
