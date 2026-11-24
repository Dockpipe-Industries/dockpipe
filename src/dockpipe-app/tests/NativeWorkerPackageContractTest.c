#include "DockpipeNativeWorkerPackage.h"

_Static_assert(DOCKPIPE_NATIVE_WORKER_ABI_V1 == 1, "Independent native worker ABI version");
_Static_assert(DOCKPIPE_NATIVE_WORKER_PROFILE_LIMIT_V1 == 16384, "Bounded owner profile");
_Static_assert(DOCKPIPE_NATIVE_WORKER_FAILURE_V1 == 2, "Stable outcome values");

static int stop_requested(void *context)
{
    return *(int *)context;
}

int main(void)
{
    int stop = 1;
    DockpipeNativeWorkerHostV1 host = {0};
    host.abi_version = DOCKPIPE_NATIVE_WORKER_ABI_V1;
    host.struct_size = sizeof(host);
    host.host_context = &stop;
    host.stop_requested = stop_requested;
    DockpipeNativeWorkerResultV1 result = {sizeof(result), DOCKPIPE_NATIVE_WORKER_FAILURE_V1, 0};
    return host.stop_requested(host.host_context) == 1 && result.cleanup_verified == 0 ? 0 : 1;
}
