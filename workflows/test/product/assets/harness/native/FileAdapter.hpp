#pragma once

#include "ConfinedFiles.hpp"
#include "Model.hpp"
#include <mutex>
#include <string>
#include <stdexcept>
#include <vector>

namespace product_harness {

struct FileReceipt {
    std::string path;
    std::int64_t byteLimit = 0;
    bool success = false;
    bool exists = false;
    std::string content;
    std::string error;
};

class FileAdapter final : public dockpipe_product::source::Dockpipe::Product::FilePort {
public:
    using Observation = dockpipe_product::source::Dockpipe::Product::FileObservation;
    using Result = pipelang::model::Result<Observation, std::string>;

    explicit FileAdapter(const ConfinedFiles& files) : files_(files) {}

    Result Observe(std::string absolutePath, std::int64_t byteLimit) override {
        std::lock_guard lock(mutex_);
        if (receipts_.size() == 128) {
            exhausted_ = true;
            return Result::Failure("128-entry file observation evidence bound exhausted");
        }
        FileReceipt receipt;
        receipt.path = absolutePath.substr(0, 4096);
        receipt.byteLimit = byteLimit;
        try {
            if (absolutePath.size() > 4096)
                throw std::runtime_error("file path exceeds 4096 bytes; evidence path prefix retained");
            if (byteLimit <= 0 || byteLimit > 32768)
                throw std::runtime_error("file observation limit must be 1..32768 bytes");
            auto [exists, content] = files_.read(receipt.path, static_cast<std::size_t>(byteLimit));
            receipt.success = true;
            receipt.exists = exists;
            receipt.content = std::move(content);
            Observation observation{};
            observation.exists = receipt.exists;
            observation.content = receipt.content;
            receipts_.push_back(std::move(receipt));
            return Result::Success(std::move(observation));
        } catch (const std::exception& error) {
            receipt.error = error.what();
            receipts_.push_back(std::move(receipt));
            return Result::Failure(error.what());
        }
    }

    std::vector<FileReceipt> receipts() {
        std::lock_guard lock(mutex_);
        return receipts_;
    }

    bool exhausted() {
        std::lock_guard lock(mutex_);
        return exhausted_;
    }

private:
    const ConfinedFiles& files_;
    std::mutex mutex_;
    std::vector<FileReceipt> receipts_;
    bool exhausted_ = false;
};
}
