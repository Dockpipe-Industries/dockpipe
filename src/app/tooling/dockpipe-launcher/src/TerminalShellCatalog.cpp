#include "TerminalShellCatalog.h"

#include <QCoreApplication>
#include <QCryptographicHash>
#include <QDir>
#include <QDirIterator>
#include <QFile>
#include <QFileInfo>
#include <QProcess>
#include <QPointer>
#include <QSet>
#include <QTimer>
#ifdef Q_OS_UNIX
#include <pwd.h>
#include <unistd.h>
#include <cstring>
#endif

namespace {
// Interactive menus admit at most 128 profiles. Discovery examines at most 256
// search/version entries and 64 KiB of process or configuration output; exhaustion
// is reported rather than presenting an apparently complete truncated catalog.
constexpr int MaximumProfiles = 128;
constexpr int MaximumEntries = 256;
constexpr int MaximumOutputBytes = 64 * 1024;
constexpr int ProbeTimeoutMilliseconds = 3000;

QString profileId(const QString &program, const QStringList &arguments, bool windows,
                  const QString &canonical = {})
{
    const QString target = canonical.isEmpty() ? program : canonical;
    QByteArray identity = (windows ? target.toCaseFolded() : target).toUtf8();
    identity.append('\0');
    const QString invocation = QFileInfo(program).fileName();
    identity.append((windows ? invocation.toCaseFolded() : invocation).toUtf8());
    for (const QString &argument : arguments) {
        identity.append('\0');
        identity.append(argument.toUtf8());
    }
    return QString::fromLatin1(QCryptographicHash::hash(identity, QCryptographicHash::Sha256).toHex());
}

QString shellLabel(const QString &path)
{
    const QString name = QFileInfo(path).fileName();
    if (name == "pwsh")
        return QStringLiteral("PowerShell");
    return name;
}

bool validPath(const QString &path)
{
    return QDir::isAbsolutePath(path) && !path.contains(QChar::Null)
        && !path.contains('\n') && !path.contains('\r');
}

QMap<QString, QString> gitEnvironment(const QString &root)
{
    QMap<QString, QString> environment{{"CHERE_INVOKING", "1"}};
    for (const QString &runtime : {QString("clangarm64"), QString("mingw64"), QString("mingw32")}) {
        if (QFileInfo(QDir(root).filePath(runtime)).isDir()) {
            environment.insert("MSYSTEM", runtime.toUpper());
            break;
        }
    }
    return environment;
}

QString decodeWsl(const QByteArray &bytes)
{
    // Windows wsl.exe emits UTF-16LE when redirected. Some versions emit UTF-8.
    if (bytes.startsWith("\xff\xfe") || bytes.contains('\0')) {
        if (bytes.size() % 2)
            return {};
        QString text;
        text.reserve(bytes.size() / 2);
        int offset = bytes.startsWith("\xff\xfe") ? 2 : 0;
        for (; offset < bytes.size(); offset += 2) {
            const auto low = static_cast<unsigned char>(bytes[offset]);
            const auto high = static_cast<unsigned char>(bytes[offset + 1]);
            text.append(QChar(low | (high << 8)));
        }
        return text;
    }
    return QString::fromUtf8(bytes);
}

const QStringList UnixShellNames{QStringLiteral("bash"), QStringLiteral("zsh"),
    QStringLiteral("fish"), QStringLiteral("sh"), QStringLiteral("ksh"),
    QStringLiteral("tcsh"), QStringLiteral("pwsh")};

// No discovered string is inserted into this program. Paths remain quoted data.
// Listing and executable checks do not start any of the discovered shells.
const QString HostDiscoveryScript = QStringLiteral(
    "LC_ALL=C; export LC_ALL; "
    "emit() { case \"$1\" in /*) [ -f \"$1\" ] && [ -x \"$1\" ] || return; "
    "p=$(readlink -f -- \"$1\") || return; printf '%s\\0%s\\0' \"$1\" \"$p\";; esac; }; "
    "emit \"$SHELL\"; "
    "if [ -r /etc/shells ]; then data=$(head -c 65537 /etc/shells && printf x) || exit 23; data=${data%x}; "
    "[ \"${#data}\" -le 65536 ] || exit 23; "
    "if ! printf '%s\\n' \"$data\" | (n=0; while IFS= read -r p; do "
    "n=$((n+1)); [ \"$n\" -le 256 ] || exit 23; "
    "case \"$p\" in ''|\\#*) continue;; esac; emit \"$p\"; done); then exit 23; fi; fi; "
    "for d in /bin /usr/bin /usr/local/bin /opt/homebrew/bin; do "
    "for s in bash zsh fish sh ksh tcsh pwsh; do emit \"$d/$s\"; done; done; exit 0");
}

