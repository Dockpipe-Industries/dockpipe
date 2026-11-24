// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include "DockpipeSDKBridge.hpp"
#include "CallerObserver.hpp"
#include "InputFacts.hpp"
#include "LostOfferWitness.hpp"
#include "WindowsFiles.hpp"

#include <algorithm>
#include <cstring>
#include <limits>
#include <memory>
#include <stdexcept>
#include <utility>
#include <vector>

namespace sdk_fault_fixture {
namespace proof = sdk_fault::source::Dockpipe::SDK::FaultProof;
namespace source = sdk_fault_host::source;

class FixtureTransport final : public source::Transport, public proof::FixtureControl {
public:
    using Token = pipelang::model::CancellationToken;
    using StartResult = sdk_fault_host::Transport::StartResult;
    using ObservationResult = sdk_fault_host::Transport::ObservationResult;
    using BoolResult = sdk_fault_host::Transport::BoolResult;
    using ScopeResult = sdk_fault_host::Transport::ScopeResult;
    using VoidResult = pipelang::model::Result<void, std::string>;
    using ReceiptResult = pipelang::model::Result<proof::FaultReceipt, std::string>;
    using IntegerResult = pipelang::model::Result<std::int64_t, std::string>;
    using LateOfferResult = pipelang::model::Result<proof::LateOfferReceipt, std::string>;
    using CallerResult = pipelang::model::Result<proof::CallerObservation, std::string>;
    using MetricsResult = pipelang::model::Result<proof::ArgumentMetrics, std::string>;
    using StatsResult = pipelang::model::Result<proof::ArgumentStats, std::string>;
    using ChildInputsResult = pipelang::model::Result<proof::ChildInputs, std::string>;

    static std::size_t maximumReceiptCount(std::size_t maximumStoredRuns) {
        if (maximumStoredRuns > std::numeric_limits<std::size_t>::max() / helperArtifactCount)
            throw std::invalid_argument("Fixture receipt archive capacity overflows");

        return helperArtifactCount * maximumStoredRuns;
    }

    FixtureTransport(dockpipe::sdk::ClientConfiguration configuration, std::shared_ptr<CallerObserver> caller)
        : configuration_(std::move(configuration)), files_(configuration_.allowedWorkdirRoot), caller_(std::move(caller)) {
        if (!caller_)
            throw std::invalid_argument("Fixture requires a caller observer installed before native client creation");

        // Four explicit artifacts: normal launch/signal, lost, partial and late
        // offer. Each client has its configured finite receipt capacity. Source
        // selects which optional artifacts to use; replacement never drops facts.
        archiveCapacity_ = maximumReceiptCount(configuration_.maximumStoredRuns);
        archive_.reserve(archiveCapacity_);
        client_ = std::make_shared<dockpipe::sdk::Client>(configuration_);
        transport_ = std::make_shared<sdk_fault_host::Transport>(client_);
        selections_ = 1;
    }

    VoidResult SelectConsoleHelper(std::string absolutePath) override {
        try {
            if (finalized_)
                return VoidResult::Failure("Fixture native transport was finalized");

            if (selections_ >= helperArtifactCount)
                return VoidResult::Failure("Fixture helper artifact selection capacity exhausted");

            if (client_ && client_->hasUnreapedTree())
                return VoidResult::Failure("Fixture selection refuses to replace retained native custody");

            // Admit the new artifact and copy complete prior receipts before
            // releasing the old client. No force, fallback or hidden trimming.
            auto selected = dockpipe::sdk::openWindowsPath(absolutePath, false);
            auto next = configuration_;
            next.consoleSignalExecutable = dockpipe::sdk::windowsUtf8(selected.nativePath);
            archiveCurrent();
            transport_.reset();
            client_.reset();
            configuration_ = std::move(next);
            ++selections_;
            client_ = std::make_shared<dockpipe::sdk::Client>(configuration_);
            transport_ = std::make_shared<sdk_fault_host::Transport>(client_);
            return VoidResult::Success();
        } catch (const std::exception& error) {
            return VoidResult::Failure(error.what());
        }
    }

    VoidResult FinalizeTransport() override {
        try {
            if (finalized_)
                return VoidResult::Success();

            if (client_ && client_->hasUnreapedTree())
                return VoidResult::Failure("Fixture finalization refuses retained native custody");

            archiveCurrent();
            transport_.reset();
            client_.reset();
            finalized_ = true;
            return VoidResult::Success();
        } catch (const std::exception& error) {
            return VoidResult::Failure(error.what());
        }
    }

