// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "Model.hpp"
#include "ReportRowsAdapter.hpp"

#include <pipelang/json/DocumentWriterAdapter.hpp>
#include <pipelang/json/ReaderAdapter.hpp>

#include <fstream>
#include <iostream>
#include <iterator>
#include <memory>
#include <span>
#include <stdexcept>
#include <string>
#include <string_view>

namespace source = qualification_staged_report_rows::source;
namespace json = source::PipeLang::Json;
namespace report = source::Dockpipe::Product::ReportProjection;
using Reader = pipelang::json::ReaderAdapter<json::JsonReader, json::JsonReadLimits, json::JsonNode, json::JsonKind>;
using Output = pipelang::json::DocumentWriterAdapter<json::JsonDocumentOutput, json::JsonNode>;
using Writer = pipelang::json::DocumentWriter;
using Rows = product_harness::ReportRowsAdapter<report::ReportRows, json::JsonNode>;

namespace {

void require(bool condition, std::string_view reason) {
    if (!condition)
        throw std::runtime_error(std::string(reason));
}

template<class Operation>
void refuses(Operation&& action, std::string_view expected) {
    try {
        action();
    } catch (const pipelang::json::ReadFailure& failure) {
        require(failure.what() == expected, "Unexpected staged row refusal");
        return;
    }
    throw std::runtime_error("Expected a staged row refusal");
}

void budget() {
    pipelang::json::Reader reader;
    product_harness::StagedCaseRows rows(2, {3, pipelang::json::RenderLayout::Compact, false, 1});
    rows.Stage(0, reader, [](Writer& output) { output.integer(42); });
    refuses([&] { rows.Stage(0, reader, [](Writer& output) { output.integer(8); }); },
            "qualification.report.duplicate_row");
    refuses([&] { rows.Stage(1, reader, [](Writer& output) { output.integer(99); }); },
            "json.writer.byte_limit");
    rows.Stage(1, reader, [](Writer& output) { output.integer(0); });

    std::string report;
    Writer output(reader, {7}, [&](std::span<const std::uint8_t> bytes) {
        report.append(reinterpret_cast<const char*>(bytes.data()), bytes.size());
    });
    output.beginArray();
    rows.Write(0, output);
    rows.Write(1, output);
    output.endArray();
    require(output.finish() == 7 && report == "[42,0]\n",
            "The failed row leaves earlier rows intact and consumes no budget");
}

void writeReport(const char* path) {
    std::ofstream file(path, std::ios::binary | std::ios::out);
    file.exceptions(std::ios::failbit | std::ios::badbit);
    constexpr std::size_t maximum = 33554432;
    constexpr auto layout = pipelang::json::RenderLayout::Indented;
    auto reader = std::make_shared<Reader>();
    Writer writer(reader->nativeReader(), {maximum, layout, true}, [&](std::span<const std::uint8_t> bytes) {
        require(reader->nativeReader().liveDocuments() == 0, "Close every receipt before final output");
        file.write(reinterpret_cast<const char*>(bytes.data()), static_cast<std::streamsize>(bytes.size()));
    });
    product_harness::StagedCaseRows rows(2, {maximum, layout, true, 1});
    auto output = std::make_shared<Output>(writer);
    auto adapter = std::make_shared<Rows>(reader->nativeReader(), writer, rows,
        [&](Writer& sink, json::JsonNode observed, bool critical, const std::string& evidenceRoot,
            pipelang::model::CancellationToken token) {
            auto scoped = std::make_shared<Output>(sink);
            report::ProjectedCaseWriter projection(reader, scoped);
            return projection.Write(observed, critical, evidenceRoot, token);
        });

    report::StagedTests::StagedRowsSuite suite(reader, output, adapter);
    const auto written = suite.Run({});
    if (!written.is_success())
        throw std::runtime_error(written.error());

    file.close();
    const std::string_view expected = R"([
  {
    "critical": true,
    "evidence_root": "t1/d1/output",
    "id": "feature.a",
    "status": "failed",
    "extension": {
      "detail": "\u20ac"
    }
  },
  {
    "critical": false,
    "evidence_root": "t1/d2/output",
    "id": "feature.b",
    "status": "passed",
    "extension": true
  }
]
)";
    std::ifstream recorded(path, std::ios::binary);
    require(recorded.is_open(), "Read back the actual staged-row output");
    const std::string observed{std::istreambuf_iterator<char>(recorded), {}};
    require(observed == expected && written.value() == static_cast<std::int64_t>(expected.size()),
            "Generated Source orders the staged rows and preserves status, extensions and trusted bindings");
    require(pipelang::json::Reader::aggregateRetainedBytes() == 0, "Release all source reader storage");
}

} // namespace

int main(int argc, char** argv) {
    try {
        require(argc == 2, "Select the staged-row proof output file");
        budget();
        writeReport(argv[1]);
        std::cout << "Staged report rows preserve bounded custody, exact order and owned case data\n";
        return 0;
    } catch (const std::exception& error) {
        std::cerr << error.what() << '\n';
        return 1;
    }
}
