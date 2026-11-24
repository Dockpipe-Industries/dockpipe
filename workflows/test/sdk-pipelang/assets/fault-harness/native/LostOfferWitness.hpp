// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include <cstdint>

namespace sdk_fault_fixture {
inline constexpr char lostOfferWitnessName[] = "/lost-offer-child.bin";
inline constexpr char partialOfferWitnessName[] = "/partial-offer-child.bin";
inline constexpr char lateOfferWitnessName[] = "/late-offer-attempt.bin";
inline constexpr std::uint32_t lateWitnessMagic = 0x44504c54;
inline constexpr std::uint32_t witnessMagic = 0x44504c57;

// Written once, only after successful suspended child creation. It is a factual
// fixture receipt, separate from CLI output and the private offer channel.
struct LostOfferWitness final {
    std::uint32_t magic = witnessMagic;
    std::uint32_t bytes = sizeof(LostOfferWitness);
    std::uint32_t processId = 0;
    std::uint32_t threadId = 0;
    std::uint64_t createdAt = 0;
};
static_assert(sizeof(LostOfferWitness) == 24);

struct LateOfferWitness final {
    std::uint32_t magic = lateWitnessMagic;
    std::uint32_t bytes = sizeof(LateOfferWitness);
    std::uint32_t processId = 0;
    std::uint32_t threadId = 0;
    std::uint64_t createdAt = 0;
    std::uint64_t requestDeadline = 0;
    std::uint64_t attemptedAt = 0;
    std::uint32_t writeSucceeded = 0;
    std::uint32_t reserved = 0;
};
static_assert(sizeof(LateOfferWitness) == 48);
}
