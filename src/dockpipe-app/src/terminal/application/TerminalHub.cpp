#include "TerminalHub.h"
#include "TerminalSession.h"
#include "TerminalShellCatalog.h"
#include "TerminalHostLaunch.h"
#include "SshMachineTransport.h"

#include <QFileInfo>
#include <QDir>
#include <QUuid>

namespace {
constexpr qsizetype outputLimit = 256 * 1024;
constexpr qsizetype inputLimit = 64 * 1024;
constexpr qsizetype renderChunkLimit = 32 * 1024;
constexpr int sessionLimit = 8;
}

TerminalHub::TerminalHub(QObject *parent)
    : TerminalHub(TerminalLaunchContext::Automatic, parent)
{
}

TerminalHub::TerminalHub(TerminalLaunchContext context, QObject *parent)
    : QObject(parent),
      m_shellCatalog(new TerminalShellCatalog(TerminalShellCatalog::systemDiscovery(context), this)),
      m_launchContext(context)
{
    connect(m_shellCatalog, &TerminalShellCatalog::profilesChanged, this, &TerminalHub::shellProfilesChanged);
    m_shellCatalog->refresh();
}

TerminalShellCatalog &TerminalHub::shellCatalog() const { return *m_shellCatalog; }
QString TerminalHub::creationError() const { return m_creationError; }

QVector<TerminalShellProfile> TerminalHub::shellProfiles() const
{
    QVector<TerminalShellProfile> profiles;
    if (m_hasSshMachineConnection) {
        TerminalShellProfile remote;
        remote.id = QStringLiteral("ssh-machine:") + m_sshMachineConnection.id;
        remote.label = tr("%1 · %2@%3 · SSH").arg(m_sshMachineConnection.label,
            m_sshMachineConnection.user, m_sshMachineConnection.host);
        profiles.append(remote);
    }
    QString localLabel = qEnvironmentVariable("DOCKPIPE_LOCAL_MACHINE_LABEL").trimmed().left(160);
    if (localLabel.isEmpty() && m_hasSshMachineConnection)
        localLabel = tr("This computer");
    for (auto profile : m_shellCatalog->profiles()) {
        if (!localLabel.isEmpty())
            profile.label = localLabel + QStringLiteral(" · ") + profile.label;
        profiles.append(profile);
    }
    return profiles;
}

void TerminalHub::setSshMachineConnection(const SshMachineConnection &connection, const QString &stateDirectory)
{
    m_hasSshMachineConnection = true;
    m_sshMachineConnection = connection;
    m_sshStateDirectory = stateDirectory;
    emit shellProfilesChanged();
}

void TerminalHub::clearSshMachineConnection()
{
    m_hasSshMachineConnection = false;
    m_sshMachineConnection = {};
    m_sshStateDirectory.clear();
    emit shellProfilesChanged();
}

bool TerminalHub::hasSshMachineConnection() const { return m_hasSshMachineConnection; }
SshMachineConnection TerminalHub::sshMachineConnection() const { return m_sshMachineConnection; }

TerminalHub::SessionLaunch TerminalHub::prepareSessionLaunch(const QString &workdir, const QString &shellProfileId) const
{
    SessionLaunch launch;
    launch.context = m_launchContext;
    TerminalShellProfile selected;
    QString directory = workdir.isEmpty() ? QDir::homePath() : workdir;
    const bool remote = m_hasSshMachineConnection && (shellProfileId.isEmpty()
        || shellProfileId == QStringLiteral("ssh-machine:") + m_sshMachineConnection.id);
    const auto profiles = shellProfiles();
    if (remote) {
        const auto plan = SshMachineTransport::interactive(m_sshMachineConnection, m_sshStateDirectory);
        if (!plan.valid()) {
            launch.error = plan.error;
            return launch;
        }
        selected.program = plan.program;
        selected.arguments = plan.arguments;
        selected.label = tr("%1 · %2@%3 · SSH").arg(m_sshMachineConnection.label,
            m_sshMachineConnection.user, m_sshMachineConnection.host);
        directory = plan.workingDirectory;
        launch.context = TerminalLaunchContext::Local;
        launch.remote = true;
    } else if (!shellProfileId.isEmpty()) {
        for (const auto &profile : profiles) {
            if (profile.id == shellProfileId) {
                selected = profile;
                break;
            }
        }
        if (selected.program.isEmpty()) {
            launch.error = tr("This shell is no longer available. Refresh the shell list and try again.");
            return launch;
        }
    } else if (TerminalHostLaunch::required(m_launchContext)) {
        if (profiles.isEmpty()) {
            launch.error = m_shellCatalog->discovering()
                ? tr("Discovering installed host shells…") : tr("No host shells are available. Check Flatpak host access and refresh the shell list.");
            return launch;
        }
        selected = profiles.first();
    }
#ifdef Q_OS_WIN
    if (!remote && shellProfileId.isEmpty() && !profiles.isEmpty())
        selected = profiles.first();
#endif
    launch.program = selected.program;
    launch.arguments = selected.arguments;
    launch.workingDirectory = remote ? directory : workdir;
    launch.environment = selected.environment;
    launch.label = selected.label;
    if (!selected.workingDirectoryArgument.isEmpty()) {
        launch.arguments.prepend(QDir::toNativeSeparators(directory));
        launch.arguments.prepend(selected.workingDirectoryArgument);
    }
    return launch;
}

