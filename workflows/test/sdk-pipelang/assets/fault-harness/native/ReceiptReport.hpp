// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include "Client.hpp"

#include <ostream>
#include <string>
#include <vector>

namespace sdk_fault_fixture {
inline const char* boolean(bool value) noexcept {
    return value ? "true" : "false";
}

inline void reportReceipts(std::ostream& output, const std::vector<dockpipe::sdk::RunReceipt>& receipts,
    std::string (*quote)(const std::string&)) {
    output << '[';
    bool first = true;
    for (const auto& receipt : receipts) {
        if (!first)
            output << ',';

        first = false;
        const auto& observed = receipt.observation;
        output << "{\"id\":{\"clientId\":" << observed.id.clientId
               << ",\"value\":" << observed.id.value << "},\"leaderPid\":" << receipt.leaderPid
               << ",\"exited\":" << boolean(observed.exited)
               << ",\"treeComplete\":" << boolean(observed.treeComplete)
               << ",\"forced\":" << boolean(observed.forced)
               << ",\"exitCode\":" << observed.exitCode
               << ",\"reapedChildren\":" << observed.reapedChildren
               << ",\"stdoutText\":" << quote(observed.stdoutText)
               << ",\"stderrText\":" << quote(observed.stderrText)
               << ",\"stdoutTruncated\":" << boolean(observed.stdoutTruncated)
               << ",\"stderrTruncated\":" << boolean(observed.stderrTruncated)
               << ",\"scopePath\":" << quote(observed.scopePath)
               << ",\"infrastructureFailure\":" << quote(receipt.infrastructureFailure)
               << ",\"interruptRequested\":" << boolean(receipt.interruptRequested)
               << ",\"scopeQuery\":" << boolean(receipt.scopeQuery)
               << ",\"scopePathQuery\":" << boolean(receipt.scopePathQuery)
               << ",\"arguments\":[";
        bool firstArgument = true;
        for (const auto& argument : receipt.arguments) {
            if (!firstArgument)
                output << ',';

            firstArgument = false;
            output << quote(argument);
        }
        output << "]}";
    }
    output << ']';
}
}