    CallerResult InitialCaller() override {
        try {
            return CallerResult::Success(caller_->initial());
        } catch (const std::exception& error) {
            return CallerResult::Failure(error.what());
        }
    }

    CallerResult ObserveCaller() override {
        try {
            return CallerResult::Success(caller_->observe());
        } catch (const std::exception& error) {
            return CallerResult::Failure(error.what());
        }
    }

    const std::vector<proof::CallerObservation>& callerObservations() const noexcept {
        return caller_->history();
    }

    ReceiptResult Receipt(source::OperationId id) override {
        try {
            if (client_) {
                const auto current = client_->receipts();
                for (const auto& receipt : current) {
                    if (matches(receipt, id))
                        return ReceiptResult::Success(project(receipt));
                }
            }

            for (const auto& receipt : archive_) {
                if (matches(receipt, id))
                    return ReceiptResult::Success(project(receipt));
            }

            return ReceiptResult::Failure("Fixture receipt ID is unknown");
        } catch (const std::exception& error) {
            return ReceiptResult::Failure(error.what());
        }
    }

    MetricsResult MeasureArguments(source::OperationId id) override {
        try {
            const auto stored = receipts();
            for (const auto& receipt : stored) {
                if (matches(receipt, id))
                    return MetricsResult::Success(measureArguments(receipt.arguments));
            }

            return MetricsResult::Failure("Input metrics refer to an unknown stored receipt");
        } catch (const std::exception& error) {
            return MetricsResult::Failure(error.what());
        }
    }

    IntegerResult StoredReceiptCount() override {
        try {
            return IntegerResult::Success(sourceInputInteger(receipts().size()));
        } catch (const std::exception& error) {
            return IntegerResult::Failure(error.what());
        }
    }

    StatsResult DecodeArgumentStats(std::string output) override {
        try {
            return StatsResult::Success(decodeArgumentStats(output, configuration_.maximumOutputBytesPerStream));
        } catch (const std::exception& error) {
            return StatsResult::Failure(error.what());
        }
    }

    ChildInputsResult DecodeChildInputs(std::string output) override {
        try {
            return ChildInputsResult::Success(decodeChildInputs(output, configuration_.maximumOutputBytesPerStream));
        } catch (const std::exception& error) {
            return ChildInputsResult::Failure(error.what());
        }
    }

    IntegerResult LostOfferChildProcessId() override {
        return creationProcessId(lostOfferWitnessName);
    }

    IntegerResult PartialOfferChildProcessId() override {
        return creationProcessId(partialOfferWitnessName);
    }

    LateOfferResult LateOfferAttempt() override {
        try {
            const auto path = configuration_.allowedWorkdirRoot + lateOfferWitnessName;
            const auto [exists, bytes] = files_.read(path, sizeof(LateOfferWitness));
            if (!exists || bytes.size() != sizeof(LateOfferWitness))
                return LateOfferResult::Failure("Late-offer witness is absent or has an invalid extent");

            LateOfferWitness witness;
            std::memcpy(&witness, bytes.data(), sizeof(witness));
            constexpr auto maximum = static_cast<std::uint64_t>(std::numeric_limits<std::int64_t>::max());
            if (witness.magic != lateWitnessMagic || witness.bytes != sizeof(witness) ||
                witness.reserved != 0 || witness.writeSucceeded > 1 || witness.createdAt > maximum ||
                witness.requestDeadline > maximum || witness.attemptedAt > maximum)
                return LateOfferResult::Failure("Late-offer witness has an incompatible format");

            proof::LateOfferReceipt result{};
            result.processId = witness.processId;
            result.threadId = witness.threadId;
            result.createdAt = static_cast<std::int64_t>(witness.createdAt);
            result.requestDeadline = static_cast<std::int64_t>(witness.requestDeadline);
            result.attemptedAt = static_cast<std::int64_t>(witness.attemptedAt);
            result.writeSucceeded = witness.writeSucceeded != 0;
            return LateOfferResult::Success(std::move(result));
        } catch (const std::exception& error) {
            return LateOfferResult::Failure(error.what());
        }
    }

    std::vector<dockpipe::sdk::RunReceipt> receipts() const {
        auto result = archive_;
        if (client_) {
            const auto current = client_->receipts();
            if (current.size() > archiveCapacity_ - result.size())
                throw std::runtime_error("Fixture aggregate receipt capacity exhausted");

            result.insert(result.end(), current.begin(), current.end());
        }

        return result;
    }

    StartResult Start(source::WorkflowRequest request) override {
        if (!transport_)
            return StartResult::Failure("Fixture has no selected native transport");

        return transport_->Start(std::move(request));
    }

