#include "Model.hpp"
#include "../harness/native/Json.hpp"

#include <pipelang/json/ReaderAdapter.hpp>

#include <cstddef>
#include <cstdint>
#include <iostream>
#include <memory>
#include <stdexcept>
#include <string>

namespace api = qualification_driver_claims_proof::source::PipeLang::Json;
namespace tests = qualification_driver_claims_proof::source::PipeLang::Testing;
namespace qualification = qualification_driver_claims_proof::source::Dockpipe::Product::DriverClaims::Tests;
using Reader = pipelang::json::ReaderAdapter<api::JsonReader, api::JsonReadLimits, api::JsonNode, api::JsonKind>;

namespace {

template<class OptionalFailure>
void writeFailure(const OptionalFailure& failure) {
    if (!failure.has_value()) {
        std::cout << "null";
        return;
    }
    std::cout << "{\"code\":" << product_harness::json(failure.value().code)
              << ",\"message\":" << product_harness::json(failure.value().message) << '}';
}

void writeResult(const tests::TestResult& result) {
    std::cout << "{\"id\":" << product_harness::json(result.id)
              << ",\"name\":" << product_harness::json(result.name)
              << ",\"outcome\":" << static_cast<std::int64_t>(result.outcome)
              << ",\"primaryOutcome\":" << static_cast<std::int64_t>(result.primaryOutcome)
              << ",\"primaryPhase\":" << static_cast<std::int64_t>(result.primaryPhase)
              << ",\"primaryFailure\":";
    writeFailure(result.primaryFailure);
    std::cout << ",\"cleanupOutcome\":" << static_cast<std::int64_t>(result.cleanupOutcome)
              << ",\"cleanupFailure\":";
    writeFailure(result.cleanupFailure);
    std::cout << ",\"bodyRan\":" << product_harness::json(result.bodyRan)
              << ",\"cleanupRan\":" << product_harness::json(result.cleanupRan) << '}';
}

} // namespace

int main() {
    try {
        auto reader = std::make_shared<Reader>();
        // This fixed predicate corpus uses the existing explicitly admitted
        // registry evaluation profile. SDK defaults and cleanup stay 65536;
        // no Source batching/reset or physical containment limit is changed.
        constexpr std::size_t predicateEvaluationSteps = 33554432;
        pipelang::model::TaskOptions options;
        options.evaluationStepsPerSegment = predicateEvaluationSteps;
        qualification::DriverClaimsSuite suite(reader, options);
        pipelang::model::CancellationSource cancellation;
        cancellation.cancel();
        const auto evaluated = suite.Run({}, cancellation.token());
        if (!evaluated.is_success())
            throw std::runtime_error(evaluated.error());

        const auto& report = evaluated.value();
        std::cout << "{\"schema\":\"dockpipe.qualification-driver-claims-source-tests/v1\",\"allPassed\":"
                  << product_harness::json(report.allPassed)
                  << ",\"evaluationStepsPerSegment\":" << options.evaluationStepsPerSegment
                  << ",\"defaultEvaluationSteps\":" << pipelang::model::defaultEvaluationSteps
                  << ",\"cleanupEvaluationSteps\":" << pipelang::model::EvaluationBudget::maximumSteps
                  << ",\"stringEncoding\":\"utf8-bytes-as-u00xx\",\"remainingDocuments\":"
                  << reader->nativeReader().liveDocuments() << ",\"results\":[";
        bool first = true;
        for (const auto& result : report.results.values()) {
            if (!first)
                std::cout << ',';
            first = false;
            writeResult(result);
        }
        std::cout << "]}\n";
        return report.allPassed && reader->nativeReader().liveDocuments() == 0 ? 0 : 1;
    } catch (const std::exception& error) {
        std::cerr << "Driver claims Source consumer infrastructure: " << error.what() << '\n';
        return 2;
    }
}
