#include "TerminalHub.h"
#include "ManagedTerminalSession.h"
#include "DockpipeChoices.h"
#include "NativeTerminalLaunch.h"
#include "RemoteTerminalSession.h"
#include "TerminalShellCatalog.h"
#include "TerminalHostLaunch.h"

#include <QFileInfo>
#include <QDir>
#include <QUuid>
#include <QPointer>
#include <QRegularExpression>

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
    m_shellCatalog->refresh();
}

TerminalShellCatalog &TerminalHub::shellCatalog() const { return *m_shellCatalog; }
QString TerminalHub::creationError() const { return m_creationError; }

QString TerminalHub::createSession(const QString &workdir, const QString &shellProfileId)
{
    return createSession(workdir, shellProfileId, {}, {});
}

QString TerminalHub::createSession(const QString &workdir, const QString &shellProfileId,
                                   const QJsonObject &catalog, const QString &root)
{
    m_creationError.clear();
    if (m_sessions.size() + m_closingSessions.size() >= sessionLimit) {
        m_creationError = tr("Eight terminals are already open. Close one before opening another.");
        return {};
    }
    if (!catalog.isEmpty())
        return createRemoteSession(catalog, root, {}, {});
    TerminalShellProfile selected;
    const auto profiles = m_shellCatalog->profiles();
    if (!shellProfileId.isEmpty()) {
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
    if (shellProfileId.isEmpty() && !profiles.isEmpty())
        selected = profiles.first();
#endif
    const QString directory = workdir.isEmpty() ? QDir::homePath() : workdir;
    auto title = tr("Terminal %1 · %2").arg(m_nextTitle++).arg(QDir::toNativeSeparators(directory));
    if (!selected.label.isEmpty())
        title = selected.label + QStringLiteral(" · ") + title;
    QStringList arguments = selected.arguments;
    if (!selected.workingDirectoryArgument.isEmpty()) {
        arguments.prepend(QDir::toNativeSeparators(directory));
        arguments.prepend(selected.workingDirectoryArgument);
    }
    const auto profile = NativeTerminalLaunch::resolve(workdir, selected.program, arguments, selected.environment,
        m_launchContext, selected.id.isEmpty() ? QStringLiteral("native-default") : selected.id);
    return createLocalSession(profile, title, {});
}

QString TerminalHub::createLocalContainerSession(const QString &container, const QString &name, const QString &shell)
{
    m_creationError.clear();
    if (m_sessions.size() + m_closingSessions.size() >= sessionLimit) {
        m_creationError = tr("Eight terminals are already open. Close one before opening another.");
        return {};
    }
    static const QRegularExpression fullId(QStringLiteral("\\A[0-9a-f]{64}\\z"));
    if (!fullId.match(container).hasMatch() || (shell != "/bin/sh" && shell != "/bin/bash")) {
        m_creationError = tr("The container terminal target is unavailable.");
        return {};
    }
    const auto binary = DockpipeChoices::operatorDockpipeBinary(&m_creationError);
    if (binary.isEmpty())
        return {};

    const auto profile = NativeTerminalLaunch::resolve({}, binary,
        {"containers", "exec", "--container", container, "--shell", shell}, {},
        TerminalLaunchContext::Local, QStringLiteral("container-terminal"));
    const auto title = tr("Terminal %1 · %2").arg(m_nextTitle++).arg(name);
    return createLocalSession(profile, title, "container:" + container + ":" + shell);
}

QString TerminalHub::createLocalSession(const NativeTerminalLaunch::Snapshot &profile, const QString &title, const QString &root)
{
    const QString id = QUuid::createUuid().toString(QUuid::WithoutBraces);
    auto *session = new ManagedTerminalSession(this);
    Entry entry;
    entry.session = session;
    entry.title = title;
    entry.root = root;
    m_sessions.insert(id, entry);
    m_order.append(id);
    connect(session, &ManagedTerminalSession::output, this, [this, id](const QByteArray &bytes, quint64 cursor, bool truncated) {
        auto it = m_sessions.find(id);
        if (it == m_sessions.end())
            return;
        if (truncated || cursor - quint64(bytes.size()) != it->end)
            it->output.clear();
        it->end = cursor;
        it->output.append(bytes);
        if (it->output.size() > outputLimit)
            it->output.remove(0, it->output.size() - outputLimit);
        emit outputAvailable(id);
    });
    connect(session, &ManagedTerminalSession::stateChanged, this, &TerminalHub::sessionsChanged);
    if (!session->start(profile)) {
        m_creationError = session->error();
        m_sessions.remove(id);
        m_order.removeAll(id);
        session->deleteLater();
        return {};
    }
    emit sessionsChanged();
    return id;
}

QString TerminalHub::createContainerSession(const QJsonObject &catalog, const QString &container, const QString &shell)
{
    m_creationError.clear();
    if (catalog.isEmpty() || m_sessions.size() + m_closingSessions.size() >= sessionLimit) {
        m_creationError = tr("Eight terminals are already open. Close one before opening another.");
        return {};
    }
    return createRemoteSession(catalog, "container:" + container + ":" + shell, container, shell);
}

QString TerminalHub::createRemoteSession(const QJsonObject &catalog, const QString &root,
                                       const QString &container, const QString &shell)
{
    const QString id = QUuid::createUuid().toString(QUuid::WithoutBraces);
    auto *session = new RemoteTerminalSession(this);
    Entry entry;
    entry.remote = session;
    entry.catalog = catalog;
    entry.root = root;
    entry.title = tr("Terminal %1 · %2").arg(m_nextTitle++)
        .arg(catalog.value("target").toObject().value("node").toString() + QStringLiteral(" · ") + root);
    m_sessions.insert(id, entry);
    m_order.append(id);
    connect(session, &RemoteTerminalSession::output, this,
        [this, id](const QByteArray &bytes, quint64 cursor, bool truncated) {
            auto it = m_sessions.find(id);
            if (it == m_sessions.end())
                return;
            if (truncated || cursor - quint64(bytes.size()) != it->end)
                it->output.clear();
            it->end = cursor;
            it->output.append(bytes);
            if (it->output.size() > outputLimit)
                it->output.remove(0, it->output.size() - outputLimit);
            emit outputAvailable(id);
        });
    connect(session, &RemoteTerminalSession::stateChanged, this, &TerminalHub::sessionsChanged);
    const bool started = container.isEmpty() ? session->start(catalog, root) : session->startContainer(catalog, container, shell);
    if (!started) {
        m_creationError = session->error();
        m_sessions.remove(id);
        m_order.removeAll(id);
        session->deleteLater();
        return {};
    }
    emit sessionsChanged();
    return id;
}

QStringList TerminalHub::sessionIds() const { return m_order; }

QStringList TerminalHub::sessionIds(const QJsonObject &catalog, const QString &root) const
{
    QStringList result;
    for (const auto &id : m_order) {
        const auto &entry = m_sessions[id];
        if (catalog.isEmpty()) {
            if (entry.catalog.isEmpty() && entry.root == root)
                result.append(id);
        } else if (entry.catalog.value("target") == catalog.value("target")
                   && entry.root == root) {
            result.append(id);
        }
    }
    return result;
}

bool TerminalHub::connecting(const QString &id) const
{
    const auto entry = m_sessions.constFind(id);
    return entry != m_sessions.cend() && entry->remote && entry->remote->running() && !entry->remote->ready();
}
QString TerminalHub::title(const QString &id) const { return m_sessions.value(id).title; }

bool TerminalHub::running(const QString &id) const
{
    const auto it = m_sessions.constFind(id);
    return it != m_sessions.cend() && (it->remote ? it->remote->running() : it->session->running());
}

QString TerminalHub::error(const QString &id) const
{
    const auto it = m_sessions.constFind(id);
    if (it == m_sessions.cend())
        return tr("This terminal session is closed.");
    if (it->remote)
        return it->remote->error();
    return it->session->error();
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
    return it != m_sessions.cend() && (it->remote ? it->remote->send(data) : it->session->send(data));
}

void TerminalHub::sendAsync(const QString &id, const QString &text,
                            std::function<void(bool, const QString &)> completion)
{
    const auto entry = m_sessions.constFind(id);
    if (entry == m_sessions.cend()) {
        completion(false, tr("This terminal session is closed."));
        return;
    }
    const QByteArray data = text.toUtf8();
    if (data.size() >= inputLimit) {
        completion(false, tr("Input must be smaller than 64 KiB."));
        return;
    }
    const QPointer<TerminalHub> guard(this);
    auto finished = [guard, id, completion](bool accepted) {
        if (guard)
            completion(accepted, accepted ? QString() : guard->error(id));
    };
    const bool accepted = entry->remote
        ? entry->remote->sendAcknowledged(data + '\r', std::move(finished))
        : entry->session->sendAcknowledged(data + '\r', std::move(finished));
    if (!accepted)
        completion(false, tr("The terminal could not accept input. Try again when it is ready."));
}

void TerminalHub::resize(const QString &id, int columns, int rows)
{
    const auto it = m_sessions.constFind(id);
    if (it != m_sessions.cend()) {
        if (it->remote)
            it->remote->resize(columns, rows);
        else
            it->session->resize(columns, rows);
    }
}

void TerminalHub::closeSession(const QString &id)
{
    auto it = m_sessions.find(id);
    if (it == m_sessions.end())
        return;
    if (it->remote) {
        auto *session = it->remote;
        m_sessions.erase(it);
        m_order.removeAll(id);
        if (!session->cleanupVerified()) {
            m_closingSessions.insert(session);
            connect(session, &RemoteTerminalSession::finished, this, [this, session] {
                if (session->cleanupVerified()) {
                    m_closingSessions.remove(session);
                    session->deleteLater();
                }
                emit sessionsChanged();
            });
        }
        session->stop();
        if (session->cleanupVerified()) {
            m_closingSessions.remove(session);
            session->deleteLater();
        }
        emit sessionsChanged();
        return;
    }
    auto *session = it->session;
    m_sessions.erase(it);
    m_order.removeAll(id);
    // A vanished core process is not proof of native cleanup. Preserve an
    // unknown reservation rather than opening more shells over unreaped jobs.
    if (session->active()) {
        m_closingSessions.insert(session);
        connect(session, &ManagedTerminalSession::finished, this, [this, session] {
            if (session->cleanupVerified()) {
                m_closingSessions.remove(session);
                session->deleteLater();
            }
            emit sessionsChanged();
        });
        session->stop();
    } else if (session->cleanupVerified()) {
        session->deleteLater();
    } else {
        m_closingSessions.insert(session);
    }
    emit sessionsChanged();
}

void TerminalHub::releaseRecoveredSession(const QJsonObject &reference)
{
    for (auto &entry : m_sessions) {
        if (entry.remote)
            entry.remote->confirmRecovery(reference);
    }
    const auto closing = m_closingSessions;
    for (auto *object : closing) {
        auto *session = qobject_cast<RemoteTerminalSession *>(object);
        if (session && session->confirmRecovery(reference)) {
            m_closingSessions.remove(session);
            if (!session->active())
                session->deleteLater();
        }
    }
    emit sessionsChanged();
}

void TerminalHub::releaseRecoveredContainerSession(const QJsonObject &reference)
{
    releaseRecoveredSession(reference);
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
