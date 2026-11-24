// Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
#include "WindowsFiles.hpp"
#include <winternl.h>

#include <algorithm>
#include <array>
#include <limits>
#include <stdexcept>

namespace dockpipe::sdk {
namespace {
constexpr std::size_t maximumWindowsPath = 32767;
constexpr std::size_t volumeGuidRootCharacters = 49;

std::runtime_error windowsError(const char* operation) {
    const auto error = ::GetLastError();
    return std::runtime_error(std::string(operation) + ": Win32 " + std::to_string(error));
}

bool equalOrdinal(std::wstring_view left, std::wstring_view right) {
    return left.size() == right.size() &&
        ::CompareStringOrdinal(left.data(), static_cast<int>(left.size()), right.data(),
            static_cast<int>(right.size()), TRUE) == CSTR_EQUAL;
}

std::size_t absoluteRootCharacters(std::wstring_view path) {
    if (path.size() >= 3 &&
        ((path[0] >= L'A' && path[0] <= L'Z') || (path[0] >= L'a' && path[0] <= L'z')) &&
        path[1] == L':' && path[2] == L'\\')
        return 3;

    // Admit the OS volume-GUID spelling only. The remaining Win32 device and
    // network namespaces are not filesystem paths accepted by this profile.
    if (path.size() < volumeGuidRootCharacters || !equalOrdinal(path.substr(0, 11), L"\\\\?\\Volume{") ||
        path[47] != L'}' || path[48] != L'\\')
        throw std::runtime_error("Windows path must have a drive root or an exact volume-GUID root");

    for (std::size_t index = 0; index < 36; ++index) {
        const auto character = path[11 + index];
        if (index == 8 || index == 13 || index == 18 || index == 23) {
            if (character != L'-')
                throw std::runtime_error("Windows volume GUID has a noncanonical separator");
        } else if (!((character >= L'0' && character <= L'9') ||
            (character >= L'a' && character <= L'f') || (character >= L'A' && character <= L'F'))) {
            throw std::runtime_error("Windows volume GUID must contain exactly hexadecimal identity components");
        }
    }
    return volumeGuidRootCharacters;
}

void validateComponent(std::wstring_view component) {
    if (component.empty() || component == L"." || component == L".." ||
        component.back() == L'.' || component.back() == L' ')
        throw std::runtime_error("Windows path contains an ambiguous component");
    for (wchar_t value : component) {
        if (value < 32 || std::wstring_view(L"<>:\"|?*").find(value) != std::wstring_view::npos)
            throw std::runtime_error("Windows path contains a reserved character");
    }

    const auto stem = component.substr(0, component.find(L'.'));
    for (const auto reserved : {L"CON", L"PRN", L"AUX", L"NUL", L"CONIN$", L"CONOUT$"}) {
        if (equalOrdinal(stem, reserved))
            throw std::runtime_error("Windows path contains a reserved device name");
    }
    if (stem.size() == 4 && (equalOrdinal(stem.substr(0, 3), L"COM") ||
        equalOrdinal(stem.substr(0, 3), L"LPT"))) {
        const auto number = stem[3];
        if ((number >= L'0' && number <= L'9') || number == L'\u00b9' ||
            number == L'\u00b2' || number == L'\u00b3')
            throw std::runtime_error("Windows path contains a reserved numbered device name");
    }
}

std::string fileIdentity(HANDLE file) {
    FILE_ID_INFO identity{};
    FILE_STANDARD_INFO metadata{};
    if (!::GetFileInformationByHandleEx(file, FileIdInfo, &identity, sizeof(identity)) ||
        !::GetFileInformationByHandleEx(file, FileStandardInfo, &metadata, sizeof(metadata)))
        throw windowsError("cannot inspect opened Windows path identity");

    constexpr char hexadecimal[] = "0123456789abcdef";
    std::string result = "volume=" + std::to_string(identity.VolumeSerialNumber) + ";file=";
    for (unsigned char byte : identity.FileId.Identifier) {
        result.push_back(hexadecimal[byte >> 4]);
        result.push_back(hexadecimal[byte & 15]);
    }
    return result + ";bytes=" + std::to_string(metadata.EndOfFile.QuadPart);
}

WindowsHandle openComponent(HANDLE parent, std::wstring_view name, bool directory, bool create) {
    using NativeOpen = NTSTATUS (NTAPI*)(PHANDLE, ACCESS_MASK, POBJECT_ATTRIBUTES, PIO_STATUS_BLOCK,
        PLARGE_INTEGER, ULONG, ULONG, ULONG, ULONG, PVOID, ULONG);
    const auto nativeOpen = reinterpret_cast<NativeOpen>(
        ::GetProcAddress(::GetModuleHandleW(L"ntdll.dll"), "NtCreateFile"));
    if (!nativeOpen)
        throw std::runtime_error("handle-relative Windows file opening is unavailable");

    const DWORD access = directory ? FILE_LIST_DIRECTORY | FILE_READ_ATTRIBUTES :
        (create ? GENERIC_WRITE | FILE_READ_ATTRIBUTES : GENERIC_READ | FILE_READ_ATTRIBUTES);
    // Denying write/delete sharing on directory objects also prevents changing
    // an already inspected component into a junction before the next open.
    UNICODE_STRING relative{};
    relative.Length = static_cast<USHORT>(name.size() * sizeof(wchar_t));
    relative.MaximumLength = relative.Length;
    relative.Buffer = const_cast<wchar_t*>(name.data());
    OBJECT_ATTRIBUTES attributes{};
    attributes.Length = sizeof(attributes);
    attributes.RootDirectory = parent;
    attributes.ObjectName = &relative;
    attributes.Attributes = OBJ_CASE_INSENSITIVE;
    IO_STATUS_BLOCK status{};
    HANDLE opened = nullptr;
    constexpr ULONG synchronous = 0x00000020;
    constexpr ULONG openReparsePoint = 0x00200000;
    constexpr ULONG directoryFile = 0x00000001;
    constexpr ULONG nonDirectoryFile = 0x00000040;
    constexpr ULONG createNew = 2;
    constexpr ULONG openExisting = 1;
    const auto resultCode = nativeOpen(&opened, access | SYNCHRONIZE, &attributes, &status,
        nullptr, FILE_ATTRIBUTE_NORMAL, FILE_SHARE_READ, create ? createNew : openExisting,
        synchronous | openReparsePoint | (directory ? directoryFile : nonDirectoryFile), nullptr, 0);
    if (resultCode < 0) {
        using MapError = ULONG (WINAPI*)(NTSTATUS);
        const auto mapError = reinterpret_cast<MapError>(
            ::GetProcAddress(::GetModuleHandleW(L"ntdll.dll"), "RtlNtStatusToDosError"));
        ::SetLastError(mapError ? mapError(resultCode) : ERROR_GEN_FAILURE);
        return WindowsHandle{};
    }
    WindowsHandle result(opened);

    FILE_ATTRIBUTE_TAG_INFO tag{};
    FILE_STANDARD_INFO metadata{};
    if (!::GetFileInformationByHandleEx(result.get(), FileAttributeTagInfo, &tag, sizeof(tag)) ||
        !::GetFileInformationByHandleEx(result.get(), FileStandardInfo, &metadata, sizeof(metadata)))
        throw windowsError("cannot inspect opened Windows path type");
    if ((tag.FileAttributes & FILE_ATTRIBUTE_REPARSE_POINT) != 0 ||
        metadata.Directory != directory || ::GetFileType(result.get()) != FILE_TYPE_DISK)
        throw std::runtime_error("Windows path must have the selected disk type and no reparse points");
    return result;
}

std::wstring nativePath(HANDLE file) {
    std::wstring result(maximumWindowsPath, L'\0');
    const auto length = ::GetFinalPathNameByHandleW(file, result.data(), static_cast<DWORD>(result.size()),
        FILE_NAME_NORMALIZED | VOLUME_NAME_GUID);
    if (length == 0 || length >= result.size())
        throw windowsError("cannot retain opened Windows volume path");
    result.resize(length);
    auto normalized = windowsAbsolute(windowsUtf8(result));
    if (absoluteRootCharacters(normalized) != volumeGuidRootCharacters)
        throw std::runtime_error("opened Windows path did not resolve to a stable volume-GUID path");
    return normalized;
}

WindowsPath openCanonical(const std::wstring& absolute, bool directory, bool create, bool missingAllowed,
    HANDLE selectedRoot = nullptr, std::size_t firstComponent = 0) {
    WindowsPath result;
    result.absolute = absolute;
    const auto rootCharacters = absoluteRootCharacters(absolute);
    WindowsHandle ancestor;
    if (selectedRoot) {
        HANDLE copied = nullptr;
        if (!::DuplicateHandle(::GetCurrentProcess(), selectedRoot, ::GetCurrentProcess(), &copied,
            0, FALSE, DUPLICATE_SAME_ACCESS))
            throw windowsError("cannot retain selected Windows root handle");
        ancestor.reset(copied);
    } else {
        const auto volumeRoot = absolute.substr(0, rootCharacters);
        const auto rootPath = rootCharacters == 3 ? L"\\\\?\\" + volumeRoot : volumeRoot;
        ancestor.reset(::CreateFileW(rootPath.c_str(),
            FILE_LIST_DIRECTORY | FILE_READ_ATTRIBUTES | SYNCHRONIZE, FILE_SHARE_READ, nullptr,
            OPEN_EXISTING, FILE_FLAG_BACKUP_SEMANTICS | FILE_FLAG_OPEN_REPARSE_POINT, nullptr));
        firstComponent = rootCharacters;
    }
    if (!ancestor)
        throw windowsError("cannot open selected Windows volume root");
    FILE_ATTRIBUTE_TAG_INFO rootTag{};
    if (!::GetFileInformationByHandleEx(ancestor.get(), FileAttributeTagInfo, &rootTag, sizeof(rootTag)) ||
        (rootTag.FileAttributes & FILE_ATTRIBUTE_DIRECTORY) == 0 ||
        (rootTag.FileAttributes & FILE_ATTRIBUTE_REPARSE_POINT) != 0 ||
        ::GetFileType(ancestor.get()) != FILE_TYPE_DISK)
        throw std::runtime_error("selected Windows root must be a disk directory without reparse points");
    result.ancestors.push_back(std::move(ancestor));

    std::size_t offset = firstComponent;
    while (offset < absolute.size()) {
        const auto separator = absolute.find(L'\\', offset);
        const bool final = separator == std::wstring::npos;
        const auto component = std::wstring_view(absolute).substr(offset,
            final ? absolute.size() - offset : separator - offset);
        auto opened = openComponent(result.ancestors.back().get(), component,
            final ? directory : true, final && create);
        if (!opened) {
            const auto error = ::GetLastError();
            if (missingAllowed && (error == ERROR_FILE_NOT_FOUND || error == ERROR_PATH_NOT_FOUND))
                return result;
            ::SetLastError(error);
            throw windowsError("cannot open Windows path with retained ancestors");
        }
        if (final) {
            result.identity = fileIdentity(opened.get());
            result.nativePath = nativePath(opened.get());
            result.handle = std::move(opened);
            return result;
        }
        result.ancestors.push_back(std::move(opened));
        offset = separator + 1;
    }

    result.handle = std::move(result.ancestors.back());
    result.ancestors.pop_back();
    result.identity = fileIdentity(result.handle.get());
    result.nativePath = nativePath(result.handle.get());
    return result;
}
}

std::wstring windowsWide(std::string_view value) {
    if (value.find('\0') != value.npos || value.size() > static_cast<std::size_t>(std::numeric_limits<int>::max()))
        throw std::runtime_error("Windows text must be bounded UTF-8 without NUL bytes");
    if (value.empty())
        return {};
    const auto length = ::MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, value.data(),
        static_cast<int>(value.size()), nullptr, 0);
    if (length <= 0)
        throw windowsError("cannot decode Windows UTF-8 text");
    std::wstring result(static_cast<std::size_t>(length), L'\0');
    if (::MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, value.data(),
        static_cast<int>(value.size()), result.data(), length) != length)
        throw windowsError("cannot decode Windows UTF-8 text");
    return result;
}

