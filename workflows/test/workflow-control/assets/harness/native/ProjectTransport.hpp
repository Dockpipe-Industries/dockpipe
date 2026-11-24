// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include "ControlSession.hpp"

#include <cstdint>
#include <memory>
#include <utility>

namespace workflow_control_native {
namespace source = workflow_control_host::source;

class ProjectTransport final : public source::Transport {
public:
    using Token = pipelang::model::CancellationToken;
    using StartResult = workflow_control_host::Transport::StartResult;
    using ObservationResult = workflow_control_host::Transport::ObservationResult;
    using BoolResult = workflow_control_host::Transport::BoolResult;
    using ScopeResult = workflow_control_host::Transport::ScopeResult;

    ProjectTransport(std::shared_ptr<ControlSession> session, std::size_t index)
        : session_(std::move(session)), index_(index) {
        if (!session_)
            throw std::invalid_argument("Project transport requires its native session owner");
    }

    StartResult Start(source::WorkflowRequest request) override {
        try {
            return session_->activate(index_)->Start(std::move(request));
        } catch (const std::exception& error) {
            return StartResult::Failure(error.what());
        }
    }

    StartResult StartScope(source::ScopeRequest request) override {
        try {
            return session_->activate(index_)->StartScope(std::move(request));
        } catch (const std::exception& error) {
            return StartResult::Failure(error.what());
        }
    }

    ObservationResult Observe(source::OperationId id, std::int64_t milliseconds, Token token) override {
        const auto transport = session_->active(index_);
        if (!transport)
            return ObservationResult::Failure("Project transport has no active native custody");

        return transport->Observe(id, milliseconds, token);
    }

    BoolResult Interrupt(source::OperationId id) override {
        const auto transport = session_->active(index_);
        if (!transport)
            return BoolResult::Failure("Project transport has no active native custody");

        return transport->Interrupt(id);
    }

    StartResult RetainedOperation() override {
        const auto transport = session_->active(index_);
        if (!transport)
            return StartResult::Failure("Project transport has no active native custody");

        return transport->RetainedOperation();
    }

    ObservationResult ForceDispose(source::OperationId id, std::int64_t milliseconds, Token token) override {
        const auto transport = session_->active(index_);
        if (!transport)
            return ObservationResult::Failure("Project transport has no active native custody");

        return transport->ForceDispose(id, milliseconds, token);
    }

    ScopeResult Scope(source::ScopeRequest request, std::int64_t milliseconds, Token token) override {
        try {
            return session_->activate(index_)->Scope(std::move(request), milliseconds, token);
        } catch (const std::exception& error) {
            return ScopeResult::Failure(error.what());
        }
    }

private:
    std::shared_ptr<ControlSession> session_;
    std::size_t index_;
};
}
