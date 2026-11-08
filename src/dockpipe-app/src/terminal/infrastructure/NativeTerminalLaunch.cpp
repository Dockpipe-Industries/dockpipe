#include "NativeTerminalLaunch.h"
#include "TerminalHostLaunch.h"

#include <QCoreApplication>
#include <QDir>
#include <QFileInfo>
#include <QJsonArray>
#include <QProcessEnvironment>
#include <QSet>

#ifdef Q_OS_MACOS
#include <pwd.h>
#include <unistd.h>
#endif

namespace {
// The private start frame is limited to 2 MiB; additionally bound the decoded
// environment to the existing native backend's 1 MiB UTF-16 contract.
constexpr qsizetype MaximumEnvironmentCharacters = 512 * 1024;
constexpr qsizetype MaximumLaunchCharacters = 768 * 1024;
constexpr qsizetype MaximumEntries = 4096;

bool safeString(const QString &value)
{
    return !value.contains(QChar::Null);
}

bool stringArray(const QJsonValue &value, QStringList &result)
{
    if (!value.isArray() || value.toArray().size() > MaximumEntries)
        return false;
    for (const auto &entry : value.toArray()) {
        if (!entry.isString())
            return false;
        result.append(entry.toString());
    }
    return true;
}
}

QJsonObject NativeTerminalLaunch::Snapshot::toJson() const
{
    return {{QStringLiteral("id"), id}, {QStringLiteral("executable"), executable},
        {QStringLiteral("args"), QJsonArray::fromStringList(arguments)},
        {QStringLiteral("env"), QJsonArray::fromStringList(environment)},
        {QStringLiteral("dir"), directory},
        {QStringLiteral("route"), host ? QStringLiteral("host") : QStringLiteral("native")},
        {QStringLiteral("environment_mode"), host ? QStringLiteral("inherit-host") : QStringLiteral("complete")}};
}

bool NativeTerminalLaunch::valid(const Snapshot &snapshot)
{
    if (snapshot.id.isEmpty() || snapshot.id.size() > 128 || !safeString(snapshot.id)
        || !QDir::isAbsolutePath(snapshot.executable) || !QDir::isAbsolutePath(snapshot.directory)
        || !safeString(snapshot.executable) || !safeString(snapshot.directory)
        || snapshot.arguments.size() > 64 || snapshot.environment.size() > MaximumEntries)
        return false;
    qsizetype characters = snapshot.id.size() + snapshot.executable.size() + snapshot.directory.size();
    for (const auto &argument : snapshot.arguments) {
        if (!safeString(argument) || argument.size() > 64 * 1024)
            return false;
        characters += argument.size();
        if (characters > MaximumLaunchCharacters)
            return false;
    }
    qsizetype environmentCharacters = 1;
    QSet<QString> names;
    bool term = false;
    bool colorTerm = false;
    for (const auto &entry : snapshot.environment) {
        // Windows' inherited =C:=C:\\... drive-directory entries are real
        // environment entries. Keep them verbatim in complete snapshots.
        const qsizetype separator = entry.indexOf(QLatin1Char('='), entry.startsWith(QLatin1Char('=')) ? 1 : 0);
        if (separator <= 0 || !safeString(entry) || entry.size() > 32767)
            return false;
        const QString name = entry.left(separator);
        if (name.startsWith(QLatin1Char('='))) {
#ifdef Q_OS_WIN
            if (snapshot.host || name.size() != 3 || !name.at(1).isLetter() || name.at(2) != QLatin1Char(':'))
                return false;
#else
            return false;
#endif
        }
#ifdef Q_OS_WIN
        const QString identity = name.toCaseFolded();
#else
        const QString identity = name;
#endif
        if (names.contains(identity))
            return false;
        names.insert(identity);
        environmentCharacters += entry.size() + 1;
        if (environmentCharacters > MaximumEnvironmentCharacters)
            return false;
        term = term || name == QStringLiteral("TERM");
        colorTerm = colorTerm || name == QStringLiteral("COLORTERM");
    }
    if (snapshot.host && (snapshot.environment.size() > 66 || !term || !colorTerm))
        return false;
    return characters + environmentCharacters <= MaximumLaunchCharacters;
}

