// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include <cstddef>

#include "Descriptor.hpp"
#include <string>
#include <string_view>
#include <utility>

namespace dockpipe::sdk {

// All components, including the selected root's ancestors, are opened without
// following symlinks. Writes create new files exclusively; nothing is overwritten.
class ConfinedFiles final {
public:
    explicit ConfinedFiles(std::string root);
    const std::string& root() const noexcept { return root_; }
    std::string relative(std::string_view absolute) const;
    Descriptor directory(std::string_view absolute) const;
    std::pair<bool, std::string> read(std::string_view absolute, std::size_t limit) const;
    Descriptor createNew(std::string_view absolute) const;
    static void writeAll(int descriptor, const std::string& contents);
    static Descriptor openAbsolute(std::string_view absolute, bool directory);
private:
    std::pair<Descriptor, std::string> parent(std::string_view absolute) const;
    std::string root_;
    Descriptor rootDescriptor_;
};

}
