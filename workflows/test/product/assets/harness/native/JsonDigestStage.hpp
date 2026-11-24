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

// Render the canonical request through the same writer used for a fresh lane,
// but retain only its digest and extent when checking an existing invocation.
template<class Contract, class Digest, class Sha256Streams>
class JsonDigestStage final : public Contract {
    template<class T> using Result = pipelang::model::Result<T, std::string>;
    using Token = pipelang::model::CancellationToken;

public:
    JsonDigestStage(Sha256Streams& streams, std::int64_t maximumBytes)
        : streams_(streams), maximumBytes_(maximumBytes) {}

    ~JsonDigestStage() override {
        if (hash_ != 0)
            (void)streams_.CloseSha256(hash_);
    }

    JsonDigestStage(const JsonDigestStage&) = delete;
    JsonDigestStage& operator=(const JsonDigestStage&) = delete;

    void Append(std::span<const std::uint8_t> bytes) {
        if (sealed_)
            throw pipelang::json::ReadFailure("qualification.output.sealed");

        if (hash_ == 0) {
            const auto started = streams_.BeginSha256(maximumBytes_);
            if (!started.is_success())
                throw pipelang::json::ReadFailure("qualification.output." + started.error().code);

            hash_ = started.value();
        }

        pipelang::model::ByteBuffer fragment(std::vector<std::uint8_t>(bytes.begin(), bytes.end()));
        const auto updated = streams_.UpdateSha256(hash_, std::move(fragment), {});
        if (!updated.is_success())
            throw pipelang::json::ReadFailure("qualification.output." + updated.error().code);
    }

    Result<Digest> Commit(std::int64_t expectedBytes, Token token) override {
        if (sealed_ || hash_ == 0)
            return Result<Digest>::Failure("qualification.output.byte_identity");

        sealed_ = true;
        auto finished = streams_.FinishSha256(hash_, token);
        if (finished.is_cancelled())
            return Result<Digest>::Cancelled("qualification.output." + finished.error().code);
        if (!finished.is_success())
            return Result<Digest>::Failure("qualification.output." + finished.error().code);
        if (finished.value().byteCount != expectedBytes)
            return Result<Digest>::Failure("qualification.output.byte_identity");

        Digest digest;
        digest.byteCount = expectedBytes;
        digest.sha256 = std::move(finished.value().lowercaseHex);
        return Result<Digest>::Success(std::move(digest));
    }

    Result<void> Close() override {
        sealed_ = true;
        if (hash_ == 0)
            return Result<void>::Success();

        const auto closed = streams_.CloseSha256(hash_);
        if (!closed.is_success())
            return Result<void>::Failure("qualification.output." + closed.error().code);

        hash_ = 0;
        return Result<void>::Success();
    }

private:
    Sha256Streams& streams_;
    std::int64_t maximumBytes_;
    std::int64_t hash_ = 0;
    bool sealed_ = false;
};

} // namespace product_harness
