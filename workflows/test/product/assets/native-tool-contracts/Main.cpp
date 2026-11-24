// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "Client.hpp"

#include <array>
#include <chrono>
#include <cstdio>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <stdexcept>
#include <string>
#include <string_view>
#include <thread>

namespace {

using dockpipe::sdk::Client;
using dockpipe::sdk::ClientConfiguration;
using dockpipe::sdk::ExecutableMode;

constexpr std::array<char, 5> expectedOutput{'x', '\0', static_cast<char>(255), '\n', 'y'};
constexpr std::string_view testArgument = "space \" quote \\ end";

int child(int argc, char** argv) {
    if (argc != 4 || argv[2] != testArgument || std::string_view(argv[3]) != "" ||
        std::filesystem::current_path().filename() != "tool-workdir")
        return 81;

    if (std::fwrite(expectedOutput.data(), 1, expectedOutput.size(), stdout) != expectedOutput.size() ||
        std::fputs("native-error\n", stderr) == EOF)
        return 82;

    return 17;
}

void check(bool result, const char* message) {
    if (!result)
        throw std::runtime_error(message);
}

ClientConfiguration select(const std::filesystem::path& binary, const std::filesystem::path& root,
                           ExecutableMode mode) {
    ClientConfiguration configuration;
    configuration.executable = binary.string();
    configuration.allowedWorkdirRoot = root.string();
    configuration.maximumStoredRuns = 4;
    configuration.maximumActiveRuns = 1;
    configuration.maximumOutputBytesPerStream = 4194304;
    configuration.maximumInventoryBytes = 262144;
    configuration.maximumArgumentBytes = 32768;
    configuration.executableMode = mode;
    return configuration;
}

dockpipe::sdk::OperationObservation waitForTree(Client& tool, dockpipe::sdk::OperationId id) {
    const auto stop = std::chrono::steady_clock::now() + std::chrono::seconds(10);
    while (std::chrono::steady_clock::now() < stop) {
        const auto observed = tool.Observe(id, 100, {});
        if (!observed.is_success())
            throw std::runtime_error(observed.error());

        if (observed.value().treeComplete)
            return observed.value();
    }

    const auto forced = tool.ForceDispose(id, 5000, {});
    if (!forced.is_success() || !forced.value().treeComplete)
        throw std::runtime_error("native tool deadline left uncertain cleanup");

    throw std::runtime_error("native tool fixture missed its normal completion deadline");
}

void run(const std::filesystem::path& binary, const std::filesystem::path& dockpipe,
         const std::filesystem::path& root) {
    {
        Client workflow(select(dockpipe, root, ExecutableMode::Dockpipe));
        check(!workflow.StartNativeTool({root.string(), {}, 32}).is_success() && workflow.receipts().empty(),
              "a workflow-mode client rejects raw argument admission");

        const auto workflowFile = root / "workflow.yml";
        std::ofstream program(workflowFile, std::ios::binary);
        program.exceptions(std::ios::badbit | std::ios::failbit);
        program << "name: executable-mode-compatibility\nvault: none\ndocker_preflight: false\n"
                   "steps:\n  - id: witness\n    kind: host\n    cmd: /usr/bin/printf compatibility-witness\n";
        program.close();
        dockpipe::sdk::WorkflowRequest request;
        request.workdir = root.string();
        request.workflowFile = workflowFile.string();
        request.outputLimitBytes = 32768;
        const auto started = workflow.Start(request);
        if (!started.is_success())
            throw std::runtime_error(started.error());

        const auto result = waitForTree(workflow, started.value());
        check(result.exited && result.exitCode == 0 &&
              result.stdoutText.find("compatibility-witness") != result.stdoutText.npos,
              "ordinary configured Dockpipe workflow invocation remains intact");
    }

    const auto workdir = root / "tool-workdir";
    check(std::filesystem::create_directory(workdir), "select a fresh task-owned workdir");
    Client tool(select(binary, root, ExecutableMode::NativeTool));
    check(!tool.Start({}).is_success() && !tool.StartScope({}).is_success() && !tool.Scope({}, 20, {}).is_success(),
          "tool client refuses Dockpipe workflow and scope dispatch");
    check(!tool.StartNativeTool({workdir.string(), {std::string("x\0y", 3)}, 32}).is_success(),
          "embedded NUL is refused before spawn");
    check(!tool.StartNativeTool({workdir.string(), {std::string(32768, 'a')}, 32}).is_success(),
          "an oversized request takes no process custody");
    check(!tool.StartNativeTool({(root.parent_path()).string(), {}, 32}).is_success() && tool.receipts().empty(),
          "outside workdir is refused before spawn");

    const auto started = tool.StartNativeTool({workdir.string(), {"--child", std::string(testArgument), ""}, 32});
    if (!started.is_success())
        throw std::runtime_error(started.error());

    const auto status = waitForTree(tool, started.value());
    check(status.exited && status.exitCode == 17 && !status.forced &&
          status.stdoutText == std::string(expectedOutput.begin(), expectedOutput.end()) &&
          status.stderrText == "native-error\n" && !status.stdoutTruncated && !status.stderrTruncated &&
          !tool.hasUnreapedTree(), "real raw streams, argv, cwd, nonzero exit and tree custody");

    constexpr std::int64_t streamLimit = 4194304;
    const auto overflowing = tool.StartNativeTool({workdir.string(), {"--child-overflow"}, streamLimit});
    check(overflowing.is_success(), "launch actual overflowing tool");
    bool overflowObserved = false;
    const auto end = std::chrono::steady_clock::now() + std::chrono::seconds(10);
    while (!overflowObserved && std::chrono::steady_clock::now() < end) {
        const auto observed = tool.Observe(overflowing.value(), 100, {});
        if (!observed.is_success()) {
            check(observed.is_failure() && observed.error().find("per-stream byte limit") != std::string::npos,
                  "stream overflow preserves the established observation failure");
            overflowObserved = true;
        }
    }
    const auto closed = tool.ForceDispose(overflowing.value(), 5000, {});
    const auto receipts = tool.receipts();
    const auto& overflow = receipts.back().observation;
    check(overflowObserved && closed.is_success() && closed.value().treeComplete &&
          overflow.stdoutTruncated && overflow.stdoutText.size() == static_cast<std::size_t>(streamLimit) &&
          overflow.stderrText == "overflow\n" && overflow.exited,
          "an overflowing tool retains the exact bounded raw prefix and closes its tree");

    const auto waiting = tool.StartNativeTool({workdir.string(), {"--child-wait"}, 32});
    check(waiting.is_success(), "launch independently cancellable tool");
    pipelang::model::CancellationSource cancellation;
    cancellation.cancel();
    const auto cancelled = tool.Observe(waiting.value(), 100, cancellation.token());
    const auto retained = tool.RetainedOperation();
    check(cancelled.is_cancelled() && retained.is_success() && retained.value() == waiting.value(),
          "a cancelled observation preserves original native process custody");

    const auto disposed = tool.ForceDispose(waiting.value(), 5000, {});
    check(disposed.is_success() && disposed.value().treeComplete && disposed.value().forced &&
          !tool.hasUnreapedTree(), "force disposal closes the exact retained native tool tree");
}

} // namespace

