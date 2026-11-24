#include "Model.hpp"
#include "../harness/native/Json.hpp"

#include <iostream>
#include <stdexcept>

namespace coverage = qualification_coverage_proof::source::Dockpipe::Product::Coverage::Tests;

int main() {
    try {
        pipelang::model::CancellationSource cancelled;
        cancelled.cancel();

        pipelang::model::TaskOptions options;
        options.evaluationStepsPerSegment = 33554432;
        coverage::CoverageSuite suite(options);
        const auto evaluated = suite.Run({}, cancelled.token());
        if (!evaluated.is_success())
            throw std::runtime_error(evaluated.error());

        const auto& report = evaluated.value();
        std::cout << "{\"schema\":\"dockpipe.qualification-coverage-source-tests/v1\",\"allPassed\":"
                  << product_harness::json(report.allPassed)
                  << ",\"evaluationStepsPerSegment\":" << options.evaluationStepsPerSegment
                  << ",\"results\":[";
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
            if (result.primaryFailure.has_value())
                std::cout << product_harness::json(result.primaryFailure.value().message);
            else
                std::cout << "null";

            std::cout << '}';
        }

        std::cout << "]}\n";
        return report.allPassed ? 0 : 1;
    } catch (const std::exception& error) {
        std::cerr << "Coverage Source consumer infrastructure: " << error.what() << '\n';
        return 2;
    }
}
