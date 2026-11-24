#include "DockpipeSDKBridge.hpp"
#include "FixtureTransport.hpp"
#include "CallerReport.hpp"
#include "ReceiptReport.hpp"
#include "WindowsFiles.hpp"

#include <chrono>
#include <cstdint>
#include <iostream>
#include <memory>
#include <stdexcept>
#include <string>
#include <thread>
#include <utility>

namespace {
namespace proof = sdk_fault::source::Dockpipe::SDK::FaultProof;
namespace testing = sdk_fault::source::PipeLang::Testing;
namespace async_testing = sdk_fault::source::PipeLang::Testing::Async;

std::string ambientQualificationSentinel() {
    constexpr wchar_t name[] = L"DOCKPIPE_QUALIFICATION_SENTINEL";
    ::SetLastError(ERROR_SUCCESS);
    const auto needed = ::GetEnvironmentVariableW(name, nullptr, 0);
    if (needed == 0) {
        const auto error = ::GetLastError();
        if (error == ERROR_ENVVAR_NOT_FOUND || error == ERROR_SUCCESS)
            return {};

        throw std::runtime_error("Cannot observe ambient qualification input: Win32 " + std::to_string(error));
    }
    if (needed > 32767)
        throw std::runtime_error("Ambient qualification input exceeds the Windows character contract");

    std::wstring value(needed, L'\0');
    ::SetLastError(ERROR_SUCCESS);
    const auto copied = ::GetEnvironmentVariableW(name, value.data(), needed);
    if (copied == 0) {
        if (::GetLastError() == ERROR_SUCCESS)
            return {};

        throw std::runtime_error("Ambient qualification input changed while observing it");
    }
    if (copied >= needed)
        throw std::runtime_error("Ambient qualification input changed while observing it");

    value.resize(copied);
    return dockpipe::sdk::windowsUtf8(value);
}

class MonotonicClock final : public async_testing::TestClock {
public:
    pipelang::model::Result<std::int64_t, std::string> NowMilliseconds() override {
        const auto elapsed = std::chrono::steady_clock::now().time_since_epoch();
        const auto milliseconds = std::chrono::duration_cast<std::chrono::milliseconds>(elapsed).count();

        return pipelang::model::Result<std::int64_t, std::string>::Success(milliseconds);
    }
};

class FaultSignals final : public proof::FaultSignals {
public:
    explicit FaultSignals(std::shared_ptr<sdk_fault_fixture::FixtureTransport> transport)
        : transport_(std::move(transport)) {
        if (!transport_)
            throw std::invalid_argument("Cancellation fixture requires the selected transport");
    }

