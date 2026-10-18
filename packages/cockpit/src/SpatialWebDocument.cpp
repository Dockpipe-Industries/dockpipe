#include "SpatialWebDocument.h"
#include "BrowserRuntime.h"
#include "SpatialWebWorld.h"
#include "SpatialAnimalModel.h"
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QPainter>
#include <QSet>
#include <QUrlQuery>
#include <QRegularExpression>
#include <cmath>
#include <QCryptographicHash>

QString SpatialWebDocument::environmentKey() const
{
    if (stableLayout)
        return "site:" + layoutIdentity;
    const auto source = url.adjusted(QUrl::RemoveFragment | QUrl::RemoveUserInfo).toEncoded();
    return "web:" + QString::fromLatin1(QCryptographicHash::hash(source, QCryptographicHash::Sha256).toHex());
}


bool SpatialWebDocument::allowedUrl(const QUrl &url)
{
    return BrowserRuntime::allowsUrl(url);
}

bool SpatialWebDocument::searchResultsUrl(const QUrl &url)
{
    if (!allowedUrl(url))
        return false;
    static const QRegularExpression searchHost(
        R"((^|\.)(google\.(?:[a-z]{2,3}|(?:co|com)\.[a-z]{2})|bing\.com|duckduckgo\.com|search\.yahoo\.com)$)");
    const QUrlQuery query(url);
    return searchHost.match(url.host()).hasMatch()
        && (!query.queryItemValue("q").trimmed().isEmpty() || !query.queryItemValue("p").trimmed().isEmpty());
}

bool SpatialWebDocument::samePage(const QUrl &left, const QUrl &right)
{
    if (!allowedUrl(left) || !allowedUrl(right))
        return false;
    const auto normalize = [](const QUrl &value) {
        auto url = value.adjusted(QUrl::RemoveFragment | QUrl::NormalizePathSegments | QUrl::StripTrailingSlash);
        if (url.path().isEmpty())
            url.setPath("/");
        if ((url.scheme() == "https" && url.port() == 443) || (url.scheme() == "http" && url.port() == 80))
            url.setPort(-1);
        return url;
    };
    return normalize(left) == normalize(right);
}

bool SpatialWebDocument::parse(const QByteArray &json, SpatialWebDocument &out, QString &error)
{
    error.clear();
    if (json.size() > maximumBytes) {
        error = "The page exceeds the spatial document budget.";
        return false;
    }
    const auto parsed = QJsonDocument::fromJson(json);
    const auto object = parsed.object();
    SpatialWebDocument document;
    document.url = QUrl(object.value("url").toString());
    if (!parsed.isObject() || !allowedUrl(document.url) || !object.value("items").isArray()) {
        error = "The browser did not return a valid web document.";
        return false;
    }
    document.title = object.value("title").toString().left(180);
    document.venue = object.value("venue").toString();
    if (!QStringList{"store", "cinema", "city", "repositories", "mall", "library", "showroom", "park"}.contains(document.venue))
        document.venue = "gallery";
    document.evidence = object.value("evidence").toString().left(2200);
    const QString accent = object.value("accent").toString();
    if (QRegularExpression("^#[0-9a-fA-F]{6}$").match(accent).hasMatch())
        document.accent = QColor(accent);
    document.limited = object.value("limited").toBool();
    const auto items = object.value("items").toArray();
    if (items.size() > maximumItems) {
        error = "The page exceeds the twelve-display room capacity.";
        return false;
    }
    QSet<QString> ids;
    for (const auto &value : items) {
        const auto itemObject = value.toObject();
        SpatialWebItem item;
        item.id = itemObject.value("id").toString();
        item.title = itemObject.value("title").toString().left(140);
        item.url = QUrl(itemObject.value("href").toString());
        item.kind = itemObject.value("kind").toString().left(16);
        item.description = itemObject.value("description").toString().left(200);
        item.detail = itemObject.value("detail").toString().left(60);
        item.category = itemObject.value("category").toString().left(60);
        item.vehicleBody = itemObject.value("vehicleBody").toString();
        item.vehicleYear = itemObject.value("vehicleYear").toInt();
        if (!item.vehicleBody.isEmpty() && (!QStringList{"sedan", "coupe", "suv", "pickup", "van", "dirt-bike"}.contains(item.vehicleBody)
            || item.kind != "vehicle" || (item.vehicleYear != 0 && (item.vehicleYear < 1886 || item.vehicleYear > 2100)))) {
            error = "The page returned invalid vehicle metadata.";
            return false;
        }
        const double aspect = itemObject.value("visualAspect").toDouble(1.6);
        if (std::isfinite(aspect))
            item.visualAspect = qBound(0.5, aspect, 2.4);
        item.hasImage = itemObject.value("hasImage").toBool();
        bool numeric = false;
        const int index = item.id.toInt(&numeric);
        if (!numeric || index < 0 || index >= maximumItems || ids.contains(item.id)
            || (!item.url.isEmpty() && !allowedUrl(item.url))) {
            error = "The page returned an invalid display or destination.";
            return false;
        }
        ids.insert(item.id);
        SpatialAnimalModel::interpret(item, document.title + " " + document.evidence + " " + document.url.path());
        document.items.append(item);
    }
    out = document;
    return true;
}

