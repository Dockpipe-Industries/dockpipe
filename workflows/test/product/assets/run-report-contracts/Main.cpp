// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "Model.hpp"
#include "LinuxRunnerCapture.hpp"
#include "PrivateJsonFileStage.hpp"
#include "ReportTargetAdapter.hpp"

#include <pipelang/io/AtomicReplacementAdapter.hpp>
#include <pipelang/io/ExclusiveOutputAdapter.hpp>
#include <pipelang/io/FileAdapter.hpp>
#include <pipelang/json/DocumentWriterAdapter.hpp>
#include <pipelang/json/ReaderAdapter.hpp>

#include <filesystem>
#include <fstream>
#include <iostream>
#include <iterator>
#include <memory>
#include <optional>
#include <span>
#include <stdexcept>
#include <string>
#include <string_view>
#include <utility>
#include <sys/stat.h>

namespace source = qualification_run_report::source;
namespace io = source::PipeLang::IO;
namespace json = source::PipeLang::Json;
namespace crypto = source::PipeLang::Crypto;
namespace files = source::Dockpipe::Product::QualificationFiles;
namespace run = source::Dockpipe::Product::RunIdentity;
namespace request = source::Dockpipe::Product::LaneRequest;
namespace report = source::Dockpipe::Product::RunReports;
namespace coverage = source::Dockpipe::Product::Coverage;

using Reader = pipelang::json::ReaderAdapter<json::JsonReader, json::JsonReadLimits, json::JsonNode, json::JsonKind>;
using Files = pipelang::io::FileAdapter<io::FileStreams, io::CapturedFile, io::FileDigest,
    io::FileIdentity, io::IoFailure, io::OutputEffect, crypto::Sha256Streams, crypto::CryptoFailure, crypto::Sha256Digest>;
using Writer = pipelang::io::ExclusiveOutputAdapter<io::ExclusiveFileOutput, io::FileDigest, io::FileIdentity,
    io::IoFailure, io::OutputEffect, crypto::Sha256Streams, crypto::CryptoFailure, crypto::Sha256Digest>;
using Replacement = pipelang::io::AtomicReplacementAdapter<io::AtomicFileReplacement, io::FileIdentity,
    io::IoFailure, io::OutputEffect>;
using Output = pipelang::json::DocumentWriterAdapter<json::JsonDocumentOutput, json::JsonNode>;
using Stage = product_harness::PrivateJsonFileStage<files::PrivateJsonStage, io::FileDigest, Writer>;
using Capture = product_harness::LinuxRunnerCapture<run::RunnerCapture, run::QualificationRunnerIdentity, Files, io::FileDigest>;
using Targets = product_harness::ReportTargetAdapter<report::ReportTargetOutput>;
using Structured = pipelang::json::DocumentWriter;
using Path = std::filesystem::path;

