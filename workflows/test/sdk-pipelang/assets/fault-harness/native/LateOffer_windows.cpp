// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "BrokerFixtureLaunch.hpp"

int wmain(int argc, wchar_t** argv) {
    try {
        sdk_fault_fixture::BrokerFixtureLaunch launch(argc, argv);
        const auto offer = launch.offer();
        launch.waitPastDeadline();
        const auto attemptedAt = ::GetTickCount64();
        const bool sent = launch.writeOffer(offer);
        launch.closeOffer();
        // A broken-pipe write is still an actual late offer attempt. Source
        // checks this factual witness before it may explicitly force custody.
        launch.writeLateWitness(attemptedAt, sent);
        return launch.awaitAcknowledgement();
    } catch (...) {
        return 41;
    }
}
