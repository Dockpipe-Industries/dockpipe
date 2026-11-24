// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "Client.hpp"
#include "WindowsFiles.hpp"
#include "ConsoleLaunchProtocol.hpp"

#include <algorithm>
#include <array>
#include <atomic>
#include <chrono>
#include <iterator>
#include <limits>
#include <mutex>
#include <stdexcept>
#include <utility>

#if !defined(_WIN32)
#error "The Dockpipe SDK Windows transport requires Windows"
#endif

namespace dockpipe::sdk {
namespace {
constexpr char waitContract[] = "native operation wait must be 0..5000 milliseconds";
constexpr std::size_t maximumCommandCharacters = 32767;
constexpr DWORD signalHelperBudget = 1000;
// Match pipe capacity and the one-read observation bound. A 4 KiB block with
// Windows timer-quantized polling stalls sustained writers behind each sleep.
constexpr DWORD captureDrainBytes = 64 * 1024;
std::atomic<bool> exclusiveClient{false};
std::atomic<std::int64_t> nextClientId{1};

std::runtime_error windowsError(const char* operation) {
    const auto error = ::GetLastError();
    return std::runtime_error(std::string(operation) + ": Win32 " + std::to_string(error));
}

class LaunchCancelled final : public std::runtime_error {
public:
    LaunchCancelled() : std::runtime_error("Windows CLI launch cancelled; any acquired custody is retained") {}
};

void checkLaunch(Client::Token token, ULONGLONG deadline, const char* phase) {
    if (token.is_cancelled())
        throw LaunchCancelled();

    if (::GetTickCount64() >= deadline)
        throw std::runtime_error(std::string("Windows CLI launch deadline exhausted during ") + phase +
            "; any acquired custody is retained");
}

class MappingView final {
public:
    MappingView(HANDLE mapping, DWORD access, std::size_t bytes)
        : address_(::MapViewOfFile(mapping, access, 0, 0, bytes)) {
        if (!address_)
            throw windowsError("cannot map Windows console launch snapshot");
    }

    ~MappingView() { ::UnmapViewOfFile(address_); }
    MappingView(const MappingView&) = delete;
    MappingView& operator=(const MappingView&) = delete;
    void* get() const noexcept { return address_; }

private:
    void* address_;
};

WindowsHandle launchSnapshot(const std::vector<unsigned char>& bytes) {
    const auto extent = static_cast<std::uint64_t>(bytes.size());
    WindowsHandle writable(::CreateFileMappingW(INVALID_HANDLE_VALUE, nullptr, PAGE_READWRITE,
        static_cast<DWORD>(extent >> 32), static_cast<DWORD>(extent), nullptr));
    if (!writable)
        throw windowsError("cannot allocate Windows console launch snapshot");

    {
        MappingView view(writable.get(), FILE_MAP_WRITE, bytes.size());
        std::memcpy(view.get(), bytes.data(), bytes.size());
    }

    HANDLE readonly = nullptr;
    if (!::DuplicateHandle(::GetCurrentProcess(), writable.get(), ::GetCurrentProcess(), &readonly,
        FILE_MAP_READ, TRUE, 0))
        throw windowsError("cannot restrict Windows console launch snapshot capability");

    // Both the writable view and mapping capability end before process creation.
    WindowsHandle result(readonly);
    writable.reset();
    return result;
}

class ExclusiveClient final {
public:
    ExclusiveClient() {
        bool available = false;
        if (!exclusiveClient.compare_exchange_strong(available, true))
            throw std::runtime_error("native transport requires one exclusive client per host process");
    }

    ~ExclusiveClient() {
        if (releasable_)
            exclusiveClient.store(false);
    }

    void retain() noexcept { releasable_ = false; }

private:
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
        configuration.maximumOutputBytesPerStream == 0 || configuration.maximumArgumentBytes == 0 ||
        configuration.maximumInventoryBytes < offsetof(JOBOBJECT_BASIC_PROCESS_ID_LIST, ProcessIdList) + 2 * sizeof(ULONG_PTR) ||
        configuration.consoleSignalExecutable.empty())
        throw std::runtime_error("Windows native client requires explicit limits, one active run and a console signal helper");

    const auto maximum = std::numeric_limits<std::size_t>::max();
    if (configuration.maximumStoredRuns > static_cast<std::size_t>(std::numeric_limits<std::int64_t>::max()) ||
        configuration.maximumOutputBytesPerStream > static_cast<std::size_t>(std::numeric_limits<std::int64_t>::max()) ||
        configuration.maximumOutputBytesPerStream > (maximum - sizeof(RunReceipt)) / 3 ||
        configuration.maximumInventoryBytes > std::numeric_limits<DWORD>::max())
        throw std::runtime_error("Windows native client limits exceed representable storage");

    const auto captureBytes = configuration.maximumOutputBytesPerStream * 3 + sizeof(RunReceipt);
    if (configuration.maximumArgumentBytes > maximum - captureBytes ||
        configuration.maximumStoredRuns > maximum / (captureBytes + configuration.maximumArgumentBytes))
        throw std::runtime_error("Windows native client retained-storage limit overflows");
}

ClientConfiguration validatedConfiguration(ClientConfiguration selected) {
    validateConfiguration(selected);
    return selected;
}

void requireNativeImage(const WindowsPath& file) {
    IMAGE_DOS_HEADER dos{};
    DWORD count = 0;
    LARGE_INTEGER position{};
    if (!::SetFilePointerEx(file.handle.get(), position, nullptr, FILE_BEGIN) ||
        !::ReadFile(file.handle.get(), &dos, sizeof(dos), &count, nullptr) || count != sizeof(dos) ||
        dos.e_magic != IMAGE_DOS_SIGNATURE || dos.e_lfanew < static_cast<LONG>(sizeof(dos)))
        throw std::runtime_error("selected Windows executable must be a regular native PE image");

    FILE_STANDARD_INFO metadata{};
    if (!::GetFileInformationByHandleEx(file.handle.get(), FileStandardInfo, &metadata, sizeof(metadata)) ||
        static_cast<LONGLONG>(dos.e_lfanew) > metadata.EndOfFile.QuadPart - sizeof(DWORD))
        throw std::runtime_error("selected Windows executable has an invalid PE offset");
    position.QuadPart = dos.e_lfanew;
    DWORD signature = 0;
    if (!::SetFilePointerEx(file.handle.get(), position, nullptr, FILE_BEGIN) ||
        !::ReadFile(file.handle.get(), &signature, sizeof(signature), &count, nullptr) ||
        count != sizeof(signature) || signature != IMAGE_NT_SIGNATURE)
        throw std::runtime_error("selected Windows executable has no native PE signature");
}

// Every expanded string, terminator and argv slot is charged before allocation.
class Arguments final {
public:
    explicit Arguments(std::size_t limit) : remaining_(limit) {
        charge(sizeof(char*));
    }

