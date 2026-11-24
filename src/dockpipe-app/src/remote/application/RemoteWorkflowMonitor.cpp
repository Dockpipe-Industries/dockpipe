#include "RemoteWorkflowMonitor.h"

#include "DockpipeChoices.h"

#include <QGuiApplication>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>

RemoteWorkflowMonitor::RemoteWorkflowMonitor(QObject *parent)
    : QObject(parent), m_command(this)
{
    m_pollTimer.setInterval(5000);
    connect(&m_pollTimer, &QTimer::timeout, this, &RemoteWorkflowMonitor::refresh);
    if (auto *application = qobject_cast<QGuiApplication *>(QCoreApplication::instance())) {
        connect(application, &QGuiApplication::applicationStateChanged, this,
                [this](Qt::ApplicationState state) {
            updatePolling();
            if (state == Qt::ApplicationActive)
                refresh();
        });
    }
}

RemoteWorkflowMonitor::~RemoteWorkflowMonitor() = default;

bool RemoteWorkflowMonitor::track(const QString &jobId, const QString &title)
{
    if (jobId.trimmed().isEmpty())
        return false;

    if (m_pending.contains(jobId))
        return true;
    constexpr qsizetype maximumPendingJobs = 128;
    if (m_pending.size() >= maximumPendingJobs)
        return false;

    m_pending.insert(jobId, title);
    updatePolling();
    refresh();
    return true;
}

void RemoteWorkflowMonitor::setAccessUnlocked(bool unlocked)
{
    if (m_accessUnlocked == unlocked)
        return;
    m_accessUnlocked = unlocked;
    ++m_accessGeneration;
    if (!unlocked)
        m_command.cancel();
    updatePolling();
    if (unlocked)
        refresh();
}

void RemoteWorkflowMonitor::refresh()
{
    updatePolling();
    if (!m_accessUnlocked || m_pending.isEmpty() || m_command.busy())
        return;

    const QString program = DockpipeChoices::operatorDockpipeBinary();
    if (program.isEmpty())
        return;

    const quint64 accessGeneration = m_accessGeneration;
    m_command.run(program,
                  {QStringLiteral("remote"), QStringLiteral("jobs"),
                   QStringLiteral("--auth"), QStringLiteral("never")},
                  [this, accessGeneration](bool succeeded, const QByteArray &output) {
        if (!succeeded || !m_accessUnlocked || accessGeneration != m_accessGeneration)
            return;

        QJsonParseError error;
        const QJsonDocument document = QJsonDocument::fromJson(output, &error);
        if (error.error != QJsonParseError::NoError || !document.isArray())
            return;

        // Queued/running/unknown and missing jobs remain pending. A disconnected
        // worker may have executed unknown work; never present that as failure.
        for (const QJsonValue &value : document.array()) {
            const QJsonObject job = value.toObject();
            const QString id = job.value(QStringLiteral("id")).toString();
            auto pending = m_pending.find(id);
            if (pending == m_pending.end())
                continue;

            const QString status = job.value(QStringLiteral("status")).toString();
            QString outcome;
            if (status == QStringLiteral("success"))
                outcome = QStringLiteral("completed");
            else if (status == QStringLiteral("failure"))
                outcome = QStringLiteral("failed");
            else if (status == QStringLiteral("cancelled"))
                outcome = QStringLiteral("cancelled");
            else
                continue;

            const QString title = pending.value();
            m_pending.erase(pending);
            emit finished(id, title, outcome);
        }
        updatePolling();
    }, 30000);
}

void RemoteWorkflowMonitor::updatePolling()
{
    if (m_accessUnlocked && !m_pending.isEmpty()
        && QGuiApplication::applicationState() == Qt::ApplicationActive) {
        if (!m_pollTimer.isActive())
            m_pollTimer.start();
    } else {
        m_pollTimer.stop();
    }
}
