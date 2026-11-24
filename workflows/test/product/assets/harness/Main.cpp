#include "native/EvidenceReport.hpp"
#include "native/MonotonicClock.hpp"
#include "DockpipeSDKBridge.hpp"
#include "native/FailedStartControl.hpp"

#include <chrono>
#include <iostream>
#include <memory>
#include <thread>

int main(int argc, char** argv) {
    using namespace product_harness;
    try {
        const auto options = Options::parse(argc, argv);
        ConfinedFiles proofFiles(options.proofRoot);
#ifdef _WIN32
        auto recoveryDirectory = proofFiles.directory(options.recoveryProject);
        auto cancellationDirectory = proofFiles.directory(options.cancellationProject);
#else
        (void)proofFiles.directory(options.recoveryProject);
        (void)proofFiles.directory(options.cancellationProject);
#endif
        auto reportOutput = proofFiles.createReport(options.report);
        auto clock = std::make_shared<MonotonicClock>();
        auto files = std::make_shared<FileAdapter>(proofFiles);
        std::shared_ptr<dockpipe::sdk::Client> processes;
        EvidenceReport report;
        std::string identityEvidence;
        std::string executableIdentity;
        bool emergency = false;
        bool unreaped = false;
        try {
            if (!options.identityFile.empty()) {
                auto [exists, contents] = proofFiles.read(options.identityFile, 1024 * 1024);
                if (!exists)
                    throw std::runtime_error("selected identity evidence file does not exist");
                identityEvidence = std::move(contents);
            }
            dockpipe::sdk::ClientConfiguration configuration;
            configuration.executable = options.cli;
            configuration.allowedWorkdirRoot = options.proofRoot;
            configuration.stateDirectory = options.stateDirectory;
            configuration.cacheDirectory = options.cacheDirectory;
            configuration.maximumStoredRuns = 64;
            configuration.maximumActiveRuns = 1;
            configuration.maximumOutputBytesPerStream = 32768;
            configuration.maximumInventoryBytes = 1024 * 1024;
            configuration.maximumArgumentBytes = 65536;
#ifdef _WIN32
            configuration.consoleSignalExecutable = options.consoleSignalExecutable;
#endif
            processes = std::make_shared<dockpipe::sdk::Client>(std::move(configuration));
            auto transport = std::make_shared<product_sdk::Transport>(processes);
            executableIdentity = processes->executableIdentity();
            product::HarnessInputs inputs{};
            inputs.runId = options.runId;
            inputs.recoveryProject = options.recoveryProject;
            inputs.cancellationProject = options.cancellationProject;
            inputs.workflowFile = options.workflow;
            inputs.outsideFile = options.outsideFile;
            inputs.outsideDirectory = options.outsideDirectory;
#ifdef _WIN32
            auto workflowFile = dockpipe::sdk::openWindowsPath(options.workflow, false);
            auto outsideFile = dockpipe::sdk::openWindowsPath(options.outsideFile, false);
            auto outsideDirectory = dockpipe::sdk::openWindowsPath(options.outsideDirectory, true);
            inputs.recoveryProject = dockpipe::sdk::windowsUtf8(recoveryDirectory.nativePath);
            inputs.cancellationProject = dockpipe::sdk::windowsUtf8(cancellationDirectory.nativePath);
            inputs.workflowFile = dockpipe::sdk::windowsUtf8(workflowFile.nativePath);
            inputs.outsideFile = dockpipe::sdk::windowsUtf8(outsideFile.nativePath);
            inputs.outsideDirectory = dockpipe::sdk::windowsUtf8(outsideDirectory.nativePath);
#endif

            {
                pipelang::model::CancellationSource bodyCancellation;
                pipelang::model::CancellationSource cleanupCancellation;
                product::Harness model(clock, transport, files, inputs);
                auto task = model.Run(bodyCancellation.token(), cleanupCancellation.token());
                // An infrastructure watchdog only: all case ordering, phase
                // deadlines, interruption decisions and assertions live in source.
                const auto watchdog = std::chrono::steady_clock::now() + std::chrono::seconds(60);
                while (!task.completed()) {
                    model.poll();
                    if (std::chrono::steady_clock::now() >= watchdog) {
                        task.cancel();
                        throw std::runtime_error("source task exceeded the 60-second infrastructure watchdog");
                    }
                    std::this_thread::sleep_for(std::chrono::milliseconds(1));
                }
                report.results(task.result());
            }
            verifyFailedStartCleanup(clock, proofFiles, inputs, processes->receipts());
            std::cout << "Failed-start custody recovery control passed\n";
            // Model destruction drains scoped callbacks before these shared ports
            // can be released, including the exception and watchdog paths below.
        } catch (const std::exception& error) {
            report.fail(error.what());
        }
        std::vector<ProcessReceipt> processReceipts;
        if (processes) {
            emergency = processes->emergencyDispose();
            unreaped = processes->hasUnreapedTree();
            if (emergency)
                report.fail("native emergency disposal was needed; source cleanup is not qualified");
            if (unreaped)
                report.fail("owned descendant tree remains unconfirmed; outer containment is the emergency backstop");
            processReceipts = processes->receipts();
            for (const auto& receipt : processReceipts) {
                if (!receipt.infrastructureFailure.empty())
                    report.fail(receipt.infrastructureFailure);
                if (receipt.observation.forced)
                    report.fail("a CLI invocation required forced disposal");
            }
        }
        if (files->exhausted())
            report.fail("file evidence capacity exhausted");
        const auto content = report.render(options, executableIdentity, identityEvidence, processReceipts,
                                           files->receipts(), emergency, unreaped);
        ConfinedFiles::writeReport(reportOutput.get(), content);
        std::cout << "PipeLang product harness report: " << options.report << '\n';
        return report.passed() ? 0 : 1;
    } catch (const std::exception& error) {
        std::cerr << "Product harness infrastructure failure: " << error.what() << '\n';
        return 2;
    }
}
