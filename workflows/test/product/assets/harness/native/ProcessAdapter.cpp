#ifndef _GNU_SOURCE
#define _GNU_SOURCE
#endif
#include "ProcessAdapter.hpp"

#include <algorithm>
#include <array>
#include <cerrno>
#include <charconv>
#include <csignal>
#include <cstdlib>
#include <dirent.h>
#include <fcntl.h>
#include <memory>
#include <poll.h>
#include <spawn.h>
#include <stdexcept>
#include <sys/prctl.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <sys/wait.h>

#if !defined(__linux__) || !defined(SYS_pidfd_open) || !defined(SYS_pidfd_send_signal)
#error "The private product harness requires Linux subreaper and pidfd support"
#endif

namespace product_harness {
namespace {
// At most 64 receipts with 32 KiB per stream: 4 MiB retained output. Source
// requests five invocations; the bound is a storage contract, not case policy.
constexpr std::size_t maximumReceipts = 64;
constexpr std::size_t maximumOutput = 32768;
constexpr std::size_t maximumInventoryBytes = 1024 * 1024;
constexpr char waitContract[] = "native operation wait must be 0..5000 milliseconds";

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

std::string readInventory(const std::string& path, std::size_t& bytesRemaining) {
    Descriptor input(::open(path.c_str(), O_RDONLY | O_CLOEXEC | O_NOFOLLOW));
    if (!input) {
        if (errno == ENOENT)
            return {}; // An SDK worker may finish between task inventory and open.
        throw std::runtime_error("cannot inspect private subreaper child inventory");
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
            throw std::runtime_error("cannot read private child inventory");
        }
        if (static_cast<std::size_t>(count) > bytesRemaining)
            throw std::runtime_error("private child inventory exceeds 1 MiB operation bound");
        bytesRemaining -= static_cast<std::size_t>(count);
        result.append(buffer, static_cast<std::size_t>(count));
    }
}
}

ProcessAdapter::ProcessAdapter(const ConfinedFiles& files, std::string executable, std::string workflow,
                               std::string stateDirectory, std::string cacheDirectory)
    : files_(files), executable_(std::move(executable)), workflow_(std::move(workflow)),
      executableDescriptor_(ConfinedFiles::openAbsolute(executable_, false)) {
    struct stat metadata {};
    std::array<char, 4> magic {};
    if (::fstat(executableDescriptor_.get(), &metadata) != 0 || !S_ISREG(metadata.st_mode) ||
        ::pread(executableDescriptor_.get(), magic.data(), magic.size(), 0) != 4 ||
        magic != std::array<char, 4>{'\x7f', 'E', 'L', 'F'})
        throw std::runtime_error("selected CLI must be a regular native ELF binary");
    executableIdentity_ = "device=" + std::to_string(metadata.st_dev) + ";inode=" +
        std::to_string(metadata.st_ino) + ";bytes=" + std::to_string(metadata.st_size);
    (void)ConfinedFiles::openAbsolute(workflow_, false);
    (void)files_.directory(stateDirectory);
    (void)files_.directory(cacheDirectory);

    // Do not forward inherited Dockpipe session/scope bindings, loader injection,
    // provider secrets or package overrides. HOME is retained, never reassigned.
    for (const char* key : {"PATH", "HOME", "LANG", "LC_ALL", "TZ"}) {
        if (const char* value = std::getenv(key))
            environment_.push_back(std::string(key) + "=" + value);
    }
    environment_.push_back("XDG_STATE_HOME=" + stateDirectory);
    environment_.push_back("XDG_CACHE_HOME=" + cacheDirectory);
    environment_.push_back("NO_COLOR=1");
    struct sigaction childDisposition {};
    if (::sigaction(SIGCHLD, nullptr, &childDisposition) != 0 ||
        childDisposition.sa_handler != SIG_DFL || (childDisposition.sa_flags & SA_NOCLDWAIT) != 0)
        throw std::runtime_error("private harness requires default SIGCHLD and exclusive child reaping");
    if (::prctl(PR_SET_CHILD_SUBREAPER, 1) != 0)
        throw std::runtime_error("Linux child subreaper is required");
    int enabled = 0;
    if (::prctl(PR_GET_CHILD_SUBREAPER, &enabled) != 0 || enabled != 1)
        throw std::runtime_error("cannot verify child subreaper ownership");
    auto capability = pidDescriptor(::getpid());
    if (::syscall(SYS_pidfd_send_signal, capability.get(), 0, nullptr, 0) != 0)
        throw std::runtime_error("pidfd signaling is unavailable");
    discoverChildren();
    if (!children_.empty())
        throw std::runtime_error("private harness already has children before its first CLI spawn");
}

