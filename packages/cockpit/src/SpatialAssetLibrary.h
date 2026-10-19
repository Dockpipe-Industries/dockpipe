#pragma once
#include "SpatialMesh.h"
#include <QString>

// Shared by authored spaces and procedural worlds. Only package resources are
// accepted; web content cannot supply filesystem paths or executable importers.
namespace SpatialAssetLibrary {
inline constexpr int maximumModelFaces = 2048;
inline constexpr int maximumSceneFaces = 8192;
SpatialMesh model(const QString &id);
}
