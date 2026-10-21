#pragma once

#include <windows.h>
#include <string>

namespace WindowsTestDesktop {
inline DWORD objectName(HANDLE object, std::wstring &name)
{
    DWORD bytes = 0;
    GetUserObjectInformationW(object, UOI_NAME, nullptr, 0, &bytes);
    // Retain the existing test launcher's 4 KiB object-name diagnostic bound.
    if (bytes == 0 || bytes > 4096 || bytes % sizeof(wchar_t) != 0)
        return ERROR_INVALID_DATA;
    name.resize(bytes / sizeof(wchar_t));
    if (!GetUserObjectInformationW(object, UOI_NAME, name.data(), bytes, &bytes))
        return GetLastError();
    if (name.empty() || name.back() != L'\0')
        return ERROR_INVALID_DATA;
    name.pop_back();
    return name.empty() ? ERROR_INVALID_DATA : ERROR_SUCCESS;
}

inline DWORD currentName(std::wstring &name)
{
    std::wstring station;
    std::wstring desktop;
    DWORD result = objectName(GetProcessWindowStation(), station);
    if (result != ERROR_SUCCESS)
        return result;
    result = objectName(GetThreadDesktop(GetCurrentThreadId()), desktop);
    if (result != ERROR_SUCCESS)
        return result;
    name = station + L"\\" + desktop;
    return ERROR_SUCCESS;
}
}
