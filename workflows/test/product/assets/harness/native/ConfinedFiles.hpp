#pragma once

#ifdef _WIN32
#include "ConfinedFiles_windows.hpp"
#else
#include "Descriptor.hpp"
#include <string>
#include <string_view>
#include <utility>

namespace product_harness {

// All components, including the proof root's ancestors, are opened without
// following symlinks. No operation creates a directory or removes a marker.
class ConfinedFiles final {
public:
    explicit ConfinedFiles(std::string root);
    const std::string& root() const noexcept { return root_; }
    std::string relative(std::string_view absolute) const;
    Descriptor directory(std::string_view absolute) const;
    std::pair<bool, std::string> read(std::string_view absolute, std::size_t limit) const;
    Descriptor createReport(std::string_view absolute) const;
    static void writeReport(int descriptor, const std::string& contents);
    static Descriptor openAbsolute(std::string_view absolute, bool directory);
private:
    std::pair<Descriptor, std::string> parent(std::string_view absolute) const;
    std::string root_;
    Descriptor rootDescriptor_;
};

}

#endif