bool NativeTerminalLaunch::fromJson(const QJsonObject &object, Snapshot &snapshot)
{
    static const QSet<QString> keys = {QStringLiteral("id"), QStringLiteral("executable"),
        QStringLiteral("args"), QStringLiteral("env"), QStringLiteral("dir"),
        QStringLiteral("route"), QStringLiteral("environment_mode")};
    if (object.size() != keys.size())
        return false;
    for (auto entry = object.begin(); entry != object.end(); ++entry) {
        if (!keys.contains(entry.key()))
            return false;
    }
    for (const auto &key : {QStringLiteral("id"), QStringLiteral("executable"), QStringLiteral("dir"),
             QStringLiteral("route"), QStringLiteral("environment_mode")}) {
        if (!object.value(key).isString())
            return false;
    }
    Snapshot candidate;
    candidate.id = object.value(QStringLiteral("id")).toString();
    candidate.executable = object.value(QStringLiteral("executable")).toString();
    candidate.directory = object.value(QStringLiteral("dir")).toString();
    const QString route = object.value(QStringLiteral("route")).toString();
    const QString mode = object.value(QStringLiteral("environment_mode")).toString();
    if (route == QStringLiteral("host") && mode == QStringLiteral("inherit-host")) {
        candidate.host = true;
    } else if (route != QStringLiteral("native") || mode != QStringLiteral("complete")) {
        return false;
    }
    if (!stringArray(object.value(QStringLiteral("args")), candidate.arguments)
        || !stringArray(object.value(QStringLiteral("env")), candidate.environment)
        || !valid(candidate))
        return false;
    snapshot = std::move(candidate);
    return true;
}

NativeTerminalLaunch::Snapshot NativeTerminalLaunch::resolve(const QString &directory, const QString &program,
    const QStringList &arguments, const QMap<QString, QString> &overrides,
    TerminalLaunchContext context, const QString &profileId)
{
    Snapshot snapshot;
    snapshot.id = profileId;
    snapshot.directory = directory.isEmpty() ? QDir::homePath() : directory;
    snapshot.executable = program;
    snapshot.arguments = arguments;
    snapshot.host = TerminalHostLaunch::required(context);
    if (overrides.size() > 64) {
        snapshot.error = QCoreApplication::translate("NativeTerminalLaunch", "A terminal profile may override at most 64 environment variables.");
        return snapshot;
    }
    for (auto entry = overrides.begin(); entry != overrides.end(); ++entry) {
        if (entry.key().isEmpty() || entry.key().contains(QLatin1Char('=')) || !safeString(entry.key())
            || !safeString(entry.value())) {
            snapshot.error = QCoreApplication::translate("NativeTerminalLaunch", "The terminal profile contains an invalid environment variable.");
            return snapshot;
        }
    }
    if (snapshot.executable.isEmpty()) {
#ifdef Q_OS_WIN
        snapshot.executable = QDir::toNativeSeparators(qEnvironmentVariable("SystemRoot") + QStringLiteral("/System32/cmd.exe"));
        snapshot.arguments = {QStringLiteral("/D")};
#else
        snapshot.executable = qEnvironmentVariable("SHELL");
        const auto available = [&snapshot] {
            const QFileInfo shell(snapshot.executable);
            return shell.isAbsolute() && (snapshot.host || (shell.isFile() && shell.isExecutable()));
        };
        if (!available()) {
#ifdef Q_OS_MACOS
            QByteArray accountBuffer(64 * 1024, '\0');
            passwd account{};
            passwd *found = nullptr;
            if (::getpwuid_r(::getuid(), &account, accountBuffer.data(), size_t(accountBuffer.size()), &found) == 0
                && found && account.pw_shell)
                snapshot.executable = QString::fromLocal8Bit(account.pw_shell);
#endif
            if (!available())
                snapshot.executable = QStringLiteral("/bin/sh");
        }
        snapshot.arguments = {QStringLiteral("-i")};
#endif
    }
    if (!snapshot.host && (!QFileInfo(snapshot.directory).isDir()
        || !QFileInfo(snapshot.executable).isAbsolute() || !QFileInfo(snapshot.executable).isFile()
#ifndef Q_OS_WIN
        || !QFileInfo(snapshot.executable).isExecutable()
#endif
        )) {
        snapshot.error = QCoreApplication::translate("NativeTerminalLaunch", "The terminal requires an existing directory and an absolute executable path.");
        return snapshot;
    }
    QProcessEnvironment environment = snapshot.host ? QProcessEnvironment() : QProcessEnvironment::systemEnvironment();
    environment.insert(QStringLiteral("TERM"), QStringLiteral("xterm-256color"));
    environment.insert(QStringLiteral("COLORTERM"), QStringLiteral("truecolor"));
    for (auto entry = overrides.begin(); entry != overrides.end(); ++entry)
        environment.insert(entry.key(), entry.value());
    snapshot.environment = environment.toStringList();
    if (!valid(snapshot))
        snapshot.error = QCoreApplication::translate("NativeTerminalLaunch", "The terminal launch exceeds its bounded profile or environment contract.");
    return snapshot;
}
