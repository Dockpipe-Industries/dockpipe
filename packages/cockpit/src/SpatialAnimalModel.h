#pragma once
#include "SpatialWebDocument.h"
#include "SpatialMesh.h"

// Small, illustrative body plans. Source photos remain authoritative; these
// assemblies do not claim species-accurate anatomy, dimensions or colour.
namespace SpatialAnimalModel {
bool validForm(const QString &form);
void interpret(SpatialWebItem &item, const QString &context);
SpatialMesh assemble(const QString &form);
}
