#include "Model.hpp"
#include "../harness/native/Json.hpp"

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

namespace source = qualification_lane_request_proof::source;
namespace json_api = source::PipeLang::Json;
namespace io_api = source::PipeLang::IO;
namespace crypto_api = source::PipeLang::Crypto;
namespace proof = source::Dockpipe::Product::LaneRequest::Tests;

using Reader = pipelang::json::ReaderAdapter<json_api::JsonReader, json_api::JsonReadLimits,
    json_api::JsonNode, json_api::JsonKind>;
using Indexer = pipelang::json::StringIndexAdapter<json_api::JsonStringIndexer, json_api::JsonStringIndexLimits,
    json_api::JsonStringIndex, json_api::JsonNode>;
using Files = pipelang::io::FileAdapter<io_api::FileStreams, io_api::CapturedFile, io_api::FileDigest,
    io_api::FileIdentity, io_api::IoFailure, io_api::OutputEffect, crypto_api::Sha256Streams,
    crypto_api::CryptoFailure, crypto_api::Sha256Digest>;
using Output = pipelang::json::DocumentWriterAdapter<json_api::JsonDocumentOutput, json_api::JsonNode>;

namespace {

std::string read(const std::filesystem::path& path) {
    std::ifstream input(path, std::ios::binary);
    if (!input.is_open())
        throw std::runtime_error("Expected a regular fixture file");

    return {std::istreambuf_iterator<char>(input), {}};
}

void fixture(const std::filesystem::path& input, const std::filesystem::path& scratch) {
    if (!input.is_absolute() || !scratch.is_absolute() || ::mkdir(scratch.c_str(), 0700) != 0)
        throw std::runtime_error("Select an absolute fixture and fresh private output");

    for (const auto* name : {"plan.json", "registry.json"})
        std::filesystem::copy_file(input / name, scratch / name);

    std::ofstream driver(scratch / "driver.py", std::ios::binary);
    driver.exceptions(std::ios::failbit | std::ios::badbit);
    driver.write("abc", 3);
    driver.close();
}

void run(const std::filesystem::path& fixtureRoot, const std::filesystem::path& destination,
         int position, bool cancelled, std::string_view expected) {
    constexpr pipelang::io::PathProfile profile{4096, 64, 262144};
    auto reader = std::make_shared<Reader>();
    auto indexer = std::make_shared<Indexer>(reader->nativeReader());
    auto files = std::make_shared<Files>(fixtureRoot.string(), profile);
    std::ofstream saved;
    auto output = std::make_shared<Output>(reader->nativeReader(),
        pipelang::json::RenderOptions{4194304, pipelang::json::RenderLayout::Indented, true},
        [&](std::span<const std::uint8_t> bytes) {
            if (!saved.is_open()) {
                saved.exceptions(std::ios::failbit | std::ios::badbit);
                saved.open(destination, std::ios::binary);
            }

            saved.write(reinterpret_cast<const char*>(bytes.data()), static_cast<std::streamsize>(bytes.size()));
        });

    pipelang::model::TaskOptions options;
    options.evaluationStepsPerSegment = 33554432;
    proof::LaneRequestSuite suite(reader, indexer, files, output, options);
    pipelang::model::CancellationSource probe;
    probe.cancel();
    const auto result = suite.Run(position, cancelled, {}, probe.token());
    if (saved.is_open())
        saved.close();

    if (reader->nativeReader().liveDocuments() != 0 ||
        pipelang::json::Reader::aggregateRetainedBytes() != 0)
        throw std::runtime_error("Lane request leaked an admitted input document");

    if (cancelled) {
        if (!result.is_cancelled() || std::filesystem::exists(destination))
            throw std::runtime_error("Cancelled lane projection created output or returned an incorrect status");
    } else if (position == 2) {
        if (!result.is_success() || result.value().cases != 0 || result.value().bytes != 0 ||
            std::filesystem::exists(destination))
            throw std::runtime_error("Unavailable-only lane created a request");
    } else {
        if (!result.is_success())
            throw std::runtime_error(result.error());

        if (result.value().cases != 2 || result.value().bytes != static_cast<std::int64_t>(expected.size()) ||
            read(destination) != expected)
            throw std::runtime_error("Projected lane request differs from the retained exact bytes");
    }
}
} // namespace

int main(int argc, char** argv) {
    try {
        if (argc != 3)
            throw std::runtime_error("Select exact input fixtures and a fresh run directory");

        const std::filesystem::path inputs(argv[1]);
        const std::filesystem::path scratch(argv[2]);
        fixture(inputs, scratch);
        const auto expected = read(inputs / "request.golden.json");
        run(scratch, scratch / "request.json", 1, false, expected);
        run(scratch, scratch / "unavailable.json", 2, false, {});
        run(scratch, scratch / "cancelled.json", 1, true, {});
        std::cout << "{\"schema\":\"dockpipe.qualification-lane-request-tests/v1\",\"passed\":3}\n";
        return 0;
    } catch (const std::exception& error) {
        std::cerr << "Lane request projection: " << error.what() << '\n';
        return 1;
    }
}
