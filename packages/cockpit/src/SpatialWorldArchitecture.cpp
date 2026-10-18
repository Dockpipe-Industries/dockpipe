#include "SpatialWorldArchitecture.h"
#include <algorithm>
#include <cmath>

namespace {
QColor blend(const QColor &left, const QColor &right, float amount)
{
    return QColor::fromRgbF(left.redF() * (1 - amount) + right.redF() * amount,
        left.greenF() * (1 - amount) + right.greenF() * amount,
        left.blueF() * (1 - amount) + right.blueF() * amount);
}
bool valid(const QVector<float> &features)
{
    return features.size() == 8 && std::all_of(features.cbegin(), features.cend(), [](float value) {
        return std::isfinite(value) && value >= 0 && value <= 1;
    });
}
}

void SpatialWorldArchitecture::interpret(SpatialWorldOptions &options, const SpatialWebDocument &document)
{
    if (!valid(document.worldFeatures))
        return;
    options.features = document.worldFeatures;
    const auto &scores = options.features;
    // Continuous, independent controls. A fantasy bookshop can combine stone
    // towers, timber shelving and woodland, rather than select one room preset.
    const float nature = scores[0];
    const float stone = scores[1];
    const float space = scores[2];
    const float library = scores[3];
    const float workshop = scores[5];
    const float driving = scores[6];
    options.style.repetitions = 0;
    options.style.ground = blend(options.style.ground, QColor("#416945"), nature);
    options.style.ground = blend(options.style.ground, QColor("#777568"), stone * 0.65f);
    options.style.structure = blend(options.style.structure, QColor("#815d3c"), qMax(nature, library));
    options.style.structure = blend(options.style.structure, QColor("#9493a0"), stone);
    options.style.structure = blend(options.style.structure, QColor("#55778b"), workshop);
    options.style.outdoors = qMax(qMax(nature, stone), qMax(driving, scores[7])) > 0.4f;
    if (options.style.outdoors)
        options.style.sky = blend(QColor("#8fb4c8"), QColor("#b5c6ac"), nature);
    if (space > 0.6f && space > nature && space > stone) {
        options.style.sky = QColor("#080c22");
        options.style.ground = QColor("#10182d");
        options.style.outdoors = true;
        options.locomotion = "fly";
        options.topology = SpatialTopology::Layers;
    } else if (driving > 0.55f) {
        options.topology = SpatialTopology::Loop;
        options.spacing = 1.4f;
    } else if (nature > 0.45f || stone > 0.45f || scores[7] > 0.55f) {
        options.topology = SpatialTopology::Branches;
        options.spacing = 1.2f;
    }
}

void SpatialWorldArchitecture::assemble(SpatialWorldPlan &plan, const SpatialWebItem &item,
    int transform, float ground, float width, int ordinal)
{
    const auto &options = plan.options;
    auto scores = options.features;
    if (valid(item.worldFeatures)) {
        for (int index = 0; index < scores.size(); ++index)
            scores[index] = item.worldFeatures[index] * 0.75f + scores[index] * 0.25f;
    }
    if (!valid(scores))
        return;
    QVector<int> ranked{0, 1, 2, 3, 4, 5, 6, 7};
    std::stable_sort(ranked.begin(), ranked.end(), [&](int left, int right) { return scores[left] > scores[right]; });
    // Two independently selected side assemblies, three parts each. Together
    // with the platform and sign this stays within 8 parts x 12 destinations.
    const auto part = [&](QVector3D center, QVector3D size, const QColor &color,
                          SpatialSolidShape shape = SpatialSolidShape::Box) {
        plan.parts.append({transform, center, size, color, shape, true, {}});
    };
    const QColor structure = options.style.structure;
    const QColor accent = options.style.accent;
    for (int side = 0; side < 2; ++side) {
        const int feature = side == 1 && scores[ranked[1]] > 0.22f ? ranked[1] : ranked[0];
        const float sign = side == 0 ? -1 : 1;
        const float x = sign * (width / 2 + 85);
        const float variation = ((options.seed + ordinal * 71 + side * 29) % 5) * 18;
        const float height = 320 + variation;
        if (scores[feature] < 0.15f)
            continue;
        if (feature == 0) {
            part({x, ground + 130, 270}, {45, 260, 45}, QColor("#765339"));
            part({x, ground + height - 80, 270}, {240, 230, 240}, QColor("#3b794c"), SpatialSolidShape::Pyramid);
            part({x, ground + height + 50, 270}, {180, 200, 180}, QColor("#548f58"), SpatialSolidShape::Pyramid);
        } else if (feature == 1) {
            part({x, ground + height / 2, 270}, {125, height, 140}, structure);
            part({x - 42, ground + height + 30, 270}, {40, 60, 140}, structure.lighter(115));
            part({x + 42, ground + height + 30, 270}, {40, 60, 140}, structure.lighter(115));
        } else if (feature == 3) {
            part({x, ground + 180, 270}, {100, 360, 110}, QColor("#674b37"));
            part({x, ground + 120, 195}, {140, 18, 100}, structure);
            part({x, ground + 255, 195}, {140, 18, 100}, structure);
        } else if (feature == 2 || feature == 5) {
            part({x, ground + 160, 270}, {65, 320, 65}, structure);
            part({x, ground + 335, 270}, {150, 100, 150}, accent, SpatialSolidShape::Octahedron);
            part({x, ground + 35, 270}, {180, 70, 170}, structure.darker(130));
        } else if (feature == 6) {
            part({x, ground + 90, 270}, {35, 180, 35}, structure);
            part({x, ground + 180, 270}, {160, 40, 45}, accent);
            part({x, ground + 15, 270}, {190, 30, 110}, QColor("#30383e"));
        } else {
            part({x, ground + 170, 270}, {35, 340, 35}, structure);
            const auto shape = feature == 7 ? SpatialSolidShape::Pyramid : SpatialSolidShape::Box;
            part({x - sign * 65, ground + height, 270}, {220, feature == 7 ? 150.0f : 35.0f, 310}, accent, shape);
            part({x, ground + 45, 270}, {110, 90, 130}, structure);
        }
    }
}