    sdk_fault_host::Transport::ObservationResult ObserveCancelled(
        sdk_fault_host::source::OperationId id, std::int64_t milliseconds) override {
        pipelang::model::CancellationSource cancellation;
        cancellation.cancel();

        return transport_->Observe(id, milliseconds, cancellation.token());
    }

private:
    std::shared_ptr<sdk_fault_fixture::FixtureTransport> transport_;
};

std::string json(const std::string& text) {
    static constexpr char digits[] = "0123456789abcdef";
    std::string result = "\"";
    for (const unsigned char byte : text) {
        if (byte == '"' || byte == '\\') {
            result += '\\';
            result += static_cast<char>(byte);
        } else if (byte < 32 || byte >= 127) {
            result += "\\u00";
            result += digits[byte >> 4];
            result += digits[byte & 15];
        } else {
            result += static_cast<char>(byte);
        }
    }
    result += '"';

    return result;
}

const char* outcome(testing::TestOutcome value) {
    switch (value) {
    case testing::TestOutcome::Passed: return "Passed";
    case testing::TestOutcome::Failed: return "Failed";
    case testing::TestOutcome::Cancelled: return "Cancelled";
    }

    throw std::runtime_error("Unknown source test outcome");
}

const char* phase(testing::TestPhase value) {
    switch (value) {
    case testing::TestPhase::NotRun: return "NotRun";
    case testing::TestPhase::Setup: return "Setup";
    case testing::TestPhase::Body: return "Body";
    case testing::TestPhase::Cleanup: return "Cleanup";
    }

    throw std::runtime_error("Unknown source test phase");
}

template<class OptionalFailure>
std::string failure(const OptionalFailure& value) {
    if (!value.has_value())
        return "null";

    return "{\"code\":" + json(value.value().code) +
        ",\"message\":" + json(value.value().message) + "}";
}

template<class SuiteResult>
bool report(const SuiteResult& result, const sdk_fault_fixture::FixtureTransport& transport) {
    // Complete copies are bounded by helper-artifact count * configured runs.
    // These are raw native facts; only the source result decides case outcomes.
    const auto receipts = transport.receipts();
    if (!result.is_success()) {
        std::cout << "{\"taskOutcome\":" << json(result.is_cancelled() ? "Cancelled" : "Failure")
                  << ",\"error\":" << json(result.error())
                  << ",\"stringEncoding\":\"bytes-as-u00xx\",\"results\":[],\"nativeReceipts\":";
        sdk_fault_fixture::reportReceipts(std::cout, receipts, json);
        std::cout << ",\"callerObservations\":";
        sdk_fault_fixture::reportCallerObservations(std::cout, transport.callerObservations(), json);
        std::cout << "}\n";
        return false;
    }

    bool passed = true;
    bool first = true;
    std::cout << "{\"taskOutcome\":\"Success\",\"stringEncoding\":\"bytes-as-u00xx\",\"results\":[";
    for (const auto& entry : result.value().values()) {
        if (!first)
            std::cout << ',';
        first = false;

        const auto& test = entry.test;
        passed = passed && test.outcome == testing::TestOutcome::Passed;
        std::cout << "{\"id\":" << json(test.id) << ",\"name\":" << json(test.name)
                  << ",\"outcome\":" << json(outcome(test.outcome))
                  << ",\"primaryOutcome\":" << json(outcome(test.primaryOutcome))
                  << ",\"primaryPhase\":" << json(phase(test.primaryPhase))
                  << ",\"primaryFailure\":" << failure(test.primaryFailure)
                  << ",\"cleanupOutcome\":" << json(outcome(test.cleanupOutcome))
                  << ",\"cleanupFailure\":" << failure(test.cleanupFailure)
                  << ",\"cleanupClockFailure\":" << failure(entry.cleanupClockFailure)
                  << ",\"bodyRan\":" << (test.bodyRan ? "true" : "false")
                  << ",\"cleanupRan\":" << (test.cleanupRan ? "true" : "false") << '}';
    }
    std::cout << "],\"passed\":" << (passed ? "true" : "false") << ",\"nativeReceipts\":";
    sdk_fault_fixture::reportReceipts(std::cout, receipts, json);
    std::cout << ",\"callerObservations\":";
    sdk_fault_fixture::reportCallerObservations(std::cout, transport.callerObservations(), json);
    std::cout << "}\n";

    return passed;
}
}