    StartResult StartScope(source::ScopeRequest request) override {
        if (!transport_)
            return StartResult::Failure("Fixture has no selected native transport");

        return transport_->StartScope(std::move(request));
    }

    ObservationResult Observe(source::OperationId id, std::int64_t milliseconds, Token token) override {
        if (!transport_)
            return ObservationResult::Failure("Fixture has no selected native transport");

        return transport_->Observe(id, milliseconds, token);
    }

    BoolResult Interrupt(source::OperationId id) override {
        if (!transport_)
            return BoolResult::Failure("Fixture has no selected native transport");

        return transport_->Interrupt(id);
    }

    StartResult RetainedOperation() override {
        if (!transport_)
            return StartResult::Failure("Fixture has no selected native transport");

        return transport_->RetainedOperation();
    }

    ObservationResult ForceDispose(source::OperationId id, std::int64_t milliseconds, Token token) override {
        if (!transport_)
            return ObservationResult::Failure("Fixture has no selected native transport");

        return transport_->ForceDispose(id, milliseconds, token);
    }

    ScopeResult Scope(source::ScopeRequest request, std::int64_t milliseconds, Token token) override {
        if (!transport_)
            return ScopeResult::Failure("Fixture has no selected native transport");

        return transport_->Scope(std::move(request), milliseconds, token);
    }

    bool emergencyDispose() { return client_ && client_->emergencyDispose(); }
    bool hasUnreapedTree() { return client_ && client_->hasUnreapedTree(); }

private:
    static constexpr std::size_t helperArtifactCount = 4;

    void archiveCurrent() {
        const auto prior = client_ ? client_->receipts() : std::vector<dockpipe::sdk::RunReceipt>{};
        if (prior.size() > archiveCapacity_ - archive_.size())
            throw std::runtime_error("Fixture receipt archive capacity exhausted");

        archive_.insert(archive_.end(), prior.begin(), prior.end());
    }

    IntegerResult creationProcessId(const char* filename) const {
        try {
            const auto path = configuration_.allowedWorkdirRoot + filename;
            const auto [exists, bytes] = files_.read(path, sizeof(LostOfferWitness));
            if (!exists || bytes.size() != sizeof(LostOfferWitness))
                return IntegerResult::Failure("Fixture creation witness is absent or has an invalid extent");

            LostOfferWitness witness;
            std::memcpy(&witness, bytes.data(), sizeof(witness));
            if (witness.magic != witnessMagic || witness.bytes != sizeof(witness))
                return IntegerResult::Failure("Fixture creation witness has an incompatible format");

            return IntegerResult::Success(witness.processId);
        } catch (const std::exception& error) {
            return IntegerResult::Failure(error.what());
        }
    }

    static bool matches(const dockpipe::sdk::RunReceipt& receipt, source::OperationId id) {
        return receipt.observation.id.clientId == id.clientId && receipt.observation.id.value == id.value;
    }

    static proof::FaultReceipt project(const dockpipe::sdk::RunReceipt& receipt) {
        proof::FaultReceipt result{};
        const auto& observation = receipt.observation;
        result.observation.id.clientId = observation.id.clientId;
        result.observation.id.value = observation.id.value;
        result.observation.exited = observation.exited;
        result.observation.treeComplete = observation.treeComplete;
        result.observation.forced = observation.forced;
        result.observation.exitCode = observation.exitCode;
        result.observation.reapedChildren = observation.reapedChildren;
        result.observation.stdoutText = observation.stdoutText;
        result.observation.stderrText = observation.stderrText;
        result.observation.stdoutTruncated = observation.stdoutTruncated;
        result.observation.stderrTruncated = observation.stderrTruncated;
        result.observation.scopePath = observation.scopePath;
        for (const auto& argument : receipt.arguments)
            result.arguments.add(argument);

        result.infrastructureFailure = receipt.infrastructureFailure;
        result.leaderPid = receipt.leaderPid;
        result.interruptRequested = receipt.interruptRequested;
        result.scopeQuery = receipt.scopeQuery;
        result.scopePathQuery = receipt.scopePathQuery;
        return result;
    }

    dockpipe::sdk::ClientConfiguration configuration_;
    dockpipe::sdk::WindowsFiles files_;
    // Declared before the native owners so the marker outlives their destruction.
    std::shared_ptr<CallerObserver> caller_;
    std::shared_ptr<dockpipe::sdk::Client> client_;
    std::shared_ptr<sdk_fault_host::Transport> transport_;
    std::vector<dockpipe::sdk::RunReceipt> archive_;
    std::size_t archiveCapacity_ = 0;
    std::size_t selections_ = 0;
    bool finalized_ = false;
};
}
