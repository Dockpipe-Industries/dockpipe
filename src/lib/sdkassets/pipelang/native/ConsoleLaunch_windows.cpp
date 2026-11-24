// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "ConsoleLaunchProtocol.hpp"

#include <array>
#include <new>

namespace dockpipe::sdk::consoleLaunch {
namespace {
struct NativeFailure final {
    DWORD error;
};

[[noreturn]] void failLastError() {
    const auto error = ::GetLastError();
    throw NativeFailure{error == ERROR_SUCCESS ? ERROR_GEN_FAILURE : error};
}

void checkDeadline(std::uint64_t deadline) {
    const auto now = ::GetTickCount64();
    if (deadline <= now || deadline - now > handoffMilliseconds)
        throw NativeFailure{ERROR_TIMEOUT};
}

class ReadOnlyView final {
public:
    ReadOnlyView(HANDLE mapping, std::size_t bytes)
        : value_(::MapViewOfFile(mapping, FILE_MAP_READ, 0, 0, bytes)) {
        if (!value_)
            failLastError();
    }

    ~ReadOnlyView() {
        ::UnmapViewOfFile(value_);
    }

    ReadOnlyView(const ReadOnlyView&) = delete;
    ReadOnlyView& operator=(const ReadOnlyView&) = delete;
    const void* get() const noexcept { return value_; }

private:
    const void* value_;
};

class StandardHandleList final {
public:
    explicit StandardHandleList(const std::array<HANDLE, 3>& handles) {
        SIZE_T bytes = 0;
        (void)::InitializeProcThreadAttributeList(nullptr, 1, 0, &bytes);
        if (bytes == 0)
            failLastError();

        storage_.resize(bytes);
        attributes_ = reinterpret_cast<LPPROC_THREAD_ATTRIBUTE_LIST>(storage_.data());
        if (!::InitializeProcThreadAttributeList(attributes_, 1, 0, &bytes)) {
            attributes_ = nullptr;
            failLastError();
        }

        if (!::UpdateProcThreadAttribute(attributes_, 0, PROC_THREAD_ATTRIBUTE_HANDLE_LIST,
            const_cast<HANDLE*>(handles.data()), handles.size() * sizeof(HANDLE), nullptr, nullptr)) {
            const auto error = ::GetLastError();
            ::DeleteProcThreadAttributeList(attributes_);
            attributes_ = nullptr;
            throw NativeFailure{error};
        }
    }

    ~StandardHandleList() {
        if (attributes_)
            ::DeleteProcThreadAttributeList(attributes_);
    }

