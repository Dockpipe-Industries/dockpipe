#pragma once

#include <QJsonDocument>
#include <QJsonObject>
#include <QRegularExpression>

// Read-only native Docker query envelope. The transport imposes the same byte
// limit before retaining output; decoding never accepts a partial JSON result.
struct NativeHostDockerQuery final {
    QJsonObject inspect;
    QString logs;
    QString logsError;
    bool logsTruncated = false;

    static bool validRequest(const QString &kind, const QString &id)
    {
        static const QRegularExpression objectId(QStringLiteral("\\A[0-9a-fA-F]{12,64}\\z"));
        static const QRegularExpression volumeName(QStringLiteral("\\A[A-Za-z0-9][A-Za-z0-9_.-]{0,254}\\z"));
        if (kind == "container" || kind == "network")
            return objectId.match(id).hasMatch();
        return kind == "volume" && volumeName.match(id).hasMatch();
    }

    static bool decode(const QByteArray &output, const QString &kind, const QString &id,
                       NativeHostDockerQuery *query)
    {
        if (!query || !validRequest(kind, id) || output.size() > 1024 * 1024)
            return false;
        const auto document = QJsonDocument::fromJson(output);
        if (!document.isObject())
            return false;
        const auto result = document.object();
        if (result.value("schema").toString() != "dockpipe.native_host_docker_query.v1"
            || result.value("outcome").toString() != "Success"
            || result.value("kind").toString() != kind || result.value("id").toString() != id
            || !result.value("inspect").isObject() || !result.value("logs").isString())
            return false;
        const auto logsError = result.value("logs_error");
        const auto truncated = result.value("logs_truncated");
        if ((!logsError.isUndefined() && !logsError.isString())
            || (!truncated.isUndefined() && !truncated.isBool()))
            return false;
        const QString error = logsError.toString();
        if (!error.isEmpty() && error != "docker_logs_unavailable" && error != "docker_logs_invalid")
            return false;
        NativeHostDockerQuery candidate;
        candidate.inspect = result.value("inspect").toObject();
        candidate.logs = result.value("logs").toString();
        candidate.logsError = error;
        candidate.logsTruncated = truncated.toBool();
        if (QJsonDocument(candidate.inspect).toJson(QJsonDocument::Compact).size() > 256 * 1024
            || candidate.logs.toUtf8().size() > 64 * 1024)
            return false;
        *query = candidate;
        return true;
    }
};
