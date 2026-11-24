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

namespace source = qualification_driver_pins_proof::source;
namespace json_api = source::PipeLang::Json;
namespace io_api = source::PipeLang::IO;
namespace crypto_api = source::PipeLang::Crypto;
namespace proof = source::Dockpipe::Product::DriverSources::Tests;

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

class Fixture final : public proof::DriverPinsFixture {
public:
    explicit Fixture(std::filesystem::path product) : product_(std::move(product)) {}

    pipelang::model::Result<void,std::string> ChangeDriver() override {
        try {
            put(product_ / "mutate.py", "new");
            return pipelang::model::Result<void,std::string>::Success();
        } catch (const std::ios_base::failure& error) {
            return pipelang::model::Result<void,std::string>::Failure(error.what());
        }
    }

private:
    std::filesystem::path product_;
};

std::filesystem::path prepare(const char* argument) {
    const std::filesystem::path root(argument);
    const auto product = root / "product";
    const auto repository = root / "repository";
    if (!root.is_absolute() || ::mkdir(root.c_str(), 0700) != 0 ||
            ::mkdir(product.c_str(), 0700) != 0 || ::mkdir(repository.c_str(), 0700) != 0)
        throw std::runtime_error("Select a fresh absolute driver pin fixture directory");

    put(product / "driver.py", "abc");
    put(product / "driver.txt", "abc");
    put(product / "mutate.py", "abc");
    put(product / ".py", "abc");
    std::filesystem::create_symlink("driver.py", product / "mirror.py");
    put(repository / "driver.py", "old");
    put(repository / "dep", "new");
    for (int index = 0; index < 64; ++index)
        put(repository / ("dep-" + std::to_string(index)), "new");

    return root;
}

} // namespace

int main(int argc, char** argv) {
    try {
        if (argc != 2)
            throw std::runtime_error("Select a fresh absolute driver pin fixture directory");

        const auto root = prepare(argv[1]);
        const auto product = root / "product";
        const auto repository = root / "repository";
        constexpr pipelang::io::PathProfile profile{4096, 64, 262144};
        auto reader = std::make_shared<Reader>();
        auto productFiles = std::make_shared<Files>(product.string(), profile);
        auto repositoryFiles = std::make_shared<Files>(repository.string(), profile);
        auto fixture = std::make_shared<Fixture>(product);
        pipelang::model::CancellationSource cancelled;
        cancelled.cancel();
        pipelang::model::TaskOptions options;
        options.evaluationStepsPerSegment = 33554432;
        proof::DriverPinsSuite suite(reader, productFiles, repositoryFiles, fixture, options);
        const auto evaluated = suite.Run({}, cancelled.token());
        if (!evaluated.is_success())
            throw std::runtime_error(evaluated.error());

        const auto& report = evaluated.value();
        const auto liveDocuments = reader->nativeReader().liveDocuments();
        std::cout << "{\"schema\":\"dockpipe.qualification-driver-pins-source-tests/v1\",\"allPassed\":"
                  << product_harness::json(report.allPassed) << ",\"remainingDocuments\":" << liveDocuments
                  << ",\"results\":[";
        bool first = true;
        for (const auto& row : report.results.values()) {
            if (!first)
                std::cout << ',';

            first = false;
            std::cout << "{\"id\":" << product_harness::json(row.id)
                      << ",\"outcome\":" << static_cast<std::int64_t>(row.outcome)
                      << ",\"bodyRan\":" << product_harness::json(row.bodyRan)
                      << ",\"cleanupRan\":" << product_harness::json(row.cleanupRan)
                      << ",\"failure\":";
            if (row.primaryFailure.has_value())
                std::cout << product_harness::json(row.primaryFailure.value().message);
            else
                std::cout << "null";

            std::cout << '}';
        }
        std::cout << "]}\n";
        return report.allPassed && liveDocuments == 0 ? 0 : 1;
    } catch (const std::exception& error) {
        std::cerr << "Driver source pin consumer infrastructure: " << error.what() << '\n';
        return 2;
    }
}
