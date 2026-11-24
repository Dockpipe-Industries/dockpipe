// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
// Bounded OS stimulus only. Source tests own observations and assertions.
#include "WindowsFiles.hpp"

#include <algorithm>
#include <array>
#include <cstdint>
#include <limits>
#include <utility>
#include <stdexcept>
#include <string>
#include <string_view>
#include <vector>

namespace {
HANDLE interrupted = nullptr;

BOOL WINAPI receiveBreak(DWORD event) {
    if (event != CTRL_BREAK_EVENT)
        return FALSE;
    return ::SetEvent(interrupted);
}

void output(HANDLE stream, std::string_view text) {
    while (!text.empty()) {
        DWORD count = 0;
        const auto requested = static_cast<DWORD>(std::min<std::size_t>(text.size(), 4096));
        if (!::WriteFile(stream, text.data(), requested, &count, nullptr) || count == 0)
            throw std::runtime_error("fixture output failed");
        text.remove_prefix(count);
    }
}

std::uint64_t boundedDecimal(std::wstring_view text, std::uint64_t maximum) {
    if (text.empty() || text.size() > 8)
        throw std::runtime_error("fixture numeric input is invalid");
    std::uint64_t result = 0;
    for (const auto character : text) {
        if (character < L'0' || character > L'9')
            throw std::runtime_error("fixture numeric input is invalid");
        result = result * 10 + character - L'0';
        if (result > maximum)
            throw std::runtime_error("fixture numeric input exceeds its bound");
    }
    return result;
}

void appendBounded(std::string& packet, std::string_view value, std::size_t maximumBytes) {
    if (packet.size() > maximumBytes || value.size() > maximumBytes - packet.size())
        throw std::runtime_error("fixture fact output exceeds its requested aggregate byte bound");

    packet.append(value);
}

void appendNumber(std::string& packet, std::uint64_t value, std::size_t maximumBytes) {
    appendBounded(packet, std::to_string(value), maximumBytes);
    appendBounded(packet, "\n", maximumBytes);
}

void appendField(std::string& packet, std::string_view value, std::size_t maximumBytes) {
    appendNumber(packet, value.size(), maximumBytes);
    appendBounded(packet, value, maximumBytes);
    appendBounded(packet, "\n", maximumBytes);
}

std::string actualDirectory() {
    const auto needed = ::GetCurrentDirectoryW(0, nullptr);
    if (needed == 0 || needed > 32767)
        throw std::runtime_error("fixture current directory exceeds the Windows character contract");

    std::wstring directory(needed, L'\0');
    const auto copied = ::GetCurrentDirectoryW(needed, directory.data());
    if (copied == 0 || copied >= needed)
        throw std::runtime_error("fixture current directory changed while observing it");

    directory.resize(copied);
    return dockpipe::sdk::windowsUtf8(directory);
}

std::pair<bool, std::wstring> environmentValue(const wchar_t* name) {
    ::SetLastError(ERROR_SUCCESS);
    const auto needed = ::GetEnvironmentVariableW(name, nullptr, 0);
    if (needed == 0) {
        const auto error = ::GetLastError();
        if (error == ERROR_ENVVAR_NOT_FOUND)
            return {false, {}};
        if (error == ERROR_SUCCESS)
            return {true, {}};

        throw std::runtime_error("fixture environment query failed: Win32 " + std::to_string(error));
    }
    if (needed > 32767)
        throw std::runtime_error("fixture environment value exceeds the Windows character contract");

    std::wstring value(needed, L'\0');
    ::SetLastError(ERROR_SUCCESS);
    const auto copied = ::GetEnvironmentVariableW(name, value.data(), needed);
    if (copied == 0) {
        if (::GetLastError() == ERROR_SUCCESS)
            return {true, {}};

        throw std::runtime_error("fixture environment changed while observing it");
    }
    if (copied >= needed)
        throw std::runtime_error("fixture environment changed while observing it");

    value.resize(copied);
    return {true, std::move(value)};
}

int argumentStats(int argc, wchar_t** argv, int begin) {
    std::uint64_t bytes = 0;
    unsigned char minimum = 0;
    unsigned char maximum = 0;
    bool anyByte = false;
    for (int index = begin; index < argc; ++index) {
        const auto value = dockpipe::sdk::windowsUtf8(argv[index]);
        if (value.size() > std::numeric_limits<std::uint64_t>::max() - bytes)
            throw std::runtime_error("fixture actual argv byte aggregate overflows");

        bytes += value.size();
        for (const unsigned char byte : value) {
            if (!anyByte) {
                minimum = byte;
                maximum = byte;
                anyByte = true;
            } else {
                minimum = std::min(minimum, byte);
                maximum = std::max(maximum, byte);
            }
        }
    }

    constexpr std::string_view header = "SDK_ARG_STATS_V1\n";
    constexpr auto maximumBytes = header.size() + 5 * (std::numeric_limits<std::uint64_t>::digits10 + 2);
    std::string packet(header);
    appendNumber(packet, static_cast<std::uint64_t>(argc), maximumBytes);
    appendNumber(packet, static_cast<std::uint64_t>(argc - begin), maximumBytes);
    appendNumber(packet, bytes, maximumBytes);
    appendNumber(packet, minimum, maximumBytes);
    appendNumber(packet, maximum, maximumBytes);
    output(::GetStdHandle(STD_OUTPUT_HANDLE), packet);
    return 0;
}

int childInputs(int argc, wchar_t** argv, int begin, std::size_t maximumBytes) {
    std::string packet;
    appendBounded(packet, "SDK_CHILD_INPUTS_V1\n", maximumBytes);
    appendField(packet, actualDirectory(), maximumBytes);
    appendNumber(packet, static_cast<std::uint64_t>(argc - begin), maximumBytes);
    for (int index = begin; index < argc; ++index) {
        appendField(packet, dockpipe::sdk::windowsUtf8(argv[index]), maximumBytes);
        const auto [present, value] = environmentValue(argv[index]);
        appendNumber(packet, present ? 1 : 0, maximumBytes);
        appendField(packet, dockpipe::sdk::windowsUtf8(value), maximumBytes);
    }

    output(::GetStdHandle(STD_OUTPUT_HANDLE), packet);
    return 0;
}

int hold(bool ignore, DWORD milliseconds, bool child) {
    dockpipe::sdk::WindowsHandle event(::CreateEventW(nullptr, TRUE, FALSE, nullptr));
    if (!event)
        throw std::runtime_error("fixture interrupt event creation failed");
    interrupted = event.get();
    if (!::SetConsoleCtrlHandler(receiveBreak, TRUE))
        throw std::runtime_error("fixture console handler installation failed");
    output(::GetStdHandle(STD_OUTPUT_HANDLE), child ? "SDK_CHILD_READY\n" : "SDK_READY\n");

    const auto deadline = ::GetTickCount64() + milliseconds;
    while (true) {
        const auto now = ::GetTickCount64();
        const auto remaining = now < deadline ? static_cast<DWORD>(deadline - now) : 0;
        const auto waited = ::WaitForSingleObject(event.get(), remaining);
        if (waited == WAIT_TIMEOUT)
            return 124;
        if (waited != WAIT_OBJECT_0)
            throw std::runtime_error("fixture console wait failed");

        output(::GetStdHandle(STD_OUTPUT_HANDLE),
            child ? "SDK_CHILD_BREAK_RECEIVED\n" : "SDK_BREAK_RECEIVED\n");
        if (!ignore)
            return 130;

        if (!::ResetEvent(event.get()))
            throw std::runtime_error("fixture interrupt observation reset failed");
    }
}

void descendant(const std::wstring& executable, DWORD milliseconds, bool newGroup) {
    auto command = dockpipe::sdk::quoteWindowsArgument(executable) + L" -- child-hold " +
        std::to_wstring(milliseconds);
    const std::array<HANDLE, 3> streams{::GetStdHandle(STD_INPUT_HANDLE),
        ::GetStdHandle(STD_OUTPUT_HANDLE), ::GetStdHandle(STD_ERROR_HANDLE)};
    SIZE_T bytes = 0;
    (void)::InitializeProcThreadAttributeList(nullptr, 1, 0, &bytes);
    if (bytes == 0)
        throw std::runtime_error("fixture inherited-handle sizing failed");
    std::vector<unsigned char> storage(bytes);
    auto attributes = reinterpret_cast<LPPROC_THREAD_ATTRIBUTE_LIST>(storage.data());
    if (!::InitializeProcThreadAttributeList(attributes, 1, 0, &bytes))
        throw std::runtime_error("fixture inherited-handle initialization failed");
    struct AttributeGuard {
        LPPROC_THREAD_ATTRIBUTE_LIST value;
        ~AttributeGuard() { ::DeleteProcThreadAttributeList(value); }
    } attributeGuard{attributes};
    if (!::UpdateProcThreadAttribute(attributes, 0, PROC_THREAD_ATTRIBUTE_HANDLE_LIST,
        const_cast<HANDLE*>(streams.data()), sizeof(streams), nullptr, nullptr))
        throw std::runtime_error("fixture inherited-handle selection failed");

    STARTUPINFOEXW startup{};
    startup.StartupInfo.cb = sizeof(startup);
    startup.StartupInfo.dwFlags = STARTF_USESTDHANDLES;
    startup.StartupInfo.hStdInput = streams[0];
    startup.StartupInfo.hStdOutput = streams[1];
    startup.StartupInfo.hStdError = streams[2];
    startup.lpAttributeList = attributes;
    PROCESS_INFORMATION process{};
    const DWORD flags = CREATE_SUSPENDED | EXTENDED_STARTUPINFO_PRESENT |
        (newGroup ? CREATE_NEW_PROCESS_GROUP : 0);
    if (!::CreateProcessW(executable.c_str(), command.data(), nullptr, nullptr, TRUE,
        flags, nullptr, nullptr, &startup.StartupInfo, &process))
        throw std::runtime_error("fixture descendant creation failed");
    dockpipe::sdk::WindowsHandle ownedProcess(process.hProcess);
    dockpipe::sdk::WindowsHandle ownedThread(process.hThread);
    BOOL member = FALSE;
    if (!::IsProcessInJob(ownedProcess.get(), nullptr, &member) || !member ||
        ::ResumeThread(ownedThread.get()) != 1) {
        (void)::TerminateProcess(ownedProcess.get(), 125);
        throw std::runtime_error("fixture descendant could not retain inherited job custody");
    }
}
}

