// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#ifndef _GNU_SOURCE
#define _GNU_SOURCE
#endif
#include "Client.hpp"
#include "ConfinedFiles.hpp"

#include <algorithm>
#include <array>
#include <atomic>
#include <cerrno>
#include <charconv>
#include <chrono>
#include <csignal>
#include <cstdlib>
#include <dirent.h>
#include <fcntl.h>
#include <limits>
#include <map>
#include <mutex>
#include <poll.h>
#include <spawn.h>
#include <stdexcept>
#include <sys/prctl.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <sys/wait.h>
#include <utility>

#if !defined(__linux__) || !defined(SYS_pidfd_open) || !defined(SYS_pidfd_send_signal)
#error "The Dockpipe SDK native transport requires Linux subreaper and pidfd support"
#endif

namespace dockpipe::sdk {
namespace {
constexpr char waitContract[] = "native operation wait must be 0..5000 milliseconds";
std::atomic<bool> exclusiveClient{false};
std::atomic<std::int64_t> nextClientId{1};

// Process-wide child reaping is an explicit profile, never silently shared.
// An unresolved tree poisons this reservation until process exit: another client
// must not inherit a predecessor's children or mistake them for a new operation.
class ExclusiveChildren final {
public:
    ExclusiveChildren() {
        bool available = false;
        if (!exclusiveClient.compare_exchange_strong(available, true))
            throw std::runtime_error("native transport requires one exclusive client per host process");
    }

    ~ExclusiveChildren() {
        if (!releasable_)
            return;
        if (changedSubreaper_ && ::prctl(PR_SET_CHILD_SUBREAPER, previousSubreaper_) != 0)
            return;
        exclusiveClient.store(false);
    }

    void enableSubreaper() {
        if (::prctl(PR_GET_CHILD_SUBREAPER, &previousSubreaper_) != 0)
            throw std::runtime_error("cannot inspect child subreaper state");
        if (previousSubreaper_ == 0) {
            if (::prctl(PR_SET_CHILD_SUBREAPER, 1) != 0)
                throw std::runtime_error("Linux child subreaper is required");
            changedSubreaper_ = true;
        }
        int enabled = 0;
        if (::prctl(PR_GET_CHILD_SUBREAPER, &enabled) != 0 || enabled != 1)
            throw std::runtime_error("cannot verify child subreaper ownership");
    }

    void retain() noexcept { releasable_ = false; }

private:
    int previousSubreaper_ = 0;
    bool changedSubreaper_ = false;
    bool releasable_ = true;
};

std::int64_t allocateClientId() {
    auto candidate = nextClientId.load();
    while (true) {
        if (candidate <= 0 || candidate == std::numeric_limits<std::int64_t>::max())
            throw std::runtime_error("native client identity space exhausted");
        if (nextClientId.compare_exchange_weak(candidate, candidate + 1))
            return candidate;
    }
}

void validateConfiguration(const ClientConfiguration& configuration) {
    if (configuration.executableMode != ExecutableMode::Dockpipe &&
        configuration.executableMode != ExecutableMode::NativeTool)
        throw std::runtime_error("unsupported explicit executable mode");

    if (configuration.maximumStoredRuns == 0 || configuration.maximumActiveRuns != 1 ||
        configuration.maximumOutputBytesPerStream == 0 || configuration.maximumInventoryBytes == 0 ||
        configuration.maximumArgumentBytes == 0)
        throw std::runtime_error("native client requires positive limits and exactly one active run");
    const auto maximum = std::numeric_limits<std::size_t>::max();
    if (configuration.maximumStoredRuns > static_cast<std::size_t>(std::numeric_limits<std::int64_t>::max()) ||
        configuration.maximumOutputBytesPerStream > static_cast<std::size_t>(std::numeric_limits<std::int64_t>::max()) ||
        configuration.maximumOutputBytesPerStream > (maximum - sizeof(RunReceipt)) / 3)
        throw std::runtime_error("native client limits exceed representable storage");
    // A path query retains its normalized path alongside both captured streams.
    const auto capturedBytes = configuration.maximumOutputBytesPerStream * 3 + sizeof(RunReceipt);
    if (configuration.maximumArgumentBytes > maximum - capturedBytes ||
        configuration.maximumStoredRuns > maximum / (capturedBytes + configuration.maximumArgumentBytes))
        throw std::runtime_error("native client aggregate retained-storage limit overflows");
}

Descriptor pidDescriptor(pid_t pid) {
    Descriptor result(static_cast<int>(::syscall(SYS_pidfd_open, pid, 0)));
    if (!result)
        throw std::runtime_error("pidfd_open unavailable for owned child; refusing unsafe PID fallback");
    return result;
}

std::vector<char*> pointers(std::vector<std::string>& values) {
    std::vector<char*> result;
    result.reserve(values.size() + 1);
    for (auto& value : values)
        result.push_back(value.data());
    result.push_back(nullptr);
    return result;
}

void checkSpawn(int result) {
    if (result != 0)
        throw std::runtime_error("cannot prepare isolated CLI spawn: errno " + std::to_string(result));
}

void requireText(const std::string& value) {
    if (value.find('\0') != std::string::npos)
        throw std::runtime_error("CLI arguments cannot contain NUL bytes");
}

// Charge every string terminator and argv slot, including empty operands. This
// bounds argument count as well as payload before creating expanded arguments.
class Arguments final {
public:
    explicit Arguments(std::size_t limit) : remaining_(limit) {
        charge(sizeof(char*));
    }

