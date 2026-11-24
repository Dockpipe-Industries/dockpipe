// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include <pipelang/json/Reader.hpp>
#include <pipelang/model/byte_buffer.hpp>
#include <pipelang/model/cancellation.hpp>
#include <pipelang/model/result.hpp>

#include <array>
#include <chrono>
#include <cstddef>
#include <cstdint>
#include <cstdio>
#include <ctime>
#include <functional>
#include <new>
#include <span>
#include <string>
#include <string_view>
#include <utility>
#include <vector>

namespace product_harness {

template<class Contract, class Event, class Projected, class Journal, class Failure, class Effect>
class QualificationEventPublisher final : public Contract {
    using Token = pipelang::model::CancellationToken;
    using Result = pipelang::model::Result<void, Failure>;
    using ProjectResult = pipelang::model::Result<Projected, std::string>;

public:
    static constexpr std::size_t maximumLineBytes = 262144;
    using Project = std::function<ProjectResult(Event, const std::string&, pipelang::json::JsonByteSink, Token)>;
    using Warnings = std::function<bool(std::string_view)>;

    QualificationEventPublisher(Journal& journal, Project project, Warnings warnings)
        : journal_(journal), project_(std::move(project)), warnings_(std::move(warnings)) {}

    Result Publish(Event selected, Token token) override {
        if (token.is_cancelled())
            return Result::Cancelled(Fail("qualification.events.cancelled", Effect::NoDurableOutput));

        Effect custody = Effect::NoDurableOutput;
        try {
            return Emit(std::move(selected), token, custody);
        } catch (const std::bad_alloc&) {
            return Result::Failure(Fail("qualification.events.memory_limit", custody));
        }
    }

private:
    Journal& journal_;
    Project project_;
    Warnings warnings_;

    Result Emit(Event selected, Token token, Effect& custody) {
        std::string stamp = Timestamp();
        if (stamp.empty())
            return Result::Failure(Fail("qualification.events.timestamp", Effect::NoDurableOutput));

        std::vector<std::uint8_t> bytes;
        auto rendered = project_(std::move(selected), stamp, [&](std::span<const std::uint8_t> fragment) {
            if (fragment.size() > maximumLineBytes - bytes.size())
                throw pipelang::json::ReadFailure("qualification.events.line_limit");

            bytes.insert(bytes.end(), fragment.begin(), fragment.end());
        }, token);
        if (rendered.is_cancelled())
            return Result::Cancelled(Fail(rendered.error(), Effect::NoDurableOutput));
        if (!rendered.is_success())
            return Result::Failure(Fail(rendered.error(), Effect::NoDurableOutput));
        if (rendered.value().bytes != static_cast<std::int64_t>(bytes.size()))
            return Result::Failure(Fail("qualification.events.line_extent", Effect::NoDurableOutput));

        pipelang::model::ByteBuffer line(std::move(bytes));
        custody = Effect::Unknown;
        auto evaluated = journal_.Append(std::move(line), token);
        if (evaluated.is_cancelled())
            return Result::Cancelled(Fail(evaluated.error(), custody));
        if (!evaluated.is_success())
            return Result::Failure(Fail(evaluated.error(), custody));

        Result appended = std::move(evaluated.value());
        if (!appended.is_success())
            return appended;
        custody = Effect::Committed;
        if (!rendered.value().warning.empty() && !warnings_(rendered.value().warning))
            return Result::Failure(Fail("qualification.events.warning_write", Effect::Committed));

        return Result::Success();
    }

    static Failure Fail(std::string code, Effect effect) {
        Failure failure;
        failure.code = std::move(code);
        failure.effect = effect;
        return failure;
    }

    static std::string Timestamp() {
        using namespace std::chrono;
        const auto now = system_clock::now();
        const auto seconds = floor<std::chrono::seconds>(now);
        const auto fraction = duration_cast<microseconds>(now - seconds).count();
        const std::time_t secondsUtc = system_clock::to_time_t(seconds);
        std::tm utc{};
#ifdef _WIN32
        if (gmtime_s(&utc, &secondsUtc) != 0)
            return {};
#else
        if (gmtime_r(&secondsUtc, &utc) == nullptr)
            return {};
#endif

        std::array<char, 20> calendar{};
        if (std::strftime(calendar.data(), calendar.size(), "%Y-%m-%dT%H:%M:%S", &utc) != calendar.size() - 1)
            return {};

        if (fraction == 0)
            return std::string(calendar.data()) + "+00:00";

        std::array<char, 14> offset{};
        const auto count = std::snprintf(offset.data(), offset.size(), ".%06lld+00:00", static_cast<long long>(fraction));
        if (count < 0 || static_cast<std::size_t>(count) >= offset.size())
            return {};

        return std::string(calendar.data()) + offset.data();
    }
};

} // namespace product_harness
