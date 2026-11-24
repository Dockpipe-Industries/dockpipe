// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include "DockpipeSDKBridge.hpp"
#ifdef _WIN32
#include "WindowsFiles.hpp"
#else
#include "ConfinedFiles.hpp"
#endif

#include <cstddef>
#include <cstdint>
#include <memory>
#include <stdexcept>
#include <string>
#include <utility>
#include <vector>

namespace workflow_control_native {
namespace proof = workflow_control_proof::source::Dockpipe::WorkflowControl::Proof;

#ifdef _WIN32
using TraceRoot = dockpipe::sdk::WindowsFiles;
#else
using TraceRoot = product_harness::ConfinedFiles;
#endif

struct TraceEvidence final {
    std::int64_t rootIndex = 0;
    std::string path;
    bool readSucceeded = false;
    bool exists = false;
    std::string contents;
    std::string error;
};

class TraceFiles final : public proof::TraceFiles {
public:
    using ReadResult = pipelang::model::Result<proof::TraceRead, std::string>;

    TraceFiles(std::vector<std::shared_ptr<TraceRoot>> roots,
        std::size_t maximumBytes, std::size_t maximumFiles)
        : roots_(std::move(roots)), maximumBytes_(maximumBytes), maximumFiles_(maximumFiles) {
        if (roots_.empty() || roots_.size() > 2 || maximumBytes_ == 0 || maximumFiles_ == 0)
            throw std::invalid_argument("Trace observation requires explicit roots and finite byte/file capacities");

        evidence_.reserve(maximumFiles_);
    }

    ReadResult Read(std::int64_t rootIndex, std::string path, std::int64_t maximumBytes) override {
        TraceEvidence evidence;
        evidence.rootIndex = rootIndex;
        evidence.path = path;
        try {
            if (rootIndex < 0 || static_cast<std::uint64_t>(rootIndex) >= roots_.size() ||
                maximumBytes <= 0 || static_cast<std::uint64_t>(maximumBytes) > maximumBytes_)
                return ReadResult::Failure("Trace root or requested byte capacity is outside the admitted contract");

            const auto [exists, contents] = roots_[static_cast<std::size_t>(rootIndex)]->read(
                path, static_cast<std::size_t>(maximumBytes));
            evidence.readSucceeded = true;
            evidence.exists = exists;
            evidence.contents = contents;
            remember(evidence);
            proof::TraceRead result{};
            result.exists = exists;
            result.contents = contents;
            return ReadResult::Success(std::move(result));
        } catch (const std::exception& error) {
            evidence.error = error.what();
            try {
                remember(evidence);
            } catch (...) {
            }

            return ReadResult::Failure(error.what());
        }
    }

    const std::vector<TraceEvidence>& evidence() const noexcept { return evidence_; }

private:
    void remember(const TraceEvidence& evidence) {
        for (auto& prior : evidence_) {
            if (prior.rootIndex == evidence.rootIndex && prior.path == evidence.path) {
                prior = evidence;
                return;
            }
        }
        if (evidence_.size() >= maximumFiles_)
            throw std::runtime_error("Trace observation file capacity exhausted");

        evidence_.push_back(evidence);
    }

    std::vector<std::shared_ptr<TraceRoot>> roots_;
    std::size_t maximumBytes_;
    std::size_t maximumFiles_;
    std::vector<TraceEvidence> evidence_;
};
}