    void append(const std::string& value) {
        requireText(value);
        charge(value.size());
        charge(1 + sizeof(char*) + sizeof(std::string));
        values_.push_back(value);
    }

    void variable(const WorkflowVariable& variable) {
        if (variable.name.empty() || variable.name.find('=') != std::string::npos)
            throw std::runtime_error("workflow variable names must be nonempty and contain no equals sign");
        requireText(variable.name);
        requireText(variable.value);
        charge(variable.name.size());
        charge(variable.value.size());
        charge(2 + sizeof(char*) + sizeof(std::string));
        values_.push_back(variable.name + "=" + variable.value);
    }

    std::vector<std::string> take() { return std::move(values_); }

private:
    void charge(std::size_t bytes) {
        if (bytes > remaining_)
            throw std::runtime_error("CLI arguments exceed the configured aggregate byte limit");
        remaining_ -= bytes;
    }

    std::size_t remaining_;
    std::vector<std::string> values_;
};

std::string readInventory(const std::string& path, std::size_t& bytesRemaining) {
    Descriptor input(::open(path.c_str(), O_RDONLY | O_CLOEXEC | O_NOFOLLOW));
    if (!input) {
        if (errno == ENOENT)
            return {};
        throw std::runtime_error("cannot inspect exclusive subreaper child inventory");
    }
    std::string result;
    char buffer[4096];
    while (true) {
        const auto count = ::read(input.get(), buffer, sizeof(buffer));
        if (count == 0)
            return result;
        if (count < 0) {
            if (errno == EINTR)
                continue;
            throw std::runtime_error("cannot read exclusive child inventory");
        }
        if (static_cast<std::size_t>(count) > bytesRemaining)
            throw std::runtime_error("exclusive child inventory exceeds its configured byte limit");
        bytesRemaining -= static_cast<std::size_t>(count);
        result.append(buffer, static_cast<std::size_t>(count));
    }
}
}

struct Client::Impl {
    using Clock = std::chrono::steady_clock;

