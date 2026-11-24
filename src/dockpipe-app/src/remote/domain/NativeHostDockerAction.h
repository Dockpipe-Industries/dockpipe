#pragma once

#include <QJsonDocument>
#include <QJsonObject>
#include <QRegularExpression>

class NativeHostDockerAction final {
public:
    static bool validRequest(const QString &action, const QString &id)
    {
        static const QRegularExpression identifier(QStringLiteral("\\A[0-9a-f]{12,64}\\z"));
        return (action == "start" || action == "stop") && identifier.match(id).hasMatch();
    }

    static bool succeeded(const QByteArray &output, const QString &action, const QString &id)
    {
        if (!validRequest(action, id) || output.size() > 64 * 1024)
            return false;
        const auto document = QJsonDocument::fromJson(output);
        if (!document.isObject())
            return false;
        const auto result = document.object();
        return result.value("schema").toString() == "dockpipe.native_host_docker_action.v1"
            && result.value("outcome").toString() == "Success"
            && result.value("action").toString() == action && result.value("id").toString() == id;
    }
};
