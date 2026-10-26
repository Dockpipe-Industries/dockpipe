#include "ContextStore.h"

#include <QCoreApplication>
#include <QDir>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QTemporaryDir>
#include <cstdio>

namespace {
bool check(bool condition, const char *message)
{
    if (!condition)
        std::fprintf(stderr, "%s\n", message);
    return condition;
}

QByteArray readFile(const QString &path)
{
    QFile file(path);
    if (!file.open(QIODevice::ReadOnly))
        return {};
    return file.readAll();
}
}

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    QCoreApplication::setApplicationName(QStringLiteral("dockpipe-context-store-regression"));
    QTemporaryDir directory;
    if (!directory.isValid())
        return 1;
    qputenv("XDG_CONFIG_HOME", directory.path().toUtf8());
    const QString configDirectory = ContextStore::configDir();
    if (!check(configDirectory.startsWith(directory.path() + QLatin1Char('/')), "configuration escaped the temporary test directory"))
        return 1;

    Context context = Context::createNew();
    context.label = QStringLiteral("original");
    context.workdir = directory.path();
    context.transientLaunchEnv.insert(QStringLiteral("PRIVATE_INPUT"), QStringLiteral("not-persisted"));
    ContextStore store;
    store.contexts.append(context);
    if (!check(store.save(), "initial save failed"))
        return 1;
    const QString path = ContextStore::contextsPath();
    const QByteArray original = readFile(path);
    const QJsonObject document = QJsonDocument::fromJson(original).object();
    if (!check(document.value(QStringLiteral("version")).toInt() == 1
                   && document.value(QStringLiteral("contexts")).toArray().size() == 1
                   && !original.contains("PRIVATE_INPUT") && !original.contains("not-persisted"),
               "context schema or transient-input exclusion changed"))
        return 1;

    ContextStore loaded;
    if (!check(loaded.load() && loaded.contexts.size() == 1
                   && loaded.contexts.front().id == context.id && loaded.contexts.front().label == context.label,
               "saved context did not round-trip"))
        return 1;

    // The destination remains writable, but replacing it atomically is forbidden.
    // A truncate-in-place save would incorrectly succeed and destroy the old value.
    const QFile::Permissions permissions = QFile::permissions(configDirectory);
    const QFile::Permissions readOnly = QFileDevice::ReadOwner | QFileDevice::ExeOwner;
    if (!check(QFile::setPermissions(configDirectory, readOnly), "could not make the fixture directory read-only"))
        return 1;
    QFile probe(QDir(configDirectory).filePath(QStringLiteral("write-probe")));
    if (probe.open(QIODevice::WriteOnly)) {
        probe.close();
        QFile::setPermissions(configDirectory, permissions);
        std::fprintf(stderr, "test requires an unprivileged process that respects directory permissions\n");
        return 1;
    }
    store.contexts.front().label = QStringLiteral("replacement");
    const bool saved = store.save();
    const QByteArray afterFailure = readFile(path);
    QFile::setPermissions(configDirectory, permissions);
    if (!check(!saved && afterFailure == original, "failed atomic save changed the previous contexts or reported success"))
        return 1;

    if (!check(store.save() && loaded.load() && loaded.contexts.front().label == QStringLiteral("replacement"),
               "save did not recover after the write restriction was removed"))
        return 1;
    return 0;
}