    explicit Impl(ClientConfiguration selected)
        : configuration(std::move(selected)), files(configuration.allowedWorkdirRoot),
          clientId(allocateClientId()), executableDescriptor(ConfinedFiles::openAbsolute(configuration.executable, false)) {
        validateConfiguration(configuration);
        struct stat metadata {};
        std::array<char, 4> magic {};
        if (::fstat(executableDescriptor.get(), &metadata) != 0 || !S_ISREG(metadata.st_mode) ||
            ::pread(executableDescriptor.get(), magic.data(), magic.size(), 0) != 4 ||
            magic != std::array<char, 4>{'\x7f', 'E', 'L', 'F'})
            throw std::runtime_error("selected CLI must be a regular native ELF binary");
        identity = "device=" + std::to_string(metadata.st_dev) + ";inode=" +
            std::to_string(metadata.st_ino) + ";bytes=" + std::to_string(metadata.st_size);
        for (const auto& directory : {configuration.stateDirectory, configuration.cacheDirectory}) {
            if (!directory.empty())
                (void)files.directory(directory);
        }

        // Never forward inherited Dockpipe bindings, loader injection, provider
        // secrets or package overrides. HOME is retained, never reassigned.
        for (const char* key : {"PATH", "HOME", "LANG", "LC_ALL", "TZ"}) {
            if (const char* value = std::getenv(key))
                environment.push_back(std::string(key) + "=" + value);
        }
        if (!configuration.stateDirectory.empty())
            environment.push_back("XDG_STATE_HOME=" + configuration.stateDirectory);
        if (!configuration.cacheDirectory.empty())
            environment.push_back("XDG_CACHE_HOME=" + configuration.cacheDirectory);
        environment.push_back("NO_COLOR=1");
        struct sigaction childDisposition {};
        if (::sigaction(SIGCHLD, nullptr, &childDisposition) != 0 ||
            childDisposition.sa_handler != SIG_DFL || (childDisposition.sa_flags & SA_NOCLDWAIT) != 0)
            throw std::runtime_error("native transport requires default SIGCHLD and exclusive child reaping");
        discoverChildren();
        if (!children.empty())
            throw std::runtime_error("native transport host already has children before its first CLI spawn");
        exclusive.enableSubreaper();
        auto capability = pidDescriptor(::getpid());
        if (::syscall(SYS_pidfd_send_signal, capability.get(), 0, nullptr, 0) != 0)
            throw std::runtime_error("pidfd signaling is unavailable");
    }

    ~Impl() {
        // Callers perform bounded disposal and inspect the receipts. This final
        // fallback never waits, and signals only already-owned stable handles.
        if (!runs.empty() && !current().observation.treeComplete)
            exclusive.retain();
        for (const auto& [pid, descriptor] : children) {
            (void)pid;
            (void)::syscall(SYS_pidfd_send_signal, descriptor.get(), SIGKILL, nullptr, 0);
        }
    }

    RunReceipt& current() { return runs.back(); }
    const RunReceipt& current() const { return runs.back(); }

    bool currentId(OperationId id) const noexcept {
        return id.clientId == clientId && id.value > 0 && !runs.empty() && current().observation.id == id;
    }

    void rememberFailure(const std::string& failure) {
        if (!runs.empty() && current().infrastructureFailure.empty())
            current().infrastructureFailure = failure;
    }

    void validateOutputLimit(std::int64_t bytes) const {
        if (bytes <= 0 || static_cast<std::uint64_t>(bytes) > configuration.maximumOutputBytesPerStream)
            throw std::runtime_error("CLI capture limit must be positive and fit the configured per-stream limit");
    }

    static bool validWait(std::int64_t milliseconds) noexcept {
        return milliseconds >= 0 && milliseconds <= 5000;
    }

