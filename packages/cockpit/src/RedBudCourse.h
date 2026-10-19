#pragma once
#include "DirtBikeRaceCourse.h"

// Map-derived 2024 layout. Elevation and jump profiles are authored approximations,
// not survey data. All distances are centimetres.
namespace RedBudCourse {
float length();
DirtBikeRaceCourse::Sample sample(float distance, float lane = 0);
float progress(const QVector3D &point);
float landscape(float x, float z);
}
