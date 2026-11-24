#pragma once

#include <unistd.h>
#include <utility>

namespace product_harness {

class Descriptor final {
public:
    explicit Descriptor(int value = -1) noexcept : value_(value) {}
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
    void reset(int value = -1) noexcept {
        if (value_ >= 0)
            ::close(value_);
        value_ = value;
    }
private:
    int value_;
};

}