TerminalShellCatalog::TerminalShellCatalog(QObject *parent)
    : TerminalShellCatalog(systemDiscovery(), parent)
{
}

TerminalShellCatalog::TerminalShellCatalog(const Discovery &discovery, QObject *parent)
    : QObject(parent), m_discovery(discovery), m_timeout(new QTimer(this))
{
    m_timeout->setSingleShot(true);
    connect(m_timeout, &QTimer::timeout, this, [this] {
        m_error = tr("Shell discovery timed out.");
        if (m_process)
            m_process->kill();
    });
}

TerminalShellCatalog::~TerminalShellCatalog()
{
    if (!m_process)
        return;
    // QProcess's destructor waits for a running child. Let the application reap
    // this killed listing process asynchronously instead of blocking a UI close.
    QProcess *process = m_process;
    process->disconnect(this);
    process->setParent(QCoreApplication::instance());
    connect(process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), process, &QObject::deleteLater);
    process->kill();
    if (process->state() == QProcess::NotRunning)
        process->deleteLater();
}

TerminalShellCatalog::Discovery TerminalShellCatalog::systemDiscovery()
{
    Discovery discovery;
#ifdef Q_OS_WIN
    discovery.windows = true;
    discovery.windowsDirectory = qEnvironmentVariable("SystemRoot");
    for (const char *key : {"ProgramFiles", "ProgramFiles(x86)", "ProgramW6432"}) {
        const QString directory = qEnvironmentVariable(key);
        if (!validPath(directory))
            continue;
        discovery.powerShellRoots.append(QDir(directory).filePath("PowerShell"));
        discovery.gitRoots.append(QDir(directory).filePath("Git"));
    }
    const QString local = qEnvironmentVariable("LOCALAPPDATA");
    if (validPath(local)) {
        discovery.powerShellRoots.append(QDir(local).filePath("Microsoft/PowerShell"));
        discovery.gitRoots.append(QDir(local).filePath("Programs/Git"));
    }
    discovery.searchDirectories = qEnvironmentVariable("PATH").split(';', Qt::SkipEmptyParts);
    discovery.wslProgram = QDir(discovery.windowsDirectory).filePath("System32/wsl.exe");
#else
    discovery.flatpak = !qEnvironmentVariableIsEmpty("FLATPAK_ID") || QFileInfo::exists("/.flatpak-info");
    discovery.flatpakSpawnProgram = QStringLiteral("/usr/bin/flatpak-spawn");
    discovery.loginShell = qEnvironmentVariable("SHELL");
    if (!validPath(discovery.loginShell) || !QFileInfo(discovery.loginShell).isExecutable()) {
        if (const passwd *account = getpwuid(getuid())) {
            if (account->pw_shell && strnlen(account->pw_shell, 4097) <= 4096)
                discovery.loginShell = QString::fromLocal8Bit(account->pw_shell);
        }
    }
    discovery.shellsFile = QStringLiteral("/etc/shells");
    discovery.searchDirectories = {QStringLiteral("/bin"), QStringLiteral("/usr/bin"),
        QStringLiteral("/usr/local/bin"), QStringLiteral("/opt/homebrew/bin")};
#endif
    discovery.powerShellRoots.removeDuplicates();
    discovery.gitRoots.removeDuplicates();
    discovery.searchDirectories.removeDuplicates();
    return discovery;
}

QVector<TerminalShellProfile> TerminalShellCatalog::profiles() const
{
    return m_profiles;
}

QString TerminalShellCatalog::error() const
{
    return m_error;
}

bool TerminalShellCatalog::discovering() const
{
    return m_discovering;
}

void TerminalShellCatalog::addExecutable(const QString &path, const QString &label, const QStringList &arguments,
                                         const QMap<QString, QString> &environment)
{
    if (!validPath(path))
        return;
    const QFileInfo file(path);
    if (!file.isFile() || (!m_discovery.windows && !file.isExecutable()))
        return;
    // Preserve argv[0] semantics: /bin/sh may be a link to Bash or Dash.
    // Aliases with the same invoked basename still share a canonical identity.
    const QString program = QDir::cleanPath(file.absoluteFilePath());
    const QString id = profileId(program, arguments, m_discovery.windows, file.canonicalFilePath());
    for (const TerminalShellProfile &profile : m_profiles) {
        if (profile.id == id)
            return;
    }
    if (m_profiles.size() >= MaximumProfiles) {
        m_error = tr("Shell discovery exceeded the 128-profile menu limit.");
        return;
    }
    m_profiles.append({id, label, program, arguments, {}, environment});
}

