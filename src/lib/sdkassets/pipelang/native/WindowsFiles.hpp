// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <windows.h>

#include <cstddef>
#include <string>
#include <string_view>
#include <utility>
#include <vector>

namespace dockpipe::sdk {

class WindowsHandle final {
public:
    explicit WindowsHandle(HANDLE value = nullptr) noexcept : value_(value) {}
    ~WindowsHandle() { reset(); }
    WindowsHandle(const WindowsHandle&) = delete;
    WindowsHandle& operator=(const WindowsHandle&) = delete;
    WindowsHandle(WindowsHandle&& other) noexcept : value_(other.release()) {}
    WindowsHandle& operator=(WindowsHandle&& other) noexcept {
        if (this != &other) {
            reset();
            value_ = other.release();
        }
        return *this;
    }

    HANDLE get() const noexcept { return value_; }
    explicit operator bool() const noexcept {
        return value_ != nullptr && value_ != INVALID_HANDLE_VALUE;
    }
    HANDLE release() noexcept { return std::exchange(value_, nullptr); }
    void reset(HANDLE value = nullptr) noexcept {
        if (*this)
            ::CloseHandle(value_);
        value_ = value;
    }

private:
    HANDLE value_;
};

// A selected path retains its ancestors without delete sharing. Executable and
// observed file handles also deny write sharing. Canonical drive paths and exact
// OS volume-GUID paths are admitted; other device/UNC paths, reparse points,
// alternate streams and ambiguous Win32 components are refused.
struct WindowsPath final {
    WindowsHandle handle;
    std::vector<WindowsHandle> ancestors;
    std::wstring absolute;
    std::wstring nativePath;
    std::string identity;

    HANDLE get() const noexcept { return handle.get(); }
};

std::wstring windowsWide(std::string_view value);
std::string windowsUtf8(std::wstring_view value);
std::wstring windowsAbsolute(std::string_view value);
bool sameWindowsPath(std::wstring_view left, std::wstring_view right);
WindowsPath openWindowsPath(std::string_view path, bool directory);
std::wstring quoteWindowsArgument(std::wstring_view value);

class WindowsFiles final {
public:
    // Once admitted, the original root spelling is only a matching label. Child
    // opens use the retained root handle and expose its stable volume-GUID path.
    explicit WindowsFiles(std::string root);
    WindowsPath directory(std::string_view absolute) const;
    std::pair<bool, std::string> read(std::string_view absolute, std::size_t limit) const;
    WindowsPath createNew(std::string_view absolute) const;
    static void writeAll(HANDLE file, const std::string& contents);

private:
    std::wstring confined(std::string_view absolute) const;
    WindowsPath root_;
};

}
