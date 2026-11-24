// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include "CallerObserver.hpp"
#include "ReceiptReport.hpp"

#include <ostream>
#include <string>
#include <vector>

namespace sdk_fault_fixture {
inline void reportCallerObservations(std::ostream& output,
    const std::vector<caller_proof::CallerObservation>& observations,
    std::string (*quote)(const std::string&)) {
    output << '[';
    bool first = true;
    for (const auto& observed : observations) {
        if (!first)
            output << ',';

        first = false;
        output << "{\"state\":" << quote(observed.state)
               << ",\"missingMarkerRemovalSucceeded\":" << boolean(observed.missingMarkerRemovalSucceeded)
               << ",\"markerRemoved\":" << boolean(observed.markerRemoved)
               << ",\"markerRestored\":" << boolean(observed.markerRestored)
               << ",\"markerEvents\":" << observed.markerEvents << '}';
    }
    output << ']';
}
}