    void append(const std::string& value) {
        if (value.find('\0') != value.npos)
            throw std::runtime_error("CLI arguments cannot contain NUL bytes");
        charge(value.size());
        charge(1 + sizeof(char*) + sizeof(std::string));
        values_.push_back(value);
    }

    void variable(const WorkflowVariable& variable) {
        if (variable.name.empty() || variable.name.find('=') != variable.name.npos ||
            variable.name.find('\0') != variable.name.npos || variable.value.find('\0') != variable.value.npos)
            throw std::runtime_error("workflow variable names must be nonempty without equals signs or NUL bytes");
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

std::wstring commandLine(const std::vector<std::string>& arguments) {
    std::wstring result;
    for (const auto& argument : arguments) {
        const auto quoted = quoteWindowsArgument(windowsWide(argument));
        if (quoted.size() + 1 >= maximumCommandCharacters - result.size())
            throw std::runtime_error("Windows command line exceeds CreateProcessW's character bound");
        if (!result.empty())
            result.push_back(L' ');
        result += quoted;
    }
    return result;
}

WindowsHandle inheritedCopy(HANDLE source, DWORD access, bool sameAccess = false) {
    HANDLE copy = nullptr;
    if (!::DuplicateHandle(::GetCurrentProcess(), source, ::GetCurrentProcess(), &copy, access, TRUE,
        sameAccess ? DUPLICATE_SAME_ACCESS : 0))
        throw windowsError("cannot prepare explicitly inherited Windows handle");
    return WindowsHandle(copy);
}

class HandleList final {
public:
    explicit HandleList(const std::vector<HANDLE>& handles) {
        SIZE_T bytes = 0;
        (void)::InitializeProcThreadAttributeList(nullptr, 1, 0, &bytes);
        if (bytes == 0)
            throw windowsError("cannot size Windows process handle attributes");
        storage_.resize(bytes);
        attributes_ = reinterpret_cast<LPPROC_THREAD_ATTRIBUTE_LIST>(storage_.data());
        if (!::InitializeProcThreadAttributeList(attributes_, 1, 0, &bytes)) {
            attributes_ = nullptr;
            throw windowsError("cannot initialize Windows process handle attributes");
        }
        if (!::UpdateProcThreadAttribute(attributes_, 0, PROC_THREAD_ATTRIBUTE_HANDLE_LIST,
            const_cast<HANDLE*>(handles.data()), handles.size() * sizeof(HANDLE), nullptr, nullptr)) {
            ::DeleteProcThreadAttributeList(attributes_);
            attributes_ = nullptr;
            throw windowsError("cannot select Windows process inherited handles");
        }
    }

    ~HandleList() {
        if (attributes_)
            ::DeleteProcThreadAttributeList(attributes_);
    }
    HandleList(const HandleList&) = delete;
    HandleList& operator=(const HandleList&) = delete;
    LPPROC_THREAD_ATTRIBUTE_LIST get() const noexcept { return attributes_; }

private:
    std::vector<unsigned char> storage_;
    LPPROC_THREAD_ATTRIBUTE_LIST attributes_ = nullptr;
};

std::pair<WindowsHandle, WindowsHandle> capturePipe(DWORD capacity = 0) {
    SECURITY_ATTRIBUTES security{sizeof(security), nullptr, TRUE};
    HANDLE read = nullptr;
    HANDLE write = nullptr;
    if (!::CreatePipe(&read, &write, &security, capacity))
        throw windowsError("cannot create Windows capture pipe");
    WindowsHandle reader(read);
    WindowsHandle writer(write);
    if (!::SetHandleInformation(reader.get(), HANDLE_FLAG_INHERIT, 0))
        throw windowsError("cannot isolate Windows capture reader");
    return {std::move(reader), std::move(writer)};
}

WindowsHandle nullStream() {
    SECURITY_ATTRIBUTES security{sizeof(security), nullptr, TRUE};
    WindowsHandle result(::CreateFileW(L"NUL", GENERIC_READ | GENERIC_WRITE,
        FILE_SHARE_READ | FILE_SHARE_WRITE, &security, OPEN_EXISTING, 0, nullptr));
    if (!result)
        throw windowsError("cannot open Windows null stream");
    return result;
}

std::string signalDiagnostic(HANDLE pipe) {
    constexpr DWORD maximumBytes = 128;
    DWORD available = 0;
    if (!::PeekNamedPipe(pipe, nullptr, 0, nullptr, &available, nullptr)) {
        const auto error = ::GetLastError();
        return "diagnostic unavailable: Win32 " + std::to_string(error);
    }
    if (available == 0)
        return "no diagnostic bytes available";

    std::array<char, maximumBytes> buffer{};
    DWORD count = 0;
    if (!::ReadFile(pipe, buffer.data(), std::min(available, maximumBytes), &count, nullptr)) {
        const auto error = ::GetLastError();
        return "diagnostic read failed: Win32 " + std::to_string(error);
    }

    constexpr char hexadecimal[] = "0123456789abcdef";
    std::string result = "diagnostic_bytes=" + std::to_string(available);
    result += " prefix_bytes=" + std::to_string(count) + " text=";
    for (DWORD index = 0; index < count; ++index) {
        const auto byte = static_cast<unsigned char>(buffer[index]);
        if (byte >= 32 && byte <= 126 && byte != '\\') {
            result.push_back(static_cast<char>(byte));
        } else {
            result += "\\x";
            result.push_back(hexadecimal[byte >> 4]);
            result.push_back(hexadecimal[byte & 15]);
        }
    }
    if (available > maximumBytes)
        result += " [diagnostic truncated]";
    return result;
}

void verifyProcessImage(HANDLE process, const WindowsPath& selected) {
    std::wstring image(maximumCommandCharacters, L'\0');
    DWORD characters = static_cast<DWORD>(image.size());
    if (!::QueryFullProcessImageNameW(process, 0, image.data(), &characters))
        throw windowsError("cannot inspect suspended Windows process image");
    image.resize(characters);
    // The selected image's open handle denies write/delete and all of its
    // ancestors are retained. Compare the actual mapped image's file ID as well.
    WindowsHandle actual(::CreateFileW(image.c_str(), GENERIC_READ | FILE_READ_ATTRIBUTES,
        FILE_SHARE_READ, nullptr, OPEN_EXISTING, FILE_FLAG_OPEN_REPARSE_POINT, nullptr));
    FILE_ID_INFO actualId{};
    FILE_ID_INFO selectedId{};
    if (!actual || !::GetFileInformationByHandleEx(actual.get(), FileIdInfo, &actualId, sizeof(actualId)) ||
        !::GetFileInformationByHandleEx(selected.handle.get(), FileIdInfo, &selectedId, sizeof(selectedId)))
        throw windowsError("cannot compare suspended process image file identity");
    if (actualId.VolumeSerialNumber != selectedId.VolumeSerialNumber ||
        !std::equal(std::begin(actualId.FileId.Identifier), std::end(actualId.FileId.Identifier),
            std::begin(selectedId.FileId.Identifier)))
        throw std::runtime_error("suspended Windows process image differs from the opened selected executable");
}

void requireInJob(HANDLE process, HANDLE job) {
    BOOL member = FALSE;
    if (!::IsProcessInJob(process, job, &member) || !member)
        throw std::runtime_error("cannot prove suspended Windows process membership in the owned job");
}
}

struct Client::Impl {
    using Clock = std::chrono::steady_clock;
    enum class LaunchState { Prepared, BrokerCreated, BrokerAdmitted, ChildAdopted, Running };

