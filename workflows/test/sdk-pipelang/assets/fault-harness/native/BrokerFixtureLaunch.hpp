// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include "ConsoleLaunchProtocol.hpp"
#include "LostOfferWitness.hpp"

#include <algorithm>
#include <array>
#include <cstring>
#include <stdexcept>
#include <vector>

namespace sdk_fault_fixture {
namespace protocol = dockpipe::sdk::consoleLaunch;
using dockpipe::sdk::WindowsHandle;

class ReadOnlyView final {
public:
    ReadOnlyView(HANDLE mapping, std::size_t bytes)
        : value_(::MapViewOfFile(mapping, FILE_MAP_READ, 0, 0, bytes)) {
        if (!value_)
            throw std::runtime_error("Cannot map fixture launch snapshot");
    }

    ~ReadOnlyView() { ::UnmapViewOfFile(value_); }
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
            throw std::runtime_error("Cannot size fixture standard handle list");

        storage_.resize(bytes);
        attributes_ = reinterpret_cast<LPPROC_THREAD_ATTRIBUTE_LIST>(storage_.data());
        if (!::InitializeProcThreadAttributeList(attributes_, 1, 0, &bytes)) {
            attributes_ = nullptr;
            throw std::runtime_error("Cannot initialize fixture standard handle list");
        }

