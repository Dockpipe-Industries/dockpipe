// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include <pipelang/model/result.hpp>
#include <pipelang/model/task.hpp>

#include <cstddef>
#include <cstdint>
#include <memory>
#include <string>
#include <vector>

namespace dockpipe::sdk {

struct OperationId {
    std::int64_t clientId = 0;
    std::int64_t value = 0;

    bool operator==(const OperationId&) const = default;
};

struct WorkflowVariable {
    std::string name;
    std::string value;
};

struct WorkflowRequest {
    std::string workdir;
    std::string workflowFile;
    std::string packageName;
    std::string workflowName;
    std::vector<WorkflowVariable> variables;
    std::vector<std::string> arguments;
    std::int64_t outputLimitBytes = 0;
};

struct NativeProcessRequest {
    std::string workdir;
    std::vector<std::string> arguments;
    std::int64_t outputLimitBytes = 0;
};

struct ScopeRequest {
    std::string workdir;
    std::string scope;
    std::string packageName;
    std::vector<std::string> path;
    std::int64_t outputLimitBytes = 0;
};

struct OperationObservation {
    OperationId id;
    bool exited = false;
    bool treeComplete = false;
    bool forced = false;
    std::int64_t exitCode = 0;
    std::int64_t reapedChildren = 0;
    std::string stdoutText;
    std::string stderrText;
    bool stdoutTruncated = false;
    bool stderrTruncated = false;
    // Populated only for a normally completed, fully captured path-scope query.
    std::string scopePath;
};

// Object JSON remains complete and bounded. The generated source bridge owns
// conversion into Dockpipe.SDK.ScopeObject; native code knows no storage layout.
struct ScopeResult {
    bool isPath = false;
    std::string path;
    std::string json;
};

enum class ExecutableMode { Dockpipe, NativeTool };

struct ClientConfiguration {
    std::string executable;
    std::string allowedWorkdirRoot;
    // Optional explicit XDG roots must be existing directories within the
    // allowed root. Inherited XDG and Dockpipe session bindings are not copied.
    std::string stateDirectory;
    std::string cacheDirectory;
    // The initial headless profile disables vault access unless its caller
    // explicitly selects the engine's normal vault policy.
    bool disableVault = true;
    std::size_t maximumStoredRuns = 0;
    std::size_t maximumActiveRuns = 0;
    std::size_t maximumOutputBytesPerStream = 0;
    std::size_t maximumInventoryBytes = 0;
    std::size_t maximumArgumentBytes = 0;
    // Windows requires the matching separately built SDK launch/signal helper as an explicit
    // absolute native artifact. Its opened image is pinned for this client.
    // Other platforms ignore this additive configuration field.
    std::string consoleSignalExecutable;
    // NativeTool admits an explicitly selected binary and caller-supplied argv.
    // Workflow and scope operations remain available only to Dockpipe mode.
    ExecutableMode executableMode = ExecutableMode::Dockpipe;
};

struct RunReceipt {
    OperationObservation observation;
    std::vector<std::string> arguments;
    std::string infrastructureFailure;
    std::int64_t leaderPid = 0;
    bool interruptRequested = false;
    bool scopeQuery = false;
    bool scopePathQuery = false;
};

// Native profiles require a dedicated headless process and one live Client.
// Linux additionally requires exclusive child reaping and default SIGCHLD.
// Windows uses nested non-breakaway jobs and a separately selected console helper;
// native Windows qualification is required before claiming platform acceptance.
// Limits are explicit caller contracts; maximumActiveRuns currently must be 1.
// Observe/ForceDispose wait 0..5000 ms; Scope requires 1..5000 ms. Cancellation
// ends a wait and retains custody. Tree disposal is always explicit. A cancelled
// or expired Scope leaves its internal operation ID available in receipts().
// Windows startup uses a shared 5000 ms handoff deadline, capped by Scope's
// overall budget. Failed startup may retain an unknown root (PID 0, exit -1);
// treeComplete proves custody closure independently of a known root result.
class Client final {
public:
    using Token = pipelang::model::CancellationToken;
    using StartResult = pipelang::model::Result<OperationId, std::string>;
    using ObservationResult = pipelang::model::Result<OperationObservation, std::string>;
    using BoolResult = pipelang::model::Result<bool, std::string>;
    using ScopeQueryResult = pipelang::model::Result<ScopeResult, std::string>;

    explicit Client(ClientConfiguration configuration);
    ~Client();
    Client(const Client&) = delete;
    Client& operator=(const Client&) = delete;
    Client(Client&&) = delete;
    Client& operator=(Client&&) = delete;

    StartResult Start(WorkflowRequest request);
    StartResult StartNativeTool(NativeProcessRequest request);
    // Starts the public scope command under the same custody as a workflow.
    // Observe supplies the eventual normalized path; object queries leave it empty.
    StartResult StartScope(ScopeRequest request);
    ObservationResult Observe(OperationId id, std::int64_t waitMilliseconds, Token token);
    BoolResult Interrupt(OperationId id);
    StartResult RetainedOperation() const;
    ObservationResult ForceDispose(OperationId id, std::int64_t waitMilliseconds, Token token);
    ScopeQueryResult Scope(ScopeRequest request, std::int64_t budgetMilliseconds, Token token);

    bool emergencyDispose();
    bool hasUnreapedTree();
    std::vector<RunReceipt> receipts() const;
    std::size_t maximumArgumentBytes() const noexcept;
    const std::string& executable() const noexcept;
    const std::string& executableIdentity() const noexcept;

private:
    struct Impl;
    std::unique_ptr<Impl> implementation_;
};

}
