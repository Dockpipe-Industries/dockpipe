#pragma once

#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>

// SSH authenticates the selected endpoint. Its OS hostname may differ from the
// configured alias; accept a bounded reported identity rather than equating them.
class SshMachineSnapshot final {
public:
    static bool parse(const QByteArray &output, const QString &component, QJsonObject *snapshot,
        QString *reportedNode = nullptr)
    {
        if (!snapshot || output.size() > 1024 * 1024)
            return false;
        const auto document = QJsonDocument::fromJson(output);
        if (!document.isObject())
            return false;
        const auto envelope = document.object();
        const QString node = envelope.value("node").toString();
        if (envelope.value("schema").toString() != "dockpipe.machine_snapshot.v1"
            || envelope.value("state").toString() != "live" || node.trimmed().isEmpty()
            || node.toUtf8().size() > 253 || !envelope.value(component).isObject())
            return false;
        for (const QChar character : node) {
            if (character.category() == QChar::Other_Control)
                return false;
        }
        const auto candidate = envelope.value(component).toObject();
        if (component == "execution") {
            if (candidate.value("schema").toString() != "dockpipe.execution_snapshot.v1"
                || !objectRows(candidate.value("executions"), 256)
                || !objectRows(candidate.value("processes"), 4096))
                return false;
        } else if (component == "docker") {
            if (!candidate.value("available").isBool())
                return false;
            for (const QString &collection : {QStringLiteral("containers"), QStringLiteral("networks"),
                     QStringLiteral("volumes"), QStringLiteral("topology")}) {
                const auto rows = candidate.value(collection);
                if (!candidate.value("available").toBool() && (rows.isNull() || rows.isUndefined()))
                    continue;
                if (!objectRows(rows, 256))
                    return false;
            }
        } else {
            return false;
        }
        *snapshot = candidate;
        if (reportedNode)
            *reportedNode = node;
        return true;
    }

private:
    static bool objectRows(const QJsonValue &value, int limit)
    {
        if (!value.isArray() || value.toArray().size() > limit)
            return false;
        for (const auto &row : value.toArray()) {
            if (!row.isObject())
                return false;
        }
        return true;
    }
};
