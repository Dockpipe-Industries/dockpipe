// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "ScopeCodec.hpp"

#include <cstdint>
#include <stdexcept>
#include <utility>

namespace dockpipe::sdk {
namespace {
class ScopeDecodingFailure final : public std::runtime_error {
public:
    explicit ScopeDecodingFailure(const char* message) : std::runtime_error(message) {}
};

void appendCodePoint(std::string& output, std::uint32_t value) {
    if (value <= 0x7f) {
        output.push_back(static_cast<char>(value));
    } else if (value <= 0x7ff) {
        output.push_back(static_cast<char>(0xc0 | (value >> 6)));
        output.push_back(static_cast<char>(0x80 | (value & 0x3f)));
    } else if (value <= 0xffff) {
        output.push_back(static_cast<char>(0xe0 | (value >> 12)));
        output.push_back(static_cast<char>(0x80 | ((value >> 6) & 0x3f)));
        output.push_back(static_cast<char>(0x80 | (value & 0x3f)));
    } else {
        output.push_back(static_cast<char>(0xf0 | (value >> 18)));
        output.push_back(static_cast<char>(0x80 | ((value >> 12) & 0x3f)));
        output.push_back(static_cast<char>(0x80 | ((value >> 6) & 0x3f)));
        output.push_back(static_cast<char>(0x80 | (value & 0x3f)));
    }
}

bool validUtf8(std::string_view input) {
    std::size_t index = 0;
    while (index < input.size()) {
        const auto first = static_cast<unsigned char>(input[index++]);
        if (first <= 0x7f)
            continue;
        std::uint32_t value = 0;
        std::uint32_t minimum = 0;
        std::size_t remaining = 0;
        if (first >= 0xc2 && first <= 0xdf) {
            value = first & 0x1f;
            minimum = 0x80;
            remaining = 1;
        } else if (first >= 0xe0 && first <= 0xef) {
            value = first & 0x0f;
            minimum = 0x800;
            remaining = 2;
        } else if (first >= 0xf0 && first <= 0xf4) {
            value = first & 0x07;
            minimum = 0x10000;
            remaining = 3;
        } else {
            return false;
        }
        if (remaining > input.size() - index)
            return false;
        for (std::size_t part = 0; part < remaining; ++part) {
            const auto next = static_cast<unsigned char>(input[index++]);
            if ((next & 0xc0) != 0x80)
                return false;
            value = (value << 6) | (next & 0x3f);
        }
        if (value < minimum || value > 0x10ffff || (value >= 0xd800 && value <= 0xdfff))
            return false;
    }
    return true;
}

class ScopeParser final {
public:
    explicit ScopeParser(std::string_view input) : input_(input) {}

    ScopeFields parse() {
        whitespace();
        expect('{');
        ScopeFields fields;
        whitespace();
        if (!take('}')) {
            while (true) {
                whitespace();
                auto name = string();
                whitespace();
                expect(':');
                whitespace();
                auto value = string();
                if (!fields.emplace(std::move(name), std::move(value)).second)
                    throw ScopeDecodingFailure("scope JSON contains duplicate fields");
                whitespace();
                if (take('}'))
                    break;
                expect(',');
            }
        }
        whitespace();
        if (position_ != input_.size())
            throw ScopeDecodingFailure("scope JSON contains trailing data");
        for (const char* required : {"kind", "root", "scope", "workdir"}) {
            const auto found = fields.find(required);
            if (found == fields.end() || found->second.empty())
                throw ScopeDecodingFailure("scope JSON is missing a required string field");
        }
        const auto& kind = fields.at("kind");
        if (kind != "workflow" && kind != "package")
            throw ScopeDecodingFailure("scope JSON contains an unsupported object kind");
        return fields;
    }

private:
    void whitespace() {
        while (position_ < input_.size()) {
            const char value = input_[position_];
            if (value != ' ' && value != '\t' && value != '\r' && value != '\n')
                break;
            ++position_;
        }
    }

    bool take(char expected) {
        if (position_ == input_.size() || input_[position_] != expected)
            return false;
        ++position_;
        return true;
    }

    void expect(char expected) {
        if (!take(expected))
            throw ScopeDecodingFailure("scope JSON has an invalid or truncated structure");
    }

    char next() {
        if (position_ == input_.size())
            throw ScopeDecodingFailure("scope JSON has a truncated string");
        return input_[position_++];
    }

    std::uint32_t hexadecimal() {
        std::uint32_t value = 0;
        for (std::size_t index = 0; index < 4; ++index) {
            const char digit = next();
            value <<= 4;
            if (digit >= '0' && digit <= '9') {
                value |= static_cast<std::uint32_t>(digit - '0');
            } else if (digit >= 'a' && digit <= 'f') {
                value |= static_cast<std::uint32_t>(digit - 'a' + 10);
            } else if (digit >= 'A' && digit <= 'F') {
                value |= static_cast<std::uint32_t>(digit - 'A' + 10);
            } else {
                throw ScopeDecodingFailure("scope JSON has an invalid Unicode escape");
            }
        }
        return value;
    }

    void unicode(std::string& output) {
        auto value = hexadecimal();
        if (value >= 0xd800 && value <= 0xdbff) {
            expect('\\');
            expect('u');
            const auto low = hexadecimal();
            if (low < 0xdc00 || low > 0xdfff)
                throw ScopeDecodingFailure("scope JSON has an invalid surrogate pair");
            value = 0x10000 + ((value - 0xd800) << 10) + (low - 0xdc00);
        } else if (value >= 0xdc00 && value <= 0xdfff) {
            throw ScopeDecodingFailure("scope JSON has an unpaired surrogate");
        }
        appendCodePoint(output, value);
    }

    std::string string() {
        expect('"');
        std::string result;
        while (true) {
            const char value = next();
            if (value == '"')
                break;
            if (static_cast<unsigned char>(value) < 0x20)
                throw ScopeDecodingFailure("scope JSON has an unescaped control character");
            if (value != '\\') {
                result.push_back(value);
                continue;
            }
            switch (next()) {
            case '"':
                result.push_back('"');
                break;
            case '\\':
                result.push_back('\\');
                break;
            case '/':
                result.push_back('/');
                break;
            case 'b':
                result.push_back('\b');
                break;
            case 'f':
                result.push_back('\f');
                break;
            case 'n':
                result.push_back('\n');
                break;
            case 'r':
                result.push_back('\r');
                break;
            case 't':
                result.push_back('\t');
                break;
            case 'u':
                unicode(result);
                break;
            default:
                throw ScopeDecodingFailure("scope JSON has an invalid escape");
            }
        }
        if (!validUtf8(result))
            throw ScopeDecodingFailure("scope JSON contains invalid UTF-8");
        return result;
    }

    std::string_view input_;
    std::size_t position_ = 0;
};
}

pipelang::model::Result<ScopeFields, std::string> decodeScopeObject(
    std::string_view input, std::size_t maximumBytes) {
    using Result = pipelang::model::Result<ScopeFields, std::string>;
    if (maximumBytes == 0 || input.size() > maximumBytes)
        return Result::Failure("scope JSON exceeds its transport byte contract");
    try {
        return Result::Success(ScopeParser(input).parse());
    } catch (const ScopeDecodingFailure& error) {
        return Result::Failure(error.what());
    }
}

}
