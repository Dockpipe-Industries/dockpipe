// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "Model.hpp"
#include "JsonDigestStage.hpp"
#include "LinuxRunnerCapture.hpp"
#include "PrivateJsonFileStage.hpp"

#include <pipelang/io/ExclusiveOutputAdapter.hpp>
#include <pipelang/io/FileAdapter.hpp>
#include <pipelang/json/DocumentWriterAdapter.hpp>
#include <pipelang/json/ReaderAdapter.hpp>

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

namespace source = qualification_run_identity::source;
namespace io = source::PipeLang::IO;
namespace json = source::PipeLang::Json;
namespace crypto = source::PipeLang::Crypto;
namespace files = source::Dockpipe::Product::QualificationFiles;
namespace run = source::Dockpipe::Product::RunIdentity;

using Reader = pipelang::json::ReaderAdapter<json::JsonReader, json::JsonReadLimits, json::JsonNode, json::JsonKind>;
using Files = pipelang::io::FileAdapter<io::FileStreams, io::CapturedFile, io::FileDigest,
    io::FileIdentity, io::IoFailure, io::OutputEffect, crypto::Sha256Streams, crypto::CryptoFailure, crypto::Sha256Digest>;
using Writer = pipelang::io::ExclusiveOutputAdapter<io::ExclusiveFileOutput, io::FileDigest, io::FileIdentity,
    io::IoFailure, io::OutputEffect, crypto::Sha256Streams, crypto::CryptoFailure, crypto::Sha256Digest>;
using Output = pipelang::json::DocumentWriterAdapter<json::JsonDocumentOutput, json::JsonNode>;
using Crypto = pipelang::crypto::StreamAdapter<crypto::Sha256Streams, crypto::CryptoFailure, crypto::Sha256Digest>;
using FileStage = product_harness::PrivateJsonFileStage<files::PrivateJsonStage, io::FileDigest, Writer>;
using DigestStage = product_harness::JsonDigestStage<files::PrivateJsonStage, io::FileDigest, Crypto>;
using Capture = product_harness::LinuxRunnerCapture<run::RunnerCapture, run::QualificationRunnerIdentity, Files, io::FileDigest>;

namespace {

using Path = std::filesystem::path;
constexpr pipelang::io::PathProfile profile{4096, 64, 262144};
constexpr pipelang::json::RenderOptions render{65536, pipelang::json::RenderLayout::Indented, true};

void require(bool condition, std::string_view reason) {
    if (!condition)
        throw std::runtime_error(std::string(reason));
}

std::string load(const Path& path) {
    std::ifstream stream(path, std::ios::binary);
    require(stream.is_open(), "Expected retained run input");
    return {std::istreambuf_iterator<char>(stream), {}};
}

void requirePrivate(const Path& path) {
    struct stat status {};
    require(::lstat(path.c_str(), &status) == 0 && S_ISREG(status.st_mode) &&
            (status.st_mode & 0777) == 0600, "Run input file must be private and regular");
}

run::QualificationRunnerIdentity create(const Path& sourcePath, const Path& outputPath, const Path& driverPath) {
    auto input = std::make_shared<Files>(sourcePath.string(), profile);
    auto files = std::make_shared<Files>(outputPath.string(), profile);
    auto writer = std::make_shared<Writer>(outputPath.string(), profile);
    auto runner = std::make_shared<Capture>(driverPath, profile);
    auto reader = std::make_shared<Reader>();
    auto stage = std::make_shared<FileStage>(*writer, "runner.json", 65536);
    auto output = std::make_shared<Output>(reader->nativeReader(), render,
        [&](std::span<const std::uint8_t> data) { stage->Append(data); });

    run::Tests::RunInputWriterSuite suite(reader, input, files, writer, runner, output, stage);
    auto written = suite.Run({});
    if (!written.is_success())
        throw std::runtime_error(written.error());

    require(reader->nativeReader().liveDocuments() == 0, "Release the two copied input documents");
    return std::move(written.value());
}

void verify(const Path& sourcePath, const Path& outputPath, const Path& driverPath, std::string_view expectedFailure,
            pipelang::model::CancellationToken token = {}) {
    auto input = std::make_shared<Files>(sourcePath.string(), profile);
    auto files = std::make_shared<Files>(outputPath.string(), profile);
    auto runner = std::make_shared<Capture>(driverPath, profile);
    auto reader = std::make_shared<Reader>();
    Crypto crypto;
    auto stage = std::make_shared<DigestStage>(crypto, 65536);
    auto output = std::make_shared<Output>(reader->nativeReader(), render,
        [&](std::span<const std::uint8_t> data) { stage->Append(data); });

    const auto retainedPlan = load(outputPath / "plan.json");
    const auto retainedRegistry = load(outputPath / "registry.json");
    const auto retainedIdentity = load(outputPath / "runner.json");
    run::Tests::RunInputVerifierSuite suite(input, files, runner, output, stage);
    const auto accepted = suite.Run(token);
    if (token.is_cancelled()) {
        require(accepted.is_cancelled(), "Admission preserves cancellation");
    } else if (expectedFailure.empty()) {
        if (!accepted.is_success())
            throw std::runtime_error(accepted.error());
    } else {
        require(accepted.is_failure(), "Changed runner metadata must refuse resume");
        if (accepted.error() != expectedFailure)
            throw std::runtime_error(accepted.error());
    }

    require(load(outputPath / "plan.json") == retainedPlan &&
            load(outputPath / "registry.json") == retainedRegistry &&
            load(outputPath / "runner.json") == retainedIdentity, "Run admission revalidation is read-only");
}

} // namespace

