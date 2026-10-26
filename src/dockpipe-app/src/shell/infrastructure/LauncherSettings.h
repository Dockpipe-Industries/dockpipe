#pragma once

#include <QString>
#include <QStringList>
#include <QJsonObject>
#include "CockpitSettings.h"

/// Dockpipe UI preferences (Basic vs Advanced, project folder, view mode). Stored in `launcher.json` under the app config dir.
class LauncherSettings {
public:
    static constexpr int kMaxRecentProjects = 20;

    QString uiMode;     ///< `"basic"` (default) or `"advanced"`
    QString basicView;  ///< `"icons"` or `"list"`
    QString projectFolder;
    QString repoRootOverride;
    QString globalRootOverride;
    QStringList extraWorkflowRoots;
    QStringList packageRemotes;
    /// Most recently opened project folders (newest first), capped at `kMaxRecentProjects`.
    QStringList recentProjectFolders;
    /// When true, the status-bar third-party disclaimer is hidden until restored from Help.
    bool thirdPartyDisclaimerDismissed = false;
    bool sidebarVisible = true;
    bool defaultDashboardVisible = true;
    bool tilingEnabled = false;
    CockpitSettings cockpit;

    bool load();
    bool save() const;

    /// Dedupes, moves `path` to the front, and trims to `kMaxRecentProjects`.
    void addRecentProject(const QString &path);

    bool isAdvanced() const { return uiMode == QStringLiteral("advanced"); }
    bool isBasicIcons() const { return basicView != QStringLiteral("list"); }

    static LauncherSettings current();
    static QString filePath();
    /// Read or atomically replace only the browser member of launcher.json.
    /// Missing files are defaults; malformed or unreadable files are failures.
    static bool loadBrowserSettings(QJsonObject &settings);
    static bool saveBrowserSettings(const QJsonObject &settings);
    /// Terminal appearance shares the same locked launcher configuration.
    static bool loadTerminalSettings(QJsonObject &settings);
    static bool saveTerminalSettings(const QJsonObject &settings);
    /// Dashboard onboarding shares launcher.json and preserves unrelated preferences.
    static bool loadDashboardTourState(bool &completed, bool &doNotShowAgain);
    static bool saveDashboardTourState(bool completed, bool doNotShowAgain);
};
