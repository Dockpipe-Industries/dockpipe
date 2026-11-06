#include "TerminalHub.h"
#include "TerminalSession.h"
#include "TerminalShellCatalog.h"
#include "TerminalHostLaunch.h"
#include "NativeHostConnection.h"

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
    if (m_nativeHostConfigured) {
        TerminalShellProfile native;
        native.id = NativeHostConnection::selectionKey();
        native.label = m_nativeHost ? m_nativeHost->label() : tr("Native host — unavailable");
        profiles.append(native);
    }
    for (auto local : m_shellCatalog->profiles()) {
        if (m_nativeHostConfigured)
            local.label = tr("Application container · %1").arg(local.label);
        profiles.append(local);
    }
    return profiles;
}

void TerminalHub::setNativeHostConnection(NativeHostConnection *connection)
{
    if (m_nativeHost)
        disconnect(m_nativeHost, nullptr, this, nullptr);
    m_nativeHost = connection;
    m_nativeHostConfigured = true;
    if (connection) {
        connect(connection, &NativeHostConnection::statusChanged, this, &TerminalHub::nativeHostChanged);
        connect(connection, &QObject::destroyed, this, &TerminalHub::nativeHostChanged);
    }
    nativeHostChanged();
}

NativeHostConnection *TerminalHub::nativeHostConnection() const { return m_nativeHost; }
bool TerminalHub::nativeHostConfigured() const { return m_nativeHostConfigured; }

void TerminalHub::nativeHostChanged()
{
    const QStringList existing = m_order;
    for (const auto &id : existing) {
        if (m_sessions.value(id).native)
            closeSession(id);
    }
    emit shellProfilesChanged();
}

QString TerminalHub::createSession(const QString &workdir, const QString &shellProfileId)
{
    m_creationError.clear();
    if (m_sessions.size() >= sessionLimit) {
        m_creationError = tr("Eight terminals are already open. Close one before opening another.");
        return {};
    }
    TerminalShellProfile selected;
    QString directory = workdir.isEmpty() ? QDir::homePath() : workdir;
    const bool native = m_nativeHostConfigured && (shellProfileId.isEmpty()
        || shellProfileId == NativeHostConnection::selectionKey());
    const auto profiles = shellProfiles();
    if (native) {
        if (!m_nativeHost || !m_nativeHost->available()) {
            m_creationError = tr("Native host unavailable. Open this application through its authenticated host session.");
            return {};
        }
        const auto plan = m_nativeHost->plan(NativeHostConnection::Operation::Terminal);
        if (!plan.valid()) {
            m_creationError = plan.error;
            return {};
        }
        selected.program = plan.program;
        selected.arguments = plan.arguments;
        selected.label = m_nativeHost->label();
        directory = plan.workingDirectory;
    } else if (!shellProfileId.isEmpty()) {
        for (const auto &profile : profiles) {
            if (profile.id == shellProfileId) {
                selected = profile;
                break;
            }
        }
        if (selected.program.isEmpty()) {
            m_creationError = tr("This shell is no longer available. Refresh the shell list and try again.");
            return {};
        }
    } else if (TerminalHostLaunch::required(m_launchContext)) {
        if (profiles.isEmpty()) {
            m_creationError = m_shellCatalog->discovering()
                ? tr("Discovering installed host shells…") : tr("No host shells are available. Check Flatpak host access and refresh the shell list.");
            return {};
        }
        selected = profiles.first();
    }
#ifdef Q_OS_WIN
    if (!native && shellProfileId.isEmpty() && !profiles.isEmpty())
        selected = profiles.first();
#endif
    const QString id = QUuid::createUuid().toString(QUuid::WithoutBraces);
    auto *session = new TerminalSession(native ? TerminalLaunchContext::Local : m_launchContext, this);
    Entry entry;
    entry.session = session;
    entry.native = native;
    if (native)
        entry.title = tr("Terminal %1").arg(m_nextTitle++);
    else
        entry.title = tr("Terminal %1 · %2").arg(m_nextTitle++).arg(QDir::toNativeSeparators(directory));
    if (!selected.label.isEmpty())
        entry.title = selected.label + QStringLiteral(" · ") + entry.title;
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
    QStringList arguments = selected.arguments;
    if (!selected.workingDirectoryArgument.isEmpty()) {
        arguments.prepend(QDir::toNativeSeparators(directory));
        arguments.prepend(selected.workingDirectoryArgument);
    }
    session->start(native ? directory : workdir, selected.program, arguments, selected.environment);
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