int main(int argc, char** argv) {
    if (argc > 1 && std::string_view(argv[1]) == "--child")
        return child(argc, argv);

    if (argc > 1 && std::string_view(argv[1]) == "--child-overflow") {
        std::array<char, 65536> block{};
        block.fill('p');
        for (int chunk = 0; chunk < 64; ++chunk) {
            if (std::fwrite(block.data(), 1, block.size(), stdout) != block.size())
                return 83;
        }
        if (std::fputs("overflow\n", stderr) == EOF || std::fputc('!', stdout) == EOF)
            return 84;

        return 0;
    }
    if (argc > 1 && std::string_view(argv[1]) == "--child-wait") {
        std::this_thread::sleep_for(std::chrono::seconds(30));
        return 85;
    }

    try {
        if (argc != 3)
            throw std::runtime_error("Select a fresh absolute process fixture root and Dockpipe executable");

        const std::filesystem::path root(argv[1]);
        check(root.is_absolute() && std::filesystem::create_directory(root), "fresh process fixture root");
        run(std::filesystem::canonical(argv[0]), std::filesystem::canonical(argv[2]), root);
        std::cout << "Native tool executable mode, argv, raw capture and custody passed\n";
        return 0;
    } catch (const std::exception& error) {
        std::cerr << error.what() << '\n';
        return 1;
    }
}
