#pragma once

#include <QString>
#include <QUrl>

class QWidget;
class QWebEngineProfile;
class QWebEngineView;

// Shared ordinary-web setup used by the desktop Browser and spatial projection.
// The caller owns the profile and must destroy the view before the profile.
namespace BrowserRuntime {
bool allowsUrl(const QUrl &url);
bool allowsNavigation(const QUrl &url, QWebEngineProfile *profile);
QString rendererError();
QWebEngineView *createView(QWebEngineProfile *profile, QWidget *parent = nullptr);
}
