#pragma once

#include <QDir>
#include <QString>

#if defined(Q_OS_WIN)
#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <windows.h>
#endif

// Holds an additional native module reference while Qt owns plugin objects.
// Call only after the host has admitted the plugin path and ABI metadata.
class LauncherNativeLibraryScope final {
public:
    LauncherNativeLibraryScope() = default;
    LauncherNativeLibraryScope(const LauncherNativeLibraryScope &) = delete;
    LauncherNativeLibraryScope &operator=(const LauncherNativeLibraryScope &) = delete;

    ~LauncherNativeLibraryScope()
    {
#if defined(Q_OS_WIN)
        if (m_module)
            FreeLibrary(m_module);
#endif
    }

    bool load(const QString &absoluteModulePath, QString &error)
    {
        error.clear();
#if defined(Q_OS_WIN)
        if (m_module) {
            error = QStringLiteral("Native library scope is already loaded.");
            return false;
        }
        const QString nativePath = QDir::toNativeSeparators(absoluteModulePath);
        m_module = LoadLibraryExW(reinterpret_cast<LPCWSTR>(nativePath.utf16()), nullptr,
                                 LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR | LOAD_LIBRARY_SEARCH_DEFAULT_DIRS);
        if (!m_module) {
            const DWORD code = GetLastError();
            wchar_t message[1024] = {};
            const DWORD length = FormatMessageW(FORMAT_MESSAGE_FROM_SYSTEM | FORMAT_MESSAGE_IGNORE_INSERTS,
                                                 nullptr, code, 0, message, 1024, nullptr);
            error = QStringLiteral("Native module load failed (%1): %2")
                        .arg(code).arg(QString::fromWCharArray(message, int(length)).trimmed());
            return false;
        }
#else
        Q_UNUSED(absoluteModulePath);
#endif
        return true;
    }

private:
#if defined(Q_OS_WIN)
    HMODULE m_module = nullptr;
#endif
};
