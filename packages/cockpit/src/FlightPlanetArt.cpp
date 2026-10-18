#include "FlightPlanetArt.h"
#include <QPainter>
#include <QRadialGradient>
#include <QVector3D>
#include <array>
#include <cmath>

namespace {
float lattice(int x, int y, int z, unsigned seed)
{
    unsigned value = unsigned(x) * 374761393u + unsigned(y) * 668265263u
        + unsigned(z) * 2147483647u + seed * 1274126177u;
    value = (value ^ (value >> 13)) * 1274126177u;
    return float((value ^ (value >> 16)) & 0xffff) / 65535.0f;
}

float noise(float x, float y, float z, unsigned seed)
{
    const int baseX = int(std::floor(x));
    const int baseY = int(std::floor(y));
    const int baseZ = int(std::floor(z));
    const auto smooth = [](float value) { return value * value * (3 - 2 * value); };
    const float blendX = smooth(x - baseX);
    const float blendY = smooth(y - baseY);
    const float blendZ = smooth(z - baseZ);
    float value = 0;
    for (int corner = 0; corner < 8; ++corner) {
        const int dx = corner & 1;
        const int dy = (corner >> 1) & 1;
        const int dz = (corner >> 2) & 1;
        value += lattice(baseX + dx, baseY + dy, baseZ + dz, seed)
            * (dx ? blendX : 1 - blendX) * (dy ? blendY : 1 - blendY)
            * (dz ? blendZ : 1 - blendZ);
    }
    return value;
}

QImage makeSurface(int material)
{
    constexpr int size = 256;
    QImage image(size, size, QImage::Format_ARGB32_Premultiplied);
    image.fill(Qt::transparent);
    const QColor land = FlightPlanetArt::color(material);
    for (int y = 0; y < size; ++y) {
        auto *pixels = reinterpret_cast<QRgb *>(image.scanLine(y));
        for (int x = 0; x < size; ++x) {
            const float nx = (x + 0.5f - size / 2) / (size / 2 - 1);
            const float ny = (y + 0.5f - size / 2) / (size / 2 - 1);
            const float radial = nx * nx + ny * ny;
            if (radial >= 1)
                continue;
            const float nz = std::sqrt(1 - radial);
            const float terrain = noise(nx * 3, ny * 3, nz * 3, material + 1) * 0.70f
                + noise(nx * 9, ny * 9, nz * 9, material + 1) * 0.22f
                + noise(nx * 27, ny * 27, nz * 27, material + 1) * 0.08f;
            const float continent = qBound(0.0f, (terrain - 0.44f) * 8, 1.0f);
            const float cloud = qBound(0.0f,
                (noise(nx * 8 + ny * 2, ny * 5, nz * 8, material + 24) - 0.57f) * 3.5f, 0.72f);
            const float rim = 0.48f + 0.52f * nz;
            const float ice = qBound(0.0f, (std::abs(ny) - 0.89f) * 8, 0.85f);
            const float white = qMax(cloud, ice);
            const auto channel = [&](int ocean, int ground) {
                const float base = ocean * (1 - continent) + ground * continent;
                return qBound(0, qRound((base * (1 - white) + 224 * white) * rim), 255);
            };
            pixels[x] = qRgb(channel(18, land.red()), channel(52, land.green()), channel(72, land.blue()));
        }
    }
    return image;
}
}

int FlightPlanetArt::material(int index, const QString &theme)
{
    if (theme == "ocean")
        return 7;
    if (theme == "desert")
        return 8;
    if (theme == "forest")
        return 9;
    return index % 7;
}

QColor FlightPlanetArt::color(int material)
{
    static const std::array<QColor, 10> colors{QColor("#81b5a1"), QColor("#8dabc3"),
        QColor("#b09abb"), QColor("#cbb18a"), QColor("#bd8d81"), QColor("#8daa82"),
        QColor("#b0b9c7"), QColor("#7bb6cd"), QColor("#c9a675"), QColor("#82aa82")};
    return colors.at(material);
}

