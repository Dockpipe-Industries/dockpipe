#include "RemoteWorkspaceFilesDispatch.h"

#include "RemoteFeatureClient.h"
#include <QPointer>

WorkspaceFilesClient::RemoteDispatch RemoteWorkspaceFilesDispatch::create(QObject *owner)
{
    const QPointer<QObject> guard(owner);
    return [guard](const WorkspaceFilesClient::Context &context, const QJsonObject &request,
                   WorkspaceFilesClient::Completion completion) -> WorkspaceFilesClient::Cancel {
        if (!guard)
            return {};
        const QJsonObject target{{"node", context.nodeId}, {"fingerprint", context.fingerprint},
                                 {"generation", context.workerGeneration}};
        const auto intent = RemoteFeatureClient::prepareIntent(target, context.rootId,
            QStringLiteral("files.") + request.value(QStringLiteral("action")).toString(), request);
        return RemoteFeatureClient::invoke(guard, intent,
            [context, request, completion = std::move(completion)](const RemoteFeatureClient::Result &result) {
                WorkspaceFilesClient::Result response;
                response.context = context;
                response.error = result.error;
                switch (result.outcome) {
                case RemoteFeatureClient::Outcome::Success: {
                    response.outcome = WorkspaceFilesClient::Outcome::Success;
                    response.value = result.value.toObject();
                    const auto expectedPath = request.value("action").toString() == "rename"
                        ? request.value("destination") : request.value("path");
                    if (response.value.value("schema").toString() != "dockpipe.files/v1"
                        || response.value.value("path") != expectedPath) {
                        RemoteFeatureClient::retainUncertain(result.reference);
                        response.outcome = WorkspaceFilesClient::Outcome::Failure;
                        response.error = QCoreApplication::translate("WorkspaceFilesClient", "Remote file result is uncertain. Refresh before retrying a write.");
                        response.value = {};
                    }
                    break;
                }
                case RemoteFeatureClient::Outcome::Cancelled:
                    response.outcome = WorkspaceFilesClient::Outcome::Cancelled;
                    break;
                case RemoteFeatureClient::Outcome::Failure:
                case RemoteFeatureClient::Outcome::Unknown:
                    response.outcome = WorkspaceFilesClient::Outcome::Failure;
                    if (response.error.isEmpty())
                        response.error = QCoreApplication::translate("WorkspaceFilesClient", "Remote file result is uncertain. Refresh before retrying a write.");
                    break;
                }
                completion(response);
            });
    };
}