namespace {

enum class Scenario { Qualified, Incomplete, Failed, Interrupted, ChangedIdentity, Cancelled };

void require(bool condition, std::string_view reason) {
    if (!condition)
        throw std::runtime_error(std::string(reason));
}

template<class Value>
Value unwrap(pipelang::model::Result<Value, std::string> result) {
    if (!result.is_success())
        throw std::runtime_error(result.error());

    return std::move(result.value());
}

std::string load(const Path& path) {
    std::ifstream stream(path, std::ios::binary);
    require(stream.is_open(), "Read back the actual selected result file");
    return {std::istreambuf_iterator<char>(stream), {}};
}

void checkMode(const Path& path) {
    struct stat status {};
    require(::lstat(path.c_str(), &status) == 0 && S_ISREG(status.st_mode) && (status.st_mode & 0777) == 0600,
            "Report commit keeps a private regular file");
}

void runCase(const Path& directory, const Path& driver, const Path& replacementDriver, Scenario scenario) {
    const auto stagingDirectory = directory / "stage";
    const auto resultPath = directory / "result.json";
    require(::mkdir(directory.c_str(), 0700) == 0 && ::mkdir(stagingDirectory.c_str(), 0700) == 0,
            "Create exclusive private report staging directories");

    constexpr std::string_view oldReport = "{\"previous\":true}\n";
    std::ofstream previous(resultPath, std::ios::binary);
    previous << oldReport;
    previous.close();
    require(previous.good() && ::chmod(resultPath.c_str(), 0600) == 0, "Seed a prior valid report");

    constexpr auto layout = pipelang::json::RenderLayout::Indented;
    constexpr std::size_t maximum = 33554432;
    constexpr pipelang::io::PathProfile profile{4096, 64, 262144};
    auto reader = std::make_shared<Reader>();
    auto stageOwner = std::make_shared<Writer>(stagingDirectory.string(), profile, maximum);
    auto stage = std::make_shared<Stage>(*stageOwner, "report.json", maximum);
    auto replace = std::make_shared<Replacement>(directory.string(), "stage", profile);
    Capture before(driver, profile);
    const auto admitted = unwrap(before.Capture({}));
    const Path currentDriver = scenario == Scenario::ChangedIdentity ? replacementDriver : driver;
    auto current = std::make_shared<Capture>(currentDriver, profile);

    Structured writer(reader->nativeReader(), {maximum, layout, true},
        [&](std::span<const std::uint8_t> bytes) { stage->Append(bytes); });
    auto output = std::make_shared<Output>(writer);
    auto targets = std::make_shared<Targets>(writer, pipelang::json::RenderOptions{maximum, layout, true, 2});
    report::QualificationRunReport result(reader, output, targets, current, stage, replace);
    auto& nativeReader = reader->nativeReader();
    std::string_view profiles = "{\"platforms\":[{\"id\":\"p\"}]}";
    if (scenario == Scenario::Incomplete)
        profiles = "{\"platforms\":[{\"id\":\"p\"},{\"id\":\"q\"}]}";

    const auto document = nativeReader.begin({});
    nativeReader.append(document, std::span(reinterpret_cast<const std::uint8_t*>(profiles.data()), profiles.size()));
    const auto root = nativeReader.complete(document);
    const request::QualificationRunIdentity identity{"proof", std::string(64, 'b'), std::string(64, 'c')};
    const auto started = result.Begin(identity, admitted, {root.document, root.node}, {});
    nativeReader.close(document);
    if (!started.is_success())
        throw std::runtime_error(started.error());

    std::optional<coverage::TargetCoverage> emitted;
    targets->Stage(nativeReader, [&](Structured& targetWriter) {
        auto nested = std::make_shared<Output>(targetWriter);
        report::Tests::SealedTargetFixture fixture(reader, nested);
        const std::string status = scenario == Scenario::Failed ? "failed" : "passed";
        emitted.emplace(unwrap(fixture.Project(status, {})));
    });
    require(emitted.has_value() && nativeReader.liveDocuments() == 0,
            "The staged target is complete after its source documents close");
    const auto appended = result.Append(*emitted, {});
    if (!appended.is_success())
        throw std::runtime_error(appended.error());
    require(load(resultPath) == oldReport, "Prior result remains visible until atomic replacement");

    pipelang::model::CancellationSource cancellation;
    if (scenario == Scenario::Cancelled)
        cancellation.cancel();

    const auto evaluated = result.Finish(scenario == Scenario::Interrupted,
        "2026-11-17T00:00:00+00:00", cancellation.token());
    const auto closed = result.Close();
    if (!closed.is_success())
        throw std::runtime_error(closed.error());

    if (scenario == Scenario::ChangedIdentity) {
        if (!evaluated.is_success())
            throw std::runtime_error(evaluated.error());

        const auto& changed = evaluated.value();
        require(changed.is_failure() && changed.error().code == "qualification.report.runner_changed",
                "Refuse a report after observed interpreter identity drift");
        require(load(resultPath) == oldReport, "A changed runner identity leaves the prior result untouched");
    } else if (scenario == Scenario::Cancelled) {
        require(evaluated.is_cancelled() ||
                (evaluated.is_success() && evaluated.value().is_cancelled()),
                "A cancelled report retains owned staging");
        require(load(resultPath) == oldReport, "Cancellation leaves the prior result untouched");
    } else {
        if (!evaluated.is_success())
            throw std::runtime_error(evaluated.error());
        if (!evaluated.value().is_success())
            throw std::runtime_error(evaluated.value().error().code);

        int expectedExit = 0;
        if (scenario == Scenario::Failed)
            expectedExit = 1;
        if (scenario == Scenario::Incomplete)
            expectedExit = 2;
        if (scenario == Scenario::Interrupted)
            expectedExit = 130;

        const auto& committed = evaluated.value().value();
        require(committed.exitCode == expectedExit &&
                committed.bytes == static_cast<std::int64_t>(std::filesystem::file_size(resultPath)),
                "The actual atomic replacement matches the Source exit and output extent");
        require(!std::filesystem::exists(stagingDirectory / "report.json"), "Completed report consumes its stage");
    }

    checkMode(resultPath);
    require(pipelang::json::Reader::aggregateRetainedBytes() == 0, "Release all source reader storage");
}

} // namespace

int main(int argc, char** argv) {
    try {
        require(argc == 4, "Select the proof directory and two exact observed executable identities");
        const Path root(argv[1]);
        const Path driver(argv[2]);
        const Path alternate(argv[3]);
        require(root.is_absolute() && driver.is_absolute() && alternate.is_absolute() &&
                ::mkdir(root.c_str(), 0700) == 0, "Create a fresh report proof directory");
        runCase(root / "qualified", driver, alternate, Scenario::Qualified);
        runCase(root / "incomplete", driver, alternate, Scenario::Incomplete);
        runCase(root / "failed", driver, alternate, Scenario::Failed);
        runCase(root / "interrupted", driver, alternate, Scenario::Interrupted);
        runCase(root / "changed", driver, alternate, Scenario::ChangedIdentity);
        runCase(root / "cancelled", driver, alternate, Scenario::Cancelled);
        std::cout << "Six actual atomic run-report cases passed\n";
        return 0;
    } catch (const std::exception& error) {
        std::cerr << error.what() << '\n';
        return 1;
    }
}
