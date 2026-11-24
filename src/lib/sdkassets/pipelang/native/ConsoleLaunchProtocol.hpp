// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include "WindowsFiles.hpp"

#include <algorithm>
#include <array>
#include <cstdint>
#include <cstring>
#include <limits>
#include <stdexcept>
#include <string>
#include <string_view>
#include <type_traits>
#include <vector>

namespace dockpipe::sdk::consoleLaunch {

inline constexpr wchar_t mode[] = L"--launch-v1";
inline constexpr std::uint32_t magic = 0x44504c31;
inline constexpr std::uint32_t version = 1;
inline constexpr DWORD handoffMilliseconds = 5000;
inline constexpr std::uint32_t successStage = 1;
inline constexpr std::uint32_t admissionStage = 2;
inline constexpr std::uint32_t requestStage = 3;
inline constexpr std::uint32_t creationStage = 4;
inline constexpr std::uint32_t deadlineStage = 5;
inline constexpr std::uint32_t nativeFailureStage = 6;
// These are the existing Windows path and CreateProcessW command bounds,
// including terminators. Environment bytes retain the client's argument budget.
inline constexpr std::size_t maximumStringCharacters = 32767;

struct Field final {
    std::uint64_t offsetBytes = 0;
    std::uint64_t characters = 0;
};

struct Header final {
    std::uint32_t protocolMagic = magic;
    std::uint32_t protocolVersion = version;
    std::uint32_t headerBytes = sizeof(Header);
    std::uint32_t reserved = 0;
    std::uint64_t totalBytes = 0;
    std::uint64_t environmentLimitBytes = 0;
    std::uint64_t deadline = 0;
    Field application;
    Field command;
    Field directory;
    Field environment;
};

struct Reply final {
    std::uint32_t protocolMagic = magic;
    std::uint32_t protocolVersion = version;
    std::uint32_t bytes = sizeof(Reply);
    std::uint32_t stage = admissionStage;
    std::uint32_t error = 0;
    std::uint32_t processId = 0;
    std::uint32_t threadId = 0;
    std::uint32_t reserved = 0;
    std::uint64_t processHandle = 0;
    std::uint64_t threadHandle = 0;
};
static_assert(sizeof(wchar_t) == 2);
static_assert(sizeof(Header) == 104);
static_assert(sizeof(Reply) == 48);
static_assert(std::is_trivially_copyable_v<Header> && std::is_standard_layout_v<Header>);
static_assert(std::is_trivially_copyable_v<Reply> && std::is_standard_layout_v<Reply>);

inline std::size_t checkedSize(std::uint64_t value) {
    if (value > std::numeric_limits<std::size_t>::max())
        throw std::runtime_error("console launch size exceeds native address space");

    return static_cast<std::size_t>(value);
}

inline std::size_t add(std::size_t left, std::size_t right) {
    if (right > std::numeric_limits<std::size_t>::max() - left)
        throw std::runtime_error("console launch aggregate size overflows");

    return left + right;
}

inline std::size_t characterBytes(std::size_t characters) {
    if (characters > std::numeric_limits<std::size_t>::max() / sizeof(wchar_t))
        throw std::runtime_error("console launch UTF-16 byte size overflows");

    return characters * sizeof(wchar_t);
}

inline std::size_t maximumSnapshotBytes(std::size_t environmentLimitBytes) {
    const auto strings = 3 * maximumStringCharacters * sizeof(wchar_t);
    return add(add(sizeof(Header), strings), environmentLimitBytes);
}

inline void validateHeader(const Header& header, std::size_t extent) {
    if (header.protocolMagic != magic || header.protocolVersion != version ||
        header.headerBytes != sizeof(Header) || header.reserved != 0 ||
        header.totalBytes != extent || extent < sizeof(Header) ||
        extent > maximumSnapshotBytes(checkedSize(header.environmentLimitBytes)))
        throw std::runtime_error("console launch request header or aggregate bound is invalid");

    std::size_t next = sizeof(Header);
    const std::array<Field, 4> fields{header.application, header.command, header.directory, header.environment};
    for (std::size_t index = 0; index < fields.size(); ++index) {
        const auto& field = fields[index];
        const auto characters = checkedSize(field.characters);
        const auto bytes = characterBytes(characters);
        const bool environment = index == fields.size() - 1;
        if (field.offsetBytes != next || characters < 2 ||
            (!environment && characters > maximumStringCharacters) ||
            (environment && bytes > header.environmentLimitBytes) || bytes > extent - next)
            throw std::runtime_error("console launch field extent is invalid");

        next = add(next, bytes);
    }

    if (next != extent)
        throw std::runtime_error("console launch request has trailing or overlapping data");
}

inline void validateEnvironment(const wchar_t* environment, std::size_t count) {
    if (environment == nullptr || count < 2 ||
        environment[count - 1] != L'\0' || environment[count - 2] != L'\0')
        throw std::runtime_error("console launch environment is not double-NUL terminated");

    for (std::size_t index = 0; index + 2 < count; ++index) {
        if (environment[index] == L'\0' && environment[index + 1] == L'\0')
            throw std::runtime_error("console launch environment contains data after its terminator");
    }
}

inline std::vector<unsigned char> encode(std::wstring_view application, std::wstring_view command,
    std::wstring_view directory, const std::vector<wchar_t>& environment,
    std::size_t environmentLimitBytes, std::uint64_t deadline) {
    Header header;
    header.environmentLimitBytes = environmentLimitBytes;
    header.deadline = deadline;
    std::size_t next = sizeof(Header);
    const auto appendString = [&](Field& field, std::wstring_view value) {
        if (value.empty() || value.size() >= maximumStringCharacters ||
            value.find(L'\0') != std::wstring_view::npos)
            throw std::runtime_error("console launch string exceeds the existing Windows contract");

        field = {next, value.size() + 1};
        next = add(next, characterBytes(value.size() + 1));
    };
    appendString(header.application, application);
    appendString(header.command, command);
    appendString(header.directory, directory);
    header.environment = {next, environment.size()};
    next = add(next, characterBytes(environment.size()));
    header.totalBytes = next;
    validateHeader(header, next);

    validateEnvironment(environment.data(), environment.size());

    std::vector<unsigned char> bytes(next, 0);
    std::memcpy(bytes.data(), &header, sizeof(header));
    const auto copyString = [&](const Field& field, std::wstring_view value) {
        std::memcpy(bytes.data() + checkedSize(field.offsetBytes), value.data(), characterBytes(value.size()));
    };
    copyString(header.application, application);
    copyString(header.command, command);
    copyString(header.directory, directory);
    std::memcpy(bytes.data() + checkedSize(header.environment.offsetBytes), environment.data(),
        characterBytes(environment.size()));
    return bytes;
}

struct Prepared final {
    std::wstring application;
    std::wstring command;
    std::wstring directory;
    std::vector<wchar_t> environment;
    std::uint64_t deadline = 0;
};

inline Prepared decode(const void* mapping, std::size_t extent) {
    if (mapping == nullptr || extent < sizeof(Header))
        throw std::runtime_error("console launch mapping is shorter than its header");

    Header header;
    std::memcpy(&header, mapping, sizeof(header));
    validateHeader(header, extent);
    const auto bytes = static_cast<const unsigned char*>(mapping);
    const auto readString = [&](const Field& field) {
        const auto begin = reinterpret_cast<const wchar_t*>(bytes + checkedSize(field.offsetBytes));
        const auto count = checkedSize(field.characters);
        if (begin[count - 1] != L'\0' || std::find(begin, begin + count - 1, L'\0') != begin + count - 1)
            throw std::runtime_error("console launch string terminator is invalid");

        return std::wstring(begin, count - 1);
    };

    Prepared prepared;
    prepared.application = readString(header.application);
    prepared.command = readString(header.command);
    prepared.directory = readString(header.directory);
    const auto environment = reinterpret_cast<const wchar_t*>(bytes + checkedSize(header.environment.offsetBytes));
    const auto count = checkedSize(header.environment.characters);
    validateEnvironment(environment, count);

    prepared.environment.assign(environment, environment + count);
    prepared.deadline = header.deadline;
    return prepared;
}

inline std::uint64_t decimal(std::wstring_view text) {
    if (text.empty() || text.size() > 20)
        throw std::runtime_error("console launch numeric operand is invalid");

    std::uint64_t value = 0;
    for (const auto character : text) {
        if (character < L'0' || character > L'9' ||
            value > (std::numeric_limits<std::uint64_t>::max() - (character - L'0')) / 10)
            throw std::runtime_error("console launch numeric operand overflows");

        value = value * 10 + character - L'0';
    }

    return value;
}

inline HANDLE handle(std::uint64_t value) {
    if (value == 0 || value >= std::numeric_limits<std::uintptr_t>::max())
        throw std::runtime_error("console launch handle representation is invalid");

    return reinterpret_cast<HANDLE>(static_cast<std::uintptr_t>(value));
}

inline std::uint64_t handleNumber(HANDLE value) noexcept {
    return static_cast<std::uint64_t>(reinterpret_cast<std::uintptr_t>(value));
}

// Exactly: helper --launch-v1 mapping extent replyWriter synchronizationOnlyAck.
// Standard handles are the only capabilities forwarded to the suspended CLI.
int run(int argc, wchar_t** argv) noexcept;
}