ProcessAdapter::~ProcessAdapter() {
    // Main performs the bounded drain and reports its outcome. This last-resort
    // path never waits; it only signals children whose stable handles we own.
    for (const auto& [pid, descriptor] : children_) {
        (void)pid;
        (void)::syscall(SYS_pidfd_send_signal, descriptor.get(), SIGKILL, nullptr, 0);
    }
}

ProcessReceipt& ProcessAdapter::current() {
    if (receipts_.empty())
        throw std::runtime_error("no CLI invocation has been started");
    return receipts_.back();
}

void ProcessAdapter::rememberFailure(const std::string& failure) {
    if (!receipts_.empty() && current().infrastructureFailure.empty())
        current().infrastructureFailure = failure;
}

ProcessAdapter::IntResult ProcessAdapter::Start(product::ProcessRequest request) {
    std::lock_guard lock(mutex_);
    Descriptor workdir;
    std::vector<std::string> arguments;
    const bool query = request.operation == product::ProcessOperation::ScopeQuery;
    // Request admission owns these failures. No child has been requested yet,
    // and an unrelated current/completed receipt must remain unaffected.
    try {
        if (receipts_.size() == maximumReceipts)
            throw std::runtime_error("64-invocation receipt storage bound exhausted");
        if (request.outputLimitBytes <= 0 || request.outputLimitBytes > static_cast<std::int64_t>(maximumOutput))
            throw std::runtime_error("CLI capture limit must be 1..32768 bytes per stream");
        workdir = files_.directory(request.workdir);
        arguments.push_back(executable_);
        if (query) {
            if (!request.workflowFile.empty() || !request.mode.empty() || !request.correlation.empty())
                throw std::runtime_error("scope query cannot carry workflow arguments");
            if (request.workflowScope.empty() || request.workflowScope.size() > 256 ||
                request.workflowScope.front() == '-' ||
                request.workflowScope.find_first_of("\r\n\0", 0, 3) != std::string::npos)
                throw std::runtime_error("scope query requires a bounded positional workflow reference");
            arguments.insert(arguments.end(), {"scope", "workflow", request.workflowScope, "--workdir", request.workdir});
        } else if (request.operation == product::ProcessOperation::Workflow) {
            if (!request.workflowScope.empty() || request.workflowFile != workflow_ ||
                request.mode.size() > 256 || request.correlation.size() > 256 ||
                request.mode.find('\0') != std::string::npos || request.correlation.find('\0') != std::string::npos)
                throw std::runtime_error("workflow invocation differs from selected file or argument bounds");
            arguments.insert(arguments.end(), {"--workflow-file", workflow_, "--workdir", request.workdir,
                "--var", "JOURNEY_MODE=" + request.mode, "--var", "JOURNEY_CORRELATION=" + request.correlation, "--no-vault"});
        } else {
            throw std::runtime_error("unknown process operation");
        }
    } catch (const std::exception& error) {
        return IntResult::Failure(error.what());
    }

    // Refresh belongs to the existing tree. Real capture, protocol or ownership
    // failures discovered here are retained even if admission cannot continue.
    if (!receipts_.empty()) {
        try {
            update();
        } catch (const std::exception& error) {
            rememberFailure(error.what());
            return IntResult::Failure(error.what());
        }
        if (!current().observation.treeComplete)
            return IntResult::Failure("one CLI tree is already owned; it must be completely reaped first");
    }

    // Admission is complete. A failed preparation/spawn now belongs to this
    // launch attempt, never to a previous invocation. Until spawn succeeds the
    // receipt explicitly has no leader and a terminal, empty tree.
    try {
        ProcessReceipt receipt{};
        receipt.arguments = arguments;
        receipt.observation.invocationId = static_cast<std::int64_t>(receipts_.size() + 1);
        receipt.observation.exited = true;
        receipt.observation.treeComplete = true;
        receipt.observation.exitCode = -1;
        receipts_.push_back(std::move(receipt));
    } catch (const std::exception& error) {
        return IntResult::Failure(error.what());
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
        auto argv = pointers(arguments);
        auto environment = pointers(environment_);
        // Execute the already-opened ELF inode, so path replacement cannot change
        // the selected binary between invocations. argv[0] remains the operator path.
        const auto executableHandle = "/proc/self/fd/" + std::to_string(executableDescriptor_.get());
        pid_t leader = -1;
        const int spawned = ::posix_spawn(&leader, executableHandle.c_str(), &actions, &attributes, argv.data(), environment.data());
        if (spawned != 0)
            throw std::runtime_error("selected CLI spawn failed: errno " + std::to_string(spawned));
        current().leaderPid = leader;
        current().observation.exited = false;
        current().observation.treeComplete = false;
        current().observation.exitCode = 0;
        stdout_ = std::move(outputRead);
        stderr_ = std::move(errorRead);
        outputLimit_ = static_cast<std::size_t>(request.outputLimitBytes);
        scopeQuery_ = query;
        children_.emplace(leader, pidDescriptor(leader));
        return IntResult::Success(current().observation.invocationId);
    } catch (const std::exception& error) {
        rememberFailure(error.what());
        return IntResult::Failure(error.what());
    }
}