std::string windowsUtf8(std::wstring_view value) {
    if (value.size() > static_cast<std::size_t>(std::numeric_limits<int>::max()))
        throw std::runtime_error("Windows text exceeds its conversion bound");
    if (value.empty())
        return {};
    const auto length = ::WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, value.data(),
        static_cast<int>(value.size()), nullptr, 0, nullptr, nullptr);
    if (length <= 0)
        throw windowsError("cannot encode Windows UTF-8 text");
    std::string result(static_cast<std::size_t>(length), '\0');
    if (::WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, value.data(),
        static_cast<int>(value.size()), result.data(), length, nullptr, nullptr) != length)
        throw windowsError("cannot encode Windows UTF-8 text");
    return result;
}

std::wstring windowsAbsolute(std::string_view value) {
    if (value.size() > maximumWindowsPath * 4)
        throw std::runtime_error("Windows path exceeds its UTF-8 conversion byte bound");
    auto result = windowsWide(value);
    std::replace(result.begin(), result.end(), L'/', L'\\');
    if (result.size() >= maximumWindowsPath)
        throw std::runtime_error("Windows path exceeds its canonical character bound");
    const auto rootCharacters = absoluteRootCharacters(result);
    if (result.size() > rootCharacters && result.back() == L'\\')
        throw std::runtime_error("Windows path has a trailing separator");

    std::size_t offset = rootCharacters;
    while (offset < result.size()) {
        const auto separator = result.find(L'\\', offset);
        validateComponent(std::wstring_view(result).substr(offset,
            separator == result.npos ? result.size() - offset : separator - offset));
        if (separator == result.npos)
            break;
        offset = separator + 1;
    }
    return result;
}

