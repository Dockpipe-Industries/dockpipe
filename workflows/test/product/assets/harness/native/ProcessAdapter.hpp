#pragma once

#include "ConfinedFiles.hpp"
#include "Model.hpp"
#include <chrono>
#include <map>
#include <mutex>
#include <string>
#include <vector>

namespace product_harness {
namespace product = dockpipe_product::source::Dockpipe::Product;

struct ProcessReceipt {
    product::ProcessObservation observation;
    std::vector<std::string> arguments;
    std::string infrastructureFailure;
    std::int64_t leaderPid = 0;
    bool interruptRequested = false;
};

class ProcessAdapter final : public product::ProcessPort {
public:
    using ObservationResult = pipelang::model::Result<product::ProcessObservation, std::string>;
    using IntResult = pipelang::model::Result<std::int64_t, std::string>;
    using BoolResult = pipelang::model::Result<bool, std::string>;
    using Token = pipelang::model::CancellationToken;

    ProcessAdapter(const ConfinedFiles& files, std::string executable, std::string workflow,
                   std::string stateDirectory, std::string cacheDirectory);
    ~ProcessAdapter();
    IntResult Start(product::ProcessRequest request) override;
    ObservationResult Observe(std::int64_t invocationId, std::int64_t waitMilliseconds, Token token) override;
    BoolResult Interrupt(std::int64_t invocationId) override;
    ObservationResult ForceDispose(std::int64_t waitMilliseconds, Token token) override;
    bool emergencyDispose();
    bool hasUnreapedTree();
    std::vector<ProcessReceipt> receipts();
    const std::string& executable() const noexcept { return executable_; }
    const std::string& executableIdentity() const noexcept { return executableIdentity_; }

private:
    using Clock = std::chrono::steady_clock;
    void discoverChildren();
    void reapChildren();
    void drainOutput(Descriptor& pipe, std::string& output);
    void update();
    void signalOwned(int signal);
    bool signalChild(pid_t pid, int signal);
    void rememberFailure(const std::string& failure);
    ObservationResult wait(std::int64_t milliseconds, Token token, bool force);
    ProcessReceipt& current();
    static bool validWait(std::int64_t milliseconds) noexcept;

    const ConfinedFiles& files_;
    std::string executable_;
    std::string workflow_;
    std::string executableIdentity_;
    Descriptor executableDescriptor_;
    std::vector<std::string> environment_;
    std::mutex mutex_;
    std::map<pid_t, Descriptor> children_;
    std::vector<ProcessReceipt> receipts_;
    Descriptor stdout_;
    Descriptor stderr_;
    std::size_t outputLimit_ = 0;
    bool scopeQuery_ = false;
};
}
