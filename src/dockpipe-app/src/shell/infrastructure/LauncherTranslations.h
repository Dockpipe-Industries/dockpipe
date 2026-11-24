#pragma once

#include "LauncherUiExport.h"
#include <QStringList>
#include <QVector>

class QApplication;
enum class LauncherThemeMode;

namespace LauncherTranslations {
struct Language {
    QString code;
    QString nativeName;
};

DOCKPIPE_LAUNCHER_UI_EXPORT QVector<Language> languages();
DOCKPIPE_LAUNCHER_UI_EXPORT QString savedLanguage();
DOCKPIPE_LAUNCHER_UI_EXPORT QString resolveLanguage(const QString &selection, const QStringList &systemLanguages);
DOCKPIPE_LAUNCHER_UI_EXPORT bool saveAppearance(LauncherThemeMode theme, const QString &language);
// Install before constructing application widgets. Settings changes apply on restart.
DOCKPIPE_LAUNCHER_UI_EXPORT void initialize(QApplication &application);
DOCKPIPE_LAUNCHER_UI_EXPORT QString activeLanguage();
}
