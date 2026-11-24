// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "DockpipeSDKBridge.hpp"
#include "ControlSession.hpp"
#include "ProjectTransport.hpp"
#include "Report.hpp"
#include "TraceFiles.hpp"
#ifdef _WIN32
#include "WindowsProject.hpp"
#else
#include "LinuxProject.hpp"
#endif

#include <chrono>
#include <cstdint>
#include <iostream>
#include <memory>
#include <stdexcept>
#include <string>
#include <thread>
#include <utility>
#include <vector>

namespace {
namespace proof = workflow_control_proof::source::Dockpipe::WorkflowControl::Proof;
namespace async_testing = workflow_control_proof::source::PipeLang::Testing::Async;

class MonotonicClock final : public async_testing::TestClock {
public:
    pipelang::model::Result<std::int64_t, std::string> NowMilliseconds() override {
        const auto elapsed = std::chrono::steady_clock::now().time_since_epoch();
        const auto milliseconds = std::chrono::duration_cast<std::chrono::milliseconds>(elapsed).count();
        return pipelang::model::Result<std::int64_t, std::string>::Success(milliseconds);
    }
};
}

int main(int argc, char** argv) {
    std::shared_ptr<workflow_control_native::ControlSession> session;
    bool passed = false;
    try {
        if (argc != 8 && argc != 13)
            throw std::invalid_argument("Supply CLI config root state cache helper traceRoot and optional config2 root2 state2 cache2 traceRoot2");

        auto executables = workflow_control_native::admitExecutables(argv[1], argv[6]);
        const auto& cliPath = executables.cliPath;
        const auto& helperPath = executables.helperPath;
        std::vector<workflow_control_native::AdmittedProject> admitted;
        admitted.reserve(2);
        admitted.push_back(workflow_control_native::admitProject(cliPath, helperPath, argv[2], argv[3], argv[4], argv[5], argv[7], 0));
        if (argc == 13)
            admitted.push_back(workflow_control_native::admitProject(cliPath, helperPath, argv[8], argv[9], argv[10], argv[11], argv[12], 1));

        std::vector<dockpipe::sdk::ClientConfiguration> profiles;
        std::vector<std::shared_ptr<workflow_control_native::TraceRoot>> traceRoots;
        for (const auto& project : admitted) {
            profiles.push_back(project.configuration);
            traceRoots.push_back(project.traceFiles);
        }
        session = std::make_shared<workflow_control_native::ControlSession>(std::move(profiles));
        auto primary = std::make_shared<workflow_control_native::ProjectTransport>(session, 0);
        auto relocated = primary;
        if (admitted.size() == 2)
            relocated = std::make_shared<workflow_control_native::ProjectTransport>(session, 1);

        constexpr std::size_t maximumTraceBytes = 32768;
        auto files = std::make_shared<workflow_control_native::TraceFiles>(
            std::move(traceRoots), maximumTraceBytes, session->receiptCapacity());
        auto clock = std::make_shared<MonotonicClock>();
        proof::ControlInputs inputs{};
        inputs.primary = admitted[0].inputs;
        if (admitted.size() == 2)
            inputs.relocated = admitted[1].inputs;
        inputs.maximumTraceBytes = maximumTraceBytes;

        proof::ControlHarness model(primary, relocated, clock, inputs, files);
        const auto budget = model.WatchdogMilliseconds();
        if (budget <= 0)
            throw std::runtime_error("Source watchdog budget must be positive");

        pipelang::model::CancellationSource bodyCancellation;
        pipelang::model::CancellationSource cleanupCancellation;
        auto task = model.Run(bodyCancellation.token(), cleanupCancellation.token());
        const auto watchdog = std::chrono::steady_clock::now() + std::chrono::milliseconds(budget);
        while (!task.completed()) {
            model.poll();
            if (std::chrono::steady_clock::now() >= watchdog) {
                task.cancel();
                throw std::runtime_error("Source control suite exceeded its source-provided infrastructure watchdog");
            }
            std::this_thread::sleep_for(std::chrono::milliseconds(1));
        }

        passed = workflow_control_native::report(task.result(), *session, *files);
    } catch (const std::exception& error) {
        std::cerr << "Workflow-control consumer infrastructure failure: " << error.what() << '\n';
    }

    if (session) {
        if (session->emergencyDispose()) {
            std::cerr << "Native emergency disposal was required\n";
            passed = false;
        }
        if (session->hasUnreapedTree()) {
            std::cerr << "Native session retains an unconfirmed tree\n";
            passed = false;
        }
    }

    return passed ? 0 : 1;
}
