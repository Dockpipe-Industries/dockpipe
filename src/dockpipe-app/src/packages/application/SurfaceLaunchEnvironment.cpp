#include "SurfaceLaunchEnvironment.h"
#include "Context.h"
#include "PackageDisplayWidget.h"
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QProcessEnvironment>
#include <QRegularExpression>
#include <QSet>

bool SurfaceLaunchEnvironment::apply(Context &invocation, const PackageSurfaceActionMeta &action,
                                    const QString &packageRoot, QString &error)
{
    const auto bindings = action.launchEnvironment.value(PackageDisplayWidget::platformName());
    if (bindings.isEmpty())
        return true;
    if (bindings.size() > 16 || action.scope != "surface" || action.workflowId != invocation.workflow
        || action.workflowId.isEmpty()) {
        error = QObject::tr("Invalid installed public action environment.");
        return false;
    }
    QProcessEnvironment original = QProcessEnvironment::systemEnvironment();
    if (!invocation.envFile.isEmpty()) {
        QFile input(QDir(invocation.workdir).absoluteFilePath(invocation.envFile));
        if (!input.open(QIODevice::ReadOnly) || input.size() > 1024 * 1024) {
            error = QObject::tr("The selected environment file is unavailable or exceeds 1 MiB.");
            return false;
        }
        // Match the CLI's dotenv fallback: inherited process values win, then
        // explicit invocation assignments override both. Never log file contents.
        QMap<QString, QString> fileValues;
        for (const auto &bytes : input.readAll().split('\n')) {
            const QString line = QString::fromUtf8(bytes).trimmed();
            const int separator = line.indexOf('=');
            if (line.startsWith('#') || separator <= 0)
                continue;
            const QString key = line.left(separator).trimmed();
            QString value = line.mid(separator + 1).trimmed();
            if (value.size() >= 2 && ((value.startsWith('"') && value.endsWith('"'))
                || (value.startsWith('\'') && value.endsWith('\''))))
                value = value.mid(1, value.size() - 2);
            fileValues.insert(key, value);
        }
        for (auto entry = fileValues.begin(); entry != fileValues.end(); ++entry) {
            if (!original.contains(entry.key()))
                original.insert(entry.key(), entry.value());
        }
    }
    for (auto entry = invocation.transientLaunchEnv.begin(); entry != invocation.transientLaunchEnv.end(); ++entry)
        original.insert(entry.key(), entry.value());
    for (const QString &assignment : invocation.extraDockpipeEnv) {
        const int separator = assignment.indexOf('=');
        if (separator > 0)
            original.insert(assignment.left(separator).trimmed(), assignment.mid(separator + 1).trimmed());
    }
    const QRegularExpression namePattern("^[A-Za-z_][A-Za-z0-9_]{0,63}$");
    QMap<QString, QString> resolved;
    QSet<QString> inheritedSources;
    for (auto binding = bindings.begin(); binding != bindings.end(); ++binding) {
        const auto &reference = binding.value();
        if (!namePattern.match(binding.key()).hasMatch() || reference.asset.isEmpty() == reference.inherit.isEmpty()) {
            error = QObject::tr("Invalid action environment binding.");
            return false;
        }
        if (!reference.inherit.isEmpty()) {
            if (!namePattern.match(reference.inherit).hasMatch()) {
                error = QObject::tr("Invalid inherited environment reference.");
                return false;
            }
            if (original.contains(reference.inherit)) {
                const QString value = original.value(reference.inherit);
                if (value.toUtf8().size() > 4096 || value.contains(QChar(0))) {
                    error = QObject::tr("Inherited environment value exceeds its capacity.");
                    return false;
                }
                resolved.insert(binding.key(), value);
                inheritedSources.insert(reference.inherit);
            }
            continue;
        }
        const QFileInfo root(packageRoot);
        const QFileInfo asset(reference.asset);
        const QString canonicalRoot = root.canonicalFilePath();
        const QString canonicalAsset = asset.canonicalFilePath();
        const QString relative = QDir(canonicalRoot).relativeFilePath(canonicalAsset);
        if (!root.isAbsolute() || canonicalRoot.isEmpty() || !asset.isAbsolute() || !asset.isFile()
            || canonicalAsset.isEmpty() || relative == "." || QDir::isAbsolutePath(relative)
            || relative.split('/').contains("..") || !asset.isExecutable()) {
            error = QObject::tr("Action environment asset is not a contained executable.");
            return false;
        }
        resolved.insert(binding.key(), canonicalAsset);
    }
    int resolvedBytes = 0;
    for (auto entry = resolved.begin(); entry != resolved.end(); ++entry)
        resolvedBytes += entry.key().toUtf8().size() + entry.value().toUtf8().size();
    if (resolvedBytes > 16 * 1024) {
        error = QObject::tr("Action environment exceeds its 16 KiB capacity.");
        return false;
    }
    // Evaluate references against the original snapshot, then replace targets.
    // Migrate referenced explicit assignments out of argv too: inherited values
    // may be sensitive and must never appear in previews or session arguments.
    invocation.extraDockpipeEnv.removeIf([&](const QString &assignment) {
        const QString key = assignment.section('=', 0, 0).trimmed();
        if (inheritedSources.contains(key) && !resolved.contains(key))
            invocation.transientLaunchEnv.insert(key, original.value(key));
        return resolved.contains(key) || inheritedSources.contains(key);
    });
    for (auto entry = resolved.begin(); entry != resolved.end(); ++entry)
        invocation.transientLaunchEnv.insert(entry.key(), entry.value());
    error.clear();
    return true;
}