SpatialInterior SpatialWebDocument::room() const
{
    return SpatialWebWorld::build(*this);
}

QImage SpatialWebDocument::poster(const SpatialWebItem &item, const QImage &visual, int number)
{
    QImage image(512, 320, QImage::Format_ARGB32);
    image.fill(item.kind == "product" ? Qt::transparent : QColor("#101e2c"));
    QPainter painter(&image);
    if (item.kind == "excerpt") {
        painter.setPen(QColor("#72d9cb"));
        painter.setFont(QFont("Sans", 13, QFont::DemiBold));
        painter.drawText(QRect(20, 16, 472, 32), Qt::AlignLeft, item.title);
        painter.setPen(QColor("#e4ecee"));
        painter.setFont(QFont("Sans", 13));
        painter.drawText(QRect(20, 55, 472, 220), Qt::TextWordWrap, item.description);
        painter.setPen(QColor("#79c9c0"));
        painter.setFont(QFont("Sans", 9));
        painter.drawText(QRect(20, 288, 472, 24), Qt::AlignLeft, item.url.host() + " · source excerpt");
        return image;
    }
    if (item.kind == "product" && !visual.isNull()) {
        const auto scaled = visual.scaled(488, 308, Qt::KeepAspectRatio, Qt::SmoothTransformation);
        painter.drawImage(QPoint((512 - scaled.width()) / 2, 320 - scaled.height()), scaled);
        return image;
    }
    if (!visual.isNull()) {
        const auto scaled = visual.scaled(488, 218, Qt::KeepAspectRatio, Qt::SmoothTransformation);
        painter.drawImage(QPoint((512 - scaled.width()) / 2, 10 + (218 - scaled.height()) / 2), scaled);
    } else {
        painter.setPen(QColor("#72d9cb"));
        painter.setFont(QFont("Sans", 12, QFont::DemiBold));
        painter.drawText(QRect(20, 15, 470, 28), Qt::AlignLeft,
            QString("%1 / %2").arg(QString::number(number + 1).rightJustified(2, '0'), item.kind.toUpper()));
        painter.setPen(Qt::white);
        painter.setFont(QFont("Sans", 24, QFont::DemiBold));
        painter.drawText(QRect(20, 48, 470, 110), Qt::TextWordWrap | Qt::AlignVCenter, item.title.left(70));
        painter.setPen(QColor("#a6bfc9"));
        painter.setFont(QFont("Sans", 11));
        painter.drawText(QRect(20, 165, 470, 60), Qt::TextWordWrap, item.description);
    }
    painter.fillRect(QRect(0, 232, 512, 88), QColor("#07111b"));
    painter.setPen(Qt::white);
    painter.setFont(QFont("Sans", 12, QFont::DemiBold));
    painter.drawText(QRect(16, 241, 480, 52), Qt::TextWordWrap, item.title);
    painter.setFont(QFont("Sans", 9));
    painter.setPen(QColor("#79c9c0"));
    painter.drawText(QRect(16, 297, 480, 18), Qt::AlignLeft,
        item.detail.isEmpty() ? item.url.host() : item.detail);
    return image;
}
