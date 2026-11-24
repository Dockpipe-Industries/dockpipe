#pragma once

#include "MonotonicClock.hpp"
#include "FileAdapter.hpp"
#include "ProcessEvidence.hpp"

#include <chrono>
#include <map>
#include <cstdint>
#include <string>
#include <utility>
#include <memory>
#include <stdexcept>
#include <thread>
#include <vector>

namespace product_harness {
namespace sdk_source = dockpipe_product::source::Dockpipe::SDK;

// Inject the difficult boundary where launch reports Failure after retaining a
// new operation. The real source cases must discover and clean up that handle.
// This control creates no OS processes and is separate from product receipts.
class FailedStartTransport final : public sdk_source::Transport {
public:
    using IdResult = pipelang::model::Result<sdk_source::OperationId, std::string>;
    using ObservationResult = pipelang::model::Result<sdk_source::OperationObservation, std::string>;
    using BoolResult = pipelang::model::Result<bool, std::string>;
    using ScopeResult = pipelang::model::Result<sdk_source::ScopeResult, std::string>;
    using Token = pipelang::model::CancellationToken;

    explicit FailedStartTransport(std::map<std::string, std::string> paths)
        : paths_(std::move(paths)) {}

    IdResult Start(sdk_source::WorkflowRequest) override {
        current_ = makeId(++sequence_);
        retained_ = true;
        workflow_ = true;
        interrupted_ = false;
        ++failedStarts;
        return IdResult::Failure("injected failure after taking launch custody");
    }

    IdResult StartScope(sdk_source::ScopeRequest request) override {
        const auto found = paths_.find(request.workdir);
        if (found == paths_.end() || retained_)
            return IdResult::Failure("scope request is outside the control's admitted projects");
        current_ = makeId(++sequence_);
        path_ = found->second;
        retained_ = true;
        workflow_ = false;
        interrupted_ = false;
        return IdResult::Success(current_);
    }

    IdResult RetainedOperation() override {
        if (!retained_)
            return IdResult::Failure("no retained operation");
        return IdResult::Success(current_);
    }

    BoolResult Interrupt(sdk_source::OperationId id) override {
        if (!matches(id) || !workflow_)
            return BoolResult::Failure("interrupt addressed a stale or unknown handle");
        interrupted_ = true;
        recoveredHandles.push_back(id.value);
        return BoolResult::Success(true);
    }

    ObservationResult Observe(sdk_source::OperationId id, std::int64_t, Token) override {
        if (!matches(id))
            return ObservationResult::Failure("observation addressed a stale or unknown handle");
        sdk_source::OperationObservation observation{};
        observation.id = id;
        observation.exited = !workflow_ || interrupted_;
        observation.treeComplete = observation.exited;
        observation.reapedChildren = observation.exited ? 1 : 0;
        if (!workflow_)
            observation.scopePath = path_;
        if (observation.treeComplete)
            retained_ = false;
        return ObservationResult::Success(observation);
    }

    ObservationResult ForceDispose(sdk_source::OperationId id, std::int64_t wait, Token token) override {
        ++forcedDisposals;
        interrupted_ = true;
        auto result = Observe(id, wait, token);
        if (!result.is_success())
            return result;
        auto observation = result.value();
        observation.forced = true;
        return ObservationResult::Success(observation);
    }

    ScopeResult Scope(sdk_source::ScopeRequest, std::int64_t, Token) override {
        return ScopeResult::Failure("unexpected convenience scope operation in this control");
    }

    bool retainsOperation() const noexcept { return retained_; }
    int failedStarts = 0;
    int forcedDisposals = 0;
    std::vector<std::int64_t> recoveredHandles;

private:
    static sdk_source::OperationId makeId(std::int64_t value) {
        sdk_source::OperationId id{};
        id.clientId = 901;
        id.value = value;
        return id;
    }

    bool matches(sdk_source::OperationId id) const noexcept {
        return retained_ && id.clientId == current_.clientId && id.value == current_.value;
    }

    std::map<std::string, std::string> paths_;
    sdk_source::OperationId current_{};
    std::int64_t sequence_ = 0;
    std::string path_;
    bool retained_ = false;
    bool workflow_ = false;
    bool interrupted_ = false;
};

inline void verifyFailedStartCleanup(const std::shared_ptr<MonotonicClock>& clock,
                                    const ConfinedFiles& proofFiles, const product::HarnessInputs& inputs,
                                    const std::vector<ProcessReceipt>& receipts) {
    std::map<std::string, std::string> paths;
    for (const auto& receipt : receipts) {
        if (!receipt.scopeQuery)
            continue;
        for (std::size_t index = 0; index + 1 < receipt.arguments.size(); ++index) {
            if (receipt.arguments[index] == "--workdir")
                paths.emplace(receipt.arguments[index + 1], receipt.observation.scopePath);
        }
    }
    if (paths.size() != 2)
        throw std::runtime_error("failed-start control requires two independently resolved scope paths");
    auto transport = std::make_shared<FailedStartTransport>(std::move(paths));
    auto files = std::make_shared<FileAdapter>(proofFiles);
    {
        product::Harness model(clock, transport, files, inputs);
        pipelang::model::CancellationSource bodyCancellation;
        pipelang::model::CancellationSource cleanupCancellation;
        auto task = model.Run(bodyCancellation.token(), cleanupCancellation.token());
        const auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(30);
        while (!task.completed()) {
            model.poll();
            if (std::chrono::steady_clock::now() >= deadline)
                throw std::runtime_error("failed-start control exceeded its watchdog");
            std::this_thread::sleep_for(std::chrono::milliseconds(1));
        }
        const auto& result = task.result();
        if (!result.is_success() || result.value().success || result.value().cases.values().size() != 2)
            throw std::runtime_error("failed-start control lost its expected source failure results");
        for (const auto& entry : result.value().cases.values()) {
            if (!entry.test.bodyRan || !entry.test.cleanupRan ||
                entry.test.primaryOutcome != dockpipe_product::source::PipeLang::Testing::TestOutcome::Failed ||
                entry.test.cleanupOutcome != dockpipe_product::source::PipeLang::Testing::TestOutcome::Passed)
                throw std::runtime_error("failed-start control did not preserve primary failure and successful cleanup");
        }
    }
    if (transport->failedStarts != 2 || transport->forcedDisposals != 0 || transport->retainsOperation() ||
        transport->recoveredHandles != std::vector<std::int64_t>{2, 4})
        throw std::runtime_error("source cleanup did not recover both newly retained launch handles");
}
}
