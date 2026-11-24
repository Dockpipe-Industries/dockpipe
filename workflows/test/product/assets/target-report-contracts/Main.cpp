// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "Model.hpp"
#include "ReportRowsAdapter.hpp"

#include <pipelang/io/FileAdapter.hpp>
#include <pipelang/json/DocumentWriterAdapter.hpp>
#include <pipelang/json/ReaderAdapter.hpp>
#include <pipelang/json/StringIndexAdapter.hpp>

#include <filesystem>
#include <fstream>
#include <iostream>
#include <memory>
#include <span>
#include <stdexcept>
#include <string>
#include <string_view>

namespace source = qualification_target_report::source;
namespace io = source::PipeLang::IO;
namespace json = source::PipeLang::Json;
namespace crypto = source::PipeLang::Crypto;
namespace report = source::Dockpipe::Product::ReportProjection;
namespace target = source::Dockpipe::Product::TargetReports;

using Reader = pipelang::json::ReaderAdapter<json::JsonReader, json::JsonReadLimits, json::JsonNode, json::JsonKind>;
using Indexer = pipelang::json::StringIndexAdapter<json::JsonStringIndexer, json::JsonStringIndexLimits,
    json::JsonStringIndex, json::JsonNode>;
using Files = pipelang::io::FileAdapter<io::FileStreams, io::CapturedFile, io::FileDigest,
    io::FileIdentity, io::IoFailure, io::OutputEffect, crypto::Sha256Streams, crypto::CryptoFailure, crypto::Sha256Digest>;
using Output = pipelang::json::DocumentWriterAdapter<json::JsonDocumentOutput, json::JsonNode>;
using Writer = pipelang::json::DocumentWriter;
using Rows = product_harness::ReportRowsAdapter<report::ReportRows, json::JsonNode>;

int main(int argc, char** argv) {
    try {
        if (argc != 3 || !std::filesystem::path(argv[1]).is_absolute())
            throw std::runtime_error("Select the target report input directory and output file");

        std::ofstream file(argv[2], std::ios::binary | std::ios::out);
        file.exceptions(std::ios::badbit | std::ios::failbit);
        constexpr auto layout = pipelang::json::RenderLayout::Indented;
        constexpr std::size_t maximum = 33554432;
        constexpr pipelang::io::PathProfile fileProfile{4096, 64, 262144};
        auto reader = std::make_shared<Reader>();
        auto indexer = std::make_shared<Indexer>(reader->nativeReader());
        auto files = std::make_shared<Files>(argv[1], fileProfile);
        Writer writer(reader->nativeReader(), {maximum, layout, true}, [&](std::span<const std::uint8_t> bytes) {
            if (reader->nativeReader().liveDocuments() != 2)
                throw std::runtime_error("Only the run plan and registry may remain open during report output");

            file.write(reinterpret_cast<const char*>(bytes.data()), static_cast<std::streamsize>(bytes.size()));
        });
        product_harness::StagedCaseRows rows(4, {maximum, layout, true, 2});
        auto output = std::make_shared<Output>(writer);
        auto adapter = std::make_shared<Rows>(reader->nativeReader(), writer, rows,
            [&](Writer& sink, json::JsonNode node, bool critical, const std::string& evidenceRoot,
                pipelang::model::CancellationToken token) {
                auto scoped = std::make_shared<Output>(sink);
                report::ProjectedCaseWriter projection(reader, scoped);
                return projection.Write(node, critical, evidenceRoot, token);
            });

        pipelang::model::TaskOptions options;
        options.evaluationStepsPerSegment = 33554432;
        target::Tests::TargetReportSuite suite(reader, indexer, files, output, adapter, options);
        const auto result = suite.Run({});
        if (!result.is_success())
            throw std::runtime_error(result.error());

        file.close();
        const auto& summary = result.value();
        if (summary.required != 4 || summary.passed != 1 || summary.status != "failed")
            throw std::runtime_error("Incorrect qualification coverage for fixture lanes");
        if (reader->nativeReader().liveDocuments() != 0 || pipelang::json::Reader::aggregateRetainedBytes() != 0)
            throw std::runtime_error("Target report left borrowed run input storage");

        std::cout << "Target report reconciled actual receipts, absent rows and the selected denominator\n";
        return 0;
    } catch (const std::exception& error) {
        std::cerr << error.what() << '\n';
        return 1;
    }
}
