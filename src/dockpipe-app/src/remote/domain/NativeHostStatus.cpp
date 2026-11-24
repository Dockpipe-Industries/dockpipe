#include "NativeHostStatus.h"

#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <limits>

namespace {
bool boundedText(const QString &text, int maximum, bool required)
{
    if ((required && text.trimmed().isEmpty()) || text.size() > maximum || text.toUtf8().size() > maximum)
        return false;
    for (const QChar character : text) {
        if (character.category() == QChar::Other_Control)
            return false;
    }
    return true;
}

bool unsignedIdentifier(const QJsonValue &value)
{
    const double number = value.toDouble(-1);
    return value.isDouble() && number >= 0 && number <= std::numeric_limits<quint32>::max()
        && number == static_cast<quint32>(number);
}
}

QString NativeHostStatus::identity() const
{
    return QString::fromUtf8(QJsonDocument(QJsonArray{node, username, static_cast<double>(uid),
        static_cast<double>(gid), generation}).toJson(QJsonDocument::Compact));
}

bool NativeHostStatus::decode(const QByteArray &output, NativeHostStatus *status)
{
    if (!status || output.size() > 1024 * 1024)
        return false;
    const auto document = QJsonDocument::fromJson(output);
    if (!document.isObject())
        return false;
    const auto object = document.object();
    if (object.value("schema").toString() != "dockpipe.native_host_status.v1"
        || !object.value("available").isBool() || !object.value("connected").isBool())
        return false;
    NativeHostStatus candidate;
    candidate.available = object.value("available").toBool();
    if (!candidate.available) {
        if (object.value("connected").toBool())
            return false;
        *status = candidate;
        return true;
    }
    candidate.node = object.value("node").toString();
    candidate.username = object.value("username").toString();
    candidate.home = object.value("home").toString();
    candidate.generation = object.value("generation").toString();
    if (!object.value("connected").toBool() || !unsignedIdentifier(object.value("uid"))
        || object.value("uid").toDouble() == 0 || !unsignedIdentifier(object.value("gid"))
        || !boundedText(candidate.node, 255, true) || !boundedText(candidate.username, 256, true)
        || !boundedText(candidate.home, 4096, false) || (!candidate.home.isEmpty() && !candidate.home.startsWith('/'))
        || !boundedText(candidate.generation, 256, false)
        || (object.contains("generation") && !object.value("generation").isString())
        || (object.contains("home") && !object.value("home").isString())
        || (!object.value("groups").isUndefined() && !object.value("groups").isNull() && !object.value("groups").isArray())
        || object.value("groups").toArray().size() > 65536)
        return false;
    for (const auto &group : object.value("groups").toArray()) {
        if (!unsignedIdentifier(group))
            return false;
    }
    candidate.uid = static_cast<quint32>(object.value("uid").toDouble());
    candidate.gid = static_cast<quint32>(object.value("gid").toDouble());
    *status = candidate;
    return true;
}
