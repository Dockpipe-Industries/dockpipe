#include "WorkspacePackageManager.h"
#include "DockpipeChoices.h"
#include <QApplication>
#include <QDir>
#include <QFileInfo>
#include <QJsonArray>
#include <QJsonObject>
#include <QUrl>
#include <QRegularExpression>

WorkspacePackageManager &WorkspacePackageManager::instance()
{
    static auto *manager = new WorkspacePackageManager(qApp);
    return *manager;
}

WorkspacePackageManager::WorkspacePackageManager(QObject *parent) : QObject(parent), m_command(this)
{
    m_deadline.setSingleShot(true);
    connect(&m_deadline, &QTimer::timeout, this, [this]() {
        m_resolver.kill();
        fail(tr("Package discovery timed out. Try again."));
    });
    connect(&m_resolver, &QProcess::readyReadStandardOutput, this, [this]() {
        m_output += m_resolver.readAllStandardOutput();
        if (m_output.size() > 16384) {
            m_resolver.kill();
            fail(tr("Package discovery returned an invalid path."));
        }
    });
    connect(&m_resolver, &QProcess::readyReadStandardError, this, [this]() { m_resolver.readAllStandardError(); });
    connect(&m_resolver, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart)
            fail(tr("Could not start Dockpipe to find the Cockpit package."));
    });
    connect(&m_resolver, &QProcess::finished, this, [this](int code, QProcess::ExitStatus status) {
        m_deadline.stop();
        if (!m_active)
            return;
        if (status == QProcess::NormalExit && code == 0 && load(QString::fromUtf8(m_output).trimmed()))
            return;
        if (!m_download || m_installed) {
            fail(m_error.isEmpty() ? tr("The Cockpit package is not available. Enable it in Settings to install it.") : m_error);
            return;
        }
        catalog();
    });
    connect(&m_command, &PackageCommand::failed, this, [this](const QString &message) {
        if (m_active)
            fail(message);
    });
    connect(&m_command, &PackageCommand::completed, this, [this](const QJsonObject &result) {
        if (!m_active)
            return;
        if (m_operation == Operation::Install) {
            m_installed = true;
            resolve();
            return;
        }
        for (const auto &value : result.value("packages").toArray()) {
            const auto package = value.toObject();
            if (package.value("name").toString() != "cockpit" || package.value("kind").toString() != "resolver")
                continue;
            const QString checksum = package.value("sha256").toString();
            if (!QRegularExpression("^[0-9a-fA-F]{64}$").match(checksum).hasMatch()) {
                fail(tr("The Cockpit catalog entry has no valid checksum."));
                return;
            }
            m_operation = Operation::Install;
            emit progress(tr("Downloading and verifying Cockpit…"));
            m_command.start(DockpipeChoices::preferredDockpipeBinary(m_settings.projectFolder),
                {"package", "install", "--remote", m_manifest, "--kind", "resolver", "--name", "cockpit", "--sha256", checksum},
                m_settings.projectFolder);
            return;
        }
        catalog();
    });
}

WorkspacePackageManager::~WorkspacePackageManager()
{
    cancel();
    if (m_resolver.state() != QProcess::NotRunning) {
        m_resolver.kill();
        m_resolver.waitForFinished(1000);
    }
}

void WorkspacePackageManager::prepare(const LauncherSettings &settings, bool download)
{
    if (busy()) {
        m_error = tr("A package operation is still stopping. Please retry in a moment.");
        emit failed(m_error);
        return;
    }
    if (ready()) {
        emit prepared();
        return;
    }
    m_settings = settings;
    if (m_settings.projectFolder.isEmpty())
        m_settings.projectFolder = QDir::homePath();
    m_download = download;
    m_remote = 0;
    m_installed = false;
    m_active = true;
    m_error.clear();
    // Explicit developer override, never inferred from a checkout or cwd.
    const QString development = qEnvironmentVariable("DOCKPIPE_WORKSPACE_PACKAGE");
    if (!development.isEmpty()) {
        if (!load(development))
            fail(m_error);
        return;
    }
    resolve();
}

