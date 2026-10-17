#pragma once
#include "SpatialWorkspace.h"
class QWidget;
namespace SpatialDisplayDialog {
bool edit(QWidget *parent, SpatialDisplay &display, const SpatialInterior &room);
}
