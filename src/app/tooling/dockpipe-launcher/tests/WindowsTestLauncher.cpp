#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <windows.h>
#include "WindowsTestDesktop.h"

#include <cstdio>
#include <cwchar>
#include <sstream>
#include <string>
#include <vector>

namespace {
constexpr DWORD ChildDeadlineMilliseconds = 45000;
constexpr DWORD CleanupDeadlineMilliseconds = 3000;
constexpr DWORD MaximumTokenBytes = 64 * 1024;
constexpr DWORD SelfTestChildExitCode = 23;
// The observed desktop import graph has 273 DLLs. This diagnostic admits at
// most 512 paths and 1 MiB of UTF-8 metadata, never an unbounded file inventory.
constexpr DWORD MaximumRuntimeManifestBytes = 1024 * 1024;
constexpr unsigned MaximumRuntimePaths = 512;
constexpr DWORD MaximumWindowsPathCharacters = 32768;

class Handle final {
public:
    Handle() = default;
    explicit Handle(HANDLE value) : m_value(value) {}
    ~Handle() { reset(); }
    Handle(const Handle &) = delete;
    Handle &operator=(const Handle &) = delete;

    HANDLE get() const { return m_value; }
    HANDLE *put()
    {
        reset();
        return &m_value;
    }
    explicit operator bool() const { return m_value && m_value != INVALID_HANDLE_VALUE; }
    void reset(HANDLE value = nullptr)
    {
        if (*this)
            CloseHandle(m_value);
        m_value = value;
    }

private:
    HANDLE m_value = nullptr;
};

bool error(const char *operation, DWORD code = GetLastError())
{
    char message[512]{};
    FormatMessageA(FORMAT_MESSAGE_FROM_SYSTEM | FORMAT_MESSAGE_IGNORE_INSERTS,
        nullptr, code, 0, message, static_cast<DWORD>(sizeof(message)), nullptr);
    std::fprintf(stderr, "Windows test launcher: %s failed (GetLastError=%lu): %s\n",
        operation, static_cast<unsigned long>(code), message);
    return false;
}

template<typename Value>
bool tokenValue(HANDLE token, TOKEN_INFORMATION_CLASS information, Value &value)
{
    DWORD length = 0;
    if (!GetTokenInformation(token, information, &value, sizeof(value), &length))
        return error("GetTokenInformation");
    if (length != sizeof(value))
        return error("GetTokenInformation size", ERROR_INVALID_DATA);
    return true;
}

bool tokenBuffer(HANDLE token, TOKEN_INFORMATION_CLASS information, std::vector<BYTE> &buffer)
{
    DWORD length = 0;
    const BOOL queried = GetTokenInformation(token, information, nullptr, 0, &length);
    const DWORD queryError = GetLastError();
    if (!queried && queryError != ERROR_INSUFFICIENT_BUFFER)
        return error("GetTokenInformation buffer length", queryError);
    if (queried || length == 0 || length > MaximumTokenBytes)
        return error("GetTokenInformation buffer capacity", ERROR_INVALID_DATA);
    buffer.resize(length);
    if (!GetTokenInformation(token, information, buffer.data(), length, &length))
        return error("GetTokenInformation buffer");
    return true;
}

struct TokenFacts {
    TOKEN_ELEVATION elevation{};
    TOKEN_ELEVATION_TYPE elevationType = TokenElevationTypeDefault;
    DWORD session = 0;
    DWORD integrity = 0;
    DWORD uiAccess = 0;
    DWORD appContainer = 0;
    std::vector<BYTE> user;

