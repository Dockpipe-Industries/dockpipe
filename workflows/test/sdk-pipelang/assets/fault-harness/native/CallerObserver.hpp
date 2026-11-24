// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include "DockpipeSDKBridge.hpp"
#include "WindowsFiles.hpp"

#include <algorithm>
#include <array>
#include <atomic>
#include <cstdint>
#include <limits>
#include <sstream>
#include <stdexcept>
#include <vector>

namespace sdk_fault_fixture {
namespace caller_proof = sdk_fault::source::Dockpipe::SDK::FaultProof;

class CallerObserver final {
public:
    static std::size_t maximumHistoryCount(std::size_t receiptCapacity) {
        // At most a before/after observation for each of three source phases,
        // plus the pre-client baseline, post-constructor and post-close snapshots.
        constexpr std::size_t observationsPerRun = 6;
        constexpr std::size_t suiteObservations = 3;
        if (receiptCapacity > (std::numeric_limits<std::size_t>::max() - suiteObservations) / observationsPerRun)
            throw std::invalid_argument("Caller observation history capacity overflows");

        return receiptCapacity * observationsPerRun + suiteObservations;
    }

    explicit CallerObserver(std::size_t historyCapacity) : historyCapacity_(historyCapacity) {
        history_.reserve(historyCapacity_);
        bool available = false;
        if (!markerOwned_.compare_exchange_strong(available, true))
            throw std::runtime_error("One caller marker owner is required per fixture process");

        // Probe only our unique, not-yet-registered marker. Source verifies that
        // this platform distinguishes an absent marker before trusting removal.
        ::SetLastError(ERROR_SUCCESS);
        missingMarkerRemovalSucceeded_ = ::SetConsoleCtrlHandler(markerHandler, FALSE) != FALSE;
        missingMarkerRemovalError_ = missingMarkerRemovalSucceeded_ ? ERROR_SUCCESS : ::GetLastError();
        if (!::SetConsoleCtrlHandler(markerHandler, TRUE)) {
            const auto error = ::GetLastError();
            markerOwned_.store(false);
            throw std::runtime_error("Cannot install caller marker: Win32 " + std::to_string(error));
        }

        try {
            initial_ = observe();
        } catch (...) {
            (void)::SetConsoleCtrlHandler(markerHandler, FALSE);
            markerOwned_.store(false);
            throw;
        }
    }

    ~CallerObserver() {
        // Only our marker is removed. Its callback touches static atomics only,
        // so an in-flight callback never accesses a destroyed observer object.
        (void)::SetConsoleCtrlHandler(markerHandler, FALSE);
        markerOwned_.store(false);
    }

    CallerObserver(const CallerObserver&) = delete;
    CallerObserver& operator=(const CallerObserver&) = delete;

    const caller_proof::CallerObservation& initial() const noexcept { return initial_; }
    const std::vector<caller_proof::CallerObservation>& history() const noexcept { return history_; }