    explicit Impl(ClientConfiguration selected)
        : configuration(validatedConfiguration(std::move(selected))), files(configuration.allowedWorkdirRoot),
          clientId(allocateClientId()), executableFile(openWindowsPath(configuration.executable, false)),
          signalFile(openWindowsPath(configuration.consoleSignalExecutable, false)) {
        requireNativeImage(executableFile);
        requireNativeImage(signalFile);
        if (!configuration.stateDirectory.empty())
            selectedStateDirectory = files.directory(configuration.stateDirectory);
        if (!configuration.cacheDirectory.empty())
            selectedCacheDirectory = files.directory(configuration.cacheDirectory);
        prepareEnvironment();
    }

    ~Impl() {
        if (!runs.empty() && !current().observation.treeComplete) {
            exclusive.retain();
            // Destructor fallback is never evidence of successful cleanup.
            if (broker && !brokerAssigned)
                (void)::TerminateProcess(broker.get(), 137);
            if (leader && !leaderAssigned)
                (void)::TerminateProcess(leader.get(), 137);
            if (signalProcess && !signalAssigned)
                (void)::TerminateProcess(signalProcess.get(), 137);
            if (job)
                (void)::TerminateJobObject(job.get(), 137);
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

    static bool validWait(std::int64_t milliseconds) noexcept {
        return milliseconds >= 0 && milliseconds <= 5000;
    }

    void prepareEnvironment() {
        std::vector<std::wstring> entries;
        std::size_t remaining = configuration.maximumArgumentBytes;
        const auto append = [&](std::wstring key, std::wstring value) {
            if (value.find(L'\0') != value.npos || key.size() + 2 > remaining / sizeof(wchar_t) ||
                value.size() > remaining / sizeof(wchar_t) - key.size() - 2)
                throw std::runtime_error("Windows child environment exceeds the configured byte limit");
            remaining -= (key.size() + value.size() + 2) * sizeof(wchar_t);
            entries.push_back(std::move(key) + L"=" + std::move(value));
        };
        // Fresh whitelist: no inherited Dockpipe, provider, debugger or loader
        // bindings. Windows loader/tool resolution needs these platform roots.
        for (const auto key : {L"PATH", L"SystemRoot", L"WINDIR", L"ComSpec", L"PATHEXT",
            L"USERPROFILE", L"HOME", L"HOMEDRIVE", L"HOMEPATH", L"LANG", L"LC_ALL", L"TZ"}) {
            const auto length = ::GetEnvironmentVariableW(key, nullptr, 0);
            if (length == 0)
                continue;
            if (static_cast<std::size_t>(length) > remaining / sizeof(wchar_t))
                throw std::runtime_error("inherited Windows environment entry exceeds the configured byte limit");
            std::wstring value(length, L'\0');
            const auto copied = ::GetEnvironmentVariableW(key, value.data(), length);
            if (copied == 0 || copied >= length)
                throw std::runtime_error("Windows environment changed while preparing the fresh child environment");
            value.resize(copied);
            append(key, std::move(value));
        }
        if (selectedStateDirectory.handle) {
            // The Windows CLI selects durable storage through LOCALAPPDATA.
            // Carry the opened volume identity into actual child inputs rather
            // than reopening the caller's original drive-letter alias.
            append(L"LOCALAPPDATA", selectedStateDirectory.nativePath);
            append(L"XDG_STATE_HOME", selectedStateDirectory.nativePath);
        }
        if (selectedCacheDirectory.handle)
            append(L"XDG_CACHE_HOME", selectedCacheDirectory.nativePath);
        // Public SDK object queries resolve their CLI binding independently of
        // the process image. Select this client's opened artifact explicitly;
        // never inherit an ambient binding or depend on PATH finding another CLI.
        append(L"DOCKPIPE_BIN", executableFile.nativePath);
        append(L"NO_COLOR", L"1");
        std::sort(entries.begin(), entries.end(), [](const auto& left, const auto& right) {
            return ::CompareStringOrdinal(left.data(), static_cast<int>(left.size()), right.data(),
                static_cast<int>(right.size()), TRUE) == CSTR_LESS_THAN;
        });
        if (remaining < sizeof(wchar_t))
            throw std::runtime_error("Windows child environment has no room for its final terminator");
        for (const auto& entry : entries) {
            environment.insert(environment.end(), entry.begin(), entry.end());
            environment.push_back(L'\0');
        }
        environment.push_back(L'\0');
    }

    WindowsHandle createJob() const {
        WindowsHandle result(::CreateJobObjectW(nullptr, nullptr));
        if (!result)
            throw windowsError("cannot create owned Windows job");
        JOBOBJECT_EXTENDED_LIMIT_INFORMATION limits{};
        limits.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | JOB_OBJECT_LIMIT_ACTIVE_PROCESS;
        const auto header = offsetof(JOBOBJECT_BASIC_PROCESS_ID_LIST, ProcessIdList);
        const auto members = (configuration.maximumInventoryBytes - header) / sizeof(ULONG_PTR);
        limits.BasicLimitInformation.ActiveProcessLimit = static_cast<DWORD>(std::min<std::size_t>(
            members, std::numeric_limits<DWORD>::max()));
        if (!::SetInformationJobObject(result.get(), JobObjectExtendedLimitInformation, &limits, sizeof(limits)))
            throw windowsError("cannot apply non-breakaway Windows job custody limits");
        return result;
    }

    StartResult start(WindowsPath workdir, std::vector<std::string> arguments,
        std::int64_t captureLimit, bool scopeQuery, bool scopePathQuery = false,
        Token token = {}, ULONGLONG outerDeadline = 0) {
        const auto handoffDeadline = ::GetTickCount64() + consoleLaunch::handoffMilliseconds;
        const auto deadline = outerDeadline == 0 ? handoffDeadline : std::min(handoffDeadline, outerDeadline);
        std::wstring command;
        try {
            if (runs.size() >= configuration.maximumStoredRuns)
                throw std::runtime_error("native operation receipt storage limit exhausted");
            if (captureLimit <= 0 || static_cast<std::uint64_t>(captureLimit) > configuration.maximumOutputBytesPerStream)
                throw std::runtime_error("CLI capture limit must be positive and fit the configured per-stream limit");
            command = commandLine(arguments);
            if (!runs.empty()) {
                update();
                if (!current().observation.treeComplete)
                    return StartResult::Failure("one CLI tree is already owned; it must be completely drained first");
            }
        } catch (const std::exception& error) {
            return StartResult::Failure(error.what());
        }

        try {
            RunReceipt receipt;
            receipt.arguments = std::move(arguments);
            receipt.observation.id = {clientId, static_cast<std::int64_t>(runs.size() + 1)};
            receipt.observation.exited = false;
            receipt.observation.treeComplete = true;
            receipt.observation.exitCode = -1;
            receipt.scopeQuery = scopeQuery;
            receipt.scopePathQuery = scopePathQuery;
            runs.push_back(std::move(receipt));
        } catch (const std::exception& error) {
            return StartResult::Failure(error.what());
        }

        try {
            checkLaunch(token, deadline, "startup preparation");
            leader.reset();
            leaderThread.reset();
            broker.reset();
            brokerThread.reset();
            signalProcess.reset();
            signalThread.reset();
            rootKnown = false;
            brokerAssigned = false;
            leaderAssigned = false;
            launchState = LaunchState::Prepared;
            signalAssigned = false;
            previousTotalProcesses = 0;
            job = createJob();
            current().observation.treeComplete = false;
            auto [outputReader, outputWriter] = capturePipe(captureDrainBytes);
            auto [errorReader, errorWriter] = capturePipe(captureDrainBytes);
            auto [replyReader, replyWriter] = capturePipe(sizeof(consoleLaunch::Reply));
            auto input = nullStream();
            WindowsHandle acknowledgement(::CreateEventW(nullptr, TRUE, FALSE, nullptr));
            if (!acknowledgement)
                throw windowsError("cannot create console launch acknowledgement");

            auto acknowledgementCapability = inheritedCopy(acknowledgement.get(), SYNCHRONIZE);
            const auto snapshot = consoleLaunch::encode(executableFile.nativePath, command, workdir.nativePath,
                environment, configuration.maximumArgumentBytes, deadline);
            auto mapping = launchSnapshot(snapshot);
            const std::vector<HANDLE> inherited{input.get(), outputWriter.get(), errorWriter.get(),
                mapping.get(), replyWriter.get(), acknowledgementCapability.get()};
            HandleList selectedHandles(inherited);
            auto brokerCommand = commandLine({configuration.consoleSignalExecutable, "--launch-v1",
                std::to_string(reinterpret_cast<std::uintptr_t>(mapping.get())), std::to_string(snapshot.size()),
                std::to_string(reinterpret_cast<std::uintptr_t>(replyWriter.get())),
                std::to_string(reinterpret_cast<std::uintptr_t>(acknowledgementCapability.get()))});
            STARTUPINFOEXW startup{};
            startup.StartupInfo.cb = sizeof(startup);
            startup.StartupInfo.dwFlags = STARTF_USESTDHANDLES | STARTF_USESHOWWINDOW;
            startup.StartupInfo.wShowWindow = SW_HIDE;
            startup.StartupInfo.hStdInput = input.get();
            startup.StartupInfo.hStdOutput = outputWriter.get();
            startup.StartupInfo.hStdError = errorWriter.get();
            startup.lpAttributeList = selectedHandles.get();
            PROCESS_INFORMATION process{};
            stdoutPipe = std::move(outputReader);
            stderrPipe = std::move(errorReader);
            outputLimit = static_cast<std::size_t>(captureLimit);
            activeWorkdir = std::move(workdir);
            checkLaunch(token, deadline, "broker creation preparation");
            if (!::CreateProcessW(signalFile.nativePath.c_str(), brokerCommand.data(), nullptr, nullptr, TRUE,
                CREATE_SUSPENDED | CREATE_NEW_CONSOLE | CREATE_UNICODE_ENVIRONMENT | EXTENDED_STARTUPINFO_PRESENT,
                environment.data(), activeWorkdir.nativePath.c_str(), &startup.StartupInfo, &process))
                throw windowsError("selected Windows console launch broker spawn failed");

            // Store each creation before admission; failed nested-job assignment
            // leaves a suspended, individually disposable process in custody.
            broker.reset(process.hProcess);
            brokerThread.reset(process.hThread);
            launchState = LaunchState::BrokerCreated;
            outputWriter.reset();
            errorWriter.reset();
            replyWriter.reset();
            checkLaunch(token, deadline, "broker creation");
            if (!::AssignProcessToJobObject(job.get(), broker.get()))
                throw windowsError("cannot assign suspended console launch broker to the owned job");

            brokerAssigned = true;
            requireInJob(broker.get(), job.get());
            verifyProcessImage(broker.get(), signalFile);
            launchState = LaunchState::BrokerAdmitted;
            checkLaunch(token, deadline, "broker admission");
            if (::ResumeThread(brokerThread.get()) != 1)
                throw windowsError("cannot resume owned console launch broker");

            const auto reply = receiveLaunchReply(replyReader.get(), token, deadline);
            if (reply.stage != consoleLaunch::successStage)
                throw std::runtime_error("console launch broker refused launch: stage " +
                    std::to_string(reply.stage) + ", Win32 " + std::to_string(reply.error));

            if (reply.error != ERROR_SUCCESS || reply.processId == 0 || reply.threadId == 0)
                throw std::runtime_error("console launch broker returned invalid child identity");

            HANDLE adopted = nullptr;
            if (!::DuplicateHandle(broker.get(), consoleLaunch::handle(reply.processHandle), ::GetCurrentProcess(),
                &adopted, PROCESS_QUERY_LIMITED_INFORMATION | SYNCHRONIZE | PROCESS_TERMINATE, FALSE, 0))
                throw windowsError("cannot adopt suspended Windows CLI process capability");

            leader.reset(adopted);
            adopted = nullptr;
            checkLaunch(token, deadline, "child process adoption");
            if (!::DuplicateHandle(broker.get(), consoleLaunch::handle(reply.threadHandle), ::GetCurrentProcess(),
                &adopted, THREAD_QUERY_LIMITED_INFORMATION | THREAD_SUSPEND_RESUME | SYNCHRONIZE, FALSE, 0))
                throw windowsError("cannot adopt suspended Windows CLI thread capability");

            leaderThread.reset(adopted);
            if (::GetProcessId(leader.get()) != reply.processId || ::GetThreadId(leaderThread.get()) != reply.threadId ||
                ::GetProcessIdOfThread(leaderThread.get()) != reply.processId || terminated(leader.get()) ||
                terminated(leaderThread.get()))
                throw std::runtime_error("console launch child identity or suspended liveness admission failed");

            requireInJob(leader.get(), job.get());
            leaderAssigned = true;
            verifyProcessImage(leader.get(), executableFile);
            rootKnown = true;
            launchState = LaunchState::ChildAdopted;
            current().leaderPid = reply.processId;
            checkLaunch(token, deadline, "child admission");
            if (!::SetEvent(acknowledgement.get()))
                throw windowsError("cannot acknowledge suspended Windows CLI adoption");

            while (!terminated(broker.get()) || !terminated(brokerThread.get())) {
                checkLaunch(token, deadline, "broker retirement");
                ::Sleep(1);
            }

            DWORD brokerCode = 0;
            if (!::GetExitCodeProcess(broker.get(), &brokerCode))
                throw windowsError("cannot inspect console launch broker retirement");

            if (brokerCode != 0)
                throw std::runtime_error("console launch broker did not retire successfully before CLI resume");

            checkLaunch(token, deadline, "CLI resume preparation");
            if (::ResumeThread(leaderThread.get()) != 1)
                throw windowsError("cannot resume admitted job-owned Windows CLI");

            launchState = LaunchState::Running;
            checkLaunch(token, deadline, "CLI resume");
            return StartResult::Success(current().observation.id);
        } catch (const LaunchCancelled& error) {
            rememberFailure(error.what());
            return StartResult::Cancelled(error.what());
        } catch (const std::exception& error) {
            rememberFailure(error.what());
            return StartResult::Failure(error.what());
        }
    }

    consoleLaunch::Reply receiveLaunchReply(HANDLE pipe, Token token, ULONGLONG deadline) {
        consoleLaunch::Reply reply;
        std::size_t received = 0;
        while (true) {
            if (!token.is_cancelled() && ::GetTickCount64() >= deadline) {
                DWORD brokerCode = 0;
                const bool statusKnown = ::GetExitCodeProcess(broker.get(), &brokerCode) != FALSE;
                throw std::runtime_error("Windows console launch reply deadline exhausted: received_bytes=" +
                    std::to_string(received) + ", broker_status=" +
                    (statusKnown ? std::to_string(brokerCode) : std::string("unknown")) +
                    ", reply_stage=" + std::to_string(reply.stage) + "; custody retained");
            }

            checkLaunch(token, deadline, "reply and EOF");
            DWORD available = 0;
            if (!::PeekNamedPipe(pipe, nullptr, 0, nullptr, &available, nullptr)) {
                if (::GetLastError() == ERROR_BROKEN_PIPE && received == sizeof(reply))
                    break;

                throw std::runtime_error("console launch reply ended before its exact frame and EOF");
            }

            if (available > sizeof(reply) - received)
                throw std::runtime_error("console launch reply has trailing bytes");

            if (available != 0) {
                DWORD count = 0;
                if (!::ReadFile(pipe, reinterpret_cast<unsigned char*>(&reply) + received, available, &count, nullptr) ||
                    count == 0)
                    throw windowsError("cannot read available console launch reply");

                received += count;
            } else {
                ::Sleep(1);
            }
        }

        if (reply.protocolMagic != consoleLaunch::magic || reply.protocolVersion != consoleLaunch::version ||
            reply.bytes != sizeof(reply) || reply.reserved != 0)
            throw std::runtime_error("console launch reply protocol is incompatible");

        return reply;
    }

    StartResult startScope(ScopeRequest request, Token token = {}, ULONGLONG deadline = 0) {
        try {
            if (token.is_cancelled())
                return StartResult::Cancelled("scope query cancelled before CLI preparation");

            if (deadline != 0)
                checkLaunch(token, deadline, "scope preparation");

            if ((!request.scope.empty() && !request.packageName.empty()) ||
                (request.scope.empty() && request.packageName.empty() && !request.path.empty()))
                return StartResult::Failure("scope query requires at most one selection and paths require a selection");
            auto workdir = files.directory(request.workdir);
            Arguments arguments(configuration.maximumArgumentBytes);
            arguments.append(configuration.executable);
            arguments.append("scope");
            arguments.append("--workdir");
            arguments.append(windowsUtf8(workdir.nativePath));
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
            return start(std::move(workdir), arguments.take(), request.outputLimitBytes, true, pathQuery, token, deadline);
        } catch (const LaunchCancelled& error) {
            return StartResult::Cancelled(error.what());
        } catch (const std::exception& error) {
            return StartResult::Failure(error.what());
        }
    }

    void drainOutput(WindowsHandle& pipe, std::string& output, bool& truncated) {
        if (!pipe)
            return;
        // Retaining a bounded capture does not justify closing the reader and
        // inventing EOF. Discard overflow in bounded slices until real EOF.
        std::array<char, captureDrainBytes> buffer{};
        // One OS read per stream/update bounds observation work independently of
        // a caller choosing a very large retained-output allowance.
        DWORD available = 0;
        if (!::PeekNamedPipe(pipe.get(), nullptr, 0, nullptr, &available, nullptr)) {
            if (::GetLastError() == ERROR_BROKEN_PIPE) {
                pipe.reset();
                return;
            }
            throw windowsError("cannot inspect Windows CLI capture pipe");
        }
        if (available == 0)
            return;

        DWORD count = 0;
        const auto bytes = static_cast<DWORD>(std::min<std::size_t>(buffer.size(), available));
        if (!::ReadFile(pipe.get(), buffer.data(), bytes, &count, nullptr)) {
            if (::GetLastError() == ERROR_BROKEN_PIPE) {
                pipe.reset();
                return;
            }
            throw windowsError("cannot drain Windows CLI capture pipe");
        }
        if (count == 0)
            throw std::runtime_error("Windows capture read made no progress despite available bytes");

        const auto retain = std::min<std::size_t>(count, outputLimit - output.size());
        output.append(buffer.data(), retain);
        if (retain != count) {
            truncated = true;
            rememberFailure("CLI output exceeded its per-stream byte limit; retained capture is incomplete");
        }
    }

    bool terminated(HANDLE process) const {
        if (!process)
            return true;
        const auto result = ::WaitForSingleObject(process, 0);
        if (result != WAIT_OBJECT_0 && result != WAIT_TIMEOUT)
            throw windowsError("cannot inspect owned Windows process termination");
        return result == WAIT_OBJECT_0;
    }

    void update() {
        auto& observation = current().observation;
        if (observation.treeComplete)
            return;
        drainOutput(stdoutPipe, observation.stdoutText, observation.stdoutTruncated);
        drainOutput(stderrPipe, observation.stderrText, observation.stderrTruncated);
        observation.exited = rootKnown && terminated(leader.get());
        if (observation.exited) {
            DWORD code = 0;
            if (!::GetExitCodeProcess(leader.get(), &code))
                throw windowsError("cannot read owned Windows CLI exit status");
            observation.exitCode = code;
        }

        JOBOBJECT_BASIC_ACCOUNTING_INFORMATION accounting{};
        if (!::QueryInformationJobObject(job.get(), JobObjectBasicAccountingInformation,
            &accounting, sizeof(accounting), nullptr))
            throw windowsError("cannot inspect owned Windows job accounting");
        if (accounting.ActiveProcesses > accounting.TotalProcesses)
            throw std::runtime_error("Windows job returned inconsistent process accounting");
        if (accounting.TotalProcesses < previousTotalProcesses)
            throw std::runtime_error("Windows job process-accounting counter wrapped; completion proof unavailable");
        previousTotalProcesses = accounting.TotalProcesses;
        observation.reapedChildren = accounting.TotalProcesses - accounting.ActiveProcesses;
        const bool knownHandlesEnded = terminated(leader.get()) && terminated(leaderThread.get()) &&
            terminated(broker.get()) && terminated(brokerThread.get()) &&
            terminated(signalProcess.get()) && terminated(signalThread.get());
        // Unknown root results remain unknown even when all custody closes.
        observation.treeComplete = knownHandlesEnded && accounting.ActiveProcesses == 0 && !stdoutPipe && !stderrPipe;
        if (observation.treeComplete && rootKnown && launchState == LaunchState::Running && current().scopePathQuery &&
            current().infrastructureFailure.empty() && observation.scopePath.empty() &&
            !observation.forced && observation.exitCode == 0 && !observation.stdoutTruncated && !observation.stderrTruncated) {
            std::string decoded = observation.stdoutText;
            if (!decoded.empty() && decoded.back() == '\n')
                decoded.pop_back();
            if (!decoded.empty() && decoded.back() == '\r')
                decoded.pop_back();
            if (decoded.find_first_of("\r\n\0", 0, 3) != decoded.npos) {
                rememberFailure("scope service did not return one absolute Windows path");
            } else {
                try {
                    (void)windowsAbsolute(decoded);
                    observation.scopePath = std::move(decoded);
                } catch (const std::exception&) {
                    rememberFailure("scope service did not return one absolute Windows path");
                }
            }
        }
    }

    void forceOwned(Clock::time_point deadline) {
        current().observation.forced = true;
        const auto reconciliationDeadline = std::min(deadline, Clock::now() + std::chrono::milliseconds(250));
        std::string failures;
        const auto record = [&](const char* operation, DWORD error) {
            if (!failures.empty())
                failures += "; ";

            failures += std::string(operation) + ": Win32 " + std::to_string(error);
        };
        if (job && !::TerminateJobObject(job.get(), 137))
            record("cannot force disposal of owned Windows job", ::GetLastError());

        // Attempt every retained process even if job disposal or a previous
        // process failed. A denied termination is reconciled only on this handle.
        const auto terminate = [&](HANDLE process, const char* operation) {
            if (!process)
                return;

            const auto state = ::WaitForSingleObject(process, 0);
            if (state == WAIT_OBJECT_0)
                return;

            if (state != WAIT_TIMEOUT) {
                record(operation, ::GetLastError());
                return;
            }

            if (::TerminateProcess(process, 137))
                return;

            const auto error = ::GetLastError();
            if (error == ERROR_ACCESS_DENIED) {
                const auto remaining = std::chrono::duration_cast<std::chrono::milliseconds>(
                    reconciliationDeadline - Clock::now()).count();
                if (::WaitForSingleObject(process, static_cast<DWORD>(std::max<std::int64_t>(0, remaining))) == WAIT_OBJECT_0)
                    return;
            }

            record(operation, error);
        };
        terminate(broker.get(), "cannot dispose retained console launch broker");
        terminate(leader.get(), "cannot dispose retained Windows CLI");
        terminate(signalProcess.get(), "cannot dispose retained console signal helper");
        if (!failures.empty())
            throw std::runtime_error(failures);
    }

    ObservationResult wait(std::int64_t milliseconds, Token token, bool force) {
        const auto deadline = Clock::now() + std::chrono::milliseconds(milliseconds);
        if (token.is_cancelled())
            return ObservationResult::Cancelled("process observation cancelled; tree ownership retained");

        if (force)
            forceOwned(deadline);

        do {
            if (token.is_cancelled())
                return ObservationResult::Cancelled("process observation cancelled; tree ownership retained");
            update();
            if (current().observation.treeComplete || Clock::now() >= deadline)
                break;
            const auto remaining = std::chrono::duration_cast<std::chrono::milliseconds>(deadline - Clock::now()).count();
            ::Sleep(static_cast<DWORD>(std::clamp<std::int64_t>(remaining, 0, 10)));
        } while (Clock::now() < deadline);
        if (!force && !current().infrastructureFailure.empty())
            return ObservationResult::Failure(current().infrastructureFailure);
        return ObservationResult::Success(current().observation);
    }

    bool interrupt() {
        update();
        if (current().observation.treeComplete || current().observation.exited)
            return false;
        if (!leaderAssigned || launchState != LaunchState::Running)
            throw std::runtime_error("cannot gracefully interrupt CLI that did not complete admission and resume; custody retained");
        if (!terminated(signalProcess.get()) || !terminated(signalThread.get()))
            throw std::runtime_error("a previous console signal helper remains in custody");
        const auto deadline = ::GetTickCount64() + signalHelperBudget;
        const auto remainingBudget = [&]() -> DWORD {
            const auto now = ::GetTickCount64();
            if (now >= deadline)
                throw std::runtime_error("console signal helper exceeded its 1000ms deadline; custody retained");

            return static_cast<DWORD>(deadline - now);
        };
        requireInJob(leader.get(), job.get());
        auto processCapability = inheritedCopy(leader.get(), PROCESS_QUERY_LIMITED_INFORMATION | SYNCHRONIZE);
        auto jobCapability = inheritedCopy(job.get(), JOB_OBJECT_QUERY);
        auto input = nullStream();
        auto output = nullStream();
        auto [diagnosticReader, diagnosticWriter] = capturePipe();
        const std::vector<HANDLE> inherited{processCapability.get(), jobCapability.get(),
            input.get(), output.get(), diagnosticWriter.get()};
        HandleList selectedHandles(inherited);
        auto command = commandLine({configuration.consoleSignalExecutable,
            std::to_string(reinterpret_cast<std::uintptr_t>(processCapability.get())),
            std::to_string(reinterpret_cast<std::uintptr_t>(jobCapability.get())),
            std::to_string(current().leaderPid), std::to_string(configuration.maximumInventoryBytes),
            std::to_string(deadline)});
        STARTUPINFOEXW startup{};
        startup.StartupInfo.cb = sizeof(startup);
        startup.StartupInfo.dwFlags = STARTF_USESTDHANDLES;
        startup.StartupInfo.hStdInput = input.get();
        startup.StartupInfo.hStdOutput = output.get();
        startup.StartupInfo.hStdError = diagnosticWriter.get();
        startup.lpAttributeList = selectedHandles.get();
        PROCESS_INFORMATION process{};
        signalAssigned = false;
        (void)remainingBudget();
        if (!::CreateProcessW(signalFile.nativePath.c_str(), command.data(), nullptr, nullptr, TRUE,
            CREATE_SUSPENDED | DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP | CREATE_UNICODE_ENVIRONMENT |
                EXTENDED_STARTUPINFO_PRESENT,
            environment.data(), activeWorkdir.nativePath.c_str(), &startup.StartupInfo, &process))
            throw windowsError("cannot create owned console signal helper");
        signalProcess.reset(process.hProcess);
        signalThread.reset(process.hThread);
        // Only the helper retains the writer while running. The dedicated
        // diagnostic channel is separate from the CLI's stdout/stderr capture.
        diagnosticWriter.reset();
        // A failed helper admission is itself retained custody, even if the CLI
        // exits concurrently. It participates in every subsequent completion check.
        current().observation.treeComplete = false;
        (void)remainingBudget();
        if (!::AssignProcessToJobObject(job.get(), signalProcess.get()))
            throw windowsError("cannot assign suspended console signal helper to the owned job");
        signalAssigned = true;
        requireInJob(signalProcess.get(), job.get());
        verifyProcessImage(signalProcess.get(), signalFile);
        (void)remainingBudget();
        if (::ResumeThread(signalThread.get()) != 1)
            throw windowsError("cannot resume job-owned console signal helper");
        const auto waited = ::WaitForSingleObject(signalProcess.get(), remainingBudget());
        if (waited != WAIT_OBJECT_0 ||
            ::WaitForSingleObject(signalThread.get(), remainingBudget()) != WAIT_OBJECT_0)
            throw std::runtime_error("console signal helper did not complete within 1000ms; custody retained");
        DWORD code = 0;
        if (!::GetExitCodeProcess(signalProcess.get(), &code))
            throw windowsError("cannot inspect console signal helper status");
        if (code != 0)
            throw std::runtime_error("console signal helper refused graceful interruption: status " +
                std::to_string(code) + "; " + signalDiagnostic(diagnosticReader.get()));
        (void)remainingBudget();
        current().interruptRequested = true;
        return true;
    }

    ExclusiveClient exclusive;
    ClientConfiguration configuration;
    WindowsFiles files;
    std::int64_t clientId;
    WindowsPath executableFile;
    WindowsPath signalFile;
    WindowsPath selectedStateDirectory;
    WindowsPath selectedCacheDirectory;
    WindowsPath activeWorkdir;
    std::vector<wchar_t> environment;
    mutable std::mutex mutex;
    std::vector<RunReceipt> runs;
    WindowsHandle job;
    WindowsHandle broker;
    WindowsHandle brokerThread;
    WindowsHandle leader;
    WindowsHandle leaderThread;
    WindowsHandle signalProcess;
    WindowsHandle signalThread;
    WindowsHandle stdoutPipe;
    WindowsHandle stderrPipe;
    bool rootKnown = false;
    bool brokerAssigned = false;
    bool leaderAssigned = false;
    LaunchState launchState = LaunchState::Prepared;
    bool signalAssigned = false;
    std::size_t outputLimit = 0;
    DWORD previousTotalProcesses = 0;
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
        auto workdir = state.files.directory(request.workdir);
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
        arguments.append(windowsUtf8(workdir.nativePath));
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
        return state.start(std::move(workdir), arguments.take(), request.outputLimitBytes, false);
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
        auto workdir = state.files.directory(request.workdir);
        Arguments arguments(state.configuration.maximumArgumentBytes);
        arguments.append(state.configuration.executable);
        for (const auto& argument : request.arguments)
            arguments.append(argument);

        return state.start(std::move(workdir), arguments.take(), request.outputLimitBytes, false);
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
        return BoolResult::Success(state.interrupt());
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
    try {
        const auto deadline = ::GetTickCount64() + static_cast<ULONGLONG>(milliseconds);
        const auto started = state.startScope(std::move(request), token, deadline);
        if (started.is_cancelled())
            return ScopeQueryResult::Cancelled(started.error());

        if (!started.is_success())
            return ScopeQueryResult::Failure(started.error());
        const auto now = ::GetTickCount64();
        if (token.is_cancelled())
            return ScopeQueryResult::Cancelled("scope query cancelled after startup; receipt retained");

        if (now >= deadline)
            return ScopeQueryResult::Failure("scope query budget exhausted after startup; receipt retained");

        const auto remaining = static_cast<std::int64_t>(deadline - now);
        const auto waited = state.wait(remaining, token, false);
        if (waited.is_cancelled())
            return ScopeQueryResult::Cancelled(waited.error());
        if (!waited.is_success())
            return ScopeQueryResult::Failure(waited.error());
        if (::GetTickCount64() > deadline)
            return ScopeQueryResult::Failure("scope query budget exhausted during observation; receipt retained");

        const auto& observation = waited.value();
        if (!observation.treeComplete || !observation.exited)
            return ScopeQueryResult::Failure("scope query budget exhausted; operation custody and receipt retained");
        if (observation.forced || observation.exitCode != 0 ||
            observation.stdoutTruncated || observation.stderrTruncated)
            return ScopeQueryResult::Failure("scope query requires zero exit, complete capture and an unforced drained tree");
        ScopeResult result;
        result.isPath = state.current().scopePathQuery;
        if (result.isPath)
            result.path = observation.scopePath;
        else
            result.json = observation.stdoutText;
        return ScopeQueryResult::Success(std::move(result));
    } catch (const std::exception& error) {
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
        state.rememberFailure("native emergency disposal required; explicit cleanup did not establish a drained tree");
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
const std::string& Client::executableIdentity() const noexcept { return implementation_->executableFile.identity; }

std::size_t Client::maximumArgumentBytes() const noexcept {
    return implementation_->configuration.maximumArgumentBytes;
}

}