bool sameWindowsPath(std::wstring_view left, std::wstring_view right) {
    return equalOrdinal(left, right);
}

WindowsPath openWindowsPath(std::string_view path, bool directory) {
    return openCanonical(windowsAbsolute(path), directory, false, false);
}

std::wstring quoteWindowsArgument(std::wstring_view value) {
    if (value.find(L'\0') != value.npos)
        throw std::runtime_error("Windows arguments cannot contain NUL characters");
    std::wstring result = L"\"";
    std::size_t backslashes = 0;
    for (wchar_t character : value) {
        if (character == L'\\') {
            ++backslashes;
            continue;
        }
        result.append(backslashes * (character == L'\"' ? 2 : 1), L'\\');
        backslashes = 0;
        if (character == L'\"')
            result.push_back(L'\\');
        result.push_back(character);
    }
    result.append(backslashes * 2, L'\\');
    result.push_back(L'\"');
    return result;
}

WindowsFiles::WindowsFiles(std::string root) : root_(openWindowsPath(root, true)) {
    if (root_.absolute.size() == absoluteRootCharacters(root_.absolute))
        throw std::runtime_error("selected Windows root cannot be a volume root");
}

std::wstring WindowsFiles::confined(std::string_view absolute) const {
    const auto path = windowsAbsolute(absolute);
    for (const std::wstring_view spelling : {std::wstring_view(root_.nativePath), std::wstring_view(root_.absolute)}) {
        if (sameWindowsPath(path, spelling))
            return root_.nativePath;
        if (path.size() <= spelling.size() || path[spelling.size()] != L'\\' ||
            !sameWindowsPath(std::wstring_view(path).substr(0, spelling.size()), spelling))
            continue;

        const auto suffix = std::wstring_view(path).substr(spelling.size());
        if (suffix.size() >= maximumWindowsPath - root_.nativePath.size())
            throw std::runtime_error("confined Windows path exceeds its stable volume-path character bound");
        return root_.nativePath + std::wstring(suffix);
    }
    throw std::runtime_error("Windows path is outside the selected root");
}

