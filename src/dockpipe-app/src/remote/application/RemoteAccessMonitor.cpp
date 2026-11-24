#include "RemoteAccessMonitor.h"

#include "DockpipeChoices.h"

#include <QGuiApplication>
#include <QJsonDocument>
#include <QJsonObject>

RemoteAccessMonitor::RemoteAccessMonitor(QObject *parent)
    : QObject(parent), m_command(this)
{
    // One status child at a time, with a 30-second deadline and RemoteCommand's
    // existing 20 MiB output budget. Stop periodic work with the owning window.
    m_pollTimer.setInterval(15000);
    connect(&m_pollTimer, &QTimer::timeout, this, [this]() {
        if (QGuiApplication::applicationState() == Qt::ApplicationActive)
            refresh();
    });
    if (auto *application = qobject_cast<QGuiApplication *>(QCoreApplication::instance())) {
        connect(application, &QGuiApplication::applicationStateChanged, this,
                [this](Qt::ApplicationState state) {
            if (state == Qt::ApplicationActive)
                refresh();
        });
    }
    connect(&m_command, &RemoteCommand::output, this, &RemoteAccessMonitor::appendDiagnostics);
    m_pollTimer.start();
    QTimer::singleShot(0, this, &RemoteAccessMonitor::refresh);
}

RemoteAccessMonitor::~RemoteAccessMonitor() = default;

RemoteAccessMonitor::State RemoteAccessMonitor::state() const
{
    return m_state;
}

QString RemoteAccessMonitor::detail() const
{
    QString summary;
    switch (m_state) {
    case State::Checking:
        summary = tr("Checking remote access security…");
        break;
    case State::Unlocked:
        summary = tr("Remote access is unlocked.");
        break;
    case State::Locked:
        summary = tr("Remote access is locked. Authenticate to continue.");
        break;
    case State::Unavailable:
        summary = tr("Remote access security is unavailable.");
        break;
    }
    if (!m_detail.isEmpty())
        summary += QLatin1Char('\n') + m_detail;
    if (!m_unlockError.isEmpty())
        summary += QLatin1Char('\n') + m_unlockError;
    return summary;
}

bool RemoteAccessMonitor::busy() const
{
    return m_busy;
}

void RemoteAccessMonitor::refresh()
{
    if (!m_busy)
        refreshStatus();
}

void RemoteAccessMonitor::refreshStatus(bool finishingUnlock, bool unlockSucceeded)
{
    QString error;
    const QString program = DockpipeChoices::operatorDockpipeBinary(&error);
    if (program.isEmpty()) {
        m_busy = false;
        m_unlocking = false;
        setUnavailable(error);
        emit changed();
        if (finishingUnlock)
            emit unlockFinished(false);
        return;
    }

    m_busy = true;
    m_diagnostics.clear();
    emit changed();
    m_command.run(program,
                  {QStringLiteral("remote"), QStringLiteral("security-status"),
                   QStringLiteral("--auth"), QStringLiteral("never")},
                  [this, finishingUnlock, unlockSucceeded](bool succeeded, const QByteArray &output) {
        applyStatus(succeeded, output);
        m_busy = false;
        m_unlocking = false;
        emit changed();
        if (finishingUnlock)
            emit unlockFinished(unlockSucceeded && m_state == State::Unlocked);
    }, 30000);
}

void RemoteAccessMonitor::unlock()
{
    if (m_busy)
        return;

    QString error;
    const QString program = DockpipeChoices::operatorDockpipeBinary(&error);
    m_unlockError.clear();
    if (program.isEmpty()) {
        setUnavailable(error);
        emit changed();
        emit unlockFinished(false);
        return;
    }

    m_busy = true;
    m_unlocking = true;
    m_diagnostics.clear();
    emit changed();
    m_command.run(program,
                  {QStringLiteral("remote"), QStringLiteral("unlock"),
                   QStringLiteral("--auth"), QStringLiteral("auto")},
                  [this](bool succeeded, const QByteArray &) {
        m_unlocking = false;
        if (!succeeded)
            m_unlockError = tr("Unlock was not completed. %1")
                .arg(QString::fromUtf8(m_diagnostics).trimmed()).trimmed();
        // Always reconcile authority state, including a dismissed native prompt.
        // Authentication is never retried automatically.
        refreshStatus(true, succeeded);
    }, 150000);
}

void RemoteAccessMonitor::cancelUnlock()
{
    if (m_unlocking && m_command.busy())
        m_command.cancel();
}

void RemoteAccessMonitor::applyStatus(bool succeeded, const QByteArray &output)
{
    if (!succeeded) {
        setUnavailable(tr("Dockpipe could not read the remote access security status. %1")
            .arg(QString::fromUtf8(m_diagnostics).trimmed()).trimmed());
        return;
    }

    QJsonParseError error;
    const QJsonDocument document = QJsonDocument::fromJson(output, &error);
    if (error.error != QJsonParseError::NoError || !document.isObject()) {
        setUnavailable(tr("Dockpipe returned an invalid security status."));
        return;
    }

    const QJsonObject status = document.object();
    if (!status.value(QStringLiteral("supported")).isBool()
        || !status.value(QStringLiteral("authority_available")).isBool()
        || !status.value(QStringLiteral("locked")).isBool()) {
        setUnavailable(tr("Dockpipe returned an incomplete security status."));
        return;
    }

    m_detail = status.value(QStringLiteral("detail")).toString();
    if (!status.value(QStringLiteral("authority_available")).toBool()) {
        setUnavailable(m_detail.isEmpty()
            ? tr("Start or update the local broker service, then check again.") : m_detail);
        return;
    }

    m_state = status.value(QStringLiteral("locked")).toBool() ? State::Locked : State::Unlocked;
    if (!status.value(QStringLiteral("supported")).toBool()) {
        if (!m_detail.isEmpty())
            m_detail += QLatin1Char('\n');
        m_detail += tr("Required native lock capabilities are unavailable; unlock will be refused.");
    }
    if (m_state == State::Unlocked)
        m_unlockError.clear();
}

void RemoteAccessMonitor::setUnavailable(const QString &detail)
{
    m_state = State::Unavailable;
    m_detail = detail;
}

void RemoteAccessMonitor::appendDiagnostics(const QString &text)
{
    // Match the Security tab's diagnostic budget. Mark any truncation visibly.
    constexpr qsizetype maximumDiagnostics = 64 * 1024;
    const QByteArray marker("\n[diagnostics truncated]");
    const QByteArray bytes = text.toUtf8();
    const qsizetype remaining = maximumDiagnostics - m_diagnostics.size();
    if (remaining <= 0) {
        if (!bytes.isEmpty() && !m_diagnostics.endsWith(marker)) {
            m_diagnostics.truncate(maximumDiagnostics - marker.size());
            m_diagnostics.append(marker);
        }
        return;
    }
    if (bytes.size() <= remaining) {
        m_diagnostics.append(bytes);
        return;
    }
    if (remaining >= marker.size()) {
        m_diagnostics.append(bytes.left(remaining - marker.size()));
        m_diagnostics.append(marker);
    } else {
        m_diagnostics.truncate(maximumDiagnostics - marker.size());
        m_diagnostics.append(marker);
    }
}