    PSID userSid() const
    {
        return reinterpret_cast<const TOKEN_USER *>(user.data())->User.Sid;
    }
};

bool readFacts(HANDLE token, TokenFacts &facts)
{
    std::vector<BYTE> label;
    if (!tokenValue(token, TokenElevation, facts.elevation)
        || !tokenValue(token, TokenElevationType, facts.elevationType)
        || !tokenValue(token, TokenSessionId, facts.session)
        || !tokenValue(token, TokenUIAccess, facts.uiAccess)
        || !tokenValue(token, TokenIsAppContainer, facts.appContainer)
        || !tokenBuffer(token, TokenUser, facts.user)
        || !tokenBuffer(token, TokenIntegrityLevel, label))
        return false;
    const auto *mandatory = reinterpret_cast<const TOKEN_MANDATORY_LABEL *>(label.data());
    if (!IsValidSid(facts.userSid()) || !IsValidSid(mandatory->Label.Sid)
        || *GetSidSubAuthorityCount(mandatory->Label.Sid) == 0)
        return error("token SID validation", ERROR_INVALID_SID);
    const DWORD last = *GetSidSubAuthorityCount(mandatory->Label.Sid) - 1;
    facts.integrity = *GetSidSubAuthority(mandatory->Label.Sid, last);
    return true;
}

void reportFacts(const char *label, const TokenFacts &facts)
{
    std::fprintf(stdout,
        "Windows test launcher: %s elevated=%lu elevationType=%u integrity=%lu session=%lu uiAccess=%lu appContainer=%lu\n",
        label, static_cast<unsigned long>(facts.elevation.TokenIsElevated),
        static_cast<unsigned>(facts.elevationType), static_cast<unsigned long>(facts.integrity),
        static_cast<unsigned long>(facts.session), static_cast<unsigned long>(facts.uiAccess),
        static_cast<unsigned long>(facts.appContainer));
    std::fflush(stdout);
}

bool verifiedContext(const TokenFacts &facts, const TokenFacts &original)
{
    if (facts.elevation.TokenIsElevated != 0 || facts.integrity != SECURITY_MANDATORY_MEDIUM_RID
        || facts.uiAccess != 0 || facts.appContainer != 0 || facts.session != original.session
        || !EqualSid(facts.userSid(), original.userSid()))
        return error("same-user unelevated medium token verification", ERROR_ACCESS_DENIED);
    return true;
}

bool lowerIntegrity(HANDLE token)
{
    alignas(SID) BYTE sid[SECURITY_MAX_SID_SIZE]{};
    DWORD sidLength = sizeof(sid);
    if (!CreateWellKnownSid(WinMediumLabelSid, nullptr, sid, &sidLength))
        return error("CreateWellKnownSid medium integrity");
    TOKEN_MANDATORY_LABEL label{};
    label.Label.Sid = sid;
    label.Label.Attributes = SE_GROUP_INTEGRITY;
    if (!SetTokenInformation(token, TokenIntegrityLevel, &label,
            static_cast<DWORD>(sizeof(label)) + GetLengthSid(sid)))
        return error("SetTokenInformation medium integrity");
    return true;
}

bool absoluteExecutable(const wchar_t *argument, std::wstring &executable)
{
    const std::wstring input(argument);
    const bool driveAbsolute = input.size() >= 3 && input[1] == L':'
        && (input[2] == L'\\' || input[2] == L'/');
    const bool uncAbsolute = input.size() >= 3 && input[0] == L'\\' && input[1] == L'\\';
    if ((!driveAbsolute && !uncAbsolute) || input.find(L'"') != std::wstring::npos)
        return error("absolute executable path", ERROR_BAD_PATHNAME);
    executable.resize(32768);
    const DWORD length = GetFullPathNameW(input.c_str(), static_cast<DWORD>(executable.size()),
        executable.data(), nullptr);
    if (length == 0)
        return error("GetFullPathNameW");
    if (length >= executable.size())
        return error("executable path length", ERROR_FILENAME_EXCED_RANGE);
    executable.resize(length);
    const DWORD attributes = GetFileAttributesW(executable.c_str());
    if (attributes == INVALID_FILE_ATTRIBUTES)
        return error("GetFileAttributesW executable");
    if ((attributes & FILE_ATTRIBUTE_DIRECTORY) || executable.size() < 4
        || _wcsicmp(executable.c_str() + executable.size() - 4, L".exe") != 0)
        return error("regular Windows executable", ERROR_BAD_EXE_FORMAT);
    return true;
}

bool ownExecutable(std::wstring &executable)
{
    std::wstring path(MaximumWindowsPathCharacters, L'\0');
    const DWORD length = GetModuleFileNameW(nullptr, path.data(), static_cast<DWORD>(path.size()));
    if (length == 0 || length >= path.size())
        return error("GetModuleFileNameW launcher", length == 0 ? GetLastError() : ERROR_FILENAME_EXCED_RANGE);
    path.resize(length);
    return absoluteExecutable(path.c_str(), executable);
}

bool probeRuntimeAccess(const wchar_t *manifest)
{
    std::vector<wchar_t> environment(MaximumWindowsPathCharacters);
    DWORD length = GetEnvironmentVariableW(L"PATH", environment.data(), static_cast<DWORD>(environment.size()));
    if (length == 0 || length >= environment.size())
        return error("restricted child PATH", length == 0 ? GetLastError() : ERROR_BUFFER_OVERFLOW);
    std::fprintf(stdout, "Windows runtime access: restricted child PATH=%ls\n", environment.data());
    length = GetCurrentDirectoryW(static_cast<DWORD>(environment.size()), environment.data());
    if (length == 0 || length >= environment.size())
        return error("restricted child working directory", length == 0 ? GetLastError() : ERROR_BUFFER_OVERFLOW);
    std::fprintf(stdout, "Windows runtime access: restricted child cwd=%ls\n", environment.data());

    Handle file(CreateFileW(manifest, GENERIC_READ, FILE_SHARE_READ, nullptr, OPEN_EXISTING,
        FILE_ATTRIBUTE_NORMAL | FILE_FLAG_OPEN_REPARSE_POINT, nullptr));
    if (!file)
        return error("open runtime manifest");
    BY_HANDLE_FILE_INFORMATION information{};
    LARGE_INTEGER size{};
    if (!GetFileInformationByHandle(file.get(), &information) || !GetFileSizeEx(file.get(), &size))
        return error("inspect runtime manifest");
    if (GetFileType(file.get()) != FILE_TYPE_DISK
        || (information.dwFileAttributes & (FILE_ATTRIBUTE_DIRECTORY | FILE_ATTRIBUTE_REPARSE_POINT))
        || size.QuadPart <= 0 || size.QuadPart > MaximumRuntimeManifestBytes)
        return error("regular bounded runtime manifest", ERROR_INVALID_DATA);
    std::string bytes(static_cast<size_t>(size.QuadPart), '\0');
    DWORD readBytes = 0;
    if (!ReadFile(file.get(), bytes.data(), static_cast<DWORD>(bytes.size()), &readBytes, nullptr))
        return error("read runtime manifest");
    if (readBytes != bytes.size())
        return error("complete runtime manifest", ERROR_INVALID_DATA);

    std::istringstream lines(bytes);
    std::string line;
    unsigned pathCount = 0;
    unsigned deniedCount = 0;
    while (std::getline(lines, line)) {
        if (!line.empty() && line.back() == '\r')
            line.pop_back();
        if (line.empty())
            continue;
        ++pathCount;
        if (pathCount > MaximumRuntimePaths || line.find('\0') != std::string::npos)
            return error("runtime manifest path capacity", ERROR_INVALID_DATA);
        const int characters = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS,
            line.data(), static_cast<int>(line.size()), nullptr, 0);
        if (characters <= 0 || static_cast<DWORD>(characters) >= MaximumWindowsPathCharacters)
            return error("runtime manifest UTF-8 path", ERROR_INVALID_DATA);
        std::wstring path(static_cast<size_t>(characters), L'\0');
        if (MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, line.data(),
                static_cast<int>(line.size()), path.data(), characters) != characters)
            return error("convert runtime manifest path");
        if (path.size() < 3 || path[1] != L':' || (path[2] != L'\\' && path[2] != L'/'))
            return error("absolute runtime manifest path", ERROR_BAD_PATHNAME);
        Handle dependency(CreateFileW(path.c_str(), GENERIC_READ | GENERIC_EXECUTE,
            FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE, nullptr, OPEN_EXISTING,
            FILE_ATTRIBUTE_NORMAL, nullptr));
        if (dependency) {
            std::fprintf(stdout, "Windows runtime access: readable-executable=%ls\n", path.c_str());
        } else {
            const DWORD accessError = GetLastError();
            ++deniedCount;
            std::fprintf(stderr, "Windows runtime access: denied=%ls GetLastError=%lu\n",
                path.c_str(), static_cast<unsigned long>(accessError));
        }
    }
    std::fprintf(stdout, "Windows runtime access: checked=%u denied=%u; no DLLs loaded\n",
        pathCount, deniedCount);
    return std::fflush(nullptr) == 0 && pathCount != 0 && deniedCount == 0;
}

