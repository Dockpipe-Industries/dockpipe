#pragma once

#include <QString>
#include <limits>

namespace WorkspaceGraphTimestamp {
// libc++ can use a 128-bit file-clock count, which QString::number cannot take.
// Preserve every tick, including the most negative count, without narrowing.
template<typename Integer>
QString decimalCount(Integer count)
{
    static_assert(std::numeric_limits<Integer>::is_integer);
    QString text;
    text.reserve(std::numeric_limits<Integer>::digits10 + 2);
    const bool negative = count < 0;
    do {
        int digit = static_cast<int>(count % 10);
        if (digit < 0)
            digit = -digit;
        text.prepend(QChar::fromLatin1(static_cast<char>('0' + digit)));
        count /= 10;
    } while (count != 0);
    if (negative)
        text.prepend(QLatin1Char('-'));
    return text;
}
}
