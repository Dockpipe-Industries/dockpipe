// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "BrokerFixtureLaunch.hpp"

int wmain(int argc, wchar_t** argv) {
    try {
        sdk_fault_fixture::BrokerFixtureLaunch launch(argc, argv);
        launch.writeCreationWitness(sdk_fault_fixture::lostOfferWitnessName);
        launch.closeOffer();
        return launch.awaitAcknowledgement();
    } catch (...) {
        // No application diagnostics, fallback offer, resume, force or retry.
        return 41;
    }
}
