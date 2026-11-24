// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include <pipelang/json/DocumentWriter.hpp>

#include <cstddef>
#include <cstdint>
#include <functional>
#include <optional>
#include <vector>

namespace product_harness {

// The Source policy assigns registry positions. Each staged value holds the
// projected row after its source receipt can safely release all reader state.
class StagedCaseRows final {
    using Writer = pipelang::json::DocumentWriter;
    using Options = pipelang::json::RenderOptions;
    using Cancelled = std::function<bool()>;

public:
    StagedCaseRows(std::size_t caseCount, Options options)
        : rows_(CheckedCount(caseCount)), options_(options) {
        if (options_.maximumBytes == 0 || options_.maximumBytes > WriterLimit)
            throw pipelang::json::ReadFailure("qualification.report.row_budget");
    }

    StagedCaseRows(const StagedCaseRows&) = delete;
    StagedCaseRows& operator=(const StagedCaseRows&) = delete;

    void Stage(std::size_t position, pipelang::json::Reader& reader, const Writer::ValueBuilder& project,
               const Cancelled& cancelled = {}) {
        auto& target = At(position);
        if (target)
            throw pipelang::json::ReadFailure("qualification.report.duplicate_row");
        if (options_.maximumBytes == retainedBytes_)
            throw pipelang::json::ReadFailure("qualification.report.row_budget");

        Options available = options_;
        available.maximumBytes -= retainedBytes_;
        auto value = Writer::capture(reader, available, project, cancelled);

        const auto extent = value.size();
        target.emplace(std::move(value));
        retainedBytes_ += extent;
    }

    void Write(std::size_t position, Writer& output, const Cancelled& cancelled = {}) {
        auto& selected = At(position);
        if (!selected)
            throw pipelang::json::ReadFailure("qualification.report.missing_row");

        output.captured(*selected, cancelled);
    }

private:
    static constexpr std::size_t maximumCases = 8192;
    static constexpr std::size_t WriterLimit = pipelang::json::Reader::maximumRenderBytes;
    std::vector<std::optional<Writer::CapturedValue>> rows_;
    const Options options_;
    std::size_t retainedBytes_ = 0;

    static std::size_t CheckedCount(std::size_t caseCount) {
        if (caseCount > maximumCases)
            throw pipelang::json::ReadFailure("qualification.report.case_count");

        return caseCount;
    }

    std::optional<Writer::CapturedValue>& At(std::size_t position) {
        if (position >= rows_.size())
            throw pipelang::json::ReadFailure("qualification.report.row_position");

        return rows_[position];
    }
};

} // namespace product_harness