    caller_proof::CallerObservation observe() {
        if (history_.size() >= historyCapacity_)
            throw std::runtime_error("Caller observation history capacity exhausted");

        const auto eventsBefore = markerEvents_.load(std::memory_order_relaxed);
        std::ostringstream state;
        state << "window=" << reinterpret_cast<std::uintptr_t>(::GetConsoleWindow());
        standardHandle(state, STD_INPUT_HANDLE);
        standardHandle(state, STD_OUTPUT_HANDLE);
        standardHandle(state, STD_ERROR_HANDLE);

        ::SetLastError(ERROR_SUCCESS);
        const auto inputCodePage = ::GetConsoleCP();
        const auto inputCodePageError = inputCodePage == 0 ? ::GetLastError() : ERROR_SUCCESS;
        ::SetLastError(ERROR_SUCCESS);
        const auto outputCodePage = ::GetConsoleOutputCP();
        const auto outputCodePageError = outputCodePage == 0 ? ::GetLastError() : ERROR_SUCCESS;
        state << ";inputCP=" << inputCodePage << ",error=" << inputCodePageError
              << ";outputCP=" << outputCodePage << ",error=" << outputCodePageError;

        // The admitted native unit process contract is 128 processes. Console
        // inventory uses that same finite capacity and refuses overflow instead
        // of truncating or allocating from an untrusted returned count.
        constexpr std::size_t consoleProcessCapacity = 128;
        std::array<DWORD, consoleProcessCapacity> members{};
        ::SetLastError(ERROR_SUCCESS);
        const auto count = ::GetConsoleProcessList(members.data(), static_cast<DWORD>(members.size()));
        const auto inventoryError = count == 0 ? ::GetLastError() : ERROR_SUCCESS;
        if (count > members.size())
            throw std::runtime_error("Caller console inventory exceeds the admitted 128-process capacity");

        std::sort(members.begin(), members.begin() + count);
        state << ";consoleCount=" << count << ",error=" << inventoryError << ",pids=";
        for (DWORD index = 0; index < count; ++index)
            state << members[index] << ',';

        // No allocations or other operations occur between removing and
        // restoring this marker. Never change another handler or ignore flags.
        ::SetLastError(ERROR_SUCCESS);
        const bool removed = ::SetConsoleCtrlHandler(markerHandler, FALSE) != FALSE;
        const auto removeError = removed ? ERROR_SUCCESS : ::GetLastError();
        ::SetLastError(ERROR_SUCCESS);
        const bool restored = ::SetConsoleCtrlHandler(markerHandler, TRUE) != FALSE;
        const auto restoreError = restored ? ERROR_SUCCESS : ::GetLastError();
        const auto eventsAfter = markerEvents_.load(std::memory_order_relaxed);
        if (eventsAfter > static_cast<std::uint64_t>(std::numeric_limits<std::int64_t>::max()))
            throw std::runtime_error("Caller marker event count exceeds the source integer range");

        state << ";missingRemove=" << missingMarkerRemovalSucceeded_ << ",error=" << missingMarkerRemovalError_
              << ";markerRemove=" << removed << ",error=" << removeError
              << ";markerRestore=" << restored << ",error=" << restoreError
              << ";eventsBefore=" << eventsBefore << ";eventsAfter=" << eventsAfter;

        caller_proof::CallerObservation result{};
        result.state = state.str();
        result.missingMarkerRemovalSucceeded = missingMarkerRemovalSucceeded_;
        result.markerRemoved = removed;
        result.markerRestored = restored;
        result.markerEvents = static_cast<std::int64_t>(eventsAfter);
        history_.push_back(result);
        return result;
    }

private:
    static BOOL WINAPI markerHandler(DWORD) noexcept {
        markerEvents_.fetch_add(1, std::memory_order_relaxed);
        // Preserve the caller's existing handler/default processing behavior.
        return FALSE;
    }

    static void standardHandle(std::ostream& state, DWORD identifier) {
        ::SetLastError(ERROR_SUCCESS);
        const auto handle = ::GetStdHandle(identifier);
        const auto handleError = handle == INVALID_HANDLE_VALUE ? ::GetLastError() : ERROR_SUCCESS;
        ::SetLastError(ERROR_SUCCESS);
        const auto type = ::GetFileType(handle);
        const auto typeError = type == FILE_TYPE_UNKNOWN ? ::GetLastError() : ERROR_SUCCESS;
        DWORD mode = 0;
        ::SetLastError(ERROR_SUCCESS);
        const bool modeRead = ::GetConsoleMode(handle, &mode) != FALSE;
        const auto modeError = modeRead ? ERROR_SUCCESS : ::GetLastError();
        state << ";std=" << identifier << ",handle=" << reinterpret_cast<std::uintptr_t>(handle)
              << ",handleError=" << handleError << ",type=" << type << ",typeError=" << typeError
              << ",modeRead=" << modeRead << ",mode=" << mode << ",modeError=" << modeError;
    }

    static_assert(std::atomic<std::uint64_t>::is_always_lock_free);
    inline static std::atomic<std::uint64_t> markerEvents_{0};
    inline static std::atomic<bool> markerOwned_{false};
    std::size_t historyCapacity_;
    std::vector<caller_proof::CallerObservation> history_;
    caller_proof::CallerObservation initial_;
    bool missingMarkerRemovalSucceeded_ = false;
    DWORD missingMarkerRemovalError_ = ERROR_SUCCESS;
};
}
