#include "ProjectGitClient.h"
#include "DockpipeChoices.h"
#include <QJsonDocument>

ProjectGitClient::ProjectGitClient(QObject *parent) : QObject(parent), m_command(this)
{
    connect(&m_command, &RemoteCommand::busyChanged, this, &ProjectGitClient::busyChanged);
    connect(&m_command, &RemoteCommand::output, this, [this](const QString &text) {
        if (m_diagnostics.size() < 8192)
            m_diagnostics += text.left(8192 - m_diagnostics.size());
    });
}

void ProjectGitClient::run(const QString &workdir, QJsonObject request, Completion completion)
{
    if (busy()) {
        completion(false, {}, tr("A Git operation is already running."));
        return;
    }
    request["workdir"] = workdir;
    m_request = std::make_unique<QTemporaryFile>();
    const QByteArray bytes = QJsonDocument(request).toJson(QJsonDocument::Compact);
    if (bytes.size() > 1024 * 1024 || !m_request->open() || m_request->write(bytes) != bytes.size() || !m_request->flush()) {
        m_request.reset();
        completion(false, {}, tr("Cannot prepare the bounded Git request."));
        return;
    }
    m_request->close();
    m_diagnostics.clear();
    m_command.run(DockpipeChoices::preferredDockpipeBinary(workdir), {"git", "--request", m_request->fileName()},
        [this, completion = std::move(completion)](bool ok, const QByteArray &output) {
            m_request.reset();
            QJsonParseError error;
            const auto document = QJsonDocument::fromJson(output, &error);
            if (!ok || error.error != QJsonParseError::NoError || !document.isObject()) {
                completion(false, {}, tr("Git operation failed or was cancelled. Refresh before retrying.\n%1").arg(m_diagnostics));
                return;
            }
            completion(true, document.object(), {});
        }, 125000);
}