const QImage &FlightPlanetArt::surface(int material)
{
    static const std::array<QImage, 10> maps = []() {
        std::array<QImage, 10> result;
        for (int index = 0; index < int(result.size()); ++index)
            result[index] = makeSurface(index);
        return result;
    }();
    return maps.at(material);
}

const QImage &FlightPlanetArt::sun()
{
    static const QImage image = []() {
        QImage result(512, 512, QImage::Format_ARGB32_Premultiplied);
        result.fill(Qt::transparent);
        QPainter painter(&result);
        QRadialGradient halo(256, 256, 256);
        halo.setColorAt(0, QColor("#fff9e8"));
        halo.setColorAt(0.22, QColor("#fff2c5"));
        halo.setColorAt(0.28, QColor("#ffd188"));
        halo.setColorAt(0.30, QColor(255, 177, 94, 180));
        halo.setColorAt(0.40, QColor(240, 126, 55, 64));
        halo.setColorAt(0.65, QColor(215, 109, 50, 18));
        halo.setColorAt(1, Qt::transparent);
        painter.fillRect(result.rect(), halo);
        painter.end();
        for (int y = 185; y < 327; ++y) {
            auto *pixels = reinterpret_cast<QRgb *>(result.scanLine(y));
            for (int x = 185; x < 327; ++x) {
                const float radial = std::hypot(float(x - 256), float(y - 256)) / 71;
                if (radial >= 1)
                    continue;
                const float granulation = noise(x * 0.14f, y * 0.14f, 0, 92);
                const float brightness = (1 - radial * radial * 0.25f) * (0.88f + granulation * 0.12f);
                pixels[x] = qRgb(255, qRound(248 * brightness), qRound(205 * brightness));
            }
        }
        return result;
    }();
    return image;
}

const QImage &FlightPlanetArt::atmosphere(int material)
{
    static const std::array<QImage, 10> halos = []() {
        std::array<QImage, 10> result;
        for (int index = 0; index < int(result.size()); ++index) {
            const QColor accent = color(index);
            QImage image(128, 128, QImage::Format_ARGB32_Premultiplied);
            image.fill(Qt::transparent);
            QPainter painter(&image);
            QRadialGradient glow(64, 64, 64);
            glow.setColorAt(0, Qt::transparent);
            glow.setColorAt(0.78, Qt::transparent);
            glow.setColorAt(0.83, QColor(accent.red(), accent.green(), accent.blue(), 95));
            glow.setColorAt(1, Qt::transparent);
            painter.fillRect(image.rect(), glow);
            painter.end();
            result[index] = image;
        }
        return result;
    }();
    return halos.at(material);
}

const QImage &FlightPlanetArt::shadow(double phase)
{
    // Quantized Lambert illumination avoids generating a different GPU gradient
    // texture for every moving planet on every frame. 33 x 128px = 2.0625 MiB.
    static const std::array<QImage, 33> phases = []() {
        std::array<QImage, 33> result;
        for (int index = 0; index < int(result.size()); ++index) {
            const float lightZ = index / 16.0f - 1;
            const float lightX = std::sqrt(qMax(0.0f, 1 - lightZ * lightZ));
            QImage image(128, 128, QImage::Format_ARGB32_Premultiplied);
            image.fill(Qt::transparent);
            for (int y = 0; y < 128; ++y) {
                auto *pixels = reinterpret_cast<QRgb *>(image.scanLine(y));
                for (int x = 0; x < 128; ++x) {
                    const float nx = (x + 0.5f - 64) / 63.5f;
                    const float ny = (y + 0.5f - 64) / 63.5f;
                    const float radial = nx * nx + ny * ny;
                    if (radial >= 1)
                        continue;
                    const float illumination = qMax(0.0f, nx * lightX + std::sqrt(1 - radial) * lightZ);
                    const int alpha = qRound(235 * (1 - illumination));
                    pixels[x] = qPremultiply(qRgba(2, 6, 13, alpha));
                }
            }
            result[index] = image;
        }
        return result;
    }();
    return phases.at(qBound(0, qRound((phase + 1) * 16), 32));
}
