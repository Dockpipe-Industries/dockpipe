// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include "TraceFiles.hpp"

#include <cstdint>
#include <memory>
#include <stdexcept>
#include <string>
#include <utility>
#include <vector>

namespace workflow_control_native {

struct AdmittedProject final {
    dockpipe::sdk::ClientConfiguration configuration;
    proof::ProjectInputs inputs;
    std::shared_ptr<TraceRoot> traceFiles;
    std::vector<product_harness::Descriptor> paths;
};

inline AdmittedProject admitProject(const std::string& cli, const std::string& helper,
    const char* configPath, const char* rootPath, const char* statePath,
    const char* cachePath, const char* tracePath, std::int64_t index) {
    if (!helper.empty())
        throw std::invalid_argument("Linux composition requires an empty helper argument");

    product_harness::ConfinedFiles projectFiles(rootPath);
    auto root = projectFiles.directory(rootPath);
    auto config = product_harness::ConfinedFiles::openAbsolute(configPath, false);
    auto state = projectFiles.directory(statePath);
    auto cache = projectFiles.directory(cachePath);
    auto traceFiles = std::make_shared<TraceRoot>(tracePath);
    AdmittedProject admitted;
    admitted.inputs.platform = proof::ControlPlatform::Linux;
    admitted.inputs.rootIndex = index;
    admitted.inputs.workdir = rootPath;
    admitted.inputs.workflowFile = configPath;
    admitted.inputs.traceRoot = tracePath;
    admitted.traceFiles = std::move(traceFiles);

    auto& configuration = admitted.configuration;
    configuration.executable = cli;
    configuration.allowedWorkdirRoot = rootPath;
    configuration.stateDirectory = statePath;
    configuration.cacheDirectory = cachePath;
    configuration.maximumStoredRuns = 64;
    configuration.maximumActiveRuns = 1;
    configuration.maximumOutputBytesPerStream = 32768;
    configuration.maximumInventoryBytes = 1024 * 1024;
    configuration.maximumArgumentBytes = 65536;

    admitted.paths.push_back(std::move(root));
    admitted.paths.push_back(std::move(config));
    admitted.paths.push_back(std::move(state));
    admitted.paths.push_back(std::move(cache));
    return admitted;
}

struct AdmittedExecutables final {
    std::string cliPath;
    std::string helperPath;
    product_harness::Descriptor cli;
};

inline AdmittedExecutables admitExecutables(const char* cliPath, const char* helperPath) {
    auto cli = product_harness::ConfinedFiles::openAbsolute(cliPath, false);
    if (std::string(helperPath) != "")
        throw std::invalid_argument("Linux composition requires an empty helper argument");

    return {cliPath, "", std::move(cli)};
}
}
