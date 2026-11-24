#include "Model.hpp"
#include "../harness/native/Json.hpp"

#include <pipelang/io/ExclusiveOutputAdapter.hpp>
#include <pipelang/io/ExistingFileAppendAdapter.hpp>
#include <pipelang/io/FileAdapter.hpp>

#include <filesystem>
#include <iostream>
#include <memory>
#include <mutex>
#include <stdexcept>
#include <string_view>

namespace source = qualification_event_log_proof::source;
namespace api = source::PipeLang::IO;
namespace crypto = source::PipeLang::Crypto;
namespace proof = source::Dockpipe::Product::QualificationEvents::Tests;

using Writer = pipelang::io::ExclusiveOutputAdapter<api::ExclusiveFileOutput, api::FileDigest,
    api::FileIdentity, api::IoFailure, api::OutputEffect, crypto::Sha256Streams, crypto::CryptoFailure,
    crypto::Sha256Digest>;
using Append = pipelang::io::ExistingFileAppendAdapter<api::ExistingFileAppend, api::AppendFileState,
    api::FileIdentity, api::IoFailure, api::OutputEffect>;
using Reader = pipelang::io::FileAdapter<api::FileStreams, api::CapturedFile, api::FileDigest,
    api::FileIdentity, api::IoFailure, api::OutputEffect, crypto::Sha256Streams, crypto::CryptoFailure,
    crypto::Sha256Digest>;

namespace {

class Fixture final : public proof::EventLogFixture {
public:
    explicit Fixture(std::filesystem::path root) : root_(std::move(root)) {}

    pipelang::model::Result<void,std::string> ChangeObservedFile(std::string relative) override {
        if (relative != "events-4.jsonl")
            return pipelang::model::Result<void,std::string>::Failure("fixture can modify only its selected event file");

        constexpr std::string_view replacement = "{\"event\": \"external\"}\n";
        pipelang::io::detail::Descriptor output(::open((root_ / relative).c_str(),
            O_WRONLY | O_TRUNC | O_NOFOLLOW | O_CLOEXEC));
        if (output.get() < 0 || ::write(output.get(), replacement.data(), replacement.size()) !=
                static_cast<ssize_t>(replacement.size()) || !output.close())
            return pipelang::model::Result<void,std::string>::Failure("fixture event mutation");

        return pipelang::model::Result<void,std::string>::Success();
    }

    pipelang::model::Result<std::int64_t,std::string> RetainedOperations() override {
        auto& budget = pipelang::io::detail::budget;
        std::lock_guard lock(budget.mutex);
        return pipelang::model::Result<std::int64_t,std::string>::Success(
            static_cast<std::int64_t>(budget.operations + budget.snapshots));
    }

private:
    std::filesystem::path root_;
};

} // namespace

int main(int argc, char** argv) {
    try {
        if (argc != 2)
            throw std::runtime_error("Select a fresh absolute event fixture directory");

        const std::filesystem::path root(argv[1]);
        if (!root.is_absolute() || ::mkdir(root.c_str(), 0700) != 0)
            throw std::runtime_error("Could not open the private event fixture root");

        constexpr pipelang::io::PathProfile profile{4096, 32, 16384};
        auto input = std::make_shared<Reader>(root.string(), profile);
        auto output = std::make_shared<Writer>(root.string(), profile);
        auto append = std::make_shared<Append>(root.string(), profile);
        auto fixture = std::make_shared<Fixture>(root);
        pipelang::model::CancellationSource cancelled;
        cancelled.cancel();

        pipelang::model::TaskOptions options;
        options.evaluationStepsPerSegment = 33554432;
        proof::EventLogSuite suite(input, output, append, fixture, options);
        const auto evaluated = suite.Run({}, cancelled.token());
        if (!evaluated.is_success())
            throw std::runtime_error(evaluated.error());

        const auto& report = evaluated.value();
        std::cout << "{\"schema\":\"dockpipe.qualification-event-log-source-tests/v1\",\"allPassed\":"
                  << product_harness::json(report.allPassed) << ",\"results\":[";
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
        return report.allPassed ? 0 : 1;
    } catch (const std::exception& error) {
        std::cerr << "Event log Source consumer infrastructure: " << error.what() << '\n';
        return 2;
    }
}
