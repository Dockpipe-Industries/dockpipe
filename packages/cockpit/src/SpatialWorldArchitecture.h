#pragma once
#include "SpatialWorldPlan.h"

namespace SpatialWorldArchitecture {
void interpret(SpatialWorldOptions &options, const SpatialWebDocument &document);
void assemble(SpatialWorldPlan &plan, const SpatialWebItem &item, int transform,
    float ground, float width, int ordinal);
}
