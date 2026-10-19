#pragma once
#include "SpatialWorldPlan.h"

namespace SpatialWebWorld {
SpatialWorldOptions optionsFor(const SpatialWebDocument &document);
SpatialWorldPlan plan(const SpatialWebDocument &document, const SpatialWebGraph &graph,
    const SpatialWorldOptions &options);
SpatialWorldResult compile(const SpatialWorldPlan &plan);
SpatialWorldResult construct(const SpatialWebDocument &document, const SpatialWorldOptions &options);
SpatialInterior build(const SpatialWebDocument &document);
}
