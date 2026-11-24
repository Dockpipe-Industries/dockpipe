#include "Model.hpp"
#include "../harness/native/Json.hpp"

#include <pipelang/io/FileAdapter.hpp>
#include <pipelang/json/ReaderAdapter.hpp>
#ifdef DOCKPIPE_LANE_RECEIPT_CONTRACTS
#include <pipelang/json/StringIndexAdapter.hpp>
#endif
#include <filesystem>
#include <fstream>
#include <iostream>
#include <memory>
#include <stdexcept>
#include <string>
#include <string_view>

namespace source = qualification_evidence_proof::source;
namespace json_api = source::PipeLang::Json;
namespace io_api = source::PipeLang::IO;
namespace crypto_api = source::PipeLang::Crypto;

using Reader = pipelang::json::ReaderAdapter<json_api::JsonReader, json_api::JsonReadLimits,
    json_api::JsonNode, json_api::JsonKind>;
using Files = pipelang::io::FileAdapter<io_api::FileStreams, io_api::CapturedFile, io_api::FileDigest,
    io_api::FileIdentity, io_api::IoFailure, io_api::OutputEffect, crypto_api::Sha256Streams,
    crypto_api::CryptoFailure, crypto_api::Sha256Digest>;

namespace {

#ifdef DOCKPIPE_LANE_RECEIPT_CONTRACTS
using Indexer = pipelang::json::StringIndexAdapter<json_api::JsonStringIndexer, json_api::JsonStringIndexLimits,
    json_api::JsonStringIndex, json_api::JsonNode>;
constexpr auto reportSchema = "dockpipe.qualification-lane-receipt-source-tests/v1";
#else
namespace receipt = source::Dockpipe::Product::Receipt::Tests;
constexpr auto reportSchema = "dockpipe.qualification-evidence-source-tests/v1";
#endif

void put(const std::filesystem::path& file, std::string_view bytes) {
    std::ofstream stream(file, std::ios::binary | std::ios::out);
    stream.exceptions(std::ios::failbit | std::ios::badbit);
    stream.write(bytes.data(), static_cast<std::streamsize>(bytes.size()));
    stream.close();
}

std::filesystem::path prepare(std::string_view path) {
    const std::filesystem::path root(path);
    if (!root.is_absolute() || ::mkdir(root.c_str(), 0700) != 0 ||
            ::mkdir((root / "proof").c_str(), 0700) != 0)
        throw std::runtime_error("Select a fresh absolute private evidence fixture root");

    put(root / "proof/a.txt", "abc");
    put(root / "proof/empty", "");
    put(root / "proof/é.txt", "abc");
    put(root / "proof\\a.txt", "abc");
    std::filesystem::create_symlink("a.txt", root / "proof/leaf-link");
    std::filesystem::create_directory_symlink("proof", root / "ancestor-link");
#ifdef DOCKPIPE_LANE_RECEIPT_CONTRACTS
    constexpr std::string_view receiptText =
        " \n"
        R"({"schema":"dockpipe.product-integration.lane/v1","run_id":"run","plan_sha256":"plan","registry_sha256":"registry","target":{"id":"target","platform":"linux","kind":"local","artifact_sha256":"artifact"},"cases":[{"id":"feature","scenario":"failure","layer":"component","status":"failed"}]})"
        "\n";
    put(root / "lane-result.json", receiptText);
    std::string exactLimit(receiptText);
    exactLimit.resize(4194304, ' ');
    put(root / "lane-result-max.json", exactLimit);
    put(root / "lane-result-invalid.json", R"({"duplicate":0,"duplicate":1})");
#endif
    return root;
}

template<class Failure>
void writeFailure(const Failure& failure) {
    if (!failure.has_value()) {
        std::cout << "null";
        return;
    }

    std::cout << "{\"code\":" << product_harness::json(failure.value().code)
              << ",\"message\":" << product_harness::json(failure.value().message) << '}';
}

} // namespace

int main(int argc, char** argv) {
    try {
        if (argc != 2)
            throw std::runtime_error("Select a fixture root");

        const auto root = prepare(argv[1]);
        auto reader = std::make_shared<Reader>();
        auto files = std::make_shared<Files>(root.string(), pipelang::io::PathProfile{4096, 64, 262144});
        pipelang::model::CancellationSource cancelled;
        cancelled.cancel();

        pipelang::model::TaskOptions options;
        options.evaluationStepsPerSegment = 33554432;
#ifdef DOCKPIPE_LANE_RECEIPT_CONTRACTS
        auto indexer = std::make_shared<Indexer>(reader->nativeReader());
        source::Dockpipe::Product::LaneReceipt::Tests::LaneReceiptSuite suite(reader, indexer, files, options);
#else
        receipt::EvidenceSuite suite(reader, files, options);
#endif
        const auto evaluated = suite.Run({}, cancelled.token());
        if (!evaluated.is_success())
            throw std::runtime_error(evaluated.error());

        const auto& report = evaluated.value();
        const auto documents = reader->nativeReader().liveDocuments();
        std::cout << "{\"schema\":" << product_harness::json(reportSchema) << ",\"allPassed\":"
                  << product_harness::json(report.allPassed) << ",\"remainingDocuments\":" << documents
                  << ",\"evaluationStepsPerSegment\":" << options.evaluationStepsPerSegment << ",\"results\":[";
        bool first = true;
        for (const auto& result : report.results.values()) {
            if (!first)
                std::cout << ',';

            first = false;
            std::cout << "{\"id\":" << product_harness::json(result.id)
                      << ",\"outcome\":" << static_cast<std::int64_t>(result.outcome)
                      << ",\"bodyRan\":" << product_harness::json(result.bodyRan)
                      << ",\"cleanupRan\":" << product_harness::json(result.cleanupRan)
                      << ",\"failure\":";
            writeFailure(result.primaryFailure);
            std::cout << ",\"cleanupFailure\":";
            writeFailure(result.cleanupFailure);
            std::cout << '}';
        }

        std::cout << "]}\n";
        return report.allPassed && documents == 0 ? 0 : 1;
    } catch (const std::exception& error) {
        std::cerr << "Evidence Source consumer infrastructure: " << error.what() << '\n';
        return 2;
    }
}