    StandardHandleList(const StandardHandleList&) = delete;
    StandardHandleList& operator=(const StandardHandleList&) = delete;
    LPPROC_THREAD_ATTRIBUTE_LIST get() const noexcept { return attributes_; }

private:
    std::vector<unsigned char> storage_;
    LPPROC_THREAD_ATTRIBUTE_LIST attributes_ = nullptr;
};

void admitHandles(const std::array<HANDLE, 6>& handles) {
    for (std::size_t index = 0; index < handles.size(); ++index) {
        const auto value = handles[index];
        DWORD flags = 0;
        if (value == nullptr || value == INVALID_HANDLE_VALUE ||
            !::GetHandleInformation(value, &flags) || (flags & HANDLE_FLAG_INHERIT) == 0)
            throw NativeFailure{ERROR_INVALID_HANDLE};

        for (std::size_t previous = 0; previous < index; ++previous) {
            if (value == handles[previous])
                throw NativeFailure{ERROR_INVALID_HANDLE};
        }
    }

    // These private channels can never alias a capture handle, or an invalid
    // request could write a protocol frame into caller-visible output.
    if (::GetFileType(handles[1]) != FILE_TYPE_PIPE ||
        ::GetFileType(handles[2]) != FILE_TYPE_PIPE ||
        ::GetFileType(handles[4]) != FILE_TYPE_PIPE)
        throw NativeFailure{ERROR_INVALID_HANDLE};

    if (::WaitForSingleObject(handles[5], 0) != WAIT_TIMEOUT)
        throw NativeFailure{ERROR_INVALID_HANDLE};

    for (std::size_t index = 3; index < handles.size(); ++index) {
        if (!::SetHandleInformation(handles[index], HANDLE_FLAG_INHERIT, 0))
            failLastError();
    }

    BOOL inJob = FALSE;
    DWORD consoleMember = 0;
    if (!::IsProcessInJob(::GetCurrentProcess(), nullptr, &inJob))
        failLastError();

    if (!inJob || ::GetConsoleProcessList(&consoleMember, 1) == 0)
        throw NativeFailure{ERROR_INVALID_STATE};
}

Prepared readPrepared(HANDLE mapping, std::size_t extent) {
    Header header;
    {
        // Validate the fixed header and its aggregate contract before asking
        // the kernel for a potentially large view or allocating payload copies.
        ReadOnlyView view(mapping, sizeof(Header));
        std::memcpy(&header, view.get(), sizeof(header));
    }
    validateHeader(header, extent);
    checkDeadline(header.deadline);

    ReadOnlyView view(mapping, extent);
    if (std::memcmp(&header, view.get(), sizeof(header)) != 0)
        throw NativeFailure{ERROR_INVALID_DATA};

    auto prepared = decode(view.get(), extent);
    checkDeadline(prepared.deadline);
    return prepared;
}

bool writeReply(HANDLE writer, const Reply& reply) noexcept {
    // The caller supplies one initially empty private pipe with capacity for
    // this single fixed frame. No request body or diagnostic uses this channel.
    DWORD written = 0;
    return ::WriteFile(writer, &reply, sizeof(reply), &written, nullptr) && written == sizeof(reply);
}
}

int run(int argc, wchar_t** argv) noexcept {
    if (argc != 6)
        return 40;

    WindowsHandle mapping;
    WindowsHandle replyWriter;
    WindowsHandle acknowledgement;
    WindowsHandle input;
    WindowsHandle output;
    WindowsHandle error;
    WindowsHandle childProcess;
    WindowsHandle childThread;
    Reply reply;
    std::uint64_t deadline = 0;
    bool admitted = false;
    try {
        const auto mappingValue = handle(decimal(argv[2]));
        const auto extent = checkedSize(decimal(argv[3]));
        const auto replyValue = handle(decimal(argv[4]));
        const auto acknowledgementValue = handle(decimal(argv[5]));
        if (extent < sizeof(Header))
            throw NativeFailure{ERROR_INVALID_DATA};

        const std::array<HANDLE, 6> handles{
            ::GetStdHandle(STD_INPUT_HANDLE), ::GetStdHandle(STD_OUTPUT_HANDLE),
            ::GetStdHandle(STD_ERROR_HANDLE), mappingValue, replyValue, acknowledgementValue};
        admitHandles(handles);
        mapping.reset(mappingValue);
        replyWriter.reset(replyValue);
        acknowledgement.reset(acknowledgementValue);
        input.reset(handles[0]);
        output.reset(handles[1]);
        error.reset(handles[2]);
        admitted = true;

        reply.stage = requestStage;
        auto prepared = readPrepared(mapping.get(), extent);
        deadline = prepared.deadline;
        mapping.reset();

        const std::array<HANDLE, 3> childHandles{input.get(), output.get(), error.get()};
        StandardHandleList selectedHandles(childHandles);
        STARTUPINFOEXW startup{};
        startup.StartupInfo.cb = sizeof(startup);
        startup.StartupInfo.dwFlags = STARTF_USESTDHANDLES | STARTF_USESHOWWINDOW;
        startup.StartupInfo.wShowWindow = SW_HIDE;
        startup.StartupInfo.hStdInput = input.get();
        startup.StartupInfo.hStdOutput = output.get();
        startup.StartupInfo.hStdError = error.get();
        startup.lpAttributeList = selectedHandles.get();

        reply.stage = deadlineStage;
        checkDeadline(deadline);
        reply.stage = creationStage;
        PROCESS_INFORMATION child{};
        // The admitted broker already owns the isolated console and job. The
        // actual CLI inherits both, but becomes its own targetable process group.
        // Only its command buffer is mutable; argv, cwd and environment are exact
        // copies of the caller's prepared snapshot. No parent/job capabilities
        // or private launch channels are forwarded to this process.
        if (!::CreateProcessW(prepared.application.c_str(), prepared.command.data(), nullptr, nullptr, TRUE,
            CREATE_SUSPENDED | CREATE_NEW_PROCESS_GROUP | CREATE_UNICODE_ENVIRONMENT |
                EXTENDED_STARTUPINFO_PRESENT,
            prepared.environment.data(), prepared.directory.c_str(), &startup.StartupInfo, &child))
            failLastError();

        childProcess.reset(child.hProcess);
        childThread.reset(child.hThread);
        input.reset();
        output.reset();
        error.reset();

        reply.stage = deadlineStage;
        checkDeadline(deadline);
        reply.processId = child.dwProcessId;
        reply.threadId = child.dwThreadId;
        reply.processHandle = handleNumber(childProcess.get());
        reply.threadHandle = handleNumber(childThread.get());
        reply.stage = successStage;
    } catch (const NativeFailure& failure) {
        reply.error = failure.error;
    } catch (const std::bad_alloc&) {
        reply.error = ERROR_NOT_ENOUGH_MEMORY;
    } catch (...) {
        reply.error = ERROR_INVALID_DATA;
    }

    // Always release the broker's capture writers before publishing the receipt.
    // Genuine EOF then depends only on the suspended CLI and its future children.
    input.reset();
    output.reset();
    error.reset();
    mapping.reset();
    if (!admitted)
        return 41;

    const auto sent = writeReply(replyWriter.get(), reply);
    replyWriter.reset();
    if (!childProcess)
        return 42;

    // Keep both original child handles alive through acknowledgement, including
    // a lost/partial reply or a late CreateProcessW. Never resume, retry, reopen
    // by PID or terminate here: the parent retains the exact job's custody.
    const auto now = ::GetTickCount64();
    if (now >= deadline)
        return 43;

    const auto waited = ::WaitForSingleObject(acknowledgement.get(), static_cast<DWORD>(deadline - now));
    if (!sent || waited != WAIT_OBJECT_0 || ::GetTickCount64() >= deadline ||
        reply.stage != successStage || reply.error != ERROR_SUCCESS)
        return 44;

    return 0;
}
}
