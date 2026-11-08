#include "RemoteGitContext.h"

#include <QObject>
#include <QRegularExpression>
#include <QVariant>

namespace {
constexpr auto RemoteMode = "dockpipeRemoteGitMode";
constexpr auto Target = "dockpipeRemoteGitTarget";
constexpr auto Root = "dockpipeRemoteGitRoot";

QVariant inheritedProperty(const QObject *object, const char *name)
{
    for (auto *current = object; current; current = current->parent()) {
        const auto value = current->property(name);
        if (value.isValid())
            return value;
    }
    return {};
}
}

bool RemoteGitContext::isRemote(const QObject *object)
{
    return inheritedProperty(object, RemoteMode).toBool();
}

QJsonObject RemoteGitContext::target(const QObject *object)
{
    return inheritedProperty(object, Target).toJsonObject();
}

QString RemoteGitContext::root(const QObject *object)
{
    return inheritedProperty(object, Root).toString();
}

bool RemoteGitContext::validTarget(const QJsonObject &target, const QString &root)
{
    static const QRegularExpression identifier(QStringLiteral("\\A[A-Za-z0-9][A-Za-z0-9_-]{0,63}\\z"));
    static const QRegularExpression digest(QStringLiteral("\\A[0-9a-f]{64}\\z"));
    return identifier.match(target.value("node").toString()).hasMatch()
        && digest.match(target.value("fingerprint").toString()).hasMatch()
        && digest.match(target.value("generation").toString()).hasMatch()
        && identifier.match(root).hasMatch();
}

void RemoteGitContext::bind(QObject *object, bool remote, const QJsonObject &target, const QString &root)
{
    object->setProperty(RemoteMode, remote);
    object->setProperty(Target, target);
    object->setProperty(Root, root);
}

