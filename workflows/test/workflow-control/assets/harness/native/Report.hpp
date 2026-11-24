// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include "ControlSession.hpp"
#include "TraceFiles.hpp"

#include <iostream>
#include <stdexcept>
#include <string>

namespace workflow_control_native {
namespace testing = workflow_control_proof::source::PipeLang::Testing;

inline std::string json(const std::string& text) {
    constexpr char digits[] = "0123456789abcdef";
    std::string result = "\"";
    for (const unsigned char byte : text) {
        if (byte == '"' || byte == '\\') {
            result += '\\';
            result += static_cast<char>(byte);
        } else if (byte < 32 || byte >= 127) {
            result += "\\u00";
            result += digits[byte >> 4];
            result += digits[byte & 15];
        } else {
            result += static_cast<char>(byte);
        }
    }
    result += '"';
    return result;
}

inline const char* boolean(bool value) noexcept {
    return value ? "true" : "false";
}

inline const char* outcome(testing::TestOutcome value) {
    switch (value) {
    case testing::TestOutcome::Passed: return "Passed";
    case testing::TestOutcome::Failed: return "Failed";
    case testing::TestOutcome::Cancelled: return "Cancelled";
    }
    throw std::runtime_error("Unknown source test outcome");
}

inline const char* phase(testing::TestPhase value) {
    switch (value) {
    case testing::TestPhase::NotRun: return "NotRun";
    case testing::TestPhase::Setup: return "Setup";
    case testing::TestPhase::Body: return "Body";
    case testing::TestPhase::Cleanup: return "Cleanup";
    }
    throw std::runtime_error("Unknown source test phase");
}

template<class Failure>
std::string failure(const Failure& value) {
    if (!value.has_value())
        return "null";

    return "{\"code\":" + json(value.value().code) + ",\"message\":" + json(value.value().message) + "}";
}

inline void rawFacts(const ControlSession& session, const TraceFiles& files) {
    std::cout << ",\"nativeReceipts\":[";
    bool first = true;
    for (const auto& receipt : session.receipts()) {
        if (!first)
            std::cout << ',';

        first = false;
        const auto& observed = receipt.observation;
        std::cout << "{\"clientId\":" << observed.id.clientId << ",\"operation\":" << observed.id.value
                  << ",\"leaderPid\":" << receipt.leaderPid << ",\"exited\":" << boolean(observed.exited)
                  << ",\"treeComplete\":" << boolean(observed.treeComplete) << ",\"forced\":" << boolean(observed.forced)
                  << ",\"exitCode\":" << observed.exitCode << ",\"reapedChildren\":" << observed.reapedChildren
                  << ",\"infrastructureFailure\":" << json(receipt.infrastructureFailure)
                  << ",\"interruptRequested\":" << boolean(receipt.interruptRequested)
                  << ",\"stdoutText\":" << json(observed.stdoutText) << ",\"stderrText\":" << json(observed.stderrText)
                  << ",\"stdoutTruncated\":" << boolean(observed.stdoutTruncated)
                  << ",\"stderrTruncated\":" << boolean(observed.stderrTruncated) << ",\"arguments\":[";
        bool firstArgument = true;
        for (const auto& argument : receipt.arguments) {
            if (!firstArgument)
                std::cout << ',';

            firstArgument = false;
            std::cout << json(argument);
        }
        std::cout << "]}";
    }
    std::cout << "],\"traceFiles\":[";
    first = true;
    for (const auto& trace : files.evidence()) {
        if (!first)
            std::cout << ',';

        first = false;
        std::cout << "{\"rootIndex\":" << trace.rootIndex << ",\"path\":" << json(trace.path)
                  << ",\"readSucceeded\":" << boolean(trace.readSucceeded) << ",\"exists\":" << boolean(trace.exists)
                  << ",\"contents\":" << json(trace.contents) << ",\"error\":" << json(trace.error) << '}';
    }
    std::cout << ']';
}

template<class SuiteResult>
bool report(const SuiteResult& result, const ControlSession& session, const TraceFiles& files) {
    if (!result.is_success()) {
        std::cout << "{\"taskOutcome\":" << json(result.is_cancelled() ? "Cancelled" : "Failure")
                  << ",\"error\":" << json(result.error())
                  << ",\"stringEncoding\":\"bytes-as-u00xx\",\"results\":[]";
        rawFacts(session, files);
        std::cout << "}\n";
        return false;
    }

    bool passed = true;
    bool first = true;
    std::cout << "{\"taskOutcome\":\"Success\",\"stringEncoding\":\"bytes-as-u00xx\",\"results\":[";
    for (const auto& entry : result.value().values()) {
        if (!first)
            std::cout << ',';

        first = false;
        const auto& test = entry.test;
        passed = passed && test.outcome == testing::TestOutcome::Passed;
        std::cout << "{\"id\":" << json(test.id) << ",\"name\":" << json(test.name)
                  << ",\"outcome\":" << json(outcome(test.outcome))
                  << ",\"primaryOutcome\":" << json(outcome(test.primaryOutcome))
                  << ",\"primaryPhase\":" << json(phase(test.primaryPhase))
                  << ",\"primaryFailure\":" << failure(test.primaryFailure)
                  << ",\"cleanupOutcome\":" << json(outcome(test.cleanupOutcome))
                  << ",\"cleanupFailure\":" << failure(test.cleanupFailure)
                  << ",\"cleanupClockFailure\":" << failure(entry.cleanupClockFailure)
                  << ",\"bodyRan\":" << boolean(test.bodyRan) << ",\"cleanupRan\":" << boolean(test.cleanupRan) << '}';
    }
    std::cout << "],\"passed\":" << boolean(passed);
    rawFacts(session, files);
    std::cout << "}\n";
    return passed;
}
}