    StartResult start(std::string workdirPath, std::vector<std::string> arguments,
                      std::int64_t captureLimit, bool scopeQuery, bool scopePathQuery = false) {
        Descriptor workdir;
        try {
            if (runs.size() >= configuration.maximumStoredRuns)
                throw std::runtime_error("native operation receipt storage limit exhausted");
            validateOutputLimit(captureLimit);
            workdir = files.directory(workdirPath);
        } catch (const std::exception& error) {
            return StartResult::Failure(error.what());
        }
        if (!runs.empty()) {
            try {
                update();
            } catch (const std::exception& error) {
                rememberFailure(error.what());
                return StartResult::Failure(error.what());
            }
            if (!current().observation.treeComplete)
                return StartResult::Failure("one CLI tree is already owned; it must be completely reaped first");
        }

        // A preparation or spawn failure belongs to its own attempt. Until spawn
        // succeeds this receipt has no leader and an explicitly terminal tree.
        try {
            RunReceipt receipt;
            receipt.arguments = std::move(arguments);
            receipt.observation.id = {clientId, static_cast<std::int64_t>(runs.size() + 1)};
            receipt.observation.exited = true;
            receipt.observation.treeComplete = true;
            receipt.observation.exitCode = -1;
            receipt.scopeQuery = scopeQuery;
            receipt.scopePathQuery = scopePathQuery;
            runs.push_back(std::move(receipt));
        } catch (const std::exception& error) {
            return StartResult::Failure(error.what());
        }
        try {
            int outputPipe[2];
            if (::pipe2(outputPipe, O_CLOEXEC) != 0)
                throw std::runtime_error("cannot create CLI stdout pipe");
            Descriptor outputRead(outputPipe[0]);
            Descriptor outputWrite(outputPipe[1]);
            int errorPipe[2];
            if (::pipe2(errorPipe, O_CLOEXEC) != 0)
                throw std::runtime_error("cannot create CLI stderr pipe");
            Descriptor errorRead(errorPipe[0]);
            Descriptor errorWrite(errorPipe[1]);
            Descriptor input(::open("/dev/null", O_RDONLY | O_CLOEXEC));
            if (!input || ::fcntl(outputRead.get(), F_SETFL, O_NONBLOCK) != 0 ||
                ::fcntl(errorRead.get(), F_SETFL, O_NONBLOCK) != 0)
                throw std::runtime_error("cannot prepare nonblocking CLI capture");

            posix_spawn_file_actions_t actions;
            checkSpawn(::posix_spawn_file_actions_init(&actions));
            struct ActionsGuard {
                posix_spawn_file_actions_t* actions;
                ~ActionsGuard() { ::posix_spawn_file_actions_destroy(actions); }
            } actionsGuard{&actions};
            checkSpawn(::posix_spawn_file_actions_adddup2(&actions, input.get(), STDIN_FILENO));
            checkSpawn(::posix_spawn_file_actions_adddup2(&actions, outputWrite.get(), STDOUT_FILENO));
            checkSpawn(::posix_spawn_file_actions_adddup2(&actions, errorWrite.get(), STDERR_FILENO));
            checkSpawn(::posix_spawn_file_actions_addfchdir_np(&actions, workdir.get()));
            posix_spawnattr_t attributes;
            checkSpawn(::posix_spawnattr_init(&attributes));
            struct AttributesGuard {
                posix_spawnattr_t* attributes;
                ~AttributesGuard() { ::posix_spawnattr_destroy(attributes); }
            } attributesGuard{&attributes};
            sigset_t emptyMask;
            ::sigemptyset(&emptyMask);
            sigset_t defaults;
            ::sigemptyset(&defaults);
            for (int signal : {SIGINT, SIGTERM, SIGPIPE})
                ::sigaddset(&defaults, signal);
            checkSpawn(::posix_spawnattr_setsigmask(&attributes, &emptyMask));
            checkSpawn(::posix_spawnattr_setsigdefault(&attributes, &defaults));
            checkSpawn(::posix_spawnattr_setflags(&attributes, POSIX_SPAWN_SETSID | POSIX_SPAWN_SETSIGMASK | POSIX_SPAWN_SETSIGDEF));
            auto argv = pointers(current().arguments);
            auto env = pointers(environment);
            // The open ELF inode, rather than a replaceable path, selects every
            // invocation. argv[0] remains the operator-supplied executable path.
            const auto executableHandle = "/proc/self/fd/" + std::to_string(executableDescriptor.get());
            pid_t leader = -1;
            const int spawned = ::posix_spawn(&leader, executableHandle.c_str(), &actions, &attributes, argv.data(), env.data());
            if (spawned != 0)
                throw std::runtime_error("selected CLI spawn failed: errno " + std::to_string(spawned));
            current().leaderPid = leader;
            current().observation.exited = false;
            current().observation.treeComplete = false;
            current().observation.exitCode = 0;
            stdoutPipe = std::move(outputRead);
            stderrPipe = std::move(errorRead);
            outputLimit = static_cast<std::size_t>(captureLimit);
            children.emplace(leader, pidDescriptor(leader));
            return StartResult::Success(current().observation.id);
        } catch (const std::exception& error) {
            rememberFailure(error.what());
            return StartResult::Failure(error.what());
        }
    }

    StartResult startScope(ScopeRequest request) {
        try {
            if ((!request.scope.empty() && !request.packageName.empty()) ||
                (request.scope.empty() && request.packageName.empty() && !request.path.empty()))
                return StartResult::Failure("scope query requires at most one selection and paths require a selection");
            Arguments arguments(configuration.maximumArgumentBytes);
            arguments.append(configuration.executable);
            arguments.append("scope");
            arguments.append("--workdir");
            arguments.append(request.workdir);
            if (!request.packageName.empty()) {
                arguments.append("--package");
                arguments.append(request.packageName);
            }
            const auto appendPositional = [&](const std::string& value) {
                if (value.empty() || value.front() == '-')
                    throw std::runtime_error("scope positional arguments must be nonempty and cannot begin with an option prefix");
                arguments.append(value);
            };
            if (!request.scope.empty())
                appendPositional(request.scope);
            for (const auto& segment : request.path)
                appendPositional(segment);
            const bool pathQuery = !request.scope.empty() || !request.path.empty();
            return start(std::move(request.workdir), arguments.take(), request.outputLimitBytes, true, pathQuery);
        } catch (const std::exception& error) {
            // Admission failures do not belong to an unrelated active operation.
            return StartResult::Failure(error.what());
        }
    }

