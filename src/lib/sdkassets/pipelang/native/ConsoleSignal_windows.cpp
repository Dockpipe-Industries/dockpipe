// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
// Separate executable: attaching a console must never mutate the SDK caller.
#include "WindowsFiles.hpp"
#include "ConsoleLaunchProtocol.hpp"

#include <cstdio>
#include <cstdint>
#include <limits>
#include <string_view>
#include <vector>

namespace {
struct HelperFailure final {
    int status;
    const char* stage;
    DWORD error;
};

int reportFailure(HANDLE diagnostic, int status, const char* stage, DWORD error) noexcept {
    // One bounded ASCII packet, through the exact inherited handle saved before
    // any console attachment. Reporting never changes the original failure.
    char message[128];
    const auto length = std::snprintf(message, sizeof(message), "stage=%s win32=%lu\n",
        stage, static_cast<unsigned long>(error));
    if (length > 0 && static_cast<std::size_t>(length) < sizeof(message)) {
        DWORD written = 0;
        (void)::WriteFile(diagnostic, message, static_cast<DWORD>(length), &written, nullptr);
    }
    return status;
}

std::uint64_t decimal(std::wstring_view text) {
    if (text.empty() || text.size() > 20)
        throw HelperFailure{24, "argument_conversion", ERROR_SUCCESS};
    std::uint64_t result = 0;
    for (wchar_t character : text) {
        if (character < L'0' || character > L'9' ||
            result > (std::numeric_limits<std::uint64_t>::max() - (character - L'0')) / 10)
            throw HelperFailure{24, "argument_conversion", ERROR_SUCCESS};
        result = result * 10 + (character - L'0');
    }
    return result;
}

bool inJob(HANDLE process, HANDLE job) {
    BOOL member = FALSE;
    return ::IsProcessInJob(process, job, &member) && member;
}

BOOL WINAPI ignoreOwnControl(DWORD) { return TRUE; }

class AttachedConsole final {
public:
    explicit AttachedConsole(DWORD process) {
        // This executable starts without a console. A caller that launches it
        // incorrectly is refused before any console state or signal changes.
        DWORD existing = 0;
        if (::GetConsoleProcessList(&existing, 1) != 0)
            throw HelperFailure{21, "existing_console", ERROR_SUCCESS};
        if (!::AttachConsole(process))
            throw HelperFailure{22, "attach_console", ::GetLastError()};
        attached_ = true;
        if (!::SetConsoleCtrlHandler(ignoreOwnControl, TRUE)) {
            const auto error = ::GetLastError();
            ::FreeConsole();
            attached_ = false;
            throw HelperFailure{23, "console_ctrl_handler", error};
        }
    }

    ~AttachedConsole() {
        if (attached_)
            ::FreeConsole();
    }

private:
    bool attached_ = false;
};
}

int wmain(int argc, wchar_t** argv) {
    if (argc > 1 && std::wstring_view(argv[1]) == dockpipe::sdk::consoleLaunch::mode)
        return dockpipe::sdk::consoleLaunch::run(argc, argv);

    const auto diagnostic = ::GetStdHandle(STD_ERROR_HANDLE);
    if (argc != 6)
        return reportFailure(diagnostic, 10, "argument_count", ERROR_SUCCESS);
    const char* stage = "argument_conversion";
    try {
        const auto processValue = decimal(argv[1]);
        const auto jobValue = decimal(argv[2]);
        const auto groupValue = decimal(argv[3]);
        const auto inventoryBytes = decimal(argv[4]);
        const auto deadline = decimal(argv[5]);
        if (processValue > std::numeric_limits<std::uintptr_t>::max() ||
            jobValue > std::numeric_limits<std::uintptr_t>::max() || groupValue == 0 ||
            groupValue > std::numeric_limits<DWORD>::max() ||
            inventoryBytes < sizeof(JOBOBJECT_BASIC_PROCESS_ID_LIST) ||
            inventoryBytes > std::numeric_limits<DWORD>::max() ||
            deadline <= ::GetTickCount64() || deadline - ::GetTickCount64() > 1000)
            return reportFailure(diagnostic, 11, "argument_limits", ERROR_SUCCESS);

        dockpipe::sdk::WindowsHandle process(reinterpret_cast<HANDLE>(static_cast<std::uintptr_t>(processValue)));
        dockpipe::sdk::WindowsHandle job(reinterpret_cast<HANDLE>(static_cast<std::uintptr_t>(jobValue)));
        DWORD flags = 0;
        if (!process || !job || !::GetHandleInformation(process.get(), &flags) ||
            !::GetHandleInformation(job.get(), &flags) || !inJob(process.get(), job.get()) ||
            !inJob(::GetCurrentProcess(), job.get()) ||
            ::GetProcessId(process.get()) != groupValue || ::WaitForSingleObject(process.get(), 0) != WAIT_TIMEOUT)
            return reportFailure(diagnostic, 12, "capability_admission", ERROR_SUCCESS);

        stage = "console_attachment";
        AttachedConsole console(static_cast<DWORD>(groupValue));
        stage = "console_inventory_allocation";
        std::vector<DWORD> members(static_cast<std::size_t>(inventoryBytes / sizeof(DWORD)));
        const auto count = ::GetConsoleProcessList(members.data(), static_cast<DWORD>(members.size()));
        if (count == 0)
            return reportFailure(diagnostic, 13, "console_inventory", ::GetLastError());
        if (count > members.size())
            return reportFailure(diagnostic, 13, "console_inventory_limit", ERROR_SUCCESS);
        for (DWORD index = 0; index < count; ++index) {
            if (::GetTickCount64() >= deadline)
                return reportFailure(diagnostic, 14, "inventory_deadline", ERROR_SUCCESS);
            dockpipe::sdk::WindowsHandle member(::OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION | SYNCHRONIZE,
                FALSE, members[index]));
            if (!member)
                return reportFailure(diagnostic, 15, "console_member_open", ::GetLastError());
            if (!inJob(member.get(), job.get()))
                return reportFailure(diagnostic, 15, "console_membership", ERROR_SUCCESS);
        }
        if (::GetTickCount64() >= deadline || ::WaitForSingleObject(process.get(), 0) != WAIT_TIMEOUT)
            return reportFailure(diagnostic, 16, "signal_admission", ERROR_SUCCESS);

        // The broker creates the actual CLI with CREATE_NEW_PROCESS_GROUP in
        // its isolated console. Select that nonzero group; never broadcast 0.
        // Processes merely attaching from another group cannot be signalled.
        if (!::GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, static_cast<DWORD>(groupValue)))
            return reportFailure(diagnostic, 17, "generate_break", ::GetLastError());
        return 0;
    } catch (const HelperFailure& failure) {
        return reportFailure(diagnostic, failure.status, failure.stage, failure.error);
    } catch (...) {
        // Helper status describes native admission/effect failure only. It does
        // not decide any application assertion or perform fallback termination.
        return reportFailure(diagnostic, 20, stage, ERROR_SUCCESS);
    }
}
