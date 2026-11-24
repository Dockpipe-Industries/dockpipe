// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include "Client.hpp"

#include <algorithm>
#include <chrono>
#include <cstdint>
#include <optional>
#include <string>
#include <thread>
#include <utility>

namespace product_harness {

enum class DriverExecutionStatus { Complete, DeadlineExceeded, CaptureOverflow, Cancelled, Failed, CustodyUnknown };

struct DriverExecution {
    DriverExecutionStatus status = DriverExecutionStatus::Failed;
    std::optional<dockpipe::sdk::OperationObservation> observation;
    std::string detail;
};

// One dedicated SDK client owns the process and its raw streams. Its caller
// chooses the command and budgets, retains the output, and decides case status.
class DriverRuntime {
public:
    using Clock = std::chrono::steady_clock;
    using Token = pipelang::model::CancellationToken;

    explicit DriverRuntime(dockpipe::sdk::Client& client) : client_(client) {}

    DriverExecution Execute(dockpipe::sdk::NativeProcessRequest request,
                            std::chrono::milliseconds timeout, Token token) {
        if (timeout < std::chrono::seconds(1) || timeout > std::chrono::hours(4))
            return {DriverExecutionStatus::Failed, std::nullopt, "Driver deadline is outside its explicit bound"};
        if (token.is_cancelled())
            return {DriverExecutionStatus::Cancelled, std::nullopt, {}};

        const auto deadline = Clock::now() + timeout;
        auto started = client_.StartNativeTool(std::move(request));
        if (!started.is_success()) {
            const auto retained = client_.RetainedOperation();
            if (retained.is_success())
                return settle(retained.value(), DriverExecutionStatus::Failed, started.error());

            return {DriverExecutionStatus::Failed, std::nullopt, started.error()};
        }

        return observe(started.value(), deadline, token);
    }

private:
    dockpipe::sdk::Client& client_;

    DriverExecution observe(dockpipe::sdk::OperationId id, Clock::time_point deadline, Token token) {
        while (Clock::now() < deadline) {
            if (token.is_cancelled())
                return settle(id, DriverExecutionStatus::Cancelled, {});

            const auto remaining = std::chrono::ceil<std::chrono::milliseconds>(deadline - Clock::now()).count();
            const auto interval = std::clamp<std::int64_t>(remaining, 0, 100);
            auto observed = client_.Observe(id, interval, token);
            if (observed.is_cancelled())
                return settle(id, DriverExecutionStatus::Cancelled, observed.error());

            if (!observed.is_success()) {
                const auto receipts = client_.receipts();
                if (!receipts.empty()) {
                    const auto& current = receipts.back().observation;
                    if (current.id == id && (current.stdoutTruncated || current.stderrTruncated))
                        return settle(id, DriverExecutionStatus::CaptureOverflow, observed.error());
                }

                return settle(id, DriverExecutionStatus::Failed, observed.error());
            }
            if (observed.value().treeComplete) {
                auto status = observed.value().exited ? DriverExecutionStatus::Complete : DriverExecutionStatus::Failed;
                return {status, std::move(observed.value()), {}};
            }
        }

        return settle(id, DriverExecutionStatus::DeadlineExceeded, {});
    }

#ifndef _WIN32
    std::optional<dockpipe::sdk::OperationObservation> gracefullyStop(dockpipe::sdk::OperationId id) {
        const auto interrupted = client_.Interrupt(id);
        if (!interrupted.is_success())
            return std::nullopt;

        const auto deadline = Clock::now() + std::chrono::seconds(10);
        while (Clock::now() < deadline) {
            const auto observed = client_.Observe(id, 50, {});
            if (observed.is_success()) {
                if (observed.value().treeComplete)
                    return observed.value();
            } else {
                // A capture failure is sticky, but the observer still reaps.
                const auto snapshots = client_.receipts();
                if (snapshots.empty() || snapshots.back().observation.id != id)
                    return std::nullopt;

                const auto& state = snapshots.back().observation;
                if (state.treeComplete)
                    return state;

                std::this_thread::sleep_for(std::chrono::milliseconds(50));
            }
        }

        return std::nullopt;
    }
#endif

    DriverExecution settle(dockpipe::sdk::OperationId id, DriverExecutionStatus status, std::string detail) {
        // An independent cleanup budget survives cancellation of the run.
        // Use the already-owned SDK operation. No path or PID is re-resolved.
#ifndef _WIN32
        if (status == DriverExecutionStatus::Cancelled || status == DriverExecutionStatus::DeadlineExceeded ||
            status == DriverExecutionStatus::CaptureOverflow) {
            auto stopped = gracefullyStop(id);
            if (stopped.has_value())
                return {status, std::move(stopped), std::move(detail)};
        }
#endif
        auto disposed = client_.ForceDispose(id, 5000, {});
        if (!disposed.is_success())
            return {DriverExecutionStatus::CustodyUnknown, std::nullopt, disposed.error()};

        auto state = std::move(disposed.value());
        if (!state.treeComplete)
            return {DriverExecutionStatus::CustodyUnknown, std::move(state), std::move(detail)};

        return {status, std::move(state), std::move(detail)};
    }
};

} // namespace product_harness