    void discoverChildren() {
        struct DirectoryCloser {
            void operator()(DIR* directory) const noexcept { (void)::closedir(directory); }
        };
        std::unique_ptr<DIR, DirectoryCloser> tasks(::opendir("/proc/self/task"));
        if (!tasks)
            throw std::runtime_error("cannot inspect exclusive transport host threads");
        std::size_t bytesRemaining = configuration.maximumInventoryBytes;
        while (true) {
            errno = 0;
            auto* entry = ::readdir(tasks.get());
            if (!entry) {
                if (errno != 0)
                    throw std::runtime_error("exclusive thread inventory read failed");
                break;
            }
            const std::string thread(entry->d_name);
            if (thread.empty() || !std::all_of(thread.begin(), thread.end(), [](char c) { return c >= '0' && c <= '9'; }))
                continue;
            if (thread.size() > bytesRemaining)
                throw std::runtime_error("exclusive thread inventory byte limit exhausted");
            bytesRemaining -= thread.size();
            const auto inventory = readInventory("/proc/self/task/" + thread + "/children", bytesRemaining);
            const char* cursor = inventory.data();
            const char* end = cursor + inventory.size();
            while (cursor != end) {
                while (cursor != end && (*cursor == ' ' || *cursor == '\n'))
                    ++cursor;
                if (cursor == end)
                    break;
                pid_t child = 0;
                const auto parsed = std::from_chars(cursor, end, child);
                if (parsed.ec != std::errc{} || parsed.ptr == cursor || child <= 0)
                    throw std::runtime_error("invalid exclusive child inventory");
                cursor = parsed.ptr;
                // Direct children remain unreaped under this mutex. Their PID
                // cannot be reused before this profile's sole reaper runs.
                if (!children.contains(child))
                    children.emplace(child, pidDescriptor(child));
            }
        }
    }

    void reapChildren() {
        for (auto iterator = children.begin(); iterator != children.end();) {
            int status = 0;
            const pid_t result = ::waitpid(iterator->first, &status, WNOHANG | __WALL);
            if (result < 0) {
                if (errno == EINTR)
                    continue;
                throw std::runtime_error("owned child was reaped outside the exclusive reaper");
            }
            if (result == 0) {
                ++iterator;
                continue;
            }
            if (!WIFEXITED(status) && !WIFSIGNALED(status))
                throw std::runtime_error("owned child returned an unexpected nonterminal wait status");
            auto& observation = current().observation;
            if (observation.reapedChildren == std::numeric_limits<std::int64_t>::max())
                throw std::runtime_error("reaped-child observation count overflowed");
            ++observation.reapedChildren;
            if (result == current().leaderPid) {
                observation.exited = true;
                observation.exitCode = WIFEXITED(status) ? WEXITSTATUS(status) : 128 + WTERMSIG(status);
            }
            iterator = children.erase(iterator);
        }
    }

    void drainOutput(Descriptor& pipe, std::string& output, bool& truncated) {
        if (!pipe)
            return;
        // Read no more than the remaining capture allowance plus one byte per
        // update. Overflow is visible in both the receipt and transport failure.
        std::size_t remaining = outputLimit - output.size() + 1;
        char buffer[4096];
        while (remaining > 0) {
            const auto count = ::read(pipe.get(), buffer, std::min(remaining, sizeof(buffer)));
            if (count == 0) {
                pipe.reset();
                return;
            }
            if (count < 0) {
                if (errno == EINTR)
                    continue;
                if (errno == EAGAIN || errno == EWOULDBLOCK)
                    return;
                throw std::runtime_error("CLI output read failed");
            }
            const auto bytes = static_cast<std::size_t>(count);
            const auto retain = std::min(bytes, outputLimit - output.size());
            output.append(buffer, retain);
            remaining -= bytes;
            if (retain != bytes) {
                truncated = true;
                rememberFailure("CLI output exceeded its per-stream byte limit; retained capture is incomplete");
                pipe.reset();
                return;
            }
        }
    }