class StartupHandles final {
public:
    ~StartupHandles()
    {
        if (m_initialized)
            DeleteProcThreadAttributeList(m_startup.lpAttributeList);
    }

    bool initialize()
    {
        const DWORD desktopError = WindowsTestDesktop::currentName(m_desktop);
        if (desktopError != ERROR_SUCCESS)
            return error("current window station/desktop", desktopError);
        const DWORD names[] = {STD_INPUT_HANDLE, STD_OUTPUT_HANDLE, STD_ERROR_HANDLE};
        for (unsigned index = 0; index < 3; ++index) {
            const HANDLE source = GetStdHandle(names[index]);
            if (!source || source == INVALID_HANDLE_VALUE)
                return error("valid inherited standard handle", ERROR_INVALID_HANDLE);
            if (!DuplicateHandle(GetCurrentProcess(), source, GetCurrentProcess(), m_owned[index].put(),
                    0, TRUE, DUPLICATE_SAME_ACCESS))
                return error("DuplicateHandle standard handle");
            m_values[index] = m_owned[index].get();
        }
        SIZE_T size = 0;
        InitializeProcThreadAttributeList(nullptr, 1, 0, &size);
        if (size == 0 || size > MaximumTokenBytes)
            return error("startup attribute capacity", ERROR_INVALID_DATA);
        m_attributes.resize(size);
        m_startup.StartupInfo.cb = sizeof(m_startup);
        m_startup.StartupInfo.dwFlags = STARTF_USESTDHANDLES;
        m_startup.StartupInfo.hStdInput = m_values[0];
        m_startup.StartupInfo.hStdOutput = m_values[1];
        m_startup.StartupInfo.hStdError = m_values[2];
        m_startup.StartupInfo.lpDesktop = m_desktop.data();
        m_startup.lpAttributeList = reinterpret_cast<LPPROC_THREAD_ATTRIBUTE_LIST>(m_attributes.data());
        if (!InitializeProcThreadAttributeList(m_startup.lpAttributeList, 1, 0, &size))
            return error("InitializeProcThreadAttributeList");
        m_initialized = true;
        if (!UpdateProcThreadAttribute(m_startup.lpAttributeList, 0, PROC_THREAD_ATTRIBUTE_HANDLE_LIST,
                m_values, sizeof(m_values), nullptr, nullptr))
            return error("UpdateProcThreadAttribute handle allowlist");
        return true;
    }

