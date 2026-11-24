// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#pragma once

#include <pipelang/model/result.hpp>

#include <cstddef>
#include <map>
#include <string>
#include <string_view>

namespace dockpipe::sdk {

using ScopeFields = std::map<std::string, std::string>;

// The public scope service returns one flat JSON object of string fields.
// This decoder admits that contract only, with an explicit transport byte bound.
pipelang::model::Result<ScopeFields, std::string> decodeScopeObject(
    std::string_view input, std::size_t maximumBytes);

}
