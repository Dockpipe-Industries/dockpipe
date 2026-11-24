// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "Model.hpp"
#include "DriverProcessAdapter.hpp"

#include <pipelang/io/ExclusiveOutputAdapter.hpp>
#include <pipelang/json/DocumentWriterAdapter.hpp>

#include <array>
#include <chrono>
#include <csignal>
#include <cstdint>
#include <cstdio>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <iterator>
#include <memory>
#include <span>
#include <stdexcept>
#include <string>
#include <string_view>
#include <sys/stat.h>
#include <thread>
#include <unistd.h>
#include <vector>

namespace api = qualification_driver_runtime::source;
namespace io = api::PipeLang::IO;
namespace json = api::PipeLang::Json;
namespace crypto = api::PipeLang::Crypto;
namespace driver = api::Dockpipe::Product::DriverExecution;
using Writer = pipelang::io::ExclusiveOutputAdapter<io::ExclusiveFileOutput, io::FileDigest,
    io::FileIdentity, io::IoFailure, io::OutputEffect, crypto::Sha256Streams,
    crypto::CryptoFailure, crypto::Sha256Digest>;
using Tool = product_harness::DriverProcessAdapter<driver::DriverProcess, driver::DriverProcessRequest,
    driver::DriverProcessResult, driver::DriverProcessKind, Writer>;
using Metadata = pipelang::json::DocumentWriterAdapter<json::JsonDocumentOutput, json::JsonNode>;