int main(int argc, char** argv) {
    try {
        require(argc == 4, "Select the Unicode fixture root, a fresh run root and the resolved legacy interpreter");
        const Path sourcePath(argv[1]);
        const Path outputPath(argv[2]);
        const Path driverPath(argv[3]);
        require(sourcePath.is_absolute() && driverPath.is_absolute() && outputPath.is_absolute() &&
                ::mkdir(outputPath.c_str(), 0700) == 0, "Select explicit isolated run inputs");

        const auto identity = create(sourcePath, outputPath, driverPath);
        require(identity.executable == std::filesystem::read_symlink("/proc/self/exe").string() &&
                identity.driverExecutable == driverPath.string() &&
                identity.executableSha256.size() == 64 && identity.driverExecutableSha256.size() == 64,
                "The identity binds the executing binary and selected interpreter");
        require(load(outputPath / "plan.json") == load(sourcePath / "plan.json") &&
                load(outputPath / "registry.json") == load(sourcePath / "registry.json"),
                "Retain the original wire encodings");
        for (const auto* name : {"runner.json", "plan.json", "registry.json"})
            requirePrivate(outputPath / name);

        verify(sourcePath, outputPath, driverPath, "");
        pipelang::model::CancellationSource cancelled;
        cancelled.cancel();
        verify(sourcePath, outputPath, driverPath, "", cancelled.token());

        const auto runnerPath = outputPath / "runner.json";
        auto changed = load(runnerPath);
        const auto position = changed.find(identity.executableSha256);
        require(position != std::string::npos, "The runner witness contains the executable digest");
        changed[position] = changed[position] == '0' ? '1' : '0';
        std::ofstream destination(runnerPath, std::ios::binary | std::ios::trunc);
        destination << changed;
        destination.close();
        require(destination.good(), "Prepare same-size runner metadata change");

        verify(sourcePath, outputPath, driverPath, "qualification.resume.input_mismatch.runner.json");
        require(pipelang::json::Reader::aggregateRetainedBytes() == 0, "Release all JSON storage");
        std::cout << "Four actual run-input cases passed\n"
                  << "runner=" << identity.executableSha256 << '\n'
                  << "driver=" << identity.driverExecutableSha256 << '\n';
        return 0;
    } catch (const std::exception& error) {
        std::cerr << error.what() << '\n';
        return 1;
    }
}
