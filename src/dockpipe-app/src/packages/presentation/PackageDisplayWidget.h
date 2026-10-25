#pragma once

#include "LauncherUiExport.h"
#include "WorkflowCatalogData.h"
#include <QWidget>
#include <memory>

class DesktopWindowWidget;
class LauncherDisplaySurface;
class LauncherNativeLibraryScope;
class QLabel;
class QPluginLoader;
class QVBoxLayout;

// One visible resource lease shared by the package page and dashboard panels.
// Closing a view releases only its UI/transport; resource lifecycle is separate.
class DOCKPIPE_LAUNCHER_UI_EXPORT PackageDisplayWidget final : public QWidget {
    Q_OBJECT
public:
    explicit PackageDisplayWidget(QWidget *parent = nullptr);
    ~PackageDisplayWidget() override;
    void setDisplay(const PackageDisplayMeta &display);
    void clearDisplay();
    static bool validateOptions(const QJsonObject &options, QString &error);
    static QString platformName();
    bool hasPackageSurface() const { return m_surface != nullptr; }

signals:
    void displayFailed(const QString &error);

protected:
    void showEvent(QShowEvent *event) override;
    void hideEvent(QHideEvent *event) override;

private:
    void acquire();
    void release();
    QString leaseKey() const;
    PackageDisplayMeta m_display;
    DesktopWindowWidget *m_native;
    LauncherDisplaySurface *m_surface = nullptr;
    QLabel *m_status;
    QVBoxLayout *m_layout;
    std::unique_ptr<LauncherNativeLibraryScope> m_nativeLibrary;
    std::unique_ptr<QPluginLoader> m_loader;
    bool m_handedOff = false;
};