void TerminalShellCatalog::discoverLocal()
{
    const int entries = m_discovery.searchDirectories.size() + m_discovery.powerShellRoots.size()
        + m_discovery.gitRoots.size();
    if (entries > MaximumEntries) {
        m_error = tr("Shell discovery exceeded the 256-location limit.");
        return;
    }
    if (m_discovery.windows) {
        addExecutable(QDir(m_discovery.windowsDirectory).filePath("System32/WindowsPowerShell/v1.0/powershell.exe"),
            tr("Windows PowerShell"), {"-NoLogo"});
        int visited = entries;
        for (const QString &root : m_discovery.powerShellRoots) {
            if (!validPath(root))
                continue;
            addExecutable(QDir(root).filePath("pwsh.exe"), tr("PowerShell"), {"-NoLogo"});
            QDirIterator versions(root, QDir::Dirs | QDir::NoDotAndDotDot, QDirIterator::NoIteratorFlags);
            while (versions.hasNext()) {
                if (++visited > MaximumEntries) {
                    m_error = tr("Shell discovery exceeded the 256-location limit.");
                    return;
                }
                const QString directory = versions.next();
                addExecutable(QDir(directory).filePath("pwsh.exe"),
                    tr("PowerShell %1").arg(QFileInfo(directory).fileName()), {"-NoLogo"});
            }
        }
        for (const QString &root : m_discovery.gitRoots) {
            addExecutable(QDir(root).filePath("bin/bash.exe"), tr("Git Bash"), {"--login", "-i"}, gitEnvironment(root));
        }
        for (const QString &directory : m_discovery.searchDirectories) {
            addExecutable(QDir(directory).filePath("pwsh.exe"), tr("PowerShell (%1)").arg(directory), {"-NoLogo"});
            // Git's cmd directory is commonly on PATH; its Bash lives beside it.
            if (QFileInfo(QDir(directory).filePath("git.exe")).isFile())
                addExecutable(QDir(directory).filePath("../bin/bash.exe"), tr("Git Bash"), {"--login", "-i"},
                    gitEnvironment(QDir(directory).filePath("..")));
        }
        return;
    }
    addExecutable(m_discovery.loginShell, shellLabel(m_discovery.loginShell));
    QFile shells(m_discovery.shellsFile);
    if (shells.open(QIODevice::ReadOnly)) {
        const QByteArray bytes = shells.read(MaximumOutputBytes + 1);
        if (bytes.size() > MaximumOutputBytes) {
            m_error = tr("The installed-shell list exceeds 64 KiB.");
            return;
        }
        const auto lines = QString::fromUtf8(bytes).split('\n');
        if (lines.size() > MaximumEntries) {
            m_error = tr("The installed-shell list exceeds 256 entries.");
            return;
        }
        for (const QString &line : lines) {
            const QString path = line.trimmed();
            if (!path.startsWith('#'))
                addExecutable(path, shellLabel(path));
        }
    }
    for (const QString &directory : m_discovery.searchDirectories) {
        for (const QString &name : UnixShellNames)
            addExecutable(QDir(directory).filePath(name), shellLabel(name));
    }
}

void TerminalShellCatalog::refresh()
{
    if (m_discovering)
        return;
    m_discovering = true;
    m_cancelled = false;
    m_error.clear();
    m_output.clear();
    m_diagnosticBytes = 0;
    m_profiles.clear();
    QPointer<TerminalShellCatalog> guard(this);
    if (m_discovery.flatpak) {
        emit profilesChanged();
        if (!guard)
            return;
        if (m_cancelled) {
            m_error = tr("Shell discovery was cancelled.");
            publishFinished();
            return;
        }
        startProbe(m_discovery.flatpakSpawnProgram, {"--watch-bus", "--host", "/bin/sh", "-c", HostDiscoveryScript});
        return;
    }
    discoverLocal();
    emit profilesChanged();
    if (!guard)
        return;
    if (m_cancelled)
        m_error = tr("Shell discovery was cancelled.");
    if (m_error.isEmpty() && m_discovery.windows && QFileInfo(m_discovery.wslProgram).isFile()) {
        startProbe(m_discovery.wslProgram, {"--list", "--quiet"});
        return;
    }
    publishFinished();
}

