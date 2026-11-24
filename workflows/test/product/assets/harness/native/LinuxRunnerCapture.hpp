// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include <pipelang/io/LinuxFiles.hpp>
#include <pipelang/model/cancellation.hpp>
#include <pipelang/model/result.hpp>

#include <cstdint>
#include <filesystem>
#include <string>
#include <utility>

namespace product_harness {

template<class Contract, class Identity, class ScopedFiles, class Digest>
class LinuxRunnerCapture final : public Contract {
    template<class T> using Result = pipelang::model::Result<T, std::string>;
    using Token = pipelang::model::CancellationToken;
    using Path = std::filesystem::path;

public:
    LinuxRunnerCapture(Path driverExecutable, pipelang::io::PathProfile files)
        : driverExecutable_(std::move(driverExecutable)), files_(files) {}

    Result<Identity> Capture(Token token) override {
        if (token.is_cancelled())
            return Result<Identity>::Cancelled("qualification.runner_identity.cancelled");

        std::error_code error;
        const Path running = std::filesystem::read_symlink("/proc/self/exe", error);
        if (error || !CurrentImage(running))
            return Result<Identity>::Failure("qualification.runner_identity.running_image");

        auto executable = DigestExecutable(running, token);
        if (executable.is_cancelled())
            return Result<Identity>::Cancelled(executable.error());
        if (!executable.is_success())
            return Result<Identity>::Failure(executable.error());
        if (!CurrentImage(running))
            return Result<Identity>::Failure("qualification.runner_identity.running_image_changed");

        auto driver = DigestExecutable(driverExecutable_, token);
        if (driver.is_cancelled())
            return Result<Identity>::Cancelled(driver.error());
        if (!driver.is_success())
            return Result<Identity>::Failure(driver.error());

        Identity identity;
        identity.executable = running.string();
        identity.executableSha256 = std::move(executable.value().sha256);
        identity.driverExecutable = driverExecutable_.string();
        identity.driverExecutableSha256 = std::move(driver.value().sha256);
        return Result<Identity>::Success(std::move(identity));
    }

private:
    static constexpr std::int64_t maximumExecutableBytes = 536870912;
    const Path driverExecutable_;
    const pipelang::io::PathProfile files_;

    static bool CurrentImage(const Path& path) {
        std::error_code error;
        return std::filesystem::equivalent(path, "/proc/self/exe", error) && !error;
    }

    Result<Digest> DigestExecutable(const Path& executable, Token token) const {
        if (!executable.is_absolute() || executable.filename().empty())
            return Result<Digest>::Failure("qualification.runner_identity.executable_path");

        ScopedFiles directory(executable.parent_path().string(), files_);
        auto digest = directory.DigestFile(executable.filename().string(), maximumExecutableBytes, token);
        if (digest.is_cancelled())
            return Result<Digest>::Cancelled("qualification.runner_identity.cancelled");
        if (!digest.is_success())
            return Result<Digest>::Failure("qualification.runner_identity." + digest.error().code);

        return Result<Digest>::Success(std::move(digest.value()));
    }
};

} // namespace product_harness
