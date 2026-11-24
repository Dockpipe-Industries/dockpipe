#pragma once

#include "WorkspaceFilesClient.h"

namespace RemoteWorkspaceFilesDispatch {
WorkspaceFilesClient::RemoteDispatch create(QObject *owner);
}
