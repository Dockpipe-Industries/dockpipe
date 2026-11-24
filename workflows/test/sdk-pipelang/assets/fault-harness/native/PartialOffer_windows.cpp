// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "BrokerFixtureLaunch.hpp"

int wmain(int argc, wchar_t** argv) {
    try {
        sdk_fault_fixture::BrokerFixtureLaunch launch(argc, argv);
        launch.writeCreationWitness(sdk_fault_fixture::partialOfferWitnessName);
        const auto offer = launch.offer();
        // Capture the genuine offer first, then revoke only its thread handle.
        // No further handle allocation occurs before the single fixed write.
        launch.closeThreadCapability();
        (void)launch.writeOffer(offer);
        launch.closeOffer();
        return launch.awaitAcknowledgement();
    } catch (...) {
        return 41;
    }
}
