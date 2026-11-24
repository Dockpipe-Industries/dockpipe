// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "Model.hpp"
#include "DriverProcessAdapter.hpp"
#include "JsonDigestStage.hpp"
#include "LaneOutputFiles.hpp"
#include "PrivateJsonFileStage.hpp"

#include <pipelang/io/ExclusiveOutputAdapter.hpp>
#include <pipelang/io/FileAdapter.hpp>
#include <pipelang/json/DocumentWriterAdapter.hpp>
#include <pipelang/json/ReaderAdapter.hpp>
#include <pipelang/json/StringIndexAdapter.hpp>

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

namespace source = qualification_fresh_lane::source;
namespace io = source::PipeLang::IO;
namespace json = source::PipeLang::Json;
namespace crypto = source::PipeLang::Crypto;
namespace tool = source::Dockpipe::Product::DriverExecution;
namespace pins = source::Dockpipe::Product::DriverSources;
namespace lane = source::Dockpipe::Product::FreshLane;
namespace qualification_files = source::Dockpipe::Product::QualificationFiles;

using Reader = pipelang::json::ReaderAdapter<json::JsonReader, json::JsonReadLimits, json::JsonNode, json::JsonKind>;
using Indexer = pipelang::json::StringIndexAdapter<json::JsonStringIndexer, json::JsonStringIndexLimits,
    json::JsonStringIndex, json::JsonNode>;
using Files = pipelang::io::FileAdapter<io::FileStreams, io::CapturedFile, io::FileDigest,
    io::FileIdentity, io::IoFailure, io::OutputEffect, crypto::Sha256Streams, crypto::CryptoFailure, crypto::Sha256Digest>;
using LaneFiles = product_harness::LaneOutputFiles<io::FileStreams, Files, io::CapturedFile, io::FileDigest,
    io::IoFailure>;
using Writer = pipelang::io::ExclusiveOutputAdapter<io::ExclusiveFileOutput, io::FileDigest, io::FileIdentity,
    io::IoFailure, io::OutputEffect, crypto::Sha256Streams, crypto::CryptoFailure, crypto::Sha256Digest>;
using Output = pipelang::json::DocumentWriterAdapter<json::JsonDocumentOutput, json::JsonNode>;
using Staged = product_harness::PrivateJsonFileStage<qualification_files::PrivateJsonStage, io::FileDigest, Writer>;
using Crypto = pipelang::crypto::StreamAdapter<crypto::Sha256Streams, crypto::CryptoFailure, crypto::Sha256Digest>;
using DigestStage = product_harness::JsonDigestStage<qualification_files::PrivateJsonStage, io::FileDigest, Crypto>;
using Driver = product_harness::DriverProcessAdapter<tool::DriverProcess, tool::DriverProcessRequest,
    tool::DriverProcessResult, tool::DriverProcessKind, Writer>;

namespace {

void require(bool condition, std::string_view reason) {
    if (!condition)
        throw std::runtime_error(std::string(reason));
}

class Paths final : public lane::LanePaths {
    using Result = pipelang::model::Result<lane::LaneLaunchPaths, std::string>;
public:
    Paths(std::filesystem::path sourceRoot, std::filesystem::path invocation)
        : sourceRoot_(std::move(sourceRoot)), invocation_(std::move(invocation)) {}

