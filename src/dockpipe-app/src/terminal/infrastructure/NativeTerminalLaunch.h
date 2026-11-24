#pragma once

#include "TerminalLaunchContext.h"

#include <QJsonObject>
#include <QMap>
#include <QString>
#include <QStringList>

// A launch is frozen before the shared Core validation boundary. In particular,
// the trusted host never fills defaults or merges its own ambient environment.
class NativeTerminalLaunch final {
public:
    struct Snapshot {
        QString id;
        QString executable;
        QStringList arguments;
        QStringList environment;
        QString directory;
        bool host = false;
        QString error;

        QJsonObject toJson() const;
    };

    static Snapshot resolve(const QString &directory, const QString &program = {},
        const QStringList &arguments = {}, const QMap<QString, QString> &overrides = {},
        TerminalLaunchContext context = TerminalLaunchContext::Automatic,
        const QString &profileId = QStringLiteral("native-default"));
    static bool fromJson(const QJsonObject &object, Snapshot &snapshot);
    static bool valid(const Snapshot &snapshot);
};
