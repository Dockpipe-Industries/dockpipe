// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include "DockpipeSDKBridge.hpp"

#include <cstddef>
#include <limits>
#include <memory>
#include <stdexcept>
#include <utility>
#include <vector>

namespace workflow_control_native {

// Immutable caller-selected configurations. Source selects a bound Transport;
// this owner enforces one live native Client and bounded receipt preservation.
class ControlSession final {
public:
    explicit ControlSession(std::vector<dockpipe::sdk::ClientConfiguration> profiles)
        : profiles_(std::move(profiles)), attempted_(profiles_.size(), false) {
        if (profiles_.empty() || profiles_.size() > 2)
            throw std::invalid_argument("Control consumer requires one or two explicitly selected project profiles");

        for (const auto& profile : profiles_) {
            if (profile.maximumStoredRuns > std::numeric_limits<std::size_t>::max() - receiptCapacity_)
                throw std::invalid_argument("Control receipt archive capacity overflows");

            receiptCapacity_ += profile.maximumStoredRuns;
        }
        archive_.reserve(receiptCapacity_);
    }

    std::shared_ptr<workflow_control_host::Transport> activate(std::size_t index) {
        if (index >= profiles_.size())
            throw std::runtime_error("Selected control project index is outside the admitted profiles");

        if (transport_ && activeIndex_ == index)
            return transport_;

        if (attempted_[index])
            throw std::runtime_error("An already released or failed control project profile cannot be reopened");

        if (client_ && client_->hasUnreapedTree())
            throw std::runtime_error("Control project replacement refuses retained native custody");

        archiveCurrent();
        transport_.reset();
        client_.reset();
        activeIndex_ = index;
        attempted_[index] = true;
        client_ = std::make_shared<dockpipe::sdk::Client>(profiles_[index]);
        transport_ = std::make_shared<workflow_control_host::Transport>(client_);
        return transport_;
    }

    std::shared_ptr<workflow_control_host::Transport> active(std::size_t index) const noexcept {
        if (activeIndex_ == index)
            return transport_;

        return {};
    }

    std::vector<dockpipe::sdk::RunReceipt> receipts() const {
        auto result = archive_;
        if (client_) {
            const auto current = client_->receipts();
            if (current.size() > receiptCapacity_ - result.size())
                throw std::runtime_error("Control aggregate receipt capacity exhausted");

            result.insert(result.end(), current.begin(), current.end());
        }

        return result;
    }

    bool emergencyDispose() { return client_ && client_->emergencyDispose(); }
    bool hasUnreapedTree() { return client_ && client_->hasUnreapedTree(); }
    std::size_t receiptCapacity() const noexcept { return receiptCapacity_; }

private:
    void archiveCurrent() {
        const auto current = client_ ? client_->receipts() : std::vector<dockpipe::sdk::RunReceipt>{};
        if (current.size() > receiptCapacity_ - archive_.size())
            throw std::runtime_error("Control receipt archive capacity exhausted");

        archive_.insert(archive_.end(), current.begin(), current.end());
    }

    std::vector<dockpipe::sdk::ClientConfiguration> profiles_;
    std::vector<bool> attempted_;
    std::size_t activeIndex_ = std::numeric_limits<std::size_t>::max();
    std::size_t receiptCapacity_ = 0;
    std::vector<dockpipe::sdk::RunReceipt> archive_;
    std::shared_ptr<dockpipe::sdk::Client> client_;
    std::shared_ptr<workflow_control_host::Transport> transport_;
};
}
