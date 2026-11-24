#pragma once

#include "Model.hpp"
#include <chrono>

namespace product_harness {

class MonotonicClock final : public dockpipe_product::source::PipeLang::Testing::Async::TestClock {
public:
    pipelang::model::Result<std::int64_t, std::string> NowMilliseconds() override {
        const auto elapsed = std::chrono::steady_clock::now().time_since_epoch();
        const auto milliseconds = std::chrono::duration_cast<std::chrono::milliseconds>(elapsed).count();
        return pipelang::model::Result<std::int64_t, std::string>::Success(milliseconds);
    }
};
}