    void update() {
        auto& observation = current().observation;
        drainOutput(stdoutPipe, observation.stdoutText, observation.stdoutTruncated);
        drainOutput(stderrPipe, observation.stderrText, observation.stderrTruncated);
        discoverChildren();
        reapChildren();
        // Reaping may expose newly adopted descendants. An empty direct-child
        // set then has no live ancestor from which another child could appear.
        discoverChildren();
        siginfo_t pendingChild {};
        const int waited = ::waitid(P_ALL, 0, &pendingChild, WEXITED | WNOHANG | WNOWAIT | __WALL);
        const bool noChildren = waited < 0 && errno == ECHILD;
        if (waited < 0 && !noChildren && errno != EINTR)
            throw std::runtime_error("cannot establish exclusive child-tree ownership state");
        const bool treeReaped = observation.exited && children.empty() && noChildren;
        observation.treeComplete = false;
        if (treeReaped) {
            drainOutput(stdoutPipe, observation.stdoutText, observation.stdoutTruncated);
            drainOutput(stderrPipe, observation.stderrText, observation.stderrTruncated);
            if (stdoutPipe || stderrPipe)
                throw std::runtime_error("owned tree is reaped but a capture writer remains outside ownership");
            observation.treeComplete = true;
            if (current().scopePathQuery && observation.scopePath.empty() &&
                !observation.forced && observation.exitCode == 0 &&
                !observation.stdoutTruncated && !observation.stderrTruncated) {
                std::string decoded = observation.stdoutText;
                if (!decoded.empty() && decoded.back() == '\n')
                    decoded.pop_back();
                if (decoded.empty() || decoded.front() != '/' ||
                    decoded.find_first_of("\r\n\0", 0, 3) != std::string::npos)
                    rememberFailure("scope service did not return one absolute path");
                else
                    observation.scopePath = std::move(decoded);
            }
        }
    }

    bool signalChild(pid_t pid, int signal) {
        const auto found = children.find(pid);
        if (found == children.end())
            return false;
        if (::syscall(SYS_pidfd_send_signal, found->second.get(), signal, nullptr, 0) == 0)
            return true;
        if (errno == ESRCH)
            return false;
        throw std::runtime_error("cannot signal owned CLI child through its pidfd");
    }

    void signalOwned(int signal) {
        // Preserve already-owned signaling even if later inventory fails.
        for (const auto& [pid, descriptor] : children) {
            (void)descriptor;
            (void)signalChild(pid, signal);
        }
        discoverChildren();
        for (const auto& [pid, descriptor] : children) {
            (void)descriptor;
            (void)signalChild(pid, signal);
        }
    }

    ObservationResult wait(std::int64_t milliseconds, Token token, bool force) {
        const auto deadline = Clock::now() + std::chrono::milliseconds(milliseconds);
        do {
            if (token.is_cancelled())
                return ObservationResult::Cancelled("process observation cancelled; tree ownership retained");
            if (force) {
                current().observation.forced = true;
                signalOwned(SIGKILL);
            }
            update();
            if (current().observation.treeComplete || Clock::now() >= deadline)
                break;
            std::array<pollfd, 2> descriptors{{{stdoutPipe.get(), POLLIN, 0}, {stderrPipe.get(), POLLIN, 0}}};
            const auto remaining = std::chrono::duration_cast<std::chrono::milliseconds>(deadline - Clock::now()).count();
            const int pause = static_cast<int>(std::clamp<std::int64_t>(remaining, 0, 10));
            if (::poll(descriptors.data(), descriptors.size(), pause) < 0 && errno != EINTR)
                throw std::runtime_error("CLI observation poll failed");
        } while (Clock::now() < deadline);
        if (!force && !current().infrastructureFailure.empty())
            return ObservationResult::Failure(current().infrastructureFailure);
        return ObservationResult::Success(current().observation);
    }

