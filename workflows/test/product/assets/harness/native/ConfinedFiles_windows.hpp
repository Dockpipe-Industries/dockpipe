#pragma once

#include "WindowsFiles.hpp"

#include <string>
#include <string_view>
#include <utility>

namespace product_harness {

// Reuse the SDK's handle-relative Windows effects; source owns all expectations.
class ConfinedFiles final {
public:
    explicit ConfinedFiles(std::string root) : files_(std::move(root)) {}

    dockpipe::sdk::WindowsPath directory(std::string_view absolute) const {
        return files_.directory(absolute);
    }

    std::pair<bool, std::string> read(std::string_view absolute, std::size_t limit) const {
        return files_.read(absolute, limit);
    }

    dockpipe::sdk::WindowsPath createReport(std::string_view absolute) const {
        return files_.createNew(absolute);
    }

    static void writeReport(HANDLE file, const std::string& contents) {
        dockpipe::sdk::WindowsFiles::writeAll(file, contents);
    }

private:
    dockpipe::sdk::WindowsFiles files_;
};

}
