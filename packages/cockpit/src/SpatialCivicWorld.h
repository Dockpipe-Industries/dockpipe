#pragma once
#include "SpatialWebDocument.h"

// Authored civic spaces use the same bounded interior, materials and browser links.
namespace SpatialCivicWorld {
SpatialInterior cinema(const SpatialWebDocument &document);
SpatialInterior city(const SpatialWebDocument &document);
SpatialInterior headquarters();
QImage headquartersSign();
}
