// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include <pipelang/io/LinuxFiles.hpp>
#include <pipelang/model/byte_buffer.hpp>
#include <pipelang/model/result.hpp>

#include <cstdint>
#include <memory>
#include <string>
#include <utility>

namespace product_harness {

// The driver owns creation of its fresh output directory. Acquire its
// descriptor root when Source first reads its completed results.
template<class Contract, class Files, class Snapshot, class Digest, class Failure>
class LaneOutputFiles final : public Contract {
    template<class T> using Result = pipelang::model::Result<T, Failure>;
    using Token = pipelang::model::CancellationToken;
    using Bytes = pipelang::model::ByteBuffer;

public:
    LaneOutputFiles(std::string root, pipelang::io::PathProfile profile)
        : root_(std::move(root)), profile_(profile) {}

    Result<Snapshot> CaptureSnapshot(std::string relative, std::int64_t maximumBytes, Token token) override {
        return files().CaptureSnapshot(std::move(relative), maximumBytes, token);
    }

    Result<Bytes> ReadSnapshot(std::int64_t id, std::int64_t offset, std::int64_t count) override {
        return files().ReadSnapshot(id, offset, count);
    }

    Result<void> CloseSnapshot(std::int64_t id) override {
        return files().CloseSnapshot(id);
    }

    Result<Digest> DigestFile(std::string relative, std::int64_t maximumBytes, Token token) override {
        return files().DigestFile(std::move(relative), maximumBytes, token);
    }

private:
    std::string root_;
    pipelang::io::PathProfile profile_;
    std::unique_ptr<Files> files_;

    Files& files() {
        if (!files_)
            files_ = std::make_unique<Files>(root_, profile_);

        return *files_;
    }
};

} // namespace product_harness
