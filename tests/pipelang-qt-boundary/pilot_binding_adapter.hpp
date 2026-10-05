#pragma once
#include "boundary.hpp"
#include "qt_binding.hpp"

namespace pipelang_qt {
// This binding adapter knows the admitted pilot representation. Delivery does not.
inline Outcome fromArithmetic(const pipelang::Result<std::int64_t>& result) {
    if (result.ok) {
        if (result.error != pipelang::Error::none) return AdapterFailure{AdapterCode::HostFailure};
        return Integer{result.value};
    }
    if (result.value != 0) return AdapterFailure{AdapterCode::HostFailure};
    if (result.error == pipelang::Error::overflow) return DomainFailure{DomainCode::ArithmeticOverflow};
    return AdapterFailure{AdapterCode::UnsupportedError};
}
} // namespace pipelang_qt