    Result Resolve(pins::DriverSourcePin selected) override {
        if (selected.root != pins::DriverSourceRoot::Product || selected.relative != "fixture.py")
            return Result::Failure("lane fixture source selection changed");

        lane::LaneLaunchPaths paths;
        paths.workdir = invocation_.string();
        paths.entrypoint = (sourceRoot_ / selected.relative).string();
        paths.request = (invocation_ / "request.json").string();
        paths.output = (invocation_ / "output").string();
        return Result::Success(std::move(paths));
    }

private:
    std::filesystem::path sourceRoot_;
    std::filesystem::path invocation_;
};

std::string bytes(const std::filesystem::path& path) {
    std::ifstream stream(path, std::ios::binary);
    require(stream.is_open(), "Independent expected effect is missing");
    return {std::istreambuf_iterator<char>(stream), {}};
}

void readback(const std::filesystem::path& invocation, bool passes) {
    const auto evidence = invocation / "output/fixture.txt";
    require(bytes(evidence) == "runner protocol fixture", "The selected lane wrote evidence through its actual action");
    std::string expected = "{\n  \"exit_code\": 7,\n  \"failure\": \"\"\n}\n";
    if (passes)
        expected = "{\n  \"exit_code\": 0,\n  \"failure\": \"\"\n}\n";

    require(bytes(invocation / "process.json") == expected, "Process receipt binds the actual child exit");
    for (const auto* name : {"request.json", "process.json", "driver.stdout", "driver.stderr"}) {
        struct stat status {};
        require(::lstat((invocation / name).c_str(), &status) == 0 && S_ISREG(status.st_mode) &&
                (status.st_mode & 0777) == 0600, "Retained lane metadata and logs are private regular files");
    }
}

void run(const std::filesystem::path& executable, const std::filesystem::path& fixture,
         const std::filesystem::path& invocation, bool passes) {
    require(invocation.is_absolute() && ::mkdir(invocation.c_str(), 0700) == 0,
            "Select a fresh invocation root");
    dockpipe::sdk::ClientConfiguration configuration;
    configuration.executable = executable.string();
    configuration.allowedWorkdirRoot = invocation.string();
    configuration.maximumStoredRuns = 1;
    configuration.maximumActiveRuns = 1;
    configuration.maximumOutputBytesPerStream = 4194304;
    configuration.maximumInventoryBytes = 262144;
    configuration.maximumArgumentBytes = 32768;
    configuration.executableMode = dockpipe::sdk::ExecutableMode::NativeTool;
    dockpipe::sdk::Client client(std::move(configuration));
    constexpr pipelang::io::PathProfile profile{4096, 64, 262144};
    Writer writer(invocation.string(), profile);
    auto reader = std::make_shared<Reader>();
    auto indexer = std::make_shared<Indexer>(reader->nativeReader());
    auto inputFiles = std::make_shared<Files>(fixture.string(), profile);
    auto laneFiles = std::make_shared<Files>(invocation.string(), profile);
    auto laneOutput = std::make_shared<LaneFiles>((invocation / "output").string(), profile);
    auto paths = std::make_shared<Paths>(fixture, invocation);
    auto driver = std::make_shared<Driver>(client, writer);

    auto request = std::make_shared<Staged>(writer, "request.json", 4194304);
    auto process = std::make_shared<Staged>(writer, "process.json", 65536);
    auto requestOutput = std::make_shared<Output>(reader->nativeReader(),
        pipelang::json::RenderOptions{4194304, pipelang::json::RenderLayout::Indented, true},
        [&](std::span<const std::uint8_t> data) { request->Append(data); });
    auto processOutput = std::make_shared<Output>(reader->nativeReader(),
        pipelang::json::RenderOptions{65536, pipelang::json::RenderLayout::Indented, true},
        [&](std::span<const std::uint8_t> data) { process->Append(data); });

    pipelang::model::TaskOptions options;
    options.evaluationStepsPerSegment = 33554432;
    lane::Tests::FreshLaneSuite suite(reader, indexer, inputFiles, laneFiles, laneOutput, paths, driver,
        requestOutput, request, processOutput, process, options);
    const auto result = suite.Run({});
    require(!client.hasUnreapedTree() && client.receipts().size() == 1,
            "One actual child process ran with closed custody");
    require(reader->nativeReader().liveDocuments() == 0 &&
            pipelang::json::Reader::aggregateRetainedBytes() == 0, "All scoped JSON documents closed");
    if (passes) {
        if (!result.is_success())
            throw std::runtime_error(result.error());

        const auto actual = laneFiles->DigestFile("request.json", 4194304, {});
        require(result.value().requested == 1 && result.value().admitted == 1 && result.value().passed == 1
                && actual.is_success() && actual.value().sha256 == result.value().requestSha256,
                "Source joins the actual request pin, process status and disk evidence");
    } else {
        require(result.is_failure() && result.error() == "qualification.receipt.failing_process_pass",
                "A nonzero real process cannot claim passing coverage");
    }

    const auto originalPlan = inputFiles->DigestFile("plan.json", 4194304, {});
    const auto originalRegistry = inputFiles->DigestFile("registry.json", 4194304, {});
    const auto storedRequest = bytes(invocation / "request.json");
    require(originalPlan.is_success() && originalRegistry.is_success() &&
            storedRequest.find(originalPlan.value().sha256) != std::string::npos &&
            storedRequest.find(originalRegistry.value().sha256) != std::string::npos,
            "The retained request binds the exact original wire bytes of both input documents");

    readback(invocation, passes);
}


void revalidate(const std::filesystem::path& fixture, const std::filesystem::path& invocation,
                std::string_view expectedFailure, std::int64_t maximumMetadata = 33554432) {
    constexpr pipelang::io::PathProfile profile{4096, 64, 262144};
    auto reader = std::make_shared<Reader>();
    auto indexer = std::make_shared<Indexer>(reader->nativeReader());
    auto inputFiles = std::make_shared<Files>(fixture.string(), profile);
    auto laneFiles = std::make_shared<Files>(invocation.string(), profile);
    auto laneOutput = std::make_shared<Files>((invocation / "output").string(), profile);
    Crypto crypto;
    auto digest = std::make_shared<DigestStage>(crypto, 4194304);
    auto requestOutput = std::make_shared<Output>(reader->nativeReader(),
        pipelang::json::RenderOptions{4194304, pipelang::json::RenderLayout::Indented, true},
        [&](std::span<const std::uint8_t> data) { digest->Append(data); });

    const auto retainedRequest = bytes(invocation / "request.json");
    const auto retainedProcess = bytes(invocation / "process.json");
    const auto retainedEvidence = bytes(invocation / "output/fixture.txt");
    pipelang::model::TaskOptions options;
    options.evaluationStepsPerSegment = 33554432;
    lane::Tests::RetainedLaneSuite suite(reader, indexer, inputFiles, laneFiles, laneOutput,
                                         requestOutput, digest, options);
    const auto result = suite.Run(maximumMetadata, {});
    if (expectedFailure.empty()) {
        if (!result.is_success())
            throw std::runtime_error(result.error());

        require(result.value().admitted == 1 && result.value().passed == 1,
                "The retained lane contributes its revalidated evidence");
    } else {
        if (!result.is_failure())
            throw std::runtime_error("The retained lane unexpectedly admitted the request");

        if (result.error() != expectedFailure)
            throw std::runtime_error(result.error());
    }

    require(reader->nativeReader().liveDocuments() == 0 &&
            pipelang::json::Reader::aggregateRetainedBytes() == 0, "All retained-lane documents closed");
    require(bytes(invocation / "request.json") == retainedRequest &&
            bytes(invocation / "process.json") == retainedProcess &&
            bytes(invocation / "output/fixture.txt") == retainedEvidence, "Retained artifacts stay untouched");
}

void replaceFixtureBytes(const std::filesystem::path& path, const std::string& content) {
    std::ofstream stream(path, std::ios::binary | std::ios::trunc);
    stream << content;
    stream.close();
    require(stream.good(), "Change only the scratch fixture's retained evidence");
}

} // namespace

