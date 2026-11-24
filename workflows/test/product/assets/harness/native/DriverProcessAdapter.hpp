// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include "DriverRuntime.hpp"

#include <pipelang/model/byte_buffer.hpp>
#include <pipelang/model/result.hpp>

#include <algorithm>
#include <chrono>
#include <cstdint>
#include <string>
#include <string_view>
#include <utility>
#include <vector>

namespace product_harness {

template<class Contract, class Request, class Process, class Kind, class ExclusiveOutput>
class DriverProcessAdapter final : public Contract {
    using Result = pipelang::model::Result<Process, std::string>;
    using Token = pipelang::model::CancellationToken;
    using Bytes = pipelang::model::ByteBuffer;

public:
    DriverProcessAdapter(dockpipe::sdk::Client& tool, ExclusiveOutput& output)
        : runtime_(tool), output_(output) {}

    Result Run(Request request, Token token) override {
        if (token.is_cancelled())
            return Result::Cancelled("qualification.driver.cancelled");
        if (request.timeoutSeconds < 1 || request.timeoutSeconds > 14400)
            return Result::Failure("qualification.driver.deadline");

        dockpipe::sdk::NativeProcessRequest launch;
        launch.workdir = std::move(request.workdir);
        launch.arguments = request.arguments.values();
        launch.outputLimitBytes = streamLimit;
        LogOutput standard(output_);
        LogOutput errors(output_);
        auto opened = standard.open("driver.stdout", token);
        if (!opened.is_success())
            return propagate(opened);

        opened = errors.open("driver.stderr", token);
        if (!opened.is_success())
            return propagate(opened);

        auto execution = runtime_.Execute(std::move(launch), std::chrono::seconds(request.timeoutSeconds), token);
        std::string_view stdoutBytes;
        std::string_view stderrBytes;
        if (execution.observation.has_value()) {
            stdoutBytes = execution.observation->stdoutText;
            stderrBytes = execution.observation->stderrText;
        }

        // Independent cleanup preserves observed bytes even when the run was
        // cancelled. The result is usable only after both named files settle.
        const auto stdoutWritten = standard.finish(stdoutBytes);
        const auto stderrWritten = errors.finish(stderrBytes);
        if (!stdoutWritten.is_success())
            return Result::Failure(stdoutWritten.error());
        if (!stderrWritten.is_success())
            return Result::Failure(stderrWritten.error());

        Process receipt;
        receipt.kind = project(execution.status);
        if (execution.observation.has_value()) {
            receipt.exited = execution.observation->exited;
            receipt.treeComplete = execution.observation->treeComplete;
            receipt.exitCode = execution.observation->exitCode;
        }
        receipt.detail = std::move(execution.detail);
        return Result::Success(std::move(receipt));
    }

private:
    using Action = pipelang::model::Result<void, std::string>;
    static constexpr std::int64_t streamLimit = 4194304;
    DriverRuntime runtime_;
    ExclusiveOutput& output_;

    static Result propagate(const Action& action) {
        if (action.is_cancelled())
            return Result::Cancelled(action.error());

        return Result::Failure(action.error());
    }

    static Kind project(DriverExecutionStatus status) {
        switch (status) {
        case DriverExecutionStatus::Complete:
            return Kind::Complete;
        case DriverExecutionStatus::DeadlineExceeded:
            return Kind::DeadlineExceeded;
        case DriverExecutionStatus::CaptureOverflow:
            return Kind::CaptureOverflow;
        case DriverExecutionStatus::Cancelled:
            return Kind::Cancelled;
        case DriverExecutionStatus::Failed:
            return Kind::Failed;
        case DriverExecutionStatus::CustodyUnknown:
            return Kind::CustodyUnknown;
        }

        return Kind::CustodyUnknown;
    }

    class LogOutput {
    public:
        explicit LogOutput(ExclusiveOutput& output) : output_(output) {}
        ~LogOutput() {
            if (operation_ != 0)
                output_.CloseOutput(operation_);
        }

        LogOutput(const LogOutput&) = delete;
        LogOutput& operator=(const LogOutput&) = delete;

        Action open(std::string name, Token token) {
            const auto started = output_.BeginExclusiveOutput(std::move(name), streamLimit, token);
            if (started.is_cancelled())
                return Action::Cancelled("qualification.driver.log." + started.error().code);
            if (!started.is_success())
                return Action::Failure("qualification.driver.log." + started.error().code);

            operation_ = started.value();
            return Action::Success();
        }

        Action finish(std::string_view raw) {
            while (!raw.empty()) {
                const auto count = std::min(raw.size(), std::size_t{65536});
                const auto* first = reinterpret_cast<const std::uint8_t*>(raw.data());
                Bytes chunk(std::vector<std::uint8_t>(first, first + count));
                const auto appended = output_.AppendOutput(operation_, std::move(chunk), {});
                if (!appended.is_success())
                    return Action::Failure("qualification.driver.log." + appended.error().code);

                raw.remove_prefix(count);
            }

            const auto committed = output_.CommitOutput(operation_, {});
            if (!committed.is_success())
                return Action::Failure("qualification.driver.log." + committed.error().code);

            const auto closed = output_.CloseOutput(operation_);
            if (!closed.is_success())
                return Action::Failure("qualification.driver.log." + closed.error().code);

            operation_ = 0;
            return Action::Success();
        }

    private:
        ExclusiveOutput& output_;
        std::int64_t operation_ = 0;
    };
};

} // namespace product_harness
