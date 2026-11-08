#include "ProjectGitClient.h"
#include "DockpipeChoices.h"
#include "RemoteFeatureClient.h"
#include "RemoteGitContext.h"
#include <QDateTime>
#include <QJsonDocument>
#include <QRegularExpression>

ProjectGitClient::ProjectGitClient(QObject *parent) : QObject(parent), m_command(this)
{
    connect(&m_command, &RemoteCommand::busyChanged, this, &ProjectGitClient::busyChanged);
    connect(&m_command, &RemoteCommand::output, this, [this](const QString &text) {
        if (RemoteGitContext::isRemote(this))
            return;
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
    const bool remote = RemoteGitContext::isRemote(this);
    QString selectionError;
    const QString program = DockpipeChoices::operatorDockpipeBinary(&selectionError);
    if (program.isEmpty()) {
        completion(false, {}, selectionError);
        return;
    }
    QJsonObject intent;
    if (remote) {
        const auto target = RemoteGitContext::target(this);
        const QString root = RemoteGitContext::root(this);
        const QString action = request.value("action").toString();
        if (!RemoteGitContext::validTarget(target, root) || workdir != QStringLiteral("/")
            || (request.contains("workdir") && request.value("workdir").toString() != workdir)
            || action.isEmpty() || action.contains('.')) {
            completion(false, {}, tr("The selected remote Git context is unavailable. Select an authorized machine and root, then refresh."));
            return;
        }
        request.remove("workdir");
        intent = RemoteFeatureClient::prepareIntent(target, root, QStringLiteral("git.") + action, request);
    } else {
        request["workdir"] = workdir;
    }
    m_request = std::make_unique<QTemporaryFile>();
    const QByteArray bytes = QJsonDocument(remote ? intent : request).toJson(QJsonDocument::Compact);
    if (bytes.size() > 1024 * 1024 || !m_request->open() || m_request->write(bytes) != bytes.size() || !m_request->flush()) {
        m_request.reset();
        completion(false, {}, tr("Cannot prepare the bounded Git request."));
        return;
    }
    m_request->close();
    m_diagnostics.clear();
    const QStringList arguments = remote
        ? QStringList{"remote", "feature-call", "--auth", "never", "--request", m_request->fileName()}
        : QStringList{"git", "--request", m_request->fileName()};
    m_command.run(program, arguments,
        [this, remote, intent, completion = std::move(completion)](bool ok, const QByteArray &output) {
            m_request.reset();
            QJsonParseError error;
            const auto document = QJsonDocument::fromJson(output, &error);
            if (remote) {
                const auto operation = document.object();
                const auto captured = operation.value("request").toObject();
                static const QRegularExpression operationId(QStringLiteral("\\A[0-9a-f]{64}\\z"));
                const auto deadline = QDateTime::fromString(captured.value("deadline").toString(), Qt::ISODateWithMs);
                const auto expectedDeadline = QDateTime::fromString(intent.value("deadline").toString(), Qt::ISODateWithMs);
                const bool sameIntent = captured.value("schema") == intent.value("schema")
                    && captured.value("target") == intent.value("target")
                    && captured.value("root") == intent.value("root")
                    && captured.value("action") == intent.value("action")
                    && captured.value("nonce") == intent.value("nonce")
                    && captured.value("payload") == intent.value("payload")
                    && deadline.isValid() && deadline == expectedDeadline
                    && operationId.match(captured.value("id").toString()).hasMatch();
                const auto result = operation.value("result").toObject();
                if (!ok || error.error != QJsonParseError::NoError || !document.isObject() || !sameIntent
                    || operation.value("receipt_only").toBool()
                    || operation.value("status").toString() != QStringLiteral("success")
                    || result.value("status").toString() != QStringLiteral("success")
                    || !result.value("payload").isObject()) {
                    completion(false, {}, tr("The remote Git outcome is unavailable or uncertain. Refresh the captured machine and root before another write; the operation was not retried."));
                    return;
                }
                auto snapshot = result.value("payload").toObject();
                // Worker-local absolute paths are never local editor addresses.
                snapshot.insert(QStringLiteral("root"), QStringLiteral("/"));
                completion(true, snapshot, {});
                return;
            }
            if (!ok || error.error != QJsonParseError::NoError || !document.isObject()) {
                completion(false, {}, tr("Git operation failed or was cancelled. Refresh before retrying.\n%1").arg(m_diagnostics));
                return;
            }
            completion(true, document.object(), {});
        }, remote ? 30000 : 125000);
}