    STARTUPINFOEXW *get() { return &m_startup; }

private:
    Handle m_owned[3];
    HANDLE m_values[3]{};
    std::vector<BYTE> m_attributes;
    STARTUPINFOEXW m_startup{};
    std::wstring m_desktop;
    bool m_initialized = false;
};

bool runChild(const std::wstring &executable, const wchar_t *internalArgument, HANDLE token,
    const TokenFacts &original, DWORD &exitCode, DWORD deadline = ChildDeadlineMilliseconds,
    bool expectTimeout = false)
{
    Handle job(CreateJobObjectW(nullptr, nullptr));
    if (!job)
        return error("CreateJobObjectW");
    JOBOBJECT_EXTENDED_LIMIT_INFORMATION limits{};
    limits.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
    if (!SetInformationJobObject(job.get(), JobObjectExtendedLimitInformation, &limits, sizeof(limits)))
        return error("SetInformationJobObject kill-on-close");
    StartupHandles startup;
    if (!startup.initialize())
        return false;
    std::wstring commandLine = L"\"" + executable + L"\"";
    if (internalArgument)
        commandLine += std::wstring(L" ") + internalArgument;
    PROCESS_INFORMATION process{};
    const DWORD flags = CREATE_SUSPENDED | EXTENDED_STARTUPINFO_PRESENT;
    const BOOL created = token
        ? CreateProcessAsUserW(token, executable.c_str(), commandLine.data(), nullptr, nullptr, TRUE,
            flags, nullptr, nullptr, &startup.get()->StartupInfo, &process)
        : CreateProcessW(executable.c_str(), commandLine.data(), nullptr, nullptr, TRUE,
            flags, nullptr, nullptr, &startup.get()->StartupInfo, &process);
    if (!created)
        return error(token ? "CreateProcessAsUserW" : "CreateProcessW");
    Handle processHandle(process.hProcess);
    Handle threadHandle(process.hThread);
    if (!AssignProcessToJobObject(job.get(), processHandle.get())) {
        const DWORD assignmentError = GetLastError();
        // This is our exact suspended child; it has never executed test code.
        TerminateProcess(processHandle.get(), ERROR_PROCESS_ABORTED);
        WaitForSingleObject(processHandle.get(), CleanupDeadlineMilliseconds);
        return error("AssignProcessToJobObject", assignmentError);
    }
    const auto stopOwnedChild = [&]() {
        job.reset();
        if (WaitForSingleObject(processHandle.get(), CleanupDeadlineMilliseconds) != WAIT_OBJECT_0)
            return error("owned child cleanup deadline", ERROR_TIMEOUT);
        return true;
    };
    Handle childToken;
    TokenFacts childFacts;
    if (!OpenProcessToken(processHandle.get(), TOKEN_QUERY, childToken.put())) {
        const DWORD tokenError = GetLastError();
        stopOwnedChild();
        return error("OpenProcessToken suspended child", tokenError);
    }
    if (!readFacts(childToken.get(), childFacts)) {
        stopOwnedChild();
        return false;
    }
    reportFacts("suspended child", childFacts);
    if (!verifiedContext(childFacts, original)) {
        stopOwnedChild();
        return false;
    }
    if (ResumeThread(threadHandle.get()) == static_cast<DWORD>(-1)) {
        const DWORD resumeError = GetLastError();
        stopOwnedChild();
        return error("ResumeThread", resumeError);
    }
    threadHandle.reset();
    const DWORD wait = WaitForSingleObject(processHandle.get(), deadline);
    if (wait != WAIT_OBJECT_0) {
        const DWORD waitError = wait == WAIT_TIMEOUT ? ERROR_TIMEOUT : GetLastError();
        const bool stopped = stopOwnedChild();
        if (stopped && expectTimeout && wait == WAIT_TIMEOUT) {
            std::fprintf(stdout, "Windows test launcher: self-test timeout terminated the owned child job\n");
            return std::fflush(stdout) == 0;
        }
        return error("owned test process deadline", waitError);
    }
    if (!GetExitCodeProcess(processHandle.get(), &exitCode))
        return error("GetExitCodeProcess");
    // Any surviving descendants also belong to this fixture job.
    job.reset();
    if (expectTimeout)
        return error("self-test stall unexpectedly exited", ERROR_INVALID_DATA);
    return true;
}
}