    ExclusiveChildren exclusive;
    ClientConfiguration configuration;
    ConfinedFiles files;
    std::int64_t clientId;
    std::string identity;
    Descriptor executableDescriptor;
    std::vector<std::string> environment;
    mutable std::mutex mutex;
    std::map<pid_t, Descriptor> children;
    std::vector<RunReceipt> runs;
    Descriptor stdoutPipe;
    Descriptor stderrPipe;
    std::size_t outputLimit = 0;
};

Client::Client(ClientConfiguration configuration)
    : implementation_(std::make_unique<Impl>(std::move(configuration))) {}

Client::~Client() = default;

Client::StartResult Client::Start(WorkflowRequest request) {
    auto& state = *implementation_;
    std::lock_guard lock(state.mutex);
    if (state.configuration.executableMode != ExecutableMode::Dockpipe)
        return StartResult::Failure("this client does not admit Dockpipe workflows");

    try {
        const bool fileSelection = !request.workflowFile.empty();
        const bool packageSelection = !request.packageName.empty() && !request.workflowName.empty();
        if (fileSelection == packageSelection ||
            (fileSelection && (!request.packageName.empty() || !request.workflowName.empty())))
            return StartResult::Failure("select exactly a workflow file or a package and workflow name");
        Arguments arguments(state.configuration.maximumArgumentBytes);
        arguments.append(state.configuration.executable);
        if (fileSelection) {
            arguments.append("--workflow-file");
            arguments.append(request.workflowFile);
        } else {
            arguments.append("--package");
            arguments.append(request.packageName);
            arguments.append("--workflow");
            arguments.append(request.workflowName);
        }
        arguments.append("--workdir");
        arguments.append(request.workdir);
        for (const auto& variable : request.variables) {
            arguments.append("--var");
            arguments.variable(variable);
        }
        if (state.configuration.disableVault)
            arguments.append("--no-vault");
        if (!request.arguments.empty()) {
            arguments.append("--");
            for (const auto& argument : request.arguments)
                arguments.append(argument);
        }
        return state.start(std::move(request.workdir), arguments.take(), request.outputLimitBytes, false);
    } catch (const std::exception& error) {
        return StartResult::Failure(error.what());
    }
}

Client::StartResult Client::StartNativeTool(NativeProcessRequest request) {
    auto& state = *implementation_;
    std::lock_guard lock(state.mutex);
    if (state.configuration.executableMode != ExecutableMode::NativeTool)
        return StartResult::Failure("this client does not admit native tool arguments");

    try {
        Arguments arguments(state.configuration.maximumArgumentBytes);
        arguments.append(state.configuration.executable);
        for (const auto& argument : request.arguments)
            arguments.append(argument);

        return state.start(std::move(request.workdir), arguments.take(), request.outputLimitBytes, false);
    } catch (const std::exception& error) {
        return StartResult::Failure(error.what());
    }
}

Client::StartResult Client::StartScope(ScopeRequest request) {
    auto& state = *implementation_;
    std::lock_guard lock(state.mutex);
    if (state.configuration.executableMode != ExecutableMode::Dockpipe)
        return StartResult::Failure("this client does not admit Dockpipe scope queries");

    return state.startScope(std::move(request));
}

Client::ObservationResult Client::Observe(OperationId id, std::int64_t milliseconds, Token token) {
    auto& state = *implementation_;
    std::lock_guard lock(state.mutex);
    if (!state.currentId(id))
        return ObservationResult::Failure("observation refers to a foreign, stale or unknown operation");
    if (!Impl::validWait(milliseconds))
        return ObservationResult::Failure(waitContract);
    try {
        return state.wait(milliseconds, token, false);
    } catch (const std::exception& error) {
        state.rememberFailure(error.what());
        return ObservationResult::Failure(error.what());
    }
}

Client::StartResult Client::RetainedOperation() const {
    auto& state = *implementation_;
    std::lock_guard lock(state.mutex);
    if (state.runs.empty() || state.current().observation.treeComplete)
        return StartResult::Failure("no retained operation requires observation or disposal");
    return StartResult::Success(state.current().observation.id);
}

Client::BoolResult Client::Interrupt(OperationId id) {
    auto& state = *implementation_;
    std::lock_guard lock(state.mutex);
    if (!state.currentId(id))
        return BoolResult::Failure("interrupt refers to a foreign, stale or unknown operation");
    try {
        state.update();
        if (state.current().observation.exited)
            return BoolResult::Success(false);
        const int signal = state.configuration.executableMode == ExecutableMode::NativeTool ? SIGTERM : SIGINT;
        const bool accepted = state.signalChild(static_cast<pid_t>(state.current().leaderPid), signal);
        if (accepted)
            state.current().interruptRequested = true;
        return BoolResult::Success(accepted);
    } catch (const std::exception& error) {
        state.rememberFailure(error.what());
        return BoolResult::Failure(error.what());
    }
}

Client::ObservationResult Client::ForceDispose(OperationId id, std::int64_t milliseconds, Token token) {
    auto& state = *implementation_;
    std::lock_guard lock(state.mutex);
    if (!state.currentId(id))
        return ObservationResult::Failure("disposal refers to a foreign, stale or unknown operation");
    if (!Impl::validWait(milliseconds))
        return ObservationResult::Failure(waitContract);
    if (token.is_cancelled())
        return ObservationResult::Cancelled("process disposal cancelled; tree ownership retained");
    try {
        if (state.current().observation.treeComplete)
            return ObservationResult::Success(state.current().observation);
        return state.wait(milliseconds, token, true);
    } catch (const std::exception& error) {
        state.rememberFailure(error.what());
        return ObservationResult::Failure(error.what());
    }
}

Client::ScopeQueryResult Client::Scope(ScopeRequest request, std::int64_t milliseconds, Token token) {
    auto& state = *implementation_;
    std::lock_guard lock(state.mutex);
    if (state.configuration.executableMode != ExecutableMode::Dockpipe)
        return ScopeQueryResult::Failure("this client does not admit Dockpipe scope queries");

    if (milliseconds <= 0 || !Impl::validWait(milliseconds))
        return ScopeQueryResult::Failure("scope query budget must be 1..5000 milliseconds");
    if (token.is_cancelled())
        return ScopeQueryResult::Cancelled("scope query cancelled before CLI launch");
    bool launched = false;
    try {
        const auto started = state.startScope(std::move(request));
        if (!started.is_success())
            return ScopeQueryResult::Failure(started.error());
        launched = true;
        const auto waited = state.wait(milliseconds, token, false);
        if (waited.is_cancelled())
            return ScopeQueryResult::Cancelled(waited.error());
        if (!waited.is_success())
            return ScopeQueryResult::Failure(waited.error());
        const auto& observation = waited.value();
        if (!observation.treeComplete || !observation.exited)
            return ScopeQueryResult::Failure("scope query budget exhausted; operation custody and receipt retained");
        if (observation.forced || observation.exitCode != 0 ||
            observation.stdoutTruncated || observation.stderrTruncated)
            return ScopeQueryResult::Failure("scope query requires zero exit, complete capture and an unforced reaped tree");
        ScopeResult result;
        result.isPath = state.current().scopePathQuery;
        if (result.isPath)
            result.path = observation.scopePath;
        else
            result.json = observation.stdoutText;
        return ScopeQueryResult::Success(std::move(result));
    } catch (const std::exception& error) {
        // An admission failure must not overwrite an unrelated active receipt.
        if (launched)
            state.rememberFailure(error.what());
        return ScopeQueryResult::Failure(error.what());
    }
}

bool Client::hasUnreapedTree() {
    auto& state = *implementation_;
    std::lock_guard lock(state.mutex);
    if (state.runs.empty())
        return false;
    try {
        state.update();
    } catch (const std::exception& error) {
        state.rememberFailure(error.what());
        return true;
    }
    return !state.current().observation.treeComplete;
}

bool Client::emergencyDispose() {
    if (!hasUnreapedTree())
        return false;
    OperationId id;
    {
        auto& state = *implementation_;
        std::lock_guard lock(state.mutex);
        state.rememberFailure("native emergency disposal required; explicit cleanup did not establish a reaped tree");
        id = state.current().observation.id;
    }
    (void)ForceDispose(id, 5000, {});
    return true;
}

std::vector<RunReceipt> Client::receipts() const {
    auto& state = *implementation_;
    std::lock_guard lock(state.mutex);
    return state.runs;
}

const std::string& Client::executable() const noexcept { return implementation_->configuration.executable; }
const std::string& Client::executableIdentity() const noexcept { return implementation_->identity; }

std::size_t Client::maximumArgumentBytes() const noexcept {
    return implementation_->configuration.maximumArgumentBytes;
}
}
