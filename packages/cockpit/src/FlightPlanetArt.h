#pragma once
#include <QColor>
#include <QImage>
#include <QString>

// Ten immutable 256px albedo maps: seven destination palettes and three themes.
// The 2.5 MiB cache is independent of world count, viewport size and flight time.
namespace FlightPlanetArt {
int material(int index, const QString &theme);
QColor color(int material);
const QImage &surface(int material);
const QImage &atmosphere(int material);
const QImage &shadow(double phase);
const QImage &sun();
}