        if (!::UpdateProcThreadAttribute(attributes_, 0, PROC_THREAD_ATTRIBUTE_HANDLE_LIST,
            const_cast<HANDLE*>(handles.data()), handles.size() * sizeof(HANDLE), nullptr, nullptr)) {
            ::DeleteProcThreadAttributeList(attributes_);
            attributes_ = nullptr;
            throw std::runtime_error("Cannot select fixture standard handle list");
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

inline void checkDeadline(std::uint64_t deadline) {
    const auto now = ::GetTickCount64();
    if (deadline <= now || deadline - now > protocol::handoffMilliseconds)
        throw std::runtime_error("Fixture launch deadline exhausted");
}

inline protocol::Prepared readPrepared(HANDLE mapping, std::size_t extent) {
    if (extent < sizeof(protocol::Header))
        throw std::runtime_error("Fixture snapshot is shorter than its header");

    protocol::Header header;
    {
        ReadOnlyView view(mapping, sizeof(header));
        std::memcpy(&header, view.get(), sizeof(header));
    }
    protocol::validateHeader(header, extent);
    checkDeadline(header.deadline);

    ReadOnlyView view(mapping, extent);
    if (std::memcmp(&header, view.get(), sizeof(header)) != 0)
        throw std::runtime_error("Fixture launch snapshot header changed");

    return protocol::decode(view.get(), extent);
}

// Shared bounded OS effects only. The three standalone fixture entrypoints pick
// their physical channel operation; PipeLang owns case selection and assertions.
class BrokerFixtureLaunch final {
public:
    BrokerFixtureLaunch(int argc, wchar_t** argv) {
        if (argc != 6 || std::wstring_view(argv[1]) != protocol::mode)
            throw std::runtime_error("Fixture launch operands are invalid");

        const std::array<HANDLE, 6> inherited{
            ::GetStdHandle(STD_INPUT_HANDLE), ::GetStdHandle(STD_OUTPUT_HANDLE),
            ::GetStdHandle(STD_ERROR_HANDLE), protocol::handle(protocol::decimal(argv[2])),
            protocol::handle(protocol::decimal(argv[4])), protocol::handle(protocol::decimal(argv[5]))};
        for (std::size_t index = 0; index < inherited.size(); ++index) {
            DWORD flags = 0;
            if (!::GetHandleInformation(inherited[index], &flags) || (flags & HANDLE_FLAG_INHERIT) == 0)
                throw std::runtime_error("Fixture inherited capability is invalid");

            for (std::size_t previous = 0; previous < index; ++previous) {
                if (inherited[index] == inherited[previous])
                    throw std::runtime_error("Fixture inherited capabilities alias");
            }
        }

        WindowsHandle input(inherited[0]);
        WindowsHandle output(inherited[1]);
        WindowsHandle error(inherited[2]);
        WindowsHandle mapping(inherited[3]);
        reply_.reset(inherited[4]);
        acknowledgement_.reset(inherited[5]);
        for (std::size_t index = 3; index < inherited.size(); ++index) {
            if (!::SetHandleInformation(inherited[index], HANDLE_FLAG_INHERIT, 0))
                throw std::runtime_error("Cannot isolate fixture private capability");
        }

        prepared_ = readPrepared(mapping.get(), protocol::checkedSize(protocol::decimal(argv[3])));
        mapping.reset();
        BOOL inJob = FALSE;
        if (!::IsProcessInJob(::GetCurrentProcess(), nullptr, &inJob) || !inJob)
            throw std::runtime_error("Fixture broker has no inherited job custody");

        const std::array<HANDLE, 3> childHandles{input.get(), output.get(), error.get()};
        StandardHandleList attributes(childHandles);
        STARTUPINFOEXW startup{};
        startup.StartupInfo.cb = sizeof(startup);
        startup.StartupInfo.dwFlags = STARTF_USESTDHANDLES | STARTF_USESHOWWINDOW;
        startup.StartupInfo.wShowWindow = SW_HIDE;
        startup.StartupInfo.hStdInput = input.get();
        startup.StartupInfo.hStdOutput = output.get();
        startup.StartupInfo.hStdError = error.get();
        startup.lpAttributeList = attributes.get();
        PROCESS_INFORMATION child{};
        checkDeadline(prepared_.deadline);
        if (!::CreateProcessW(prepared_.application.c_str(), prepared_.command.data(), nullptr, nullptr, TRUE,
            CREATE_SUSPENDED | CREATE_NEW_PROCESS_GROUP | CREATE_UNICODE_ENVIRONMENT |
                EXTENDED_STARTUPINFO_PRESENT,
            prepared_.environment.data(), prepared_.directory.c_str(), &startup.StartupInfo, &child))
            throw std::runtime_error("Cannot create suspended fixture child");

        childProcess_.reset(child.hProcess);
        childThread_.reset(child.hThread);
        processId_ = child.dwProcessId;
        threadId_ = child.dwThreadId;
        createdAt_ = ::GetTickCount64();
        input.reset();
        output.reset();
        error.reset();
        checkDeadline(prepared_.deadline);
    }

    BrokerFixtureLaunch(const BrokerFixtureLaunch&) = delete;
    BrokerFixtureLaunch& operator=(const BrokerFixtureLaunch&) = delete;

    protocol::Reply offer() const noexcept {
        protocol::Reply reply;
        reply.stage = protocol::successStage;
        reply.processId = processId_;
        reply.threadId = threadId_;
        reply.processHandle = protocol::handleNumber(childProcess_.get());
        reply.threadHandle = protocol::handleNumber(childThread_.get());
        return reply;
    }

    void closeThreadCapability() noexcept { childThread_.reset(); }
    void closeOffer() noexcept { reply_.reset(); }

    bool writeOffer(const protocol::Reply& reply) noexcept {
        DWORD written = 0;
        return ::WriteFile(reply_.get(), &reply, sizeof(reply), &written, nullptr) && written == sizeof(reply);
    }

    void writeCreationWitness(const char* filename) const {
        LostOfferWitness witness;
        witness.processId = processId_;
        witness.threadId = threadId_;
        witness.createdAt = createdAt_;
        writeWitness(filename, &witness, sizeof(witness));
    }

    void waitPastDeadline() const noexcept {
        auto now = ::GetTickCount64();
        while (now <= prepared_.deadline) {
            const auto remaining = prepared_.deadline - now + 1;
            ::Sleep(static_cast<DWORD>(std::min<std::uint64_t>(remaining, 50)));
            now = ::GetTickCount64();
        }
    }

    void writeLateWitness(std::uint64_t attemptedAt, bool succeeded) const {
        LateOfferWitness witness;
        witness.processId = processId_;
        witness.threadId = threadId_;
        witness.createdAt = createdAt_;
        witness.requestDeadline = prepared_.deadline;
        witness.attemptedAt = attemptedAt;
        witness.writeSucceeded = succeeded ? 1 : 0;
        writeWitness(lateOfferWitnessName, &witness, sizeof(witness));
    }

    int awaitAcknowledgement() const noexcept {
        const auto now = ::GetTickCount64();
        if (now >= prepared_.deadline)
            return 42;

        const auto waited = ::WaitForSingleObject(acknowledgement_.get(),
            static_cast<DWORD>(prepared_.deadline - now));
        return waited == WAIT_OBJECT_0 ? 0 : 43;
    }

private:
    void writeWitness(const char* filename, const void* witness, std::size_t bytes) const {
        const auto root = dockpipe::sdk::windowsUtf8(prepared_.directory);
        dockpipe::sdk::WindowsFiles files(root);
        auto file = files.createNew(root + filename);
        dockpipe::sdk::WindowsFiles::writeAll(file.get(),
            std::string(static_cast<const char*>(witness), bytes));
    }

    protocol::Prepared prepared_;
    WindowsHandle reply_;
    WindowsHandle acknowledgement_;
    WindowsHandle childProcess_;
    WindowsHandle childThread_;
    DWORD processId_ = 0;
    DWORD threadId_ = 0;
    std::uint64_t createdAt_ = 0;
};
}
