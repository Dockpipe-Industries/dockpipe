#include "Model.hpp"

#include <pipelang/json/DocumentWriterAdapter.hpp>
#include <pipelang/json/ReaderAdapter.hpp>

#include <fstream>
#include <iostream>
#include <iterator>
#include <memory>
#include <span>
#include <stdexcept>
#include <string>

namespace source = qualification_report_projection_proof::source;
namespace json = source::PipeLang::Json;
namespace proof = source::Dockpipe::Product::ReportProjection::Tests;

using Reader = pipelang::json::ReaderAdapter<json::JsonReader, json::JsonReadLimits, json::JsonNode, json::JsonKind>;
using Output = pipelang::json::DocumentWriterAdapter<json::JsonDocumentOutput, json::JsonNode>;

std::string read(const char* path) {
    std::ifstream file(path, std::ios::binary);
    if (!file.is_open())
        throw std::runtime_error("Report proof file is missing");

    return {std::istreambuf_iterator<char>(file), {}};
}

int main(int argc, char** argv) {
    try {
        if (argc != 3)
            throw std::runtime_error("Select the exact golden report and proof output file");

        std::ofstream file(argv[2], std::ios::binary);
        file.exceptions(std::ios::failbit | std::ios::badbit);
        auto reader = std::make_shared<Reader>();
        auto output = std::make_shared<Output>(reader->nativeReader(),
            pipelang::json::RenderOptions{33554432, pipelang::json::RenderLayout::Indented, true},
            [&](std::span<const std::uint8_t> bytes) {
                file.write(reinterpret_cast<const char*>(bytes.data()), static_cast<std::streamsize>(bytes.size()));
            });

        pipelang::model::TaskOptions options;
        options.evaluationStepsPerSegment = 33554432;
        proof::ReportProjectionSuite suite(reader, output, options);
        const auto result = suite.Run({});
        if (!result.is_success())
            throw std::runtime_error(result.error());

        file.close();
        const auto expected = read(argv[1]);
        if (result.value() != static_cast<std::int64_t>(expected.size()) || read(argv[2]) != expected ||
            reader->nativeReader().liveDocuments() != 0)
            throw std::runtime_error("Report projection bytes or borrowed input lifetime changed");

        std::cout << "Source coverage and case projection preserve status, extensions and trusted bindings\n";
        return 0;
    } catch (const std::exception& error) {
        std::cerr << "Report projection: " << error.what() << '\n';
        return 1;
    }
}
