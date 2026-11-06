#pragma once

#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>

class NativeHostSnapshot final {
public:
    static bool decode(const QByteArray &output, const QString &node, const QString &component, QJsonObject *snapshot)
    {
        if (!snapshot || node.isEmpty() || output.size() > 1024 * 1024)
            return false;
        const auto document = QJsonDocument::fromJson(output);
        if (!document.isObject())
            return false;
        const auto envelope = document.object();
        if (envelope.value("schema").toString() != "dockpipe.machine_snapshot.v1"
            || envelope.value("node").toString() != node || envelope.value("state").toString() != "live"
            || !envelope.value(component).isObject())
            return false;
        const auto candidate = envelope.value(component).toObject();
        if (component == "execution") {
            if (candidate.value("schema").toString() != "dockpipe.execution_snapshot.v1"
                || !rows(candidate.value("executions"), 256) || !rows(candidate.value("processes"), 4096))
                return false;
        } else if (component == "docker") {
            if (!candidate.value("available").isBool())
                return false;
            for (const QString &name : {QStringLiteral("containers"), QStringLiteral("networks"),
                     QStringLiteral("volumes"), QStringLiteral("topology")}) {
                const auto collection = candidate.value(name);
                if (!candidate.value("available").toBool() && (collection.isNull() || collection.isUndefined()))
                    continue;
                if (!rows(collection, 256))
                    return false;
            }
        } else {
            return false;
        }
        *snapshot = candidate;
        return true;
    }

private:
    static bool rows(const QJsonValue &value, int maximum)
    {
        if (!value.isArray() || value.toArray().size() > maximum)
            return false;
        for (const auto &row : value.toArray()) {
            if (!row.isObject())
                return false;
        }
        return true;
    }
};
