#include "ProjectGitClient.h"
#include "DockpipeChoices.h"
#include "RemoteFeatureClient.h"
#include "RemoteGitContext.h"
#include <QJsonDocument>

namespace {
class RemoteCallState final : public QObject {
public:
    explicit RemoteCallState(QObject *parent) : QObject(parent)
    {
        setObjectName(QStringLiteral("projectGitRemoteCallState"));
    }
    RemoteFeatureClient::Cancel cancel;
    bool active = false;
    bool cancelled = false;
};

RemoteCallState *remoteState(const ProjectGitClient *client)
{
    return static_cast<RemoteCallState *>(client->findChild<QObject *>(
        QStringLiteral("projectGitRemoteCallState"), Qt::FindDirectChildrenOnly));
}
}

ProjectGitClient::ProjectGitClient(QObject *parent) : QObject(parent), m_command(this)
{
    new RemoteCallState(this);
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
        const auto intent = RemoteFeatureClient::prepareIntent(target, root, QStringLiteral("git.") + action, request);
        if (QJsonDocument(intent).toJson(QJsonDocument::Compact).size() > 1024 * 1024) {
            completion(false, {}, tr("Cannot prepare the bounded Git request."));
            return;
        }
        auto *current = remoteState(this);
        current->active = true;
        current->cancelled = false;
        const QPointer<ProjectGitClient> guard(this);
        m_command.trackExternalOperation(current, [current] {
            current->cancelled = true;
            const auto cancel = current->cancel;
            if (cancel)
                cancel();
        });
        if (!guard)
            return;
        current->cancel = RemoteFeatureClient::invoke(current, intent,
            [this, completion = std::move(completion)](const RemoteFeatureClient::Result &result) {
                auto *current = remoteState(this);
                current->active = false;
                current->cancel = {};
                setProperty("remoteFeatureReference", result.reference);
                const QPointer<ProjectGitClient> guard(this);
                m_command.completeExternalOperation();
                if (!guard)
                    return;
                if (result.outcome != RemoteFeatureClient::Outcome::Success || !result.value.isObject()) {
                    if (result.outcome == RemoteFeatureClient::Outcome::Success)
                        RemoteFeatureClient::retainUncertain(result.reference);
                    const QString error = result.error.isEmpty()
                        ? tr("The remote Git outcome is unavailable or uncertain. Refresh the captured machine and root before another write; inspect Uncertain operations when a mutation is retained.")
                        : result.error;
                    completion(false, {}, error);
                    return;
                }
                auto snapshot = result.value.toObject();
                // Worker paths must never become local editor addresses.
                snapshot.insert(QStringLiteral("root"), QStringLiteral("/"));
                completion(true, snapshot, {});
            });
        if (guard && current->active && current->cancelled && current->cancel)
            current->cancel();
        return;
    } else {
        request["workdir"] = workdir;
    }
    m_request = std::make_unique<QTemporaryFile>();
    const QByteArray bytes = QJsonDocument(request).toJson(QJsonDocument::Compact);
    if (bytes.size() > 1024 * 1024 || !m_request->open() || m_request->write(bytes) != bytes.size() || !m_request->flush()) {
        m_request.reset();
        completion(false, {}, tr("Cannot prepare the bounded Git request."));
        return;
    }
    m_request->close();
    m_diagnostics.clear();
    const QStringList arguments{"git", "--request", m_request->fileName()};
    m_command.run(program, arguments,
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
