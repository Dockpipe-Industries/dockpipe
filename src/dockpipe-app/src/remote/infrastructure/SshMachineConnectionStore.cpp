#include "SshMachineConnectionStore.h"
#include "SshMachinePrivateFiles.h"

#include <QJsonDocument>
#include <QJsonObject>
#include <QSaveFile>
#include <QSet>

namespace {
// One profile, bounded before parsing; paths account for most of the allowance.
constexpr qint64 MaximumStoreBytes = 32 * 1024;

bool fail(QString *error, const QString &message)
{
    if (error)
        *error = message;
    return false;
}
}

SshMachineConnectionStore::SshMachineConnectionStore(const QString &path) : m_path(path) {}
QString SshMachineConnectionStore::path() const { return m_path; }

bool SshMachineConnectionStore::load(SshMachineConnection *connection, QString *error) const
{
    if (error)
        error->clear();
    if (!connection || !QDir::isAbsolutePath(m_path))
        return fail(error, QStringLiteral("An absolute connection store path and destination are required."));
    const QFileInfo information(m_path);
    const QFileInfo directory(information.absolutePath());
    if (!SshMachinePrivateFiles::ownedRegularPath(information, false)
        || !SshMachinePrivateFiles::ownedRegularPath(directory, true)
        || !SshMachinePrivateFiles::privatePermissions(information)
        || !SshMachinePrivateFiles::privatePermissions(directory))
        return fail(error, QStringLiteral("The connection store is unavailable or its permissions are not private."));
    QFile file(m_path);
    if (!file.open(QIODevice::ReadOnly) || file.size() > MaximumStoreBytes)
        return fail(error, QStringLiteral("The connection store cannot be read or exceeds 32 KiB."));
    const QByteArray contents = file.read(MaximumStoreBytes + 1);
    if (contents.size() > MaximumStoreBytes || file.error() != QFileDevice::NoError)
        return fail(error, QStringLiteral("The connection store could not be read completely."));
    const auto document = QJsonDocument::fromJson(contents);
    if (!document.isObject())
        return fail(error, QStringLiteral("The connection store must contain a JSON object."));
    const auto root = document.object();
    if (root.size() != 2 || root.value("schema") != QJsonValue(1) || !root.value("connection").isObject())
        return fail(error, QStringLiteral("The connection store schema is unsupported."));
    const auto object = root.value("connection").toObject();
    const QSet<QString> stringKeys{"id", "label", "host", "user", "native_cli_path", "identity_file", "remote_directory"};
    if (object.size() != stringKeys.size() + 1)
        return fail(error, QStringLiteral("The connection store fields are invalid."));
    for (const auto &key : stringKeys) {
        if (!object.value(key).isString())
            return fail(error, QStringLiteral("The connection store fields are invalid."));
    }
    const auto port = object.value("port");
    if (!port.isDouble() || port.toDouble() != port.toInt())
        return fail(error, QStringLiteral("The SSH port must be an integer."));
    SshMachineConnection candidate;
    candidate.id = object.value("id").toString();
    candidate.label = object.value("label").toString();
    candidate.host = object.value("host").toString();
    candidate.user = object.value("user").toString();
    candidate.port = port.toInt();
    candidate.nativeCliPath = object.value("native_cli_path").toString();
    candidate.identityFile = object.value("identity_file").toString();
    candidate.remoteDirectory = object.value("remote_directory").toString();
    const QString validation = candidate.validationError();
    if (!validation.isEmpty())
        return fail(error, validation);
    *connection = candidate;
    return true;
}

bool SshMachineConnectionStore::save(const SshMachineConnection &connection, QString *error) const
{
    if (error)
        error->clear();
    const QString validation = connection.validationError();
    if (!validation.isEmpty())
        return fail(error, validation);
    if (!QDir::isAbsolutePath(m_path)
        || !SshMachinePrivateFiles::prepareDirectory(QFileInfo(m_path).absolutePath()))
        return fail(error, QStringLiteral("The connection store requires an owned private directory."));
    const QFileInfo previous(m_path);
    if (previous.isSymLink() || (previous.exists() && !SshMachinePrivateFiles::ownedRegularPath(previous, false)))
        return fail(error, QStringLiteral("The connection store destination is not an owned regular file."));
    const QJsonObject object{{"id", connection.id}, {"label", connection.label},
        {"host", connection.host}, {"user", connection.user}, {"port", connection.port},
        {"native_cli_path", connection.nativeCliPath}, {"identity_file", connection.identityFile},
        {"remote_directory", connection.remoteDirectory}};
    const auto contents = QJsonDocument(QJsonObject{{"schema", 1}, {"connection", object}}).toJson();
    QSaveFile file(m_path);
    file.setDirectWriteFallback(false);
    if (!file.open(QIODevice::WriteOnly) || !file.setPermissions(SshMachinePrivateFiles::FilePermissions)
        || file.write(contents) != contents.size() || !file.commit())
        return fail(error, QStringLiteral("The connection store could not be saved atomically."));
    return true;
}