void ProcessAdapter::discoverChildren() {
    struct DirectoryCloser {
        void operator()(DIR* directory) const noexcept { (void)::closedir(directory); }
    };
    std::unique_ptr<DIR, DirectoryCloser> tasks(::opendir("/proc/self/task"));
    if (!tasks)
        throw std::runtime_error("cannot inspect private harness threads");
    std::size_t bytesRemaining = maximumInventoryBytes;
    while (true) {
        errno = 0;
        auto* entry = ::readdir(tasks.get());
        if (!entry) {
            if (errno != 0)
                throw std::runtime_error("private thread inventory read failed");
            break;
        }
        const std::string thread(entry->d_name);
        if (thread.empty() || !std::all_of(thread.begin(), thread.end(), [](char c) { return c >= '0' && c <= '9'; }))
            continue;
        if (thread.size() > bytesRemaining)
            throw std::runtime_error("private thread inventory bound exhausted");
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
                throw std::runtime_error("invalid private child inventory");
            cursor = parsed.ptr;
            // All entries are our direct children, remain unreaped under this
            // mutex, and cannot reuse their PID before this sole reaper runs.
            if (!children_.contains(child))
                children_.emplace(child, pidDescriptor(child));
        }
    }
}

void ProcessAdapter::reapChildren() {
    for (auto iterator = children_.begin(); iterator != children_.end();) {
        int status = 0;
        const pid_t result = ::waitpid(iterator->first, &status, WNOHANG | __WALL);
        if (result < 0) {
            if (errno == EINTR)
                continue;
            throw std::runtime_error("owned child was reaped outside the private reaper");
        }
        if (result == 0) {
            ++iterator;
            continue;
        }
        if (!WIFEXITED(status) && !WIFSIGNALED(status))
            throw std::runtime_error("private child returned an unexpected nonterminal wait status");
        auto& observation = current().observation;
        ++observation.reapedChildren;
        if (result == current().leaderPid) {
            observation.exited = true;
            observation.exitCode = WIFEXITED(status) ? WEXITSTATUS(status) : 128 + WTERMSIG(status);
        }
        iterator = children_.erase(iterator);
    }
}

void ProcessAdapter::drainOutput(Descriptor& pipe, std::string& output) {
    if (!pipe)
        return;
    // Each update reads at most the remaining capture allowance plus one byte.
    // Overflow is retained as a failure, never represented as complete output.
    std::size_t remaining = outputLimit_ - output.size() + 1;
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
        const auto retain = std::min(bytes, outputLimit_ - output.size());
        output.append(buffer, retain);
        remaining -= bytes;
        if (retain != bytes) {
            rememberFailure("CLI output exceeded its per-stream byte limit; retained capture is incomplete");
            pipe.reset();
            return;
        }
    }
}

void ProcessAdapter::update() {
    drainOutput(stdout_, current().observation.stdoutText);
    drainOutput(stderr_, current().observation.stderrText);
    discoverChildren();
    reapChildren();
    // Reaping an exited parent may expose newly adopted descendants. Re-inventory
    // before asserting completeness; an empty direct-child set then has no live
    // ancestry from which another owned descendant could subsequently appear.
    discoverChildren();
    auto& observed = current().observation;
    // The kernel's ECHILD proof also covers a child moving between our thread
    // inventories while an SDK worker exits. A /proc snapshot alone cannot prove
    // absence. WNOWAIT preserves the sole registry-owned reaping path.
    siginfo_t pendingChild {};
    const int waited = ::waitid(P_ALL, 0, &pendingChild, WEXITED | WNOHANG | WNOWAIT | __WALL);
    const bool noChildren = waited < 0 && errno == ECHILD;
    if (waited < 0 && !noChildren && errno != EINTR)
        throw std::runtime_error("cannot establish private child-tree ownership state");
    observed.treeComplete = observed.exited && children_.empty() && noChildren;
    if (observed.treeComplete) {
        drainOutput(stdout_, observed.stdoutText);
        drainOutput(stderr_, observed.stderrText);
        if (stdout_ || stderr_)
            throw std::runtime_error("owned tree is reaped but a capture writer remains outside ownership");
        if (scopeQuery_ && observed.exitCode == 0 && observed.scopePath.empty()) {
            std::string decoded = observed.stdoutText;
            if (!decoded.empty() && decoded.back() == '\n')
                decoded.pop_back();
            if (decoded.empty() || decoded.find_first_of("\r\n\0", 0, 3) != std::string::npos)
                throw std::runtime_error("scope query did not return exactly one absolute path");
            (void)files_.relative(decoded);
            observed.scopePath = std::move(decoded);
        }
    }
}

