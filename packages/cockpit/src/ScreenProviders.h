#pragma once
#include "FurnishingCatalog.h"
#include <QImage>

struct ScreenCapabilities {
    QString label;
    bool available = false;
    bool interactive = false;
    bool live = false;
    QString notice;
};
// Providers own pixels and capabilities; placement owns only the binding.
namespace ScreenProviders {
ScreenCapabilities capabilities(const ScreenBinding &binding, const QStringList &sceneSources);
QImage marquee(const ScreenBinding &binding);
QImage preview(const ScreenBinding &binding, const ScreenCapabilities &capabilities);
}