int main(int argc, char** argv) {
    std::shared_ptr<sdk_fault_fixture::CallerObserver> caller;
    std::shared_ptr<sdk_fault_fixture::FixtureTransport> transport;
    bool passed = false;
    try {
        if (argc < 5 || argc > 8)
            throw std::invalid_argument("Supply fixture executable, owned absolute root, workflow selection, console helper and optional lost, partial and late offer helper paths");

        // Keep the admitted directory object alive throughout source execution.
        auto directory = dockpipe::sdk::openWindowsPath(argv[2], true);
        const auto root = dockpipe::sdk::windowsUtf8(directory.nativePath);
        dockpipe::sdk::ClientConfiguration configuration;
        configuration.executable = argv[1];
        configuration.allowedWorkdirRoot = root;
        configuration.stateDirectory = root + "/state";
        configuration.cacheDirectory = root + "/cache";
        configuration.maximumStoredRuns = 64;
        configuration.maximumActiveRuns = 1;
        configuration.maximumOutputBytesPerStream = 32768;
        configuration.maximumInventoryBytes = 1024 * 1024;
        configuration.maximumArgumentBytes = 65536;
        configuration.consoleSignalExecutable = argv[4];

        // Keep independently opened expected identities alive through the source
        // run. The sentinel is supplied by the launcher's process environment;
        // this process only observes it and never changes global environment.
        auto expectedExecutable = dockpipe::sdk::openWindowsPath(argv[1], false);
        auto expectedState = dockpipe::sdk::openWindowsPath(configuration.stateDirectory, true);
        auto expectedCache = dockpipe::sdk::openWindowsPath(configuration.cacheDirectory, true);
        proof::FaultInputs inputs{};
        inputs.ownedRoot = root;
        inputs.workflowFile = argv[3];
        inputs.expectedExecutable = dockpipe::sdk::windowsUtf8(expectedExecutable.nativePath);
        inputs.expectedStateDirectory = dockpipe::sdk::windowsUtf8(expectedState.nativePath);
        inputs.expectedCacheDirectory = dockpipe::sdk::windowsUtf8(expectedCache.nativePath);
        inputs.ambientSentinel = ambientQualificationSentinel();
        inputs.maximumArgumentBytes = sdk_fault_fixture::sourceInputInteger(configuration.maximumArgumentBytes);
        inputs.maximumOutputBytes = sdk_fault_fixture::sourceInputInteger(configuration.maximumOutputBytesPerStream);

        const auto receiptCapacity = sdk_fault_fixture::FixtureTransport::maximumReceiptCount(
            configuration.maximumStoredRuns);
        caller = std::make_shared<sdk_fault_fixture::CallerObserver>(
            sdk_fault_fixture::CallerObserver::maximumHistoryCount(receiptCapacity));
        transport = std::make_shared<sdk_fault_fixture::FixtureTransport>(std::move(configuration), caller);
        auto clock = std::make_shared<MonotonicClock>();
        auto signals = std::make_shared<FaultSignals>(transport);
        if (argc > 5) {
            inputs.lostOfferHelper = argv[5];
            if (inputs.lostOfferHelper.empty())
                throw std::invalid_argument("The explicitly supplied lost-offer helper cannot be empty");
        }

        if (argc > 6) {
            inputs.partialOfferHelper = argv[6];
            if (inputs.partialOfferHelper.empty())
                throw std::invalid_argument("The explicitly supplied partial-offer helper cannot be empty");
        }

        if (argc > 7) {
            inputs.lateOfferHelper = argv[7];
            if (inputs.lateOfferHelper.empty())
                throw std::invalid_argument("The explicitly supplied late-offer helper cannot be empty");
        }

        // Default evaluation budget is deliberate for the first qualification run.
        // The source runner owns case selection, deadlines and cleanup.
        proof::FaultHarness model(transport, clock, inputs, signals, transport);
        const auto budget = model.WatchdogMilliseconds();
        if (budget <= 0)
            throw std::runtime_error("Source watchdog budget must be positive");

        pipelang::model::CancellationSource bodyCancellation;
        pipelang::model::CancellationSource cleanupCancellation;
        auto task = model.Run(bodyCancellation.token(), cleanupCancellation.token());
        // The source supplies its aggregate phase budget. This watchdog is
        // infrastructure containment, never a second case-selection policy.
        const auto watchdog = std::chrono::steady_clock::now() + std::chrono::milliseconds(budget);
        while (!task.completed()) {
            model.poll();
            if (std::chrono::steady_clock::now() >= watchdog) {
                task.cancel();
                throw std::runtime_error("Source suite exceeded its source-provided infrastructure watchdog");
            }
            std::this_thread::sleep_for(std::chrono::milliseconds(1));
        }

        // Task<T>::result() returns T directly: here Result<List<AsyncTestResult>,string>.
        passed = report(task.result(), *transport);
    } catch (const std::exception& error) {
        std::cerr << "Fault harness infrastructure failure: " << error.what() << '\n';
        passed = false;
    }

    if (transport) {
        // Last-resort containment never upgrades an authored outcome to success.
        if (transport->emergencyDispose()) {
            std::cerr << "Native emergency disposal was required\n";
            passed = false;
        }
        if (transport->hasUnreapedTree()) {
            std::cerr << "Native client still retains an unconfirmed tree\n";
            passed = false;
        }
    }

    return passed ? 0 : 1;
}
