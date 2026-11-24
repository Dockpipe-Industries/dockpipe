// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include "DockpipeSDKBridge.hpp"
#include "WindowsFiles.hpp"

#include <cstddef>
#include <cstdint>
#include <limits>
#include <stdexcept>
#include <string>
#include <string_view>
#include <utility>
#include <vector>

namespace sdk_fault_fixture {
namespace input_proof = sdk_fault::source::Dockpipe::SDK::FaultProof;

inline std::size_t addInputBytes(std::size_t left, std::size_t right) {
    if (right > std::numeric_limits<std::size_t>::max() - left)
        throw std::runtime_error("Input fact aggregate size overflows");

    return left + right;
}

inline std::int64_t sourceInputInteger(std::uint64_t value) {
    if (value > static_cast<std::uint64_t>(std::numeric_limits<std::int64_t>::max()))
        throw std::runtime_error("Input fact exceeds the source integer range");

    return static_cast<std::int64_t>(value);
}

inline input_proof::ArgumentMetrics measureArguments(const std::vector<std::string>& arguments) {
    std::size_t characters = 0;
    std::size_t utf8Bytes = 0;
    std::size_t storageBytes = sizeof(char*);
    bool first = true;
    for (const auto& argument : arguments) {
        const auto quoted = dockpipe::sdk::quoteWindowsArgument(dockpipe::sdk::windowsWide(argument));
        if (!first)
            characters = addInputBytes(characters, 1);

        first = false;
        characters = addInputBytes(characters, quoted.size());
        utf8Bytes = addInputBytes(utf8Bytes, argument.size());
        // Native item-storage accounting: bytes/NUL plus pointer/string slots,
        // excluding allocator capacity. Source compares this charge with its
        // selected budget; native code reports it without an expected outcome.
        storageBytes = addInputBytes(storageBytes, argument.size());
        storageBytes = addInputBytes(storageBytes, 1 + sizeof(char*) + sizeof(std::string));
    }

    input_proof::ArgumentMetrics result{};
    result.argumentCount = sourceInputInteger(arguments.size());
    result.commandCharacters = sourceInputInteger(characters);
    result.utf8Bytes = sourceInputInteger(utf8Bytes);
    result.argumentStorageBytes = sourceInputInteger(storageBytes);
    return result;
}

class InputFactReader final {
public:
    InputFactReader(std::string_view text, std::size_t maximumBytes) : remaining_(text) {
        if (text.size() > maximumBytes)
            throw std::runtime_error("Input fact frame exceeds the configured output byte bound");
    }

    void header(std::string_view expected) {
        if (!remaining_.starts_with(expected))
            throw std::runtime_error("Input fact frame header is incompatible");

        remaining_.remove_prefix(expected.size());
    }

    std::uint64_t number() {
        const auto end = remaining_.find('\n');
        if (end == std::string_view::npos || end == 0 || end > 20)
            throw std::runtime_error("Input fact numeric field is incomplete");

        std::uint64_t value = 0;
        for (std::size_t index = 0; index < end; ++index) {
            const auto character = remaining_[index];
            if (character < '0' || character > '9')
                throw std::runtime_error("Input fact numeric field is invalid");

            const auto digit = static_cast<std::uint64_t>(character - '0');
            if (value > (std::numeric_limits<std::uint64_t>::max() - digit) / 10)
                throw std::runtime_error("Input fact numeric field overflows");

            value = value * 10 + digit;
        }

        remaining_.remove_prefix(end + 1);
        return value;
    }

    std::string field() {
        const auto length = number();
        if (length >= remaining_.size() || remaining_[static_cast<std::size_t>(length)] != '\n')
            throw std::runtime_error("Input fact string extent is incomplete");

        std::string value(remaining_.substr(0, static_cast<std::size_t>(length)));
        remaining_.remove_prefix(static_cast<std::size_t>(length) + 1);
        return value;
    }

    std::size_t remainingBytes() const noexcept { return remaining_.size(); }

    void finished() const {
        if (!remaining_.empty())
            throw std::runtime_error("Input fact frame has trailing bytes");
    }

private:
    std::string_view remaining_;
};

inline input_proof::ArgumentStats decodeArgumentStats(std::string_view output, std::size_t maximumBytes) {
    InputFactReader reader(output, maximumBytes);
    reader.header("SDK_ARG_STATS_V1\n");
    input_proof::ArgumentStats result{};
    result.argumentCount = sourceInputInteger(reader.number());
    result.payloadArgumentCount = sourceInputInteger(reader.number());
    result.payloadUtf8Bytes = sourceInputInteger(reader.number());
    result.minimumByte = sourceInputInteger(reader.number());
    result.maximumByte = sourceInputInteger(reader.number());
    if (result.minimumByte > 255 || result.maximumByte > 255)
        throw std::runtime_error("Input fact byte extrema exceed the byte representation");

    reader.finished();
    return result;
}

inline input_proof::ChildInputs decodeChildInputs(std::string_view output, std::size_t maximumBytes) {
    InputFactReader reader(output, maximumBytes);
    reader.header("SDK_CHILD_INPUTS_V1\n");
    input_proof::ChildInputs result{};
    result.cwd = reader.field();
    const auto count = reader.number();
    // A variable needs at least name length+newline+delimiter, presence+newline
    // and value length+newline+delimiter: eight bytes even for empty fields.
    if (count > reader.remainingBytes() / 8)
        throw std::runtime_error("Input fact variable count exceeds its frame extent");

    for (std::uint64_t index = 0; index < count; ++index) {
        input_proof::ChildInputValue variable{};
        variable.name = reader.field();
        const auto present = reader.number();
        if (present > 1)
            throw std::runtime_error("Input fact presence marker is invalid");

        variable.present = present != 0;
        variable.value = reader.field();
        result.variables.add(std::move(variable));
    }

    reader.finished();
    return result;
}
}
