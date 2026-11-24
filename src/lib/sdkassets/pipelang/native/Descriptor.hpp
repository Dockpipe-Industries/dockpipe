// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include <fcntl.h>
#include <stdexcept>
#include <unistd.h>
#include <utility>

namespace dockpipe::sdk {

class Descriptor final {
public:
    explicit Descriptor(int value = -1) : value_(value) {
        // Owned handles must survive child stdio redirection even when the host
        // started with a closed standard descriptor. Retain their identity in
        // the nonstandard range before any spawn actions can overwrite it.
        if (value_ >= 0 && value_ <= STDERR_FILENO) {
            const int retained = ::fcntl(value_, F_DUPFD_CLOEXEC, STDERR_FILENO + 1);
            ::close(value_);
            value_ = retained;
            if (value_ < 0)
                throw std::runtime_error("cannot retain owned descriptor above standard streams");
        }
    }
    ~Descriptor() { reset(); }
    Descriptor(const Descriptor&) = delete;
    Descriptor& operator=(const Descriptor&) = delete;
    Descriptor(Descriptor&& other) noexcept : value_(std::exchange(other.value_, -1)) {}
    Descriptor& operator=(Descriptor&& other) noexcept {
        if (this != &other) {
            reset();
            value_ = std::exchange(other.value_, -1);
        }
        return *this;
    }
    int get() const noexcept { return value_; }
    explicit operator bool() const noexcept { return value_ >= 0; }
    void reset() noexcept {
        if (value_ >= 0)
            ::close(value_);
        value_ = -1;
    }
private:
    int value_;
};

}
