// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "ConfinedFiles.hpp"

#include <cerrno>
#include <fcntl.h>
#include <stdexcept>
#include <sys/stat.h>
#include <vector>

namespace dockpipe::sdk {
namespace {
std::vector<std::string> components(std::string_view path) {
    if (path.empty() || path.front() != '/' || path.size() > 4096 || path.find('\0') != path.npos)
        throw std::runtime_error("path must be a bounded absolute path");
    std::vector<std::string> result;
    std::size_t start = 1;
    while (start < path.size()) {
        const auto end = path.find('/', start);
        const auto part = path.substr(start, end == path.npos ? path.size() - start : end - start);
        if (part.empty() || part == "." || part == "..")
            throw std::runtime_error("path contains a noncanonical component");
        result.emplace_back(part);
        if (end == path.npos)
            break;
        start = end + 1;
        if (start == path.size())
            throw std::runtime_error("path has a trailing separator");
    }
    return result;
}

Descriptor descend(int directory, const std::string& name, bool isDirectory) {
    int flags = O_RDONLY | O_CLOEXEC | O_NOFOLLOW | O_NONBLOCK;
    if (isDirectory)
        flags |= O_DIRECTORY;
    return Descriptor(::openat(directory, name.c_str(), flags));
}
}

Descriptor ConfinedFiles::openAbsolute(std::string_view absolute, bool isDirectory) {
    const auto parts = components(absolute);
    Descriptor current(::open("/", O_RDONLY | O_CLOEXEC | O_DIRECTORY));
    if (!current)
        throw std::runtime_error("cannot open filesystem root");
    for (std::size_t index = 0; index < parts.size(); ++index) {
        auto next = descend(current.get(), parts[index], index + 1 < parts.size() || isDirectory);
        if (!next)
            throw std::runtime_error("cannot open selected path without following symlinks");
        current = std::move(next);
    }
    return current;
}

ConfinedFiles::ConfinedFiles(std::string root)
    : root_(std::move(root)), rootDescriptor_(openAbsolute(root_, true)) {
    if (root_ == "/")
        throw std::runtime_error("selected root cannot be the filesystem root");
}

std::string ConfinedFiles::relative(std::string_view absolute) const {
    (void)components(absolute);
    if (absolute == root_)
        return {};
    const std::string prefix = root_ + "/";
    if (!absolute.starts_with(prefix))
        throw std::runtime_error("path is outside the selected root");
    return std::string(absolute.substr(prefix.size()));
}

std::pair<Descriptor, std::string> ConfinedFiles::parent(std::string_view absolute) const {
    const auto suffix = relative(absolute);
    if (suffix.empty())
        throw std::runtime_error("a file path must name a selected-root child");
    const auto parts = components("/" + suffix);
    Descriptor current(::fcntl(rootDescriptor_.get(), F_DUPFD_CLOEXEC, 3));
    if (!current)
        throw std::runtime_error("cannot retain selected root descriptor");
    for (std::size_t index = 0; index + 1 < parts.size(); ++index) {
        auto next = descend(current.get(), parts[index], true);
        if (!next) {
            if (errno == ENOENT)
                return {Descriptor(), parts.back()};
            throw std::runtime_error("file ancestor is inaccessible or a symlink");
        }
        current = std::move(next);
    }
    return {std::move(current), parts.back()};
}

Descriptor ConfinedFiles::directory(std::string_view absolute) const {
    (void)relative(absolute);
    if (absolute == root_) {
        Descriptor result(::fcntl(rootDescriptor_.get(), F_DUPFD_CLOEXEC, 3));
        if (!result)
            throw std::runtime_error("cannot retain selected directory descriptor");
        return result;
    }
    auto [owner, name] = parent(absolute);
    if (!owner)
        throw std::runtime_error("selected directory does not exist");
    auto result = descend(owner.get(), name, true);
    if (!result)
        throw std::runtime_error("selected directory is inaccessible or a symlink");
    return result;
}

std::pair<bool, std::string> ConfinedFiles::read(std::string_view absolute, std::size_t limit) const {
    if (limit == 0)
        throw std::runtime_error("file observation limit must be positive");
    auto [owner, name] = parent(absolute);
    if (!owner)
        return {false, {}};
    auto file = descend(owner.get(), name, false);
    if (!file) {
        if (errno == ENOENT)
            return {false, {}};
        throw std::runtime_error("file is inaccessible or a symlink");
    }
    struct stat metadata {};
    if (::fstat(file.get(), &metadata) != 0 || !S_ISREG(metadata.st_mode))
        throw std::runtime_error("file observation requires a regular file");
    std::string content;
    char buffer[4096];
    while (true) {
        const auto count = ::read(file.get(), buffer, sizeof(buffer));
        if (count == 0)
            return {true, std::move(content)};
        if (count < 0) {
            if (errno == EINTR)
                continue;
            throw std::runtime_error("file read failed");
        }
        if (static_cast<std::size_t>(count) > limit - content.size())
            throw std::runtime_error("file exceeds requested byte limit");
        content.append(buffer, static_cast<std::size_t>(count));
    }
}

Descriptor ConfinedFiles::createNew(std::string_view absolute) const {
    auto [owner, name] = parent(absolute);
    if (!owner)
        throw std::runtime_error("output directory does not exist");
    Descriptor output(::openat(owner.get(), name.c_str(), O_WRONLY | O_CREAT | O_EXCL | O_CLOEXEC | O_NOFOLLOW, 0600));
    if (!output)
        throw std::runtime_error("output must be a new file in the selected root");
    return output;
}

void ConfinedFiles::writeAll(int descriptor, const std::string& contents) {
    std::size_t offset = 0;
    while (offset < contents.size()) {
        const auto count = ::write(descriptor, contents.data() + offset, contents.size() - offset);
        if (count < 0 && errno == EINTR)
            continue;
        if (count <= 0)
            throw std::runtime_error("file write failed; partial evidence retained");
        offset += static_cast<std::size_t>(count);
    }
    if (::fsync(descriptor) != 0)
        throw std::runtime_error("file sync failed; evidence retained");
}
}
