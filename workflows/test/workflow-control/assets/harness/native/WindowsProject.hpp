// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include "TraceFiles.hpp"
#include "WindowsFiles.hpp"

namespace workflow_control_native {

struct AdmittedProject final {
    dockpipe::sdk::ClientConfiguration configuration;
    proof::ProjectInputs inputs;
    std::shared_ptr<workflow_control_native::TraceRoot> traceFiles;
    std::vector<dockpipe::sdk::WindowsPath> paths;
};

AdmittedProject admitProject(const std::string& cli, const std::string& helper,
    const char* configPath, const char* rootPath, const char* statePath,
    const char* cachePath, const char* tracePath, std::int64_t index) {
    auto root = dockpipe::sdk::openWindowsPath(rootPath, true);
    auto config = dockpipe::sdk::openWindowsPath(configPath, false);
    dockpipe::sdk::WindowsFiles projectFiles(dockpipe::sdk::windowsUtf8(root.absolute));
    auto state = projectFiles.directory(statePath);
    auto cache = projectFiles.directory(cachePath);
    auto trace = dockpipe::sdk::openWindowsPath(tracePath, true);
    AdmittedProject admitted;
    admitted.inputs.platform = proof::ControlPlatform::Windows;
    admitted.inputs.rootIndex = index;
    admitted.inputs.workdir = dockpipe::sdk::windowsUtf8(root.nativePath);
    // Preserve admitted absolute operator spellings for workflow/cache identity
    // and the external script's TRACE input. All reads map through retained roots.
    admitted.inputs.workflowFile = dockpipe::sdk::windowsUtf8(config.absolute);
    admitted.inputs.traceRoot = dockpipe::sdk::windowsUtf8(trace.absolute);
    admitted.traceFiles = std::make_shared<dockpipe::sdk::WindowsFiles>(admitted.inputs.traceRoot);
    auto& configuration = admitted.configuration;
    configuration.executable = cli;
    configuration.consoleSignalExecutable = helper;
    configuration.allowedWorkdirRoot = admitted.inputs.workdir;
    configuration.stateDirectory = dockpipe::sdk::windowsUtf8(state.nativePath);
    configuration.cacheDirectory = dockpipe::sdk::windowsUtf8(cache.nativePath);
    configuration.maximumStoredRuns = 64;
    configuration.maximumActiveRuns = 1;
    configuration.maximumOutputBytesPerStream = 32768;
    configuration.maximumInventoryBytes = 1024 * 1024;
    configuration.maximumArgumentBytes = 65536;
    admitted.paths.push_back(std::move(root));
    admitted.paths.push_back(std::move(config));
    admitted.paths.push_back(std::move(state));
    admitted.paths.push_back(std::move(cache));
    admitted.paths.push_back(std::move(trace));
    return admitted;
}

struct AdmittedExecutables final {
    std::string cliPath;
    std::string helperPath;
    dockpipe::sdk::WindowsPath cli;
    dockpipe::sdk::WindowsPath helper;
};

inline AdmittedExecutables admitExecutables(const char* cliPath, const char* helperPath) {
    auto cli = dockpipe::sdk::openWindowsPath(cliPath, false);
    auto helper = dockpipe::sdk::openWindowsPath(helperPath, false);
    const auto admittedCliPath = dockpipe::sdk::windowsUtf8(cli.absolute);
    const auto admittedHelperPath = dockpipe::sdk::windowsUtf8(helper.absolute);

    return {admittedCliPath, admittedHelperPath, std::move(cli), std::move(helper)};
}
}
