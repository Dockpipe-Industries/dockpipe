// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "Model.hpp"
#include "DriverProcessAdapter.hpp"
#include "JsonDigestStage.hpp"
#include "QualificationEventPublisher.hpp"
#include "LaneOutputFiles.hpp"
#include "PrivateJsonFileStage.hpp"

#include <pipelang/io/ExclusiveOutputAdapter.hpp>
#include <pipelang/io/FileAdapter.hpp>
#include <pipelang/io/ExistingFileAppendAdapter.hpp>
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

namespace source = qualification_lane_execution::source;
namespace io = source::PipeLang::IO;
namespace json = source::PipeLang::Json;
namespace crypto = source::PipeLang::Crypto;
namespace tool = source::Dockpipe::Product::DriverExecution;
namespace pins = source::Dockpipe::Product::DriverSources;
namespace lane = source::Dockpipe::Product::FreshLane;
namespace events = source::Dockpipe::Product::QualificationEventOutput;
namespace execute = source::Dockpipe::Product::LaneExecution;
namespace preflight = source::Dockpipe::Product::QualificationPreflight;
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
using Append = pipelang::io::ExistingFileAppendAdapter<io::ExistingFileAppend, io::AppendFileState,
    io::FileIdentity, io::IoFailure, io::OutputEffect>;
using Publisher = product_harness::QualificationEventPublisher<events::QualificationEventPort,
    events::QualificationEvent, events::QualificationEventWrite, events::QualificationEventJournal,
    io::IoFailure, io::OutputEffect>;
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

execute::QualificationLaneInput select(const std::shared_ptr<Reader>& reader, const std::shared_ptr<Indexer>& indexer,
    const std::shared_ptr<Files>& files, preflight::QualificationPreflight& inputs,
    const pipelang::model::TaskOptions& options) {
    const auto admitted = inputs.Open("plan.json", "registry.json", {});
    if (!admitted.is_success())
        throw std::runtime_error(admitted.error());

    execute::QualificationTargetPlan plan(reader, indexer, files, files, 4096, options);
    const auto target = plan.Select(admitted.value(), 0, {});
    if (!target.is_success())
        throw std::runtime_error(target.error());

    require(target.value().expectedCases == 1 && target.value().lanes.values().size() == 1,
            "The Source target plan selects exactly one applicable lane");
    return target.value().lanes.values().front();
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

    auto append = std::make_shared<Append>(invocation.string(), profile);
    auto eventFiles = std::make_shared<Writer>(invocation.string(), profile);
    events::QualificationEventJournal journal(eventFiles, append);
    const auto opened = journal.Open(false, 8388608, {});
    require(opened.is_success() && opened.value().is_success(), "Create the actual lane event journal");
    auto publisher = std::make_shared<Publisher>(journal, [&](events::QualificationEvent event,
        const std::string& stamp, pipelang::json::JsonByteSink sink, pipelang::model::CancellationToken token) {
        require(event.kind == events::QualificationEventKind::DriverStarted,
                "The actual fresh lane records its start before dispatch");
        require(std::filesystem::exists(invocation / "request.json") &&
                !std::filesystem::exists(invocation / "output"),
                "The event follows request commit and precedes the child");
        auto jsonOutput = std::make_shared<Output>(reader->nativeReader(),
            pipelang::json::RenderOptions{Publisher::maximumLineBytes, pipelang::json::RenderLayout::Spaced, true},
            std::move(sink));
        events::QualificationEventProjection projection(jsonOutput);
        return projection.Write(std::move(event), stamp, token);
    }, [](std::string_view) { return false; });

    pipelang::model::TaskOptions options;
    options.evaluationStepsPerSegment = 33554432;
    preflight::QualificationPreflight inputs(reader, indexer, inputFiles, inputFiles, inputFiles, inputFiles, 4096, options);
    const auto prepared = select(reader, indexer, inputFiles, inputs, options);
    execute::FreshLaneExecution execution(reader, indexer, 4096, inputFiles, inputFiles, laneFiles,
        laneOutput, paths, driver, publisher, prepared.targetId, prepared.driverId, 33554432,
        requestOutput, request, processOutput, process, options);
    const auto result = execution.Run(prepared, {});
    const auto laneClosed = execution.Close();
    const auto inputsClosed = inputs.Close();
    require(laneClosed.is_success() && inputsClosed.is_success(), "Close the lane and both original inputs");
    require(result.is_success() && result.value().exitCode.has_value(), "Preserve actual driver exit status");
    require(!client.hasUnreapedTree() && client.receipts().size() == 1, "One real child with closed custody");
    require(reader->nativeReader().liveDocuments() == 0 &&
            pipelang::json::Reader::aggregateRetainedBytes() == 0, "All borrowed documents closed");

    const auto& outcome = result.value();
    require(outcome.exitCode.value() == (passes ? 0 : 7), "The lane returns the real child exit");
    if (passes) {
        require(outcome.rejection.empty() && outcome.admitted.has_value() && outcome.admitted.value().passing == 1,
                "Accept the successful receipt and real evidence");
    } else {
        require(outcome.rejection == "qualification.receipt.failing_process_pass" && !outcome.admitted.has_value(),
                "Refuse the passing receipt from the actual failing child");
    }
    require(outcome.remainingMetadataBytes < 33554432, "Report the actual receipt charge to the run");

    const auto journalBytes = bytes(invocation / "events.jsonl");
    require(journalBytes.find("driver-started") != std::string::npos &&
            journalBytes.find(R"("driver": "fixture")") != std::string::npos,
            "Retain one start record for the selected lane");
    readback(invocation, passes);
}


