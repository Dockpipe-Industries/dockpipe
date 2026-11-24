#pragma once

#include <map>
#include <stdexcept>
#include <string>

namespace product_harness {

struct Options final {
    std::string cli;
    std::string proofRoot;
    std::string recoveryProject;
    std::string cancellationProject;
    std::string workflow;
    std::string outsideFile;
    std::string outsideDirectory;
    std::string runId;
    std::string stateDirectory;
    std::string cacheDirectory;
    std::string report;
    std::string identityFile;
    std::string consoleSignalExecutable;

    static Options parse(int argc, char** argv) {
        std::map<std::string, std::string> arguments;
        for (int index = 1; index < argc; index += 2) {
            const std::string key(argv[index]);
            if (index + 1 >= argc)
                throw std::runtime_error("every harness option requires a value");
            const std::string value(argv[index + 1]);
            if (value.empty() || value.size() > 4096 || !arguments.emplace(key, value).second)
                throw std::runtime_error("harness options must be unique, nonempty and bounded");
        }
        auto take = [&](const std::string& key) {
            const auto found = arguments.find(key);
            if (found == arguments.end())
                throw std::runtime_error("missing harness option " + key);
            auto value = found->second;
            arguments.erase(found);
            return value;
        };
        Options result;
        result.cli = take("--cli");
        result.proofRoot = take("--proof-root");
        result.recoveryProject = take("--recovery-project");
        result.cancellationProject = take("--cancellation-project");
        result.workflow = take("--workflow");
        result.outsideFile = take("--outside-file");
        result.outsideDirectory = take("--outside-directory");
        result.runId = take("--run-id");
        result.stateDirectory = take("--xdg-state");
        result.cacheDirectory = take("--xdg-cache");
        result.report = take("--report");
        if (arguments.contains("--identity-file"))
            result.identityFile = take("--identity-file");
#ifdef _WIN32
        result.consoleSignalExecutable = take("--console-signal");
#else
        if (arguments.contains("--console-signal"))
            result.consoleSignalExecutable = take("--console-signal");
#endif

        if (!arguments.empty())
            throw std::runtime_error("unknown harness option " + arguments.begin()->first);
        if (result.runId.size() > 128 || result.runId.find_first_not_of("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.") != std::string::npos)
            throw std::runtime_error("run id must be 1..128 portable identifier characters");
        return result;
    }
};
}