int main(int argc, char** argv) {
    try {
        require(argc == 4, "Select the fixture, output base and approved legacy interpreter");
        const std::filesystem::path input(argv[1]);
        const std::filesystem::path output(argv[2]);
        const std::filesystem::path interpreter(argv[3]);
        require(input.is_absolute() && output.is_absolute() && interpreter.is_absolute() &&
                ::mkdir(output.c_str(), 0700) == 0, "Select explicit fresh lane proof roots");
        run(interpreter, input / "pass", output / "pass", true);
        run(interpreter, input / "reject", output / "reject", false);
        revalidate(input / "pass", output / "pass", "");
        revalidate(input / "pass", output / "pass", "qualification.run.receipt_metadata_limit", 128);
        revalidate(input / "reject", output / "reject", "qualification.receipt.failing_process_pass");

        const auto request = output / "pass/request.json";
        const auto originalRequest = bytes(request);
        auto changed = originalRequest;
        const auto position = changed.find("protocol-lane");
        require(position != std::string::npos, "The request includes the fixture run identity");
        changed[position] = 'x';
        replaceFixtureBytes(request, changed);
        revalidate(input / "pass", output / "pass", "qualification.resume.lane.request_mismatch");
        replaceFixtureBytes(request, originalRequest);

        replaceFixtureBytes(output / "pass/process.json", "{\"exit_code\":null,\"failure\":\"\"}\n");
        revalidate(input / "pass", output / "pass", "qualification.resume.lane.process_incomplete");
        std::cout << "Two real fresh driver cases and five read-only retained cases passed\n";
        return 0;
    } catch (const std::exception& error) {
        std::cerr << error.what() << '\n';
        return 1;
    }
}
