// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "Model.hpp"
#include "QualificationEventPublisher.hpp"

#include <pipelang/io/ExclusiveOutputAdapter.hpp>
#include <pipelang/io/ExistingFileAppendAdapter.hpp>
#include <pipelang/json/DocumentWriterAdapter.hpp>

#include <filesystem>
#include <fstream>
#include <iostream>
#include <iterator>
#include <memory>
#include <stdexcept>
#include <string>
#include <string_view>
#include <sys/stat.h>

namespace source = qualification_event_output::source;
namespace io = source::PipeLang::IO;
namespace json = source::PipeLang::Json;
namespace crypto = source::PipeLang::Crypto;
namespace events = source::Dockpipe::Product::QualificationEventOutput;

using Writer = pipelang::io::ExclusiveOutputAdapter<io::ExclusiveFileOutput, io::FileDigest, io::FileIdentity,
    io::IoFailure, io::OutputEffect, crypto::Sha256Streams, crypto::CryptoFailure, crypto::Sha256Digest>;
using Append = pipelang::io::ExistingFileAppendAdapter<io::ExistingFileAppend, io::AppendFileState,
    io::FileIdentity, io::IoFailure, io::OutputEffect>;
using Output = pipelang::json::DocumentWriterAdapter<json::JsonDocumentOutput, json::JsonNode>;
using Publisher = product_harness::QualificationEventPublisher<events::QualificationEventPort,
    events::QualificationEvent, events::QualificationEventWrite, events::QualificationEventJournal,
    io::IoFailure, io::OutputEffect>;

namespace {

void require(bool condition, std::string_view reason) {
    if (!condition)
        throw std::runtime_error(std::string(reason));
}

std::string content(const std::filesystem::path& path) {
    std::ifstream file(path, std::ios::binary);
    require(file.is_open(), "Read back the actual event log");
    return {std::istreambuf_iterator<char>(file), {}};
}

void run(const std::filesystem::path& directory, bool resume, std::ostream& warnings,
         bool warningsWritable = true) {
    constexpr pipelang::io::PathProfile profile{4096, 64, 262144};
    auto exclusive = std::make_shared<Writer>(directory.string(), profile);
    auto append = std::make_shared<Append>(directory.string(), profile);
    events::QualificationEventJournal journal(exclusive, append);
    const auto opened = journal.Open(resume, 8388608, {});
    if (!opened.is_success())
        throw std::runtime_error(opened.error());
    if (!opened.value().is_success())
        throw std::runtime_error(opened.value().error().code);

    pipelang::json::Reader reader;
    auto driver = std::make_shared<Publisher>(journal,
        [&](events::QualificationEvent value, const std::string& stamp, pipelang::json::JsonByteSink sink,
            pipelang::model::CancellationToken token) {
            auto output = std::make_shared<Output>(reader,
                pipelang::json::RenderOptions{Publisher::maximumLineBytes, pipelang::json::RenderLayout::Spaced, true},
                std::move(sink));
            events::QualificationEventProjection record(output);
            return record.Write(std::move(value), stamp, token);
        }, [&](std::string_view warning) {
            if (!warningsWritable)
                return false;

            warnings << warning << '\n';
            return static_cast<bool>(warnings);
        });

    events::Tests::EventOutputSuite suite(driver);
    const auto path = directory / "events.jsonl";
    const auto before = content(path);
    pipelang::model::CancellationSource cancelled;
    cancelled.cancel();
    const auto interrupted = suite.Run(resume, cancelled.token());
    require(interrupted.is_cancelled() ||
            (interrupted.is_success() && interrupted.value().is_cancelled() &&
             interrupted.value().error().effect == io::OutputEffect::NoDurableOutput),
            "Pre-cancelled publication leaves the journal untouched");
    require(content(path) == before, "No data appended after pre-cancellation");

    const auto evaluated = suite.Run(resume, {});
    if (!evaluated.is_success())
        throw std::runtime_error(evaluated.error());

    const auto& emitted = evaluated.value();
    if (warningsWritable) {
        if (!emitted.is_success())
            throw std::runtime_error(emitted.error().code);
    } else {
        require(emitted.is_failure() && emitted.error().code == "qualification.events.warning_write" &&
                emitted.error().effect == io::OutputEffect::Committed,
                "A warning write failure preserves the observed journal append");
    }

    require(content(path).starts_with(before), "Resume keeps prior event bytes");
    struct stat status {};
    require(::lstat(path.c_str(), &status) == 0 && S_ISREG(status.st_mode) && (status.st_mode & 0777) == 0600,
            "The actual journal is a private regular file");
}

} // namespace

int main(int argc, char** argv) {
    try {
        require(argc == 2, "Select an absolute private event test root");
        const std::filesystem::path directory(argv[1]);
        const auto fallback = directory / "warning-failure";
        require(directory.is_absolute() && ::mkdir(directory.c_str(), 0700) == 0 &&
                ::mkdir(fallback.c_str(), 0700) == 0, "Create the fresh event test roots");

        std::ofstream warnings(directory / "warnings.txt", std::ios::binary);
        warnings.exceptions(std::ios::failbit | std::ios::badbit);
        run(directory, false, warnings);
        run(directory, true, warnings);
        run(fallback, false, warnings, false);
        warnings.close();

        std::cout << "Generated Source events appended to fresh and resumed private journals\n";
        return 0;
    } catch (const std::exception& error) {
        std::cerr << error.what() << '\n';
        return 1;
    }
}