namespace {

enum class Scenario { Ordinary, Timeout, Overflow, Precancelled, CancelledDuringRun };
constexpr std::array<char, 4> witness{'o', '\0', static_cast<char>(255), '\n'};
constexpr std::string_view ready = "driver-ready\n";
constexpr std::string_view stopped = "driver-terminated\n";

void require(bool condition, std::string_view reason) {
    if (!condition)
        throw std::runtime_error(std::string(reason));
}

void terminate(int) {
    const auto written = ::write(STDERR_FILENO, stopped.data(), stopped.size());
    ::_exit(written == static_cast<ssize_t>(stopped.size()) ? 42 : 98);
}

int waitChild() {
    if (std::signal(SIGTERM, terminate) == SIG_ERR ||
        ::write(STDOUT_FILENO, ready.data(), ready.size()) != static_cast<ssize_t>(ready.size()))
        return 93;

    std::ofstream marker("ready", std::ios::binary);
    marker << "ready\n";
    marker.close();
    if (!marker)
        return 94;

    std::this_thread::sleep_for(std::chrono::seconds(30));
    return 95;
}

int overflowChild() {
    std::array<char, 65536> block{};
    block.fill('x');
    for (int fragment = 0; fragment < 64; ++fragment) {
        if (std::fwrite(block.data(), 1, block.size(), stdout) != block.size())
            return 96;
    }
    if (std::fputc('!', stdout) == EOF)
        return 97;

    return 0;
}

std::string fileBytes(const std::filesystem::path& path) {
    std::ifstream input(path, std::ios::binary);
    require(input.is_open(), "Expected the actual native capture file");
    return {std::istreambuf_iterator<char>(input), {}};
}

void privateLog(const std::filesystem::path& path) {
    struct stat status {};
    require(::lstat(path.c_str(), &status) == 0 && S_ISREG(status.st_mode) &&
            (status.st_mode & 0777) == 0600, "Raw capture is an exclusively owned private regular file");
}

void run(const std::filesystem::path& binary, const std::filesystem::path& root, Scenario scenario) {
    require(::mkdir(root.c_str(), 0700) == 0, "Choose a fresh private lane directory");
    dockpipe::sdk::ClientConfiguration selected;
    selected.executable = binary.string();
    selected.allowedWorkdirRoot = root.string();
    selected.maximumStoredRuns = 1;
    selected.maximumActiveRuns = 1;
    selected.maximumOutputBytesPerStream = 4194304;
    selected.maximumInventoryBytes = 262144;
    selected.maximumArgumentBytes = 32768;
    selected.executableMode = dockpipe::sdk::ExecutableMode::NativeTool;
    dockpipe::sdk::Client client(std::move(selected));

    constexpr pipelang::io::PathProfile profile{4096, 64, 262144};
    Writer logs(root.string(), profile);
    auto tool = std::make_shared<Tool>(client, logs);
    pipelang::json::Reader reader;
    std::int64_t processOutput = 0;
    auto metadata = std::make_shared<Metadata>(reader,
        pipelang::json::RenderOptions{65536, pipelang::json::RenderLayout::Indented, true},
        [&](std::span<const std::uint8_t> bytes) {
            if (processOutput == 0) {
                const auto opened = logs.BeginExclusiveOutput("process.json", 65536, {});
                if (!opened.is_success())
                    throw pipelang::json::ReadFailure(opened.error().code);

                processOutput = opened.value();
            }

            pipelang::model::ByteBuffer fragment(std::vector<std::uint8_t>(bytes.begin(), bytes.end()));
            const auto appended = logs.AppendOutput(processOutput, std::move(fragment), {});
            if (!appended.is_success())
                throw pipelang::json::ReadFailure(appended.error().code);
        });

    driver::DriverProcessRequest request;
    request.workdir = root.string();
    request.timeoutSeconds = scenario == Scenario::Timeout ? 1 : 10;
    if (scenario == Scenario::Ordinary || scenario == Scenario::Precancelled) {
        request.arguments.add("--child");
    } else if (scenario == Scenario::Overflow) {
        request.arguments.add("--overflow");
    } else {
        request.arguments.add("--wait");
    }

    pipelang::model::CancellationSource cancellation;
    std::jthread coordinator;
    if (scenario == Scenario::Precancelled) {
        cancellation.cancel();
    } else if (scenario == Scenario::CancelledDuringRun) {
        coordinator = std::jthread([&] {
            const auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(3);
            while (!std::filesystem::exists(root / "ready") && std::chrono::steady_clock::now() < deadline)
                std::this_thread::sleep_for(std::chrono::milliseconds(10));

            cancellation.cancel();
        });
    }

    const bool cancel = scenario == Scenario::Precancelled || scenario == Scenario::CancelledDuringRun;
    driver::Tests::DriverProcessSuite suite(tool, metadata);
    const auto result = suite.Run(request, cancel, {}, cancellation.token());
    if (coordinator.joinable())
        coordinator.join();
    if (processOutput != 0) {
        if (result.is_success())
            require(logs.CommitOutput(processOutput, {}).is_success(), "Process metadata closes durably");

        require(logs.CloseOutput(processOutput).is_success(), "Process output slot retires");
    }
    require(!client.hasUnreapedTree(), "Every owned child is closed");

    if (cancel) {
        require(result.is_cancelled() && !std::filesystem::exists(root / "process.json"),
                "Cancellation leaves no process receipt to authorize replay");
        if (scenario == Scenario::Precancelled)
            require(client.receipts().empty() && !std::filesystem::exists(root / "driver.stdout"),
                    "Pre-cancel has no output or process effect");
    } else {
        if (!result.is_success())
            throw std::runtime_error(result.error());

        privateLog(root / "process.json");
        const auto persisted = fileBytes(root / "process.json");
        require(result.value() == static_cast<std::int64_t>(persisted.size()), "Process receipt extent is exact");
        if (scenario == Scenario::Ordinary) {
            require(persisted == "{\n  \"exit_code\": 17,\n  \"failure\": \"\"\n}\n", "Nonzero ordinary exit is observed data");
        } else if (scenario == Scenario::Timeout) {
            require(persisted == "{\n  \"exit_code\": 42,\n  \"failure\": \"Driver exceeded its declared deadline\"\n}\n",
                    "Timeout retains the exit of the observed termination handler");
        } else {
            require(persisted.find("Driver log exceeded its 4 MiB per-stream bound") != std::string::npos,
                    "Overflow creates the selected terminal receipt failure");
        }
    }

    if (scenario == Scenario::Precancelled)
        return;

    privateLog(root / "driver.stdout");
    privateLog(root / "driver.stderr");
    if (scenario == Scenario::Ordinary) {
        require(fileBytes(root / "driver.stdout") == std::string(witness.begin(), witness.end()),
                "Raw capture retains arbitrary bytes");
    } else if (scenario == Scenario::Overflow) {
        require(std::filesystem::file_size(root / "driver.stdout") == 4194304,
                "Overflow retains the exact 4 MiB prefix");
    } else {
        require(std::filesystem::exists(root / "ready") && fileBytes(root / "driver.stdout") == ready &&
                fileBytes(root / "driver.stderr") == stopped,
                "Graceful native termination and independent log custody are both observed");
    }
}

} // namespace

int main(int argc, char** argv) {
    if (argc > 1 && std::string_view(argv[1]) == "--child") {
        if (::write(STDOUT_FILENO, witness.data(), witness.size()) != static_cast<ssize_t>(witness.size()))
            return 99;
        return 17;
    }
    if (argc > 1 && std::string_view(argv[1]) == "--wait")
        return waitChild();
    if (argc > 1 && std::string_view(argv[1]) == "--overflow")
        return overflowChild();

    try {
        require(argc == 2, "Select a fresh proof root");
        const auto binary = std::filesystem::canonical(argv[0]);
        const std::filesystem::path root(argv[1]);
        require(root.is_absolute() && ::mkdir(root.c_str(), 0700) == 0, "Select a fresh absolute proof root");
        run(binary, root / "ordinary", Scenario::Ordinary);
        run(binary, root / "timeout", Scenario::Timeout);
        run(binary, root / "overflow", Scenario::Overflow);
        run(binary, root / "pre-cancelled", Scenario::Precancelled);
        run(binary, root / "cancelled", Scenario::CancelledDuringRun);
        std::cout << "Five real Source driver transport and process receipt cases passed\n";
        return 0;
    } catch (const std::exception& error) {
        std::cerr << error.what() << '\n';
        return 1;
    }
}