WindowsPath WindowsFiles::directory(std::string_view absolute) const {
    const auto path = confined(absolute);
    return openCanonical(path, true, false, false, root_.handle.get(),
        sameWindowsPath(path, root_.nativePath) ? path.size() : root_.nativePath.size() + 1);
}

std::pair<bool, std::string> WindowsFiles::read(std::string_view absolute, std::size_t limit) const {
    if (limit == 0)
        throw std::runtime_error("Windows file observation limit must be positive");
    const auto path = confined(absolute);
    if (sameWindowsPath(path, root_.nativePath))
        throw std::runtime_error("Windows file observation must name a selected-root child");
    auto selected = openCanonical(path, false, false, true, root_.handle.get(), root_.nativePath.size() + 1);
    if (!selected.handle)
        return {false, {}};

    std::string contents;
    std::array<char, 4096> buffer{};
    while (true) {
        DWORD count = 0;
        if (!::ReadFile(selected.handle.get(), buffer.data(), static_cast<DWORD>(buffer.size()), &count, nullptr))
            throw windowsError("cannot read selected Windows file");
        if (count == 0)
            return {true, std::move(contents)};
        if (count > limit - contents.size())
            throw std::runtime_error("Windows file exceeds its observation byte limit");
        contents.append(buffer.data(), count);
    }
}

WindowsPath WindowsFiles::createNew(std::string_view absolute) const {
    const auto path = confined(absolute);
    if (sameWindowsPath(path, root_.nativePath))
        throw std::runtime_error("Windows output must name a selected-root child");
    return openCanonical(path, false, true, false, root_.handle.get(), root_.nativePath.size() + 1);
}

void WindowsFiles::writeAll(HANDLE file, const std::string& contents) {
    std::size_t offset = 0;
    while (offset < contents.size()) {
        DWORD written = 0;
        const auto count = static_cast<DWORD>(std::min<std::size_t>(contents.size() - offset,
            std::numeric_limits<DWORD>::max()));
        if (!::WriteFile(file, contents.data() + offset, count, &written, nullptr) || written == 0)
            throw windowsError("Windows file write failed; partial evidence retained");
        offset += written;
    }
    if (!::FlushFileBuffers(file))
        throw windowsError("Windows file flush failed; evidence retained");
}

}
