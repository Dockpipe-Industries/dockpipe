// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include "StagedCaseRows.hpp"

#include <pipelang/model/cancellation.hpp>
#include <pipelang/model/result.hpp>

#include <cstdint>
#include <functional>
#include <new>
#include <string>
#include <string_view>
#include <utility>

namespace product_harness {

template<class Contract, class Node>
class ReportRowsAdapter final : public Contract {
    using Writer = pipelang::json::DocumentWriter;
    using Result = pipelang::model::Result<void, std::string>;
    using Token = pipelang::model::CancellationToken;

public:
    using Project = std::function<Result(Writer&, Node, bool, const std::string&, Token)>;

    ReportRowsAdapter(pipelang::json::Reader& reader, Writer& destination, StagedCaseRows& rows, Project project)
        : reader_(reader), destination_(destination), rows_(rows), project_(std::move(project)) {}

    Result Stage(std::int64_t position, Node row, bool critical, std::string evidenceRoot, Token token) override {
        return Invoke(position, token, [&] {
            rows_.Stage(static_cast<std::size_t>(position), reader_, [&](Writer& writer) {
                const auto projected = project_(writer, row, critical, evidenceRoot, token);
                if (projected.is_cancelled())
                    throw pipelang::json::ReadFailure("json.cancelled");
                if (!projected.is_success())
                    throw pipelang::json::ReadFailure(projected.error());
            }, [&] { return token.is_cancelled(); });
        });
    }

    Result Write(std::int64_t position, Token token) override {
        return Invoke(position, token, [&] {
            rows_.Write(static_cast<std::size_t>(position), destination_, [&] { return token.is_cancelled(); });
        });
    }

private:
    pipelang::json::Reader& reader_;
    Writer& destination_;
    StagedCaseRows& rows_;
    Project project_;

    template<class Operation>
    static Result Invoke(std::int64_t position, Token token, Operation&& operation) {
        if (token.is_cancelled())
            return Result::Cancelled("json.cancelled");
        if (position < 0 || position >= 8192)
            return Result::Failure("qualification.report.row_position");

        try {
            std::forward<Operation>(operation)();
            return Result::Success();
        } catch (const pipelang::json::ReadFailure& failure) {
            if (std::string_view(failure.what()) == "json.cancelled")
                return Result::Cancelled(failure.what());

            return Result::Failure(failure.what());
        } catch (const std::bad_alloc&) {
            return Result::Failure("json.memory_limit");
        }
    }
};

} // namespace product_harness
