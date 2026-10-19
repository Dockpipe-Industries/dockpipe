#pragma once
#include "GitReviewModel.h"
#include "GitReviewProgress.h"
#include "SpatialWebWorld.h"
#include "ScrollingPreview.h"

struct GitReviewWorld {
    static constexpr int filesPerSection = 4;
    SpatialWorldPlan plan;
    SpatialWorldResult world;
    QVector<QVector3D> filePositions;
    static GitReviewWorld build(const GitReviewModel &model, const GitReviewProgress &progress, int firstFile);
    static QImage codeSurface(const GitReviewFile &file, const GitReviewDiff *diff, int hunk, bool before);
    static ScrollingPreview codePreview(const GitReviewFile &file, const GitReviewDiff *diff, int hunk, bool before);
};
