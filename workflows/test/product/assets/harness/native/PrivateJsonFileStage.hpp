// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include <pipelang/json/Reader.hpp>
#include <pipelang/model/byte_buffer.hpp>
#include <pipelang/model/result.hpp>

#include <cstdint>
#include <span>
#include <string>
#include <utility>
#include <vector>

namespace product_harness {

// The Source renderer and Source commit operation share this one named sink.
template<class Contract, class Digest, class ExclusiveOutput>
class PrivateJsonFileStage final : public Contract {
    template<class T> using Result = pipelang::model::Result<T, std::string>;
    using Token = pipelang::model::CancellationToken;

public:
    PrivateJsonFileStage(ExclusiveOutput& owner, std::string name, std::int64_t maximumBytes)
        : owner_(owner), name_(std::move(name)), maximumBytes_(maximumBytes) {}

    ~PrivateJsonFileStage() override {
        if (operation_ != 0)
            (void)owner_.CloseOutput(operation_);
    }

    PrivateJsonFileStage(const PrivateJsonFileStage&) = delete;
    PrivateJsonFileStage& operator=(const PrivateJsonFileStage&) = delete;

    void Append(std::span<const std::uint8_t> bytes) {
        if (sealed_)
            throw pipelang::json::ReadFailure("qualification.output.sealed");

        if (operation_ == 0) {
            const auto started = owner_.BeginExclusiveOutput(name_, maximumBytes_, {});
            if (!started.is_success())
                throw pipelang::json::ReadFailure("qualification.output." + started.error().code);

            operation_ = started.value();
        }

        pipelang::model::ByteBuffer fragment(std::vector<std::uint8_t>(bytes.begin(), bytes.end()));
        const auto appended = owner_.AppendOutput(operation_, std::move(fragment), {});
        if (!appended.is_success())
            throw pipelang::json::ReadFailure("qualification.output." + appended.error().code);

        bytes_ += static_cast<std::int64_t>(bytes.size());
    }

    Result<Digest> Commit(std::int64_t expectedBytes, Token token) override {
        if (sealed_ || operation_ == 0 || bytes_ != expectedBytes)
            return Result<Digest>::Failure("qualification.output.byte_identity");

        sealed_ = true;
        auto committed = owner_.CommitOutput(operation_, token);
        // Other outputs share this owner and may reuse the retired slot.
        const auto closed = Close();
        if (committed.is_cancelled())
            return Result<Digest>::Cancelled("qualification.output." + committed.error().code);
        if (!committed.is_success())
            return Result<Digest>::Failure("qualification.output." + committed.error().code);
        if (!closed.is_success())
            return Result<Digest>::Failure(closed.error());
        if (committed.value().byteCount != expectedBytes)
            return Result<Digest>::Failure("qualification.output.committed_extent");

        return Result<Digest>::Success(std::move(committed.value()));
    }

    Result<void> Close() override {
        sealed_ = true;
        if (operation_ == 0)
            return Result<void>::Success();

        const auto closed = owner_.CloseOutput(operation_);
        if (!closed.is_success())
            return Result<void>::Failure("qualification.output." + closed.error().code);

        operation_ = 0;
        return Result<void>::Success();
    }

private:
    ExclusiveOutput& owner_;
    std::string name_;
    std::int64_t maximumBytes_;
    std::int64_t operation_ = 0;
    std::int64_t bytes_ = 0;
    bool sealed_ = false;
};

} // namespace product_harness
