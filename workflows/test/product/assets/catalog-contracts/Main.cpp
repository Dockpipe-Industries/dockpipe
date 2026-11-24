// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "Model.hpp"

#include <pipelang/io/FileAdapter.hpp>
#include <pipelang/json/DocumentWriterAdapter.hpp>
#include <pipelang/json/ReaderAdapter.hpp>
#include <pipelang/json/StringIndexAdapter.hpp>

#include <filesystem>
#include <iostream>
#include <memory>
#include <span>
#include <stdexcept>
#include <string>

namespace source = qualification_catalog::source;
namespace io = source::PipeLang::IO;
namespace json = source::PipeLang::Json;
namespace crypto = source::PipeLang::Crypto;
namespace catalog = source::Dockpipe::Product::QualificationCatalog;

using Reader = pipelang::json::ReaderAdapter<json::JsonReader, json::JsonReadLimits, json::JsonNode, json::JsonKind>;
using Indexer = pipelang::json::StringIndexAdapter<json::JsonStringIndexer, json::JsonStringIndexLimits,
    json::JsonStringIndex, json::JsonNode>;
using Files = pipelang::io::FileAdapter<io::FileStreams, io::CapturedFile, io::FileDigest,
    io::FileIdentity, io::IoFailure, io::OutputEffect, crypto::Sha256Streams, crypto::CryptoFailure, crypto::Sha256Digest>;
using Output = pipelang::json::DocumentWriterAdapter<json::JsonDocumentOutput, json::JsonNode>;

int main(int argc, char** argv) {
    try {
        if (argc != 2)
            throw std::runtime_error("Supply the absolute product registry path");

        const std::filesystem::path registry(argv[1]);
        if (!registry.is_absolute() || registry.filename().empty())
            throw std::runtime_error("Select an absolute product registry file");

        constexpr pipelang::io::PathProfile fileProfile{4096, 64, 262144};
        constexpr std::int64_t maximumReportBytes = 33554432;
        auto reader = std::make_shared<Reader>();
        auto indexer = std::make_shared<Indexer>(reader->nativeReader());
        auto files = std::make_shared<Files>(registry.parent_path().string(), fileProfile);
        std::string report;
        auto output = std::make_shared<Output>(reader->nativeReader(),
            pipelang::json::RenderOptions{maximumReportBytes, pipelang::json::RenderLayout::Indented, true},
            [&](std::span<const std::uint8_t> fragment) {
                report.append(reinterpret_cast<const char*>(fragment.data()), fragment.size());
            });

        pipelang::model::TaskOptions options;
        options.evaluationStepsPerSegment = 33554432;
        catalog::QualificationCatalog listing(reader, indexer, files, output, 33554432, options);
        const auto written = listing.List(registry.filename().string(), {});
        if (!written.is_success())
            throw std::runtime_error(written.error());

        if (reader->nativeReader().liveDocuments() != 0 || pipelang::json::Reader::aggregateRetainedBytes() != 0)
            throw std::runtime_error("Catalog retained a closed registry document");
        if (written.value() != static_cast<std::int64_t>(report.size()))
            throw std::runtime_error("Catalog extent differs from its committed output");

        std::cout << report;
        return std::cout ? 0 : 1;
    } catch (const std::exception& error) {
        std::cerr << error.what() << '\n';
        return 1;
    }
}