int wmain(int argc, wchar_t **argv)
{
    // Report unattended loader/process failures through the exit code instead
    // of leaving an OS error dialog open until the fixture deadline.
    SetErrorMode(GetErrorMode() | SEM_FAILCRITICALERRORS
        | SEM_NOGPFAULTERRORBOX | SEM_NOOPENFILEERRORBOX);
    const bool runtimeProbeChild = argc == 3 && std::wcscmp(argv[1], L"--probe-runtime-child") == 0;
    const bool runtimeProbeRequest = argc == 3 && std::wcscmp(argv[1], L"--probe-runtime") == 0;
    if (argc != 2 && !runtimeProbeChild && !runtimeProbeRequest) {
        std::fprintf(stderr, "Usage: windows-test-launcher <absolute-test.exe> | --self-test | --probe-runtime <manifest>\n");
        return 1;
    }
    Handle ownToken;
    if (!OpenProcessToken(GetCurrentProcess(), TOKEN_QUERY, ownToken.put())) {
        error("OpenProcessToken launcher");
        return 1;
    }
    TokenFacts original;
    if (!readFacts(ownToken.get(), original))
        return 1;
    reportFacts("launcher", original);
    if (runtimeProbeChild) {
        if (!verifiedContext(original, original))
            return 1;
        return probeRuntimeAccess(argv[2]) ? 0 : 1;
    }
    const bool selfTestChild = std::wcscmp(argv[1], L"--self-test-child") == 0;
    const bool selfTestStall = std::wcscmp(argv[1], L"--self-test-stall") == 0;
    if (selfTestChild || selfTestStall) {
        if (!verifiedContext(original, original))
            return 1;
        if (selfTestChild) {
            const HWND canary = CreateWindowExW(0, L"STATIC", L"Dockpipe owned restricted-token self-test",
                WS_OVERLAPPEDWINDOW, 0, 0, 160, 120, nullptr, nullptr, GetModuleHandleW(nullptr), nullptr);
            if (!canary) {
                error("CreateWindowExW inherited desktop self-test");
                return 1;
            }
            if (!DestroyWindow(canary)) {
                error("DestroyWindow owned self-test window");
                return 1;
            }
        }
        if (std::fprintf(stdout, "Windows test launcher: controlled self-test child stdout canary\n") < 0
            || std::fflush(stdout) != 0)
            return 1;
        std::fprintf(stderr, "Windows test launcher: self-test stdout successfully written/flushed\n");
        if (selfTestStall)
            Sleep(INFINITE);
        return SelfTestChildExitCode;
    }
    const bool selfTest = std::wcscmp(argv[1], L"--self-test") == 0;
    std::wstring executable;
    if (selfTest || runtimeProbeRequest) {
        if (!ownExecutable(executable))
            return 1;
    } else if (!absoluteExecutable(argv[1], executable)) {
        return 1;
    }
    Handle restrictedToken;
    if (original.elevation.TokenIsElevated != 0) {
        Handle sourceToken;
        if (!OpenProcessToken(GetCurrentProcess(), TOKEN_QUERY | TOKEN_DUPLICATE | TOKEN_ASSIGN_PRIMARY
                | TOKEN_ADJUST_DEFAULT, sourceToken.put())) {
            error("OpenProcessToken restriction source");
            return 1;
        }
        if (!CreateRestrictedToken(sourceToken.get(), LUA_TOKEN | DISABLE_MAX_PRIVILEGE,
                0, nullptr, 0, nullptr, 0, nullptr, restrictedToken.put())) {
            error("CreateRestrictedToken LUA");
            return 1;
        }
        if (!lowerIntegrity(restrictedToken.get()))
            return 1;
        TokenFacts restricted;
        if (!readFacts(restrictedToken.get(), restricted))
            return 1;
        reportFacts("restricted launcher token", restricted);
        if (!verifiedContext(restricted, original))
            return 1;
    } else if (!verifiedContext(original, original)) {
        return 1;
    }
    std::wstring probeArguments;
    const wchar_t *childArgument = nullptr;
    if (selfTest) {
        childArgument = L"--self-test-child";
    } else if (runtimeProbeRequest) {
        if (std::wcschr(argv[2], L'"')) {
            error("runtime manifest argument", ERROR_INVALID_NAME);
            return 1;
        }
        probeArguments = L"--probe-runtime-child \"" + std::wstring(argv[2]) + L"\"";
        childArgument = probeArguments.c_str();
    }
    DWORD exitCode = 1;
    if (!runChild(executable, childArgument,
            restrictedToken.get(), original, exitCode))
        return 1;
    if (runtimeProbeRequest)
        return exitCode == 0 ? 0 : 1;
    if (selfTest) {
        if (exitCode != SelfTestChildExitCode) {
            std::fprintf(stderr, "Windows test launcher: self-test child returned %lu, expected %lu\n",
                static_cast<unsigned long>(exitCode), static_cast<unsigned long>(SelfTestChildExitCode));
            return 1;
        }
        DWORD unusedExitCode = 0;
        if (!runChild(executable, L"--self-test-stall", restrictedToken.get(), original,
                unusedExitCode, 100, true))
            return 1;
        std::fprintf(stdout, "Windows test launcher: self-test passed; exact child exit=%lu and bounded job cleanup verified\n",
            static_cast<unsigned long>(exitCode));
        return std::fflush(stdout) == 0 ? 0 : 1;
    }
    std::fprintf(stderr, "Windows test launcher: test exited with code %lu (0x%08lx)\n",
        static_cast<unsigned long>(exitCode), static_cast<unsigned long>(exitCode));
    std::fflush(nullptr);
    ExitProcess(exitCode);
}
