#include "NativeHostConnection.h"
#include "DockpipeChoices.h"
#include "NativeHostDockerAction.h"
#include "NativeHostDockerQuery.h"

#include <QDir>
#include <QFileInfo>

NativeHostConnection::NativeHostConnection(const QString &reference, QObject *parent)
    : QObject(parent), m_reference(reference), m_command(this)
{
    m_timer.setInterval(2000);
    connect(&m_timer, &QTimer::timeout, this, &NativeHostConnection::refresh);
    m_timer.start();
    QTimer::singleShot(0, this, &NativeHostConnection::refresh);
}

QString NativeHostConnection::reference() const { return m_reference; }
bool NativeHostConnection::available() const { return m_status.available; }
QString NativeHostConnection::error() const { return m_error; }
quint64 NativeHostConnection::revision() const { return m_revision; }
NativeHostStatus NativeHostConnection::status() const { return m_status; }

QString NativeHostConnection::label() const
{
    if (!available())
        return tr("Native host — unavailable");
    return tr("%1 · %2 · native host").arg(m_status.node, m_status.username);
}

QString NativeHostConnection::referenceError() const
{
    if (m_reference.size() > 4096 || m_reference.toUtf8().size() > 4096)
        return tr("The native host connection reference exceeds 4096 bytes.");
    for (const QChar character : m_reference) {
        if (character.category() == QChar::Other_Control)
            return tr("The native host connection reference contains invalid characters.");
    }
    const QFileInfo information(m_reference);
    if (!information.isAbsolute() || !information.isFile() || information.isSymLink())
        return tr("The native host connection requires an available absolute file reference.");
    return {};
}

NativeHostConnection::Plan NativeHostConnection::plan(Operation operation) const
{
    Plan result;
    result.error = referenceError();
    if (!result.error.isEmpty())
        return result;
    result.program = DockpipeChoices::operatorDockpipeBinary(&result.error);
    if (result.program.isEmpty())
        return result;
    QString name;
    switch (operation) {
    case Operation::Status:
        name = QStringLiteral("status");
        break;
    case Operation::Snapshot:
        name = QStringLiteral("snapshot");
        break;
    case Operation::Terminal:
        name = QStringLiteral("terminal");
        break;
    }
    result.arguments = {QStringLiteral("host"), name, QStringLiteral("--connection"), m_reference};
    if (operation != Operation::Terminal)
        result.arguments.append(QStringLiteral("--json"));
    result.workingDirectory = QDir::homePath();
    return result;
}

void NativeHostConnection::refresh()
{
    if (m_command.busy())
        return;
    const Plan launch = plan(Operation::Status);
    if (!launch.valid()) {
        update({}, launch.error);
        return;
    }
    m_command.runBounded(launch.program, launch.arguments, [this](bool ok, const QByteArray &output) {
        NativeHostStatus candidate;
        if (!ok || !NativeHostStatus::decode(output, &candidate) || !candidate.available) {
            update({}, tr("Native host unavailable. Open this application through its authenticated host session."));
            return;
        }
        update(candidate, {});
    }, 8000, 1024 * 1024);
}

NativeHostConnection::Plan NativeHostConnection::dockerPlan(const QString &action, const QString &id) const
{
    if (!NativeHostDockerAction::validRequest(action, id)) {
        Plan invalid;
        invalid.error = tr("Native host Docker actions require start or stop and a hexadecimal container ID.");
        return invalid;
    }
    Plan result = plan(Operation::Status);
    if (!result.valid())
        return result;
    result.arguments = {QStringLiteral("host"), QStringLiteral("docker"), action,
        QStringLiteral("--connection"), m_reference, QStringLiteral("--id"), id, QStringLiteral("--json")};
    return result;
}

NativeHostConnection::Plan NativeHostConnection::dockerQueryPlan(const QString &kind, const QString &id) const
{
    if (!NativeHostDockerQuery::validRequest(kind, id)) {
        Plan invalid;
        invalid.error = tr("Native host Docker inspection requires a valid container, network or volume identifier.");
        return invalid;
    }
    Plan result = plan(Operation::Status);
    if (!result.valid())
        return result;
    result.arguments = {QStringLiteral("host"), QStringLiteral("docker-query"),
        QStringLiteral("--connection"), m_reference, QStringLiteral("--kind"), kind,
        QStringLiteral("--id"), id, QStringLiteral("--json")};
    return result;
}

void NativeHostConnection::update(const NativeHostStatus &status, const QString &error)
{
    if (m_status.available == status.available && m_status.identity() == status.identity() && m_error == error)
        return;
    m_status = status;
    m_error = error;
    ++m_revision;
    emit statusChanged();
}