void revalidate(const std::filesystem::path& fixture, const std::filesystem::path& invocation,
                std::string_view expectedFailure, std::int64_t expectedExit,
                std::int64_t maximumMetadata = 33554432) {
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

    const auto priorJournal = bytes(invocation / "events.jsonl");
    pipelang::model::TaskOptions options;
    options.evaluationStepsPerSegment = 33554432;
    preflight::QualificationPreflight inputs(reader, indexer, inputFiles, inputFiles, inputFiles, inputFiles, 4096, options);
    const auto prepared = select(reader, indexer, inputFiles, inputs, options);
    execute::RetainedLaneExecution execution(reader, indexer, 4096, inputFiles, inputFiles, laneFiles,
        laneOutput, maximumMetadata, requestOutput, digest, options);
    const auto result = execution.Run(prepared, {});
    const auto laneClosed = execution.Close();
    const auto inputsClosed = inputs.Close();
    require(laneClosed.is_success() && inputsClosed.is_success(), "Close retained documents and scratch hash");
    require(result.is_success(), "Retained observation returns a typed terminal outcome");
    const auto& outcome = result.value();
    require(outcome.exitCode.has_value() && outcome.exitCode.value() == expectedExit,
            "Retained metadata preserves the terminal exit");
    if (expectedFailure.empty()) {
        require(outcome.rejection.empty() && outcome.admitted.has_value() && outcome.admitted.value().passing == 1,
                "Retained success still requires admitted disk evidence");
    } else {
        require(!outcome.admitted.has_value() && outcome.rejection.starts_with(expectedFailure) &&
                outcome.rejection.find("owning driver must reconcile") != std::string::npos,
                "Retained rejection requires reconciliation");
    }
    require(bytes(invocation / "events.jsonl") == priorJournal &&
            pipelang::json::Reader::aggregateRetainedBytes() == 0, "Retained execution never launches or appends a start");
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
        revalidate(input / "pass", output / "pass", "", 0);
        revalidate(input / "pass", output / "pass", "qualification.run.receipt_metadata_limit", 0, 128);
        revalidate(input / "reject", output / "reject", "qualification.receipt.failing_process_pass", 7);
        replaceFixtureBytes(output / "pass/process.json", "{\"exit_code\":42,\"failure\":\"driver deadline\"}\n");
        revalidate(input / "pass", output / "pass", "driver deadline", 42);
        std::cout << "Six actual fresh and retained execution-composition cases passed\n";
        return 0;
    } catch (const std::exception& error) {
        std::cerr << error.what() << '\n';
        return 1;
    }
}