QString TerminalHub::createSession(const QString &workdir, const QString &shellProfileId)
{
    m_creationError.clear();
    if (m_sessions.size() >= sessionLimit) {
        m_creationError = tr("Eight terminals are already open. Close one before opening another.");
        return {};
    }
    const SessionLaunch launch = prepareSessionLaunch(workdir, shellProfileId);
    if (!launch.valid()) {
        m_creationError = launch.error;
        return {};
    }
    const QString id = QUuid::createUuid().toString(QUuid::WithoutBraces);
    auto *session = new TerminalSession(launch.context, this);
    Entry entry;
    entry.session = session;
    const QString directory = launch.workingDirectory.isEmpty() ? QDir::homePath() : launch.workingDirectory;
    if (launch.remote)
        entry.title = tr("Terminal %1").arg(m_nextTitle++);
    else
        entry.title = tr("Terminal %1 · %2").arg(m_nextTitle++).arg(QDir::toNativeSeparators(directory));
    if (!launch.label.isEmpty())
        entry.title = launch.label + QStringLiteral(" · ") + entry.title;
    m_sessions.insert(id, entry);
    m_order.append(id);
    connect(session, &TerminalSession::output, this, [this, id](const QByteArray &bytes) {
        auto it = m_sessions.find(id);
        if (it == m_sessions.end())
            return;
        it->end += static_cast<quint64>(bytes.size());
        it->output.append(bytes);
        if (it->output.size() > outputLimit)
            it->output.remove(0, it->output.size() - outputLimit);
        emit outputAvailable(id);
    });
    connect(session, &TerminalSession::stateChanged, this, &TerminalHub::sessionsChanged);
    session->start(launch.workingDirectory, launch.program, launch.arguments, launch.environment);
    emit sessionsChanged();
    return id;
}

QStringList TerminalHub::sessionIds() const { return m_order; }
QString TerminalHub::title(const QString &id) const { return m_sessions.value(id).title; }

bool TerminalHub::running(const QString &id) const
{
    const auto it = m_sessions.constFind(id);
    return it != m_sessions.cend() && it->session->running();
}

QString TerminalHub::error(const QString &id) const
{
    const auto it = m_sessions.constFind(id);
    return it == m_sessions.cend() ? tr("This terminal session is closed.") : it->session->error();
}

bool TerminalHub::send(const QString &id, const QString &text, QString *error)
{
    const QByteArray bytes = text.toUtf8();
    QString failure;
    if (!running(id))
        failure = tr("Select a running terminal session.");
    else if (bytes.size() >= inputLimit)
        failure = tr("Input must be smaller than 64 KiB.");
    else if (!sendBytes(id, bytes + '\r'))
        failure = tr("The terminal could not accept input. Try again when it is ready.");
    if (error)
        *error = failure;
    return failure.isEmpty();
}

bool TerminalHub::sendBytes(const QString &id, const QByteArray &data)
{
    const auto it = m_sessions.constFind(id);
    return it != m_sessions.cend() && it->session->send(data);
}

void TerminalHub::resize(const QString &id, int columns, int rows)
{
    const auto it = m_sessions.constFind(id);
    if (it != m_sessions.cend())
        it->session->resize(columns, rows);
}

void TerminalHub::closeSession(const QString &id)
{
    auto it = m_sessions.find(id);
    if (it == m_sessions.end())
        return;
    auto *session = it->session;
    m_sessions.erase(it);
    m_order.removeAll(id);
    // Keep the backend alive until its asynchronous termination completes.
    connect(session, &TerminalSession::finished, session, &QObject::deleteLater);
    session->stop();
    if (!session->running())
        session->deleteLater();
    emit sessionsChanged();
}

QByteArray TerminalHub::readOutput(const QString &id, quint64 &cursor, bool &truncated) const
{
    truncated = false;
    const auto it = m_sessions.constFind(id);
    if (it == m_sessions.cend())
        return {};
    const quint64 begin = it->end - static_cast<quint64>(it->output.size());
    if (cursor < begin || cursor > it->end) {
        cursor = begin;
        truncated = true;
    }
    const auto offset = static_cast<qsizetype>(cursor - begin);
    const QByteArray result = it->output.mid(offset, renderChunkLimit);
    cursor += static_cast<quint64>(result.size());
    return result;
}
