#include "SpatialWebWorld.h"
#include "SpatialCivicWorld.h"

SpatialInterior SpatialWebWorld::build(const SpatialWebDocument &document)
{
    if (document.venue == "cinema")
        return SpatialCivicWorld::cinema(document);
    if (document.venue == "city")
        return SpatialCivicWorld::city(document);
    return construct(document, optionsFor(document)).interior;
}
