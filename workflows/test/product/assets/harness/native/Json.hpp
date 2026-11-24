#pragma once

#include <string>
#include <string_view>

namespace product_harness {

// JSON permits Unicode but CLI captures are byte streams. Printable ASCII is
// literal; all other bytes use \u00XX so even invalid UTF-8 remains valid evidence.
// The report declares this byte-preserving representation explicitly.
inline std::string json(std::string_view text) {
    constexpr char digits[] = "0123456789abcdef";
    std::string result = "\"";
    for (unsigned char byte : text) {
        if (byte == '"' || byte == '\\') {
            result.push_back('\\');
            result.push_back(static_cast<char>(byte));
        } else if (byte >= 0x20 && byte < 0x7f) {
            result.push_back(static_cast<char>(byte));
        } else {
            result += "\\u00";
            result.push_back(digits[byte >> 4]);
            result.push_back(digits[byte & 15]);
        }
    }
    result.push_back('"');
    return result;
}

inline std::string json(bool value) {
    return value ? "true" : "false";
}
}