bool ProcessAdapter::signalChild(pid_t pid, int signal) {
    const auto found = children_.find(pid);
    if (found == children_.end())
        return false;
    if (::syscall(SYS_pidfd_send_signal, found->second.get(), signal, nullptr, 0) == 0)
        return true;
    if (errno == ESRCH)
        return false;
    throw std::runtime_error("cannot signal owned CLI child through its pidfd");
}

void ProcessAdapter::signalOwned(int signal) {
    // Signal already-owned handles even if a subsequent inventory read fails.
    for (const auto& [pid, descriptor] : children_) {
        (void)descriptor;
        (void)signalChild(pid, signal);
    }
    discoverChildren();
    for (const auto& [pid, descriptor] : children_) {
        (void)descriptor;
        (void)signalChild(pid, signal);
    }
}

bool ProcessAdapter::validWait(std::int64_t milliseconds) noexcept {
    return milliseconds >= 0 && milliseconds <= 5000;
}

ProcessAdapter::ObservationResult ProcessAdapter::wait(std::int64_t milliseconds, Token token, bool force) {
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
        std::array<pollfd, 2> descriptors{{{stdout_.get(), POLLIN, 0}, {stderr_.get(), POLLIN, 0}}};
        const auto remaining = std::chrono::duration_cast<std::chrono::milliseconds>(deadline - Clock::now()).count();
        const int pause = static_cast<int>(std::clamp<std::int64_t>(remaining, 0, 10));
        if (::poll(descriptors.data(), descriptors.size(), pause) < 0 && errno != EINTR)
            throw std::runtime_error("CLI observation poll failed");
    } while (Clock::now() < deadline);
    if (!force && !current().infrastructureFailure.empty())
        return ObservationResult::Failure(current().infrastructureFailure);
    return ObservationResult::Success(current().observation);
}

ProcessAdapter::ObservationResult ProcessAdapter::Observe(std::int64_t invocationId, std::int64_t milliseconds, Token token) {
    std::lock_guard lock(mutex_);
    if (receipts_.empty() || current().observation.invocationId != invocationId)
        return ObservationResult::Failure("observation refers to a noncurrent invocation");
    if (!validWait(milliseconds))
        return ObservationResult::Failure(waitContract);
    try {
        return wait(milliseconds, token, false);
    } catch (const std::exception& error) {
        rememberFailure(error.what());
        return ObservationResult::Failure(error.what());
    }
}

ProcessAdapter::BoolResult ProcessAdapter::Interrupt(std::int64_t invocationId) {
    std::lock_guard lock(mutex_);
    if (receipts_.empty() || current().observation.invocationId != invocationId)
        return BoolResult::Failure("interrupt refers to a noncurrent invocation");
    try {
        update();
        if (current().observation.exited)
            return BoolResult::Success(false);
        // The pidfd identifies this owned leader even if it exits after update.
        // A successful syscall proves acceptance, not that a handler ran or that
        // this signal caused the eventual exit. ESRCH is an observed no-op.
        const bool accepted = signalChild(static_cast<pid_t>(current().leaderPid), SIGINT);
        if (accepted)
            current().interruptRequested = true;
        return BoolResult::Success(accepted);
    } catch (const std::exception& error) {
        rememberFailure(error.what());
        return BoolResult::Failure(error.what());
    }
}

ProcessAdapter::ObservationResult ProcessAdapter::ForceDispose(std::int64_t milliseconds, Token token) {
    std::lock_guard lock(mutex_);
    if (!validWait(milliseconds))
        return ObservationResult::Failure(waitContract);
    try {
        if (receipts_.empty()) {
            product::ProcessObservation empty{};
            empty.exited = true;
            empty.treeComplete = true;
            return ObservationResult::Success(empty);
        }
        if (current().observation.treeComplete)
            return ObservationResult::Success(current().observation);
        return wait(milliseconds, token, true);
    } catch (const std::exception& error) {
        rememberFailure(error.what());
        return ObservationResult::Failure(error.what());
    }
}

bool ProcessAdapter::hasUnreapedTree() {
    std::lock_guard lock(mutex_);
    if (receipts_.empty())
        return false;
    try {
        update();
    } catch (const std::exception& error) {
        rememberFailure(error.what());
        return true;
    }
    return !current().observation.treeComplete;
}

bool ProcessAdapter::emergencyDispose() {
    if (!hasUnreapedTree())
        return false;
    {
        std::lock_guard lock(mutex_);
        rememberFailure("native emergency disposal required; source cleanup did not establish a reaped tree");
    }
    (void)ForceDispose(5000, {});
    return true;
}

std::vector<ProcessReceipt> ProcessAdapter::receipts() {
    std::lock_guard lock(mutex_);
    return receipts_;
}
}