void WorkspacePackageManager::resolve()
{
    m_operation = Operation::Resolve;
    m_output.clear();
    emit progress(tr("Finding the installed Cockpit package…"));
    m_resolver.setWorkingDirectory(m_settings.projectFolder);
    m_resolver.start(DockpipeChoices::preferredDockpipeBinary(m_settings.projectFolder),
        {"scope", "resolver", "cockpit", "assets-dir", "--workdir", m_settings.projectFolder});
    m_deadline.start(15000);
}

bool WorkspacePackageManager::load(const QString &assets)
{
#if defined(Q_OS_WIN)
    const QString filename = "dockpipe-cockpit.dll";
#elif defined(Q_OS_MACOS)
    const QString filename = "libdockpipe-cockpit.dylib";
#else
    const QString filename = "libdockpipe-cockpit.so";
#endif
    const QFileInfo directory(assets);
    const QFileInfo library(QDir(assets).filePath("ui/" + filename));
    const QString canonicalDirectory = directory.canonicalFilePath();
    const QString canonicalLibrary = library.canonicalFilePath();
    // Qt canonical paths use forward slashes on Windows too. Compare path
    // components relative to the package root, rather than appending the
    // platform's native separator to a canonical-path string prefix.
    const QString relativeLibrary = QDir::fromNativeSeparators(QDir(canonicalDirectory).relativeFilePath(canonicalLibrary));
    const bool contained = !canonicalDirectory.isEmpty() && !canonicalLibrary.isEmpty()
        && !relativeLibrary.isEmpty() && relativeLibrary != QStringLiteral(".")
        && !QDir::isAbsolutePath(relativeLibrary)
        && !relativeLibrary.split(QLatin1Char('/')).contains(QStringLiteral(".."));
    if (!directory.isAbsolute() || !directory.isDir() || !library.isFile() || !contained) {
        m_error = tr("The Cockpit package is missing its native UI component. Install a package matching this Dockpipe app.");
        return false;
    }
    m_loader.setFileName(library.absoluteFilePath());
    // Qt/Chromium retain process-wide types and callbacks. Destroying the views
    // stops their work; the native code stays mapped until process shutdown.
    m_loader.setLoadHints(QLibrary::PreventUnloadHint);
    auto *instance = m_loader.instance();
    m_package = qobject_cast<LauncherWorkspacePackage *>(instance);
    if (!m_package) {
        m_error = instance ? tr("The Cockpit package uses an incompatible host interface.")
                           : tr("Cockpit could not load: %1").arg(m_loader.errorString());
        return false;
    }
    m_package->initialize(canonicalDirectory);
    m_error.clear();
    m_active = false;
    emit prepared();
    return true;
}

void WorkspacePackageManager::catalog()
{
    if (m_remote >= qMin(8, m_settings.packageRemotes.size())) {
        fail(tr("Cockpit is not available in the configured package catalogs for this platform. Nothing was enabled."));
        return;
    }
    QUrl url(m_settings.packageRemotes[m_remote++]);
    if (url.scheme() != "https" || url.host().isEmpty() || !url.userInfo().isEmpty()) {
        fail(tr("Cockpit downloads require an HTTPS package catalog."));
        return;
    }
    if (url.path().isEmpty() || url.path() == "/")
        url.setPath("/packages/latest.json");
    m_manifest = url.toString();
    m_operation = Operation::Catalog;
    emit progress(tr("Looking for the Cockpit download…"));
    m_command.start(DockpipeChoices::preferredDockpipeBinary(m_settings.projectFolder),
        {"package", "catalog", "--remote", m_manifest}, m_settings.projectFolder);
}

void WorkspacePackageManager::cancel()
{
    m_active = false;
    m_deadline.stop();
    m_resolver.kill();
    m_command.cancel();
}

void WorkspacePackageManager::fail(const QString &message)
{
    m_error = message;
    m_active = false;
    m_deadline.stop();
    emit failed(message);
}

LauncherWorkspaceSurface *WorkspacePackageManager::createWorkspace(QWidget *parent)
{
    return m_package ? m_package->createWorkspace(parent) : nullptr;
}

LauncherFileSurface *WorkspacePackageManager::createFiles(const QString &root, const QString &directory, QWidget *parent)
{
    return m_package ? m_package->createFiles(root, directory, parent) : nullptr;
}
