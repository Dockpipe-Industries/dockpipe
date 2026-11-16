#pragma once

#include "FileAdapter.hpp"
#include "Json.hpp"
#include "Options.hpp"
#include "ProcessAdapter.hpp"

namespace product_harness {
namespace testing = dockpipe_product::source::PipeLang::Testing;
namespace async_testing = dockpipe_product::source::PipeLang::Testing::Async;

class EvidenceReport final {
public:
    void fail(std::string error) {
        passed_ = false;
        failures_.push_back(std::move(error));
    }

    bool passed() const noexcept { return passed_; }

    template<class SuiteResult>
    void results(const SuiteResult& result) {
        if (!result.is_success()) {
            taskOutcome_ = result.is_cancelled() ? "Cancelled" : "Failure";
            fail(result.error());
            return;
        }
        taskOutcome_ = "Success";
        bool first = true;
        passed_ = passed_ && result.value().success;
        for (const auto& entry : result.value().cases.values()) {
            if (!first)
                results_ += ",";
            first = false;
            const auto& test = entry.test;
            results_ += "{\"id\":" + json(test.id) + ",\"name\":" + json(test.name) +
                ",\"outcome\":" + json(outcome(test.outcome)) +
                ",\"primaryOutcome\":" + json(outcome(test.primaryOutcome)) +
                ",\"primaryPhase\":" + json(phase(test.primaryPhase)) +
                ",\"primaryFailure\":" + optionalFailure(test.primaryFailure) +
                ",\"cleanupOutcome\":" + json(outcome(test.cleanupOutcome)) +
                ",\"cleanupFailure\":" + optionalFailure(test.cleanupFailure) +
                ",\"cleanupClockFailure\":" + optionalFailure(entry.cleanupClockFailure) +
                ",\"bodyRan\":" + json(test.bodyRan) +
                ",\"cleanupRan\":" + json(test.cleanupRan) + "}";
        }
    }

    std::string render(const Options& options, const std::string& executableIdentity,
                       const std::string& identityEvidence, const std::vector<ProcessReceipt>& processes,
                       const std::vector<FileReceipt>& files, bool emergency, bool unreaped) const {
        std::string result = "{\n\"schema\":1,\"stringEncoding\":\"bytes-as-u00xx\",\"runId\":" + json(options.runId) +
            ",\"passed\":" + json(passed_) + ",\"taskOutcome\":" + json(taskOutcome_) +
            ",\"cli\":" + json(options.cli) + ",\"cliOpenedIdentity\":" + json(executableIdentity) +
            ",\"identityFile\":" + json(options.identityFile) + ",\"identityEvidence\":" + json(identityEvidence) +
            ",\"emergencyDisposal\":" + json(emergency) + ",\"unreapedTree\":" + json(unreaped) +
            ",\"results\":[" + results_ + "],\n\"infrastructureFailures\":[";
        bool first = true;
        for (const auto& failure : failures_) {
            if (!first)
                result += ",";
            first = false;
            result += json(failure);
        }
        result += "],\n\"processReceipts\":[";
        first = true;
        for (const auto& receipt : processes) {
            if (!first)
                result += ",";
            first = false;
            const auto& observed = receipt.observation;
            result += "{\"invocationId\":" + std::to_string(observed.invocationId) +
                ",\"leaderPid\":" + std::to_string(receipt.leaderPid) +
                ",\"exited\":" + json(observed.exited) + ",\"exitCode\":" + std::to_string(observed.exitCode) +
                ",\"treeComplete\":" + json(observed.treeComplete) + ",\"forced\":" + json(observed.forced) +
                ",\"interruptRequested\":" + json(receipt.interruptRequested) +
                ",\"reapedChildren\":" + std::to_string(observed.reapedChildren) +
                ",\"stdoutText\":" + json(observed.stdoutText) + ",\"stderrText\":" + json(observed.stderrText) +
                ",\"scopePath\":" + json(observed.scopePath) +
                ",\"infrastructureFailure\":" + json(receipt.infrastructureFailure) + ",\"argv\":[";
            bool firstArgument = true;
            for (const auto& argument : receipt.arguments) {
                if (!firstArgument)
                    result += ",";
                firstArgument = false;
                result += json(argument);
            }
            result += "]}";
        }
        result += "],\n\"fileReceipts\":[";
        first = true;
        for (const auto& receipt : files) {
            if (!first)
                result += ",";
            first = false;
            result += "{\"path\":" + json(receipt.path) + ",\"byteLimit\":" + std::to_string(receipt.byteLimit) +
                ",\"success\":" + json(receipt.success) + ",\"exists\":" + json(receipt.exists) +
                ",\"content\":" + json(receipt.content) + ",\"error\":" + json(receipt.error) + "}";
        }
        result += "]\n}\n";
        return result;
    }

private:
    static std::string outcome(testing::TestOutcome value) {
        switch (value) {
        case testing::TestOutcome::Passed: return "Passed";
        case testing::TestOutcome::Failed: return "Failed";
        case testing::TestOutcome::Cancelled: return "Cancelled";
        }
        return "Unknown";
    }

    static std::string phase(testing::TestPhase value) {
        switch (value) {
        case testing::TestPhase::NotRun: return "NotRun";
        case testing::TestPhase::Setup: return "Setup";
        case testing::TestPhase::Body: return "Body";
        case testing::TestPhase::Cleanup: return "Cleanup";
        }
        return "Unknown";
    }

    template<class OptionalFailure>
    static std::string optionalFailure(const OptionalFailure& value) {
        if (!value.has_value())
            return "null";
        return "{\"code\":" + json(value.value().code) + ",\"message\":" + json(value.value().message) + "}";
    }

    bool passed_ = true;
    std::string taskOutcome_ = "NotStarted";
    std::string results_;
    std::vector<std::string> failures_;
};
}
