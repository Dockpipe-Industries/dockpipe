#pragma once

#include <QImage>
#include <QUrl>
#include <QVector>
#include "SpatialWorkspace.h"

struct SpatialWebItem {
    QString id;
    QString title;
    QUrl url;
    QString kind;
    QString description;
    QString detail;
    QString category;
    QString vehicleBody;
    QString animalForm; // Validated illustrative body plan, never factual source data.
    int vehicleYear = 0;
    float visualAspect = 1.6f;
    int spatialSlot = -1; // Persistent site area slot; -1 retains single-page layout.
    QUrl imageUrl; // Anonymous site-discovery asset reference; never a browser DOM index.
    bool hasImage = false;
    QImage poster;
    QImage visual; // Bounded source pixels for optional local foreground extraction.
    QVector<float> worldFeatures; // Validated local semantic scores; never page instructions.
};

// A transient projection of one browser document, never a saved website copy.
struct SpatialWebDocument {
    static constexpr int maximumItems = 12;
    static constexpr int maximumBytes = 96 * 1024;
    QString title;
    QUrl url;
    QString venue;
    QString evidence;
    QColor accent;
    QVector<float> worldFeatures;
    bool limited = false;
    bool stableLayout = false;
    bool siteOverview = false;
    QString layoutIdentity;
    QVector<SpatialWebItem> items;
    static bool allowedUrl(const QUrl &url);
    static bool searchResultsUrl(const QUrl &url);
    static bool samePage(const QUrl &left, const QUrl &right);
    static bool parse(const QByteArray &json, SpatialWebDocument &out, QString &error);
    SpatialInterior room() const;
    QString environmentKey() const;
    static QImage poster(const SpatialWebItem &item, const QImage &visual, int number);
};
