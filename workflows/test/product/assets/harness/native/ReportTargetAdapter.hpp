// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include <pipelang/json/DocumentWriter.hpp>
#include <pipelang/model/cancellation.hpp>
#include <pipelang/model/result.hpp>

#include <cstddef>
#include <cstdint>
#include <new>
#include <optional>
#include <string>
#include <string_view>

namespace product_harness {

// A target is built in its own capture, so interruption before its transfer
// cannot leave an incomplete target in the run's staged report.
template<class Contract>
class ReportTargetAdapter final : public Contract {
    using Writer = pipelang::json::DocumentWriter;
    using Token = pipelang::model::CancellationToken;
    using Result = pipelang::model::Result<void, std::string>;

public:
    explicit ReportTargetAdapter(Writer& output, pipelang::json::RenderOptions options)
        : output_(output), options_(options) {}

    void Stage(pipelang::json::Reader& reader, const Writer::ValueBuilder& build, Token token = {}) {
        if (pending_ || position_ == 64)
            throw pipelang::json::ReadFailure("qualification.report.target_state");

        pending_.emplace(Writer::capture(reader, options_, build, [&] { return token.is_cancelled(); }));
    }

    Result WriteTarget(std::int64_t position, Token token) override {
        if (token.is_cancelled())
            return Result::Cancelled("qualification.report.cancelled");
        if (!pending_ || position != position_)
            return Result::Failure("qualification.report.target_position");

        try {
            output_.captured(*pending_, [&] { return token.is_cancelled(); });
        } catch (const pipelang::json::ReadFailure& error) {
            if (std::string_view(error.what()) == "json.cancelled")
                return Result::Cancelled(error.what());

            return Result::Failure(error.what());
        } catch (const std::bad_alloc&) {
            return Result::Failure("json.memory_limit");
        }

        pending_.reset();
        ++position_;
        return Result::Success();
    }

private:
    Writer& output_;
    const pipelang::json::RenderOptions options_;
    std::optional<Writer::CapturedValue> pending_;
    std::int64_t position_ = 0;
};

} // namespace product_harness
