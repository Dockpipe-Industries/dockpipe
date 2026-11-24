#include "Model.hpp"
#include "../harness/native/Json.hpp"

#include <pipelang/io/FileAdapter.hpp>
#include <pipelang/json/ReaderAdapter.hpp>

#include <filesystem>
#include <fstream>
#include <iostream>
#include <memory>
#include <stdexcept>
#include <string>
#include <string_view>
#include <sys/stat.h>

namespace source = qualification_retained_lane_proof::source;
namespace json_api = source::PipeLang::Json;
namespace io_api = source::PipeLang::IO;
namespace crypto_api = source::PipeLang::Crypto;
namespace proof = source::Dockpipe::Product::RetainedLane::Tests;

using Reader = pipelang::json::ReaderAdapter<json_api::JsonReader, json_api::JsonReadLimits,
    json_api::JsonNode, json_api::JsonKind>;
using Files = pipelang::io::FileAdapter<io_api::FileStreams, io_api::CapturedFile, io_api::FileDigest,
    io_api::FileIdentity, io_api::IoFailure, io_api::OutputEffect, crypto_api::Sha256Streams,
    crypto_api::CryptoFailure, crypto_api::Sha256Digest>;

namespace {

void put(const std::filesystem::path& path, std::string_view bytes) {
    std::ofstream output(path, std::ios::binary | std::ios::out);
    output.exceptions(std::ios::badbit | std::ios::failbit);
    output.write(bytes.data(), static_cast<std::streamsize>(bytes.size()));
    output.close();
}

void mkdirPrivate(const std::filesystem::path& path) {
    if (!path.is_absolute() || ::mkdir(path.c_str(), 0700) != 0)
        throw std::runtime_error("Select a fresh absolute retained lane fixture root");
}

void prepare(const std::filesystem::path& invocation, int scenario) {
    mkdirPrivate(invocation);
    put(invocation / "expected.json", R"({"configuration":{"n":1}})");
    if (scenario == 8) {
        std::filesystem::create_symlink("expected.json", invocation / "request.json");
    } else if (scenario == 4) {
        put(invocation / "request.json", R"({"configuration":{"n":2}})");
    } else {
        put(invocation / "request.json", R"({"configuration":{"n":1}})");
    }

    if (scenario == 5)
        return;

    std::string_view receipt = R"({"exit_code":0,"failure":""})";
    if (scenario == 1) {
        receipt = R"({"exit_code":7,"failure":""})";
    } else if (scenario == 2) {
        receipt = R"({"exit_code":null,"failure":""})";
    } else if (scenario == 3) {
        receipt = R"({"exit_code":-15,"failure":"Driver exceeded its declared deadline"})";
    } else if (scenario == 7) {
        receipt = R"({"exit_code":)";
    }

    put(invocation / "process.json", receipt);
}

} // namespace

int main(int argc, char** argv) {
    try {
        if (argc != 2)
            throw std::runtime_error("Select one fresh retained-lane fixture directory");

        const std::filesystem::path root(argv[1]);
        mkdirPrivate(root);
        auto reader = std::make_shared<Reader>();
        pipelang::model::CancellationSource cancellation;
        cancellation.cancel();
        constexpr pipelang::io::PathProfile profile{4096, 64, 262144};
        bool allPassed = true;
        for (int scenario = 0; scenario < 9; ++scenario) {
            const auto invocation = root / ("lane-" + std::to_string(scenario));
            prepare(invocation, scenario);
            auto files = std::make_shared<Files>(invocation.string(), profile);
            proof::RetainedLaneSuite suite(reader, files);
            const auto evaluated = suite.Run(scenario, {}, cancellation.token());
            if (!evaluated.is_success())
                throw std::runtime_error(evaluated.error());

            const auto& result = evaluated.value();
            const bool passed = static_cast<std::int64_t>(result.outcome) == 0 &&
                result.bodyRan && result.cleanupRan && reader->nativeReader().liveDocuments() == 0;
            allPassed = allPassed && passed;
            std::cout << "{\"id\":" << product_harness::json(result.id)
                      << ",\"passed\":" << product_harness::json(passed) << ",\"failure\":";
            if (result.primaryFailure.has_value())
                std::cout << product_harness::json(result.primaryFailure.value().message);
            else
                std::cout << "null";

            std::cout << "}\n";
        }

        return allPassed && pipelang::json::Reader::aggregateRetainedBytes() == 0 ? 0 : 1;
    } catch (const std::exception& error) {
        std::cerr << "Retained lane consumer infrastructure: " << error.what() << '\n';
        return 2;
    }
}