int wmain(int argc, wchar_t** argv) {
    try {
        int separator = 1;
        while (separator < argc && std::wstring_view(argv[separator]) != L"--")
            ++separator;
        if (separator + 1 >= argc)
            throw std::runtime_error("fixture requires a mode after the SDK argument separator");
        const std::wstring_view mode(argv[separator + 1]);
        if (mode == L"echo") {
            for (int index = separator + 2; index < argc; ++index) {
                const auto value = dockpipe::sdk::windowsUtf8(argv[index]);
                output(::GetStdHandle(STD_OUTPUT_HANDLE), std::to_string(value.size()) + ":" + value + "\n");
            }
            output(::GetStdHandle(STD_ERROR_HANDLE), "SDK_STDERR\n");
            return 23;
        }
        if (mode == L"arg-stats")
            return argumentStats(argc, argv, separator + 2);
        if (mode == L"child-inputs") {
            if (separator + 2 >= argc)
                throw std::runtime_error("fixture child inputs require an aggregate output byte limit");

            const auto maximumBytes = boundedDecimal(argv[separator + 2], 16 * 1024 * 1024);
            return childInputs(argc, argv, separator + 3, static_cast<std::size_t>(maximumBytes));
        }
        if (separator + 3 != argc)
            throw std::runtime_error("fixture mode requires exactly one bounded numeric operand");

        if (mode == L"flood") {
            auto remaining = boundedDecimal(argv[separator + 2], 16 * 1024 * 1024);
            const auto deadline = ::GetTickCount64() + 10000;
            std::array<char, 4096> bytes{};
            bytes.fill('x');
            while (remaining != 0 && ::GetTickCount64() < deadline) {
                const auto count = static_cast<std::size_t>(std::min<std::uint64_t>(remaining, bytes.size()));
                output(::GetStdHandle(STD_OUTPUT_HANDLE), std::string_view(bytes.data(), count));
                remaining -= count;
            }
            return remaining == 0 ? 0 : 124;
        }

        const auto milliseconds = static_cast<DWORD>(boundedDecimal(argv[separator + 2], 10000));
        if (mode == L"hold" || mode == L"ignore" || mode == L"child-hold")
            return hold(mode != L"hold", milliseconds, mode == L"child-hold");
        if (mode == L"root-exit" || mode == L"group-child") {
            std::wstring executable(32767, L'\0');
            const auto length = ::GetModuleFileNameW(nullptr, executable.data(), static_cast<DWORD>(executable.size()));
            if (length == 0 || length >= executable.size())
                throw std::runtime_error("fixture executable identity is unavailable");
            executable.resize(length);
            descendant(executable, milliseconds, mode == L"group-child");
            if (mode == L"group-child")
                return hold(false, milliseconds, false);
            output(::GetStdHandle(STD_OUTPUT_HANDLE), "SDK_ROOT_EXIT\n");
            return 0;
        }
        throw std::runtime_error("unknown fixture mode");
    } catch (const std::exception& error) {
        try {
            output(::GetStdHandle(STD_ERROR_HANDLE), std::string(error.what()) + "\n");
        } catch (...) {
        }
        return 125;
    }
}
