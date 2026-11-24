#include "Model.hpp"
#include "../harness/native/Json.hpp"

#include <pipelang/io/ExclusiveOutputAdapter.hpp>
#include <pipelang/io/FileAdapter.hpp>
#include <pipelang/json/ReaderAdapter.hpp>

#include <filesystem>
#include <fstream>
#include <iostream>
#include <memory>
#include <mutex>
#include <stdexcept>
#include <string>
#include <string_view>
#include <sys/stat.h>

namespace source = qualification_input_custody_proof::source;
namespace json_api = source::PipeLang::Json;
namespace io_api = source::PipeLang::IO;
namespace crypto_api = source::PipeLang::Crypto;
namespace proof = source::Dockpipe::Product::QualificationOutput::Tests;

using Reader = pipelang::json::ReaderAdapter<json_api::JsonReader, json_api::JsonReadLimits,
    json_api::JsonNode, json_api::JsonKind>;
using Files = pipelang::io::FileAdapter<io_api::FileStreams, io_api::CapturedFile, io_api::FileDigest,
    io_api::FileIdentity, io_api::IoFailure, io_api::OutputEffect, crypto_api::Sha256Streams,
    crypto_api::CryptoFailure, crypto_api::Sha256Digest>;
using Writer = pipelang::io::ExclusiveOutputAdapter<io_api::ExclusiveFileOutput, io_api::FileDigest,
    io_api::FileIdentity, io_api::IoFailure, io_api::OutputEffect, crypto_api::Sha256Streams,
    crypto_api::CryptoFailure, crypto_api::Sha256Digest>;

namespace {

void put(const std::filesystem::path& path, std::string_view bytes) {
    std::ofstream output(path, std::ios::binary | std::ios::out);
    output.exceptions(std::ios::badbit | std::ios::failbit);
    output.write(bytes.data(), static_cast<std::streamsize>(bytes.size()));
    output.close();
}

class Fixture final : public proof::InputCustodyFixture {
public:
    Fixture(std::filesystem::path source, std::filesystem::path retained)
        : source_(std::move(source)), retained_(std::move(retained)) {}

    pipelang::model::Result<void,std::string> ReplaceInput() override {
        try {
            put(source_ / "small-1.json", "{\"run_id\":\"later\"}\n");
            return pipelang::model::Result<void,std::string>::Success();
        } catch (const std::ios_base::failure& error) {
            return pipelang::model::Result<void,std::string>::Failure(error.what());
        }
    }

    pipelang::model::Result<void,std::string> ChangeRetainedRegistry() override {
        try {
            put(retained_ / "registry.json", "{\"registry\":\"other\"}\n");
            return pipelang::model::Result<void,std::string>::Success();
        } catch (const std::ios_base::failure& error) {
            return pipelang::model::Result<void,std::string>::Failure(error.what());
        }
    }

    pipelang::model::Result<std::int64_t,std::string> PendingOperations() override {
        auto& budget = pipelang::io::detail::budget;
        std::lock_guard lock(budget.mutex);
        return pipelang::model::Result<std::int64_t,std::string>::Success(
            static_cast<std::int64_t>(budget.operations + budget.snapshots));
    }

private:
    std::filesystem::path source_;
    std::filesystem::path retained_;
};

std::filesystem::path prepare(const char* argument) {
    const std::filesystem::path root(argument);
    const auto input = root / "input";
    if (!root.is_absolute() || ::mkdir(root.c_str(), 0700) != 0 ||
            ::mkdir(input.c_str(), 0700) != 0 || ::mkdir((root / "retained").c_str(), 0700) != 0)
        throw std::runtime_error("Select a fresh absolute custody fixture directory");

    constexpr std::string_view document = " \n{\n  \"run_id\" : \"captured\", \"future\" : [1, true]\n}\n";
    for (int index = 0; index < 7; ++index)
        put(input / ("small-" + std::to_string(index) + ".json"), document);

    put(input / "plan.json", "{\"run_id\":\"captured\"}\n");
    put(input / "registry.json", "{\"registry\":\"fixed\"}\n");
    put(input / "runner.json", "{\"runner\":\"first\"}\n");
    put(input / "other-runner.json", "{\"runner\":\"other\"}\n");
    std::string exact(document);
    exact.resize(4194304, ' ');
    put(input / "maximum.json", exact);
    return root;
}

} // namespace

int main(int argc, char** argv) {
    try {
        if (argc != 2)
            throw std::runtime_error("Select a fresh absolute custody fixture directory");

        const auto root = prepare(argv[1]);
        const auto sourcePath = root / "input";
        const auto retainedPath = root / "retained";
        constexpr pipelang::io::PathProfile profile{4096, 32, 16384};
        auto reader = std::make_shared<Reader>();
        auto input = std::make_shared<Files>(sourcePath.string(), profile);
        auto retained = std::make_shared<Files>(retainedPath.string(), profile);
        auto output = std::make_shared<Writer>(retainedPath.string(), profile);
        auto fixture = std::make_shared<Fixture>(sourcePath, retainedPath);

        pipelang::model::CancellationSource cancelled;
        cancelled.cancel();
        pipelang::model::TaskOptions options;
        options.evaluationStepsPerSegment = 33554432;
        proof::InputCustodySuite suite(reader, input, retained, output, fixture, options);
        const auto evaluated = suite.Run({}, cancelled.token());
        if (!evaluated.is_success())
            throw std::runtime_error(evaluated.error());

        const auto& report = evaluated.value();
        const auto liveDocuments = reader->nativeReader().liveDocuments();
        std::cout << "{\"schema\":\"dockpipe.qualification-input-custody-source-tests/v1\",\"allPassed\":"
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
        std::cerr << "Input custody Source consumer infrastructure: " << error.what() << '\n';
        return 2;
    }
}
