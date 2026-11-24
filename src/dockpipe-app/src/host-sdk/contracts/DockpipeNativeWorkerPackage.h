#ifndef DOCKPIPE_NATIVE_WORKER_PACKAGE_V1_H
#define DOCKPIPE_NATIVE_WORKER_PACKAGE_V1_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define DOCKPIPE_NATIVE_WORKER_ABI_V1 1u
#define DOCKPIPE_NATIVE_WORKER_ENTRY_V1 "dockpipe_native_worker_package_v1"
#define DOCKPIPE_NATIVE_WORKER_PROFILE_LIMIT_V1 16384u

/* Independent of the Qt Display/Workspace SDKs. All references are borrowed
 * UTF-8 strings valid until the synchronous entrypoint returns. No C++ objects,
 * allocators, frames, input events or credentials cross this interface.
 *
 * The entrypoint runs ON THE MAIN THREAD, once per dedicated host-mode process.
 * Local setup/status actions belong to the opaque owner-selected profile; the
 * host never invents permission requests or opens a remote session.
 */
typedef struct DockpipeNativeWorkerHostV1 {
    uint32_t abi_version;
    uint32_t struct_size;
    const char *cli_path;
    const char *state_directory;
    const char *profile_json;
    size_t profile_json_bytes;
    void *host_context;
    /* Thread-safe, nonblocking. Poll during active work and native waits.
     * A true result revokes local work; never use it to renew authority.
     */
    int (*stop_requested)(void *host_context);
} DockpipeNativeWorkerHostV1;

typedef enum DockpipeNativeWorkerOutcomeV1 {
    DOCKPIPE_NATIVE_WORKER_SUCCESS_V1 = 0,
    DOCKPIPE_NATIVE_WORKER_CANCELLED_V1 = 1,
    DOCKPIPE_NATIVE_WORKER_FAILURE_V1 = 2
} DockpipeNativeWorkerOutcomeV1;

typedef struct DockpipeNativeWorkerResultV1 {
    uint32_t struct_size;
    uint32_t outcome;
    /* Exactly 1 only after all owned native work and children are verified
     * stopped. This is independent of the operation outcome. Missing/false
     * cleanup must never be converted into success by a late callback.
     */
    uint32_t cleanup_verified;
} DockpipeNativeWorkerResultV1;

/* Export with C linkage and default visibility. The host initializes result to
 * FAILURE/unknown; provider validates both ABI and struct sizes before work.
 * It owns the main native run loop, polls stop_requested, and returns only after
 * bounded cleanup. It must not install process-wide signal handlers.
 *
 * No dispose/unload call exists. The host retains the module for process life,
 * including unknown cleanup. Provider must retain any state still referenced by
 * late callbacks; the host ends this dedicated process rather than unloading.
 */
typedef void (*DockpipeNativeWorkerEntryV1)(const DockpipeNativeWorkerHostV1 *host,
                                          DockpipeNativeWorkerResultV1 *result);

#ifdef __cplusplus
}
#endif

#endif