void TerminalShellCatalog::startProbe(const QString &program, const QStringList &arguments)
{
    m_process = new QProcess(this);
    m_process->setProcessChannelMode(QProcess::SeparateChannels);
    connect(m_process, &QProcess::readyReadStandardOutput, this, &TerminalShellCatalog::readProbeOutput);
    connect(m_process, &QProcess::readyReadStandardError, this, [this] {
        // Diagnostics are not displayed as profiles or retained indefinitely.
        m_diagnosticBytes += m_process->readAllStandardError().size();
        if (m_diagnosticBytes > MaximumOutputBytes) {
            m_error = tr("Shell discovery diagnostics exceeded 64 KiB.");
            m_process->kill();
        }
    });
    connect(m_process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this, &TerminalShellCatalog::finishProbe);
    connect(m_process, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart) {
            m_error = tr("Could not start installed-shell discovery.");
            finishProbe();
        }
    });
    m_timeout->start(ProbeTimeoutMilliseconds);
    m_process->start(program, arguments);
}

void TerminalShellCatalog::readProbeOutput()
{
    const QByteArray bytes = m_process->readAllStandardOutput();
    if (bytes.size() > MaximumOutputBytes - m_output.size()) {
        m_error = tr("Shell discovery output exceeded 64 KiB.");
        m_process->kill();
        return;
    }
    m_output.append(bytes);
}

void TerminalShellCatalog::finishProbe()
{
    if (!m_process)
        return;
    readProbeOutput();
    m_timeout->stop();
    if (m_cancelled)
        m_error = tr("Shell discovery was cancelled.");
    else if (m_error.isEmpty() && (m_process->exitStatus() != QProcess::NormalExit || m_process->exitCode() != 0))
        m_error = tr("Installed-shell discovery failed.");
    m_process->deleteLater();
    m_process = nullptr;
    if (m_error.isEmpty() && m_discovery.flatpak && !m_output.isEmpty() && !m_output.endsWith('\0'))
        m_error = tr("Host shell discovery returned an incomplete profile list.");
    if (m_error.isEmpty()) {
        QVector<TerminalShellProfile> discovered;
        QStringList names;
        if (m_discovery.flatpak) {
            names = QString::fromUtf8(m_output).split(QChar::Null);
            names.removeLast(); // Required trailing NUL, including an empty list.
        } else {
            names = decodeWsl(m_output).split('\n', Qt::SkipEmptyParts);
        }
        const int fieldsPerProfile = m_discovery.flatpak ? 2 : 1;
        if (names.size() % fieldsPerProfile || names.size() > MaximumProfiles * fieldsPerProfile) {
            m_error = tr("Shell discovery returned an invalid or oversized profile list.");
        } else {
            QSet<QString> ids;
            for (int index = 0; index < names.size(); index += fieldsPerProfile) {
                const QString name = m_discovery.flatpak ? names[index] : names[index].trimmed();
                const QString canonical = m_discovery.flatpak ? names[index + 1] : QString();
                if (name.isEmpty() || name.size() > 4096 || name.contains(QChar::Null)
                    || name.contains('\r') || name.contains('\n')
                    || (m_discovery.flatpak && (!validPath(name) || !validPath(canonical)))) {
                    m_error = tr("Shell discovery returned an invalid profile.");
                    break;
                }
                const QString program = m_discovery.flatpak ? name : m_discovery.wslProgram;
                const QStringList arguments = m_discovery.flatpak ? QStringList{}
                    : QStringList{"--distribution", name, "--exec", "bash", "-li"};
                const QString id = profileId(program, arguments, m_discovery.windows, canonical);
                if (ids.contains(id))
                    continue;
                ids.insert(id);
                const QString label = m_discovery.flatpak ? shellLabel(name) : tr("WSL Bash · %1").arg(name);
                discovered.append({id, label, program, arguments, m_discovery.flatpak ? QString() : QString("--cd")});
            }
            if (m_profiles.size() + discovered.size() > MaximumProfiles)
                m_error = tr("Shell discovery exceeded the 128-profile menu limit.");
            if (m_error.isEmpty())
                m_profiles += discovered;
        }
    }
    QPointer<TerminalShellCatalog> guard(this);
    emit profilesChanged();
    if (guard)
        publishFinished();
}

void TerminalShellCatalog::publishFinished()
{
    m_discovering = false;
    emit discoveryFinished();
}

void TerminalShellCatalog::cancel()
{
    if (!m_discovering)
        return;
    m_cancelled = true;
    if (m_process)
        m_process->kill();
}
