#include "DesktopAppWindow.h"

#ifdef Q_OS_WIN

#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <windows.h>

#include <QCoreApplication>
#include <QCloseEvent>
#include <QDebug>
#include <QDialog>
#include <QGuiApplication>
#include <QHash>
#include <QLabel>
#include <QPainter>
#include <QPixmap>
#include <QPointer>
#include <QPushButton>
#include <QTimer>
#include <QVBoxLayout>
#include <QWidget>
#include <QWindow>
#include <functional>

namespace {
HWND windowHandle(quint64 id)
{
    return reinterpret_cast<HWND>(static_cast<quintptr>(id));
}

QString nativeError(const QString &action)
{
    return QObject::tr("%1 (Windows error %2). This window may refuse embedding.")
        .arg(action).arg(GetLastError());
}

class Handle final {
public:
    explicit Handle(HANDLE value = nullptr) : value(value) {}
    ~Handle()
    {
        if (value)
            CloseHandle(value);
    }
    Handle(const Handle &) = delete;
    Handle &operator=(const Handle &) = delete;
    HANDLE value;
};

class WindowRecoveryDialog final : public QDialog {
public:
    explicit WindowRecoveryDialog(std::function<bool()> release)
        : m_release(std::move(release))
    {
        setAttribute(Qt::WA_DeleteOnClose);
        setWindowTitle(QObject::tr("Release external application"));
    }
    void reject() override
    {
        if (m_release())
            QDialog::reject();
    }
protected:
    void closeEvent(QCloseEvent *event) override
    {
        if (m_release())
            QDialog::closeEvent(event);
        else
            event->ignore();
    }
private:
    std::function<bool()> m_release;
};

struct CurrentDesktopIdentity {
    DWORD session = 0;
    alignas(TOKEN_USER) BYTE user[sizeof(TOKEN_USER) + SECURITY_MAX_SID_SIZE]{};
    bool valid = false;

    CurrentDesktopIdentity()
    {
        Handle token;
        DWORD length = 0;
        valid = ProcessIdToSessionId(GetCurrentProcessId(), &session)
            && OpenProcessToken(GetCurrentProcess(), TOKEN_QUERY, &token.value)
            && GetTokenInformation(token.value, TokenUser, user, sizeof(user), &length);
    }

    PSID sid() const
    {
        return reinterpret_cast<const TOKEN_USER *>(user)->User.Sid;
    }
};

bool sameUserAndUnelevated(HANDLE process, PSID ownUser)
{
    Handle targetToken;
    if (!OpenProcessToken(process, TOKEN_QUERY, &targetToken.value))
        return false;
    TOKEN_ELEVATION elevation{};
    DWORD length = 0;
    if (!GetTokenInformation(targetToken.value, TokenElevation, &elevation, sizeof(elevation), &length)
        || elevation.TokenIsElevated)
        return false;
    DWORD appContainer = 0;
    if (!GetTokenInformation(targetToken.value, TokenIsAppContainer, &appContainer, sizeof(appContainer), &length)
        || appContainer)
        return false;
    // TOKEN_USER consists of one SID pointer and one maximum-sized SID. Fixed
    // storage avoids allocations based on another process's token metadata.
    alignas(TOKEN_USER) BYTE targetUser[sizeof(TOKEN_USER) + SECURITY_MAX_SID_SIZE]{};
    if (!GetTokenInformation(targetToken.value, TokenUser, targetUser, sizeof(targetUser), &length))
        return false;
    return EqualSid(reinterpret_cast<TOKEN_USER *>(targetUser)->User.Sid, ownUser);
}

bool processIdentity(HWND window, DWORD &pid, FILETIME &created, const CurrentDesktopIdentity &own)
{
    pid = 0;
    if (!own.valid || !IsWindow(window) || !GetWindowThreadProcessId(window, &pid)
        || !pid || pid == GetCurrentProcessId())
        return false;
    DWORD targetSession = 0;
    if (!ProcessIdToSessionId(pid, &targetSession) || targetSession != own.session)
        return false;
    Handle process(OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid));
    if (!process.value || !sameUserAndUnelevated(process.value, own.sid()))
        return false;
    FILETIME exited{}, kernel{}, user{};
    return GetProcessTimes(process.value, &created, &exited, &kernel, &user);
}

bool processIdentity(HWND window, DWORD &pid, FILETIME &created)
{
    const CurrentDesktopIdentity own;
    return processIdentity(window, pid, created, own);
}

bool ordinaryDesktopWindow(HWND window)
{
    const LONG_PTR style = GetWindowLongPtrW(window, GWL_STYLE);
    const LONG_PTR extendedStyle = GetWindowLongPtrW(window, GWL_EXSTYLE);
    if ((style & WS_CHILD) || (extendedStyle & WS_EX_TOOLWINDOW))
        return false;
    const HWND owner = GetWindow(window, GW_OWNER);
    if (!owner)
        return true;
    // Some apps give their taskbar window a hidden infrastructure owner. This
    // differs from a dialog owned by a visible application window. Never detach
    // a dialog, a tool window, or an owner belonging to another process.
    DWORD pid = 0;
    DWORD ownerPid = 0;
    return (extendedStyle & WS_EX_APPWINDOW) && !IsWindowVisible(owner)
        && GetWindowThreadProcessId(window, &pid)
        && GetWindowThreadProcessId(owner, &ownerPid) && pid == ownerPid;
}

bool compatibleDpi(HWND window)
{
    // Resolve explicitly so the executable still starts on an unsupported
    // Windows version and can explain why embedding is unavailable there.
    const HMODULE user32 = GetModuleHandleW(L"user32.dll");
    using WindowContext = HANDLE(WINAPI *)(HWND);
    using ThreadContext = HANDLE(WINAPI *)();
    using ContextsEqual = BOOL(WINAPI *)(HANDLE, HANDLE);
    const auto windowContext = reinterpret_cast<WindowContext>(GetProcAddress(user32, "GetWindowDpiAwarenessContext"));
    const auto threadContext = reinterpret_cast<ThreadContext>(GetProcAddress(user32, "GetThreadDpiAwarenessContext"));
    const auto equal = reinterpret_cast<ContextsEqual>(GetProcAddress(user32, "AreDpiAwarenessContextsEqual"));
    return windowContext && threadContext && equal && equal(windowContext(window), threadContext());
}

bool responsive(HWND window)
{
    DWORD_PTR result = 0;
    // The preflight cannot guarantee a foreign app remains responsive, but it
    // prevents knowingly embedding an already-hung window. No input is sent.
    return SendMessageTimeoutW(window, WM_NULL, 0, 0,
        SMTO_ABORTIFHUNG | SMTO_BLOCK, 200, &result) != 0;
}

bool setStyle(HWND window, int index, LONG_PTR value)
{
    SetLastError(ERROR_SUCCESS);
    return SetWindowLongPtrW(window, index, value) != 0 || GetLastError() == ERROR_SUCCESS;
}

bool setParent(HWND window, HWND parent)
{
    SetLastError(ERROR_SUCCESS);
    return SetParent(window, parent) != nullptr || GetLastError() == ERROR_SUCCESS;
}

struct ProcessSnapshot {
    FILETIME created{};
    bool permitted = false;
};

struct WindowList {
    CurrentDesktopIdentity own;
    quint64 onlyWindowId = 0;
    // Enumeration admits at most256 ordinary windows, hence at most256 PIDs.
    // No cached identity survives this one snapshot; acquisition revalidates it.
    QHash<DWORD, ProcessSnapshot> processes;
    QVector<DesktopAppChoice> choices;
    int inspected = 0;
    bool exhausted = false;
};

BOOL CALLBACK collectWindow(HWND window, LPARAM data)
{
    auto &list = *reinterpret_cast<WindowList *>(data);
    if (list.onlyWindowId
        && list.onlyWindowId != static_cast<quint64>(reinterpret_cast<quintptr>(window)))
        return TRUE;
    if (!IsWindowVisible(window) || !ordinaryDesktopWindow(window))
        return TRUE;
    // Minimized taskbar windows remain visible desktop applications. Selection
    // restores them inside the host, and release restores their original state.
    // Hidden Windows infrastructure HWNDs do not occupy user picker entries.
    if (++list.inspected > 256) {
        list.exhausted = true;
        return FALSE;
    }
    DWORD pid = 0;
    GetWindowThreadProcessId(window, &pid);
    if (!pid || pid == GetCurrentProcessId())
        return TRUE;
    auto process = list.processes.constFind(pid);
    if (process == list.processes.cend()) {
        ProcessSnapshot snapshot;
        DWORD verifiedPid = 0;
        snapshot.permitted = processIdentity(window, verifiedPid, snapshot.created, list.own)
            && verifiedPid == pid;
        list.processes.insert(pid, snapshot);
        process = list.processes.constFind(pid);
    }
    if (!process->permitted)
        return TRUE;
    const int titleLength = GetWindowTextLengthW(window);
    if (titleLength <= 0 || titleLength > 512)
        return TRUE;
    WCHAR title[513]{};
    WCHAR className[257]{};
    if (!GetWindowTextW(window, title, 513) || !GetClassNameW(window, className, 257))
        return TRUE;
    list.choices.append({static_cast<quint64>(reinterpret_cast<quintptr>(window)),
        QString::fromWCharArray(title).simplified(), QString::fromWCharArray(className)});
    return TRUE;
}
}

struct DesktopAppWindow::Native {
    HWND window = nullptr;
    DWORD pid = 0;
    FILETIME created{};
    HWND originalParent = nullptr;
    HWND originalOwner = nullptr;
    LONG_PTR originalStyle = 0;
    LONG_PTR originalExtendedStyle = 0;
    WINDOWPLACEMENT originalPlacement{sizeof(WINDOWPLACEMENT)};
    RECT originalRectangle{};
    QPointer<QWindow> foreign;
    QPointer<QWidget> container;
    bool saved = false;
    bool changed = false;
    bool recoveryHosted = false;
};

QString DesktopAppWindow::unavailableReason()
{
    if (QGuiApplication::platformName() != "windows")
        return QObject::tr("Native Windows embedding requires the Windows desktop platform.");
    if (!GetProcAddress(GetModuleHandleW(L"user32.dll"), "GetWindowDpiAwarenessContext"))
        return QObject::tr("Native Windows embedding requires Windows 10 or newer with DPI-awareness support.");
    return {};
}

QVector<DesktopAppChoice> DesktopAppWindow::availableWindows(QString &error, quint64 onlyWindowId)
{
    error = unavailableReason();
    if (!error.isEmpty())
        return {};
    WindowList list;
    list.onlyWindowId = onlyWindowId;
    if (!list.own.valid) {
        error = QObject::tr("Cannot verify the current Windows user and desktop session.");
        return {};
    }
    if (!EnumWindows(collectWindow, reinterpret_cast<LPARAM>(&list))) {
        error = list.exhausted
            ? QObject::tr("The desktop window list exceeds the 256-window picker limit.")
            : nativeError(QObject::tr("Cannot list desktop windows"));
        return {};
    }
    if (list.choices.isEmpty())
        error = QObject::tr("No supported windows. Select an ordinary, non-elevated desktop application owned by your user.");
    return list.choices;
}

DesktopAppWindow::DesktopAppWindow(const DesktopAppChoice &choice)
    : m_native(std::make_unique<Native>()), m_choice(choice)
{
    m_native->window = windowHandle(choice.id);
    if (!processIdentity(m_native->window, m_native->pid, m_native->created))
        m_native->pid = 0;
}

DesktopAppWindow::~DesktopAppWindow()
{
    releaseControl();
}

bool DesktopAppWindow::alive() const
{
    DWORD pid = 0;
    FILETIME created{};
    return m_native->pid && processIdentity(m_native->window, pid, created)
        && pid == m_native->pid && CompareFileTime(&created, &m_native->created) == 0;
}

bool DesktopAppWindow::matchesProcess(qint64 expectedPid, const QString &startToken) const
{
    // Public process telemetry currently has no Windows collector. Do not
    // invent an identity token that packages cannot obtain through that API.
    Q_UNUSED(expectedPid);
    Q_UNUSED(startToken);
    return false;
}

std::optional<DesktopAppChoice> DesktopAppWindow::replacement(QString &error) const
{
    error.clear();
    if (!alive())
        error = QObject::tr("The selected window closed. Select its replacement manually.");
    return {};
}

bool DesktopAppWindow::prepareControl(QString &error)
{
    error.clear();
    if (m_native->saved)
        return true;
    if (!alive()) {
        error = QObject::tr("The window closed or cannot be accessed. Elevated, sandboxed, protected, and other-user processes are not supported.");
        return false;
    }
    if (!ordinaryDesktopWindow(m_native->window)) {
        error = QObject::tr("This is no longer an independent desktop window. Release it from its current host first.");
        return false;
    }
    if (!compatibleDpi(m_native->window)) {
        error = QObject::tr("This application uses a different DPI-awareness mode. Embedding it could change its process settings, so it was left on the desktop.");
        return false;
    }
    if (!responsive(m_native->window)) {
        error = QObject::tr("This application is not responding. Its window was left on the desktop.");
        return false;
    }
    // Every admitted HWND is top-level. GetParent would conflate its owner
    // with a child parent, so preserve those relationships separately.
    m_native->originalParent = nullptr;
    m_native->originalOwner = GetWindow(m_native->window, GW_OWNER);
    m_native->originalStyle = GetWindowLongPtrW(m_native->window, GWL_STYLE);
    m_native->originalExtendedStyle = GetWindowLongPtrW(m_native->window, GWL_EXSTYLE);
    if (!GetWindowRect(m_native->window, &m_native->originalRectangle)
        || !GetWindowPlacement(m_native->window, &m_native->originalPlacement)) {
        error = nativeError(QObject::tr("Cannot preserve the window placement"));
        return false;
    }
    m_native->saved = true;
    return true;
}

bool DesktopAppWindow::controlReady() const
{
    return m_native->saved && alive();
}

QWidget *DesktopAppWindow::takeControl(QWidget *parent, QString &error)
{
    error.clear();
    if (m_native->container)
        return m_native->container;
    if (!controlReady()) {
        error = QObject::tr("The selected Windows application is no longer available.");
        return nullptr;
    }
    if (!ordinaryDesktopWindow(m_native->window)
        || GetWindow(m_native->window, GW_OWNER) != m_native->originalOwner) {
        m_native->saved = false;
        error = QObject::tr("The window's ownership changed before attachment. Select it again.");
        return nullptr;
    }
    // From this point a failed operation may have changed native state. Merely
    // preparing or cancelling selection must never overwrite a desktop change.
    m_native->changed = true;
    if (m_native->originalOwner
        && !setStyle(m_native->window, GWLP_HWNDPARENT, 0)) {
        error = nativeError(QObject::tr("Cannot preserve the application window owner"));
        releaseControl();
        return nullptr;
    }
    // Qt documents fromWinId specifically for foreign-window reparenting. Keep
    // all original native state because the wrapper may rewrite window styles.
    m_native->foreign = QWindow::fromWinId(static_cast<WId>(m_choice.id));
    if (!m_native->foreign) {
        error = QObject::tr("Qt cannot wrap this Windows application.");
        releaseControl();
        return nullptr;
    }
    m_native->container = QWidget::createWindowContainer(m_native->foreign, parent);
    m_native->container->setFocusPolicy(Qt::StrongFocus);
    m_native->container->setObjectName("billboardNativeApp");
    // Microsoft requires WS_CHILD and no WS_POPUP before changing a desktop
    // window's parent. Do not change DPI awareness or bypass UIPI restrictions.
    const LONG_PTR childStyle = (m_native->originalStyle | WS_CHILD)
        & ~(WS_POPUP | WS_CAPTION | WS_THICKFRAME | WS_MINIMIZE | WS_MAXIMIZE);
    const HWND host = reinterpret_cast<HWND>(m_native->container->winId());
    if (!setStyle(m_native->window, GWL_STYLE, childStyle)
        || !setParent(m_native->window, host)) {
        error = nativeError(QObject::tr("Windows refused to embed this window"));
        releaseControl();
        return nullptr;
    }
    SetWindowPos(m_native->window, nullptr, 0, 0, 0, 0,
        SWP_NOMOVE | SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE | SWP_FRAMECHANGED | SWP_ASYNCWINDOWPOS);
    ShowWindowAsync(m_native->window, SW_SHOWNOACTIVATE);
    return m_native->container;
}

bool DesktopAppWindow::controlAttached() const
{
    if (!m_native->saved || !m_native->container || !m_native->foreign || !alive())
        return false;
    const HWND host = reinterpret_cast<HWND>(m_native->container->window()->winId());
    return IsChild(host, m_native->window) && IsWindowVisible(m_native->window);
}

bool DesktopAppWindow::syncControlGeometry()
{
    if (!controlAttached())
        return false;
    const HWND parent = GetParent(m_native->window);
    const HWND host = reinterpret_cast<HWND>(m_native->container->winId());
    RECT rectangle{};
    if (!GetClientRect(host, &rectangle))
        return false;
    MapWindowPoints(host, parent, reinterpret_cast<POINT *>(&rectangle), 2);
    RECT current{};
    if (!GetWindowRect(m_native->window, &current))
        return false;
    MapWindowPoints(nullptr, parent, reinterpret_cast<POINT *>(&current), 2);
    if (EqualRect(&current, &rectangle))
        return true;
    if (IsHungAppWindow(m_native->window))
        return false;
    return SetWindowPos(m_native->window, nullptr, rectangle.left, rectangle.top,
        qMax(1L, rectangle.right - rectangle.left), qMax(1L, rectangle.bottom - rectangle.top),
        SWP_NOZORDER | SWP_NOACTIVATE | SWP_ASYNCWINDOWPOS);
}

bool DesktopAppWindow::requestClose(QString &error)
{
    error.clear();
    if (!alive()) {
        error = QObject::tr("The desktop window is no longer available.");
        return false;
    }
    // Let the app handle unsaved work and cancellation. Never kill its process.
    if (!PostMessageW(m_native->window, WM_CLOSE, 0, 0)) {
        error = QObject::tr("Windows could not send a close request to this app.");
        return false;
    }
    return true;
}

bool DesktopAppWindow::focusControl()
{
    if (!controlAttached())
        return false;
    // Normal Qt container focus only. No AttachThreadInput, synthetic input or
    // foreground-window lock bypass. Some applications require a direct click.
    m_native->container->setFocus(Qt::OtherFocusReason);
    return true;
}

void DesktopAppWindow::releaseControl()
{
    if (!m_native || !m_native->saved)
        return;
    if (!m_native->changed) {
        m_native->saved = false;
        return;
    }
    const auto retainRecoveryHost = [this] {
        if (m_native->recoveryHosted)
            return;
        // Do not destroy a native host while it still owns the external
        // HWND. Keep an explicit recovery window until release succeeds or
        // the external app closes. Retries require the user's button click.
        m_native->recoveryHosted = true;
        auto recovery = std::make_shared<DesktopAppWindow>(m_choice);
        recovery->m_native = std::move(m_native);
        m_native = std::make_unique<Native>();
        auto *dialog = new WindowRecoveryDialog([recovery] {
            recovery->releaseControl();
            return !recovery->m_native->saved;
        });
        auto *layout = new QVBoxLayout(dialog);
        auto *notice = new QLabel(QObject::tr("Windows could not fully restore this application to the desktop. Release recovery remains open to protect it. Retry release, or close the application itself."), dialog);
        notice->setWordWrap(true);
        layout->addWidget(notice);
        if (recovery->m_native->container)
            layout->addWidget(recovery->m_native->container);
        auto *retry = new QPushButton(QObject::tr("Retry release"), dialog);
        layout->addWidget(retry);
        QObject::connect(retry, &QPushButton::clicked, dialog, &QWidget::close);
        auto *monitor = new QTimer(dialog);
        monitor->setInterval(500);
        QObject::connect(monitor, &QTimer::timeout, dialog, [recovery, dialog] {
            if (!recovery->alive())
                dialog->close();
        });
        monitor->start();
        dialog->resize(700, 500);
        dialog->show();
    };
    const bool exists = alive();
    if (exists) {
        if (m_native->foreign)
            m_native->foreign->setParent(nullptr);
        // Detach before deleting the container so its native destruction never
        // owns the external app window. No WM_CLOSE or DestroyWindow is sent.
        if (!setParent(m_native->window, m_native->originalParent)
            && GetParent(m_native->window) != m_native->originalParent) {
            qWarning().noquote() << nativeError(QObject::tr("Cannot restore the external window parent"));
            retainRecoveryHost();
            return;
        }
    }
    delete m_native->container.data();
    delete m_native->foreign.data();
    m_native->container = nullptr;
    m_native->foreign = nullptr;
    if (exists && alive()) {
        const bool restoredStyle = setStyle(m_native->window, GWL_STYLE, m_native->originalStyle);
        const bool restoredExtendedStyle = setStyle(m_native->window, GWL_EXSTYLE, m_native->originalExtendedStyle);
        if (!restoredStyle || !restoredExtendedStyle)
            qWarning().noquote() << nativeError(QObject::tr("Cannot restore the external window styles"));
        if (m_native->originalOwner) {
            DWORD ownerPid = 0;
            FILETIME ownerCreated{};
            // The app may destroy its hidden infrastructure owner while it is
            // embedded. Do not retry a dead or recycled owner indefinitely:
            // leave the surviving application independent on the desktop.
            const bool ownerAlive = processIdentity(m_native->originalOwner, ownerPid, ownerCreated)
                && ownerPid == m_native->pid
                && CompareFileTime(&ownerCreated, &m_native->created) == 0;
            if (ownerAlive && !setStyle(m_native->window, GWLP_HWNDPARENT,
                    reinterpret_cast<LONG_PTR>(m_native->originalOwner))) {
                qWarning().noquote() << nativeError(QObject::tr("Cannot restore the external window owner"));
                retainRecoveryHost();
                return;
            }
        }
        const auto &rectangle = m_native->originalRectangle;
        const bool restoredRectangle = SetWindowPos(m_native->window, nullptr, rectangle.left, rectangle.top,
                rectangle.right - rectangle.left, rectangle.bottom - rectangle.top,
                SWP_NOZORDER | SWP_NOACTIVATE | SWP_FRAMECHANGED | SWP_ASYNCWINDOWPOS);
        const bool restoredPlacement = SetWindowPlacement(m_native->window, &m_native->originalPlacement);
        if (!restoredRectangle || !restoredPlacement)
            qWarning().noquote() << nativeError(QObject::tr("Cannot restore the external window placement"));
        ShowWindowAsync(m_native->window, m_native->originalPlacement.showCmd);
    }
    m_native->saved = false;
    m_native->changed = false;
}

QImage DesktopAppWindow::capture(QString &error)
{
    error.clear();
    if (!alive() || IsIconic(m_native->window)) {
        error = QObject::tr("The selected window is closed, minimized, or unavailable.");
        return {};
    }
    RECT rectangle{};
    if (!GetClientRect(m_native->window, &rectangle)) {
        error = nativeError(QObject::tr("Cannot read the window size"));
        return {};
    }
    const QSize sourceSize(rectangle.right - rectangle.left, rectangle.bottom - rectangle.top);
    if (sourceSize.isEmpty() || sourceSize.width() > 7680 || sourceSize.height() > 4320) {
        error = QObject::tr("Window exceeds the 7680 × 4320 capture limit.");
        return {};
    }
    const QSize size = sourceSize.scaled(1280, 720, Qt::KeepAspectRatio);
    HDC source = GetDC(m_native->window);
    HDC destination = source ? CreateCompatibleDC(source) : nullptr;
    BITMAPINFO bitmap{};
    bitmap.bmiHeader.biSize = sizeof(BITMAPINFOHEADER);
    bitmap.bmiHeader.biWidth = size.width();
    bitmap.bmiHeader.biHeight = -size.height();
    bitmap.bmiHeader.biPlanes = 1;
    bitmap.bmiHeader.biBitCount = 32;
    bitmap.bmiHeader.biCompression = BI_RGB;
    void *pixels = nullptr;
    HBITMAP surface = destination ? CreateDIBSection(destination, &bitmap, DIB_RGB_COLORS, &pixels, nullptr, 0) : nullptr;
    QImage image;
    if (surface && pixels) {
        HGDIOBJ previous = SelectObject(destination, surface);
        SetStretchBltMode(destination, HALFTONE);
        if (StretchBlt(destination, 0, 0, size.width(), size.height(), source,
                0, 0, sourceSize.width(), sourceSize.height(), SRCCOPY | CAPTUREBLT)) {
            // Bound CPU-owned pixels to one 1280x720 frame. GDI capture may not
            // render protected/GPU-only applications; never use blocking PrintWindow.
            image = QImage(static_cast<const uchar *>(pixels), size.width(), size.height(),
                size.width() * 4, QImage::Format_RGB32).copy();
        }
        SelectObject(destination, previous);
    }
    if (surface)
        DeleteObject(surface);
    if (destination)
        DeleteDC(destination);
    if (source)
        ReleaseDC(m_native->window, source);
    if (image.isNull()) {
        error = QObject::tr("This application's surface cannot be captured. GPU-only or protected windows may not provide a preview.");
        return {};
    }
    QImage preview(1280, 720, QImage::Format_RGB32);
    preview.fill(QColor("#101d28"));
    QPainter painter(&preview);
    painter.drawImage(QPoint((preview.width() - image.width()) / 2, (preview.height() - image.height()) / 2), image);
    return preview;
}


QIcon DesktopAppWindow::applicationIcon() const
{
    if (!alive())
        return {};
    const HWND window = windowHandle(m_choice.id);
    DWORD_PTR result = 0;
    SendMessageTimeoutW(window, WM_GETICON, ICON_SMALL2, 0,
        SMTO_ABORTIFHUNG | SMTO_BLOCK, 50, &result);
    HICON icon = reinterpret_cast<HICON>(result);
    if (!icon)
        icon = reinterpret_cast<HICON>(GetClassLongPtrW(window, GCLP_HICONSM));
    if (!icon)
        return {};
    ICONINFO info{};
    if (!GetIconInfo(icon, &info))
        return {};
    BITMAP bitmap{};
    const bool measured = GetObjectW(info.hbmColor ? info.hbmColor : info.hbmMask,
        sizeof(bitmap), &bitmap) != 0;
    if (info.hbmColor)
        DeleteObject(info.hbmColor);
    if (info.hbmMask)
        DeleteObject(info.hbmMask);
    if (!measured || bitmap.bmWidth <= 0 || bitmap.bmWidth > 512
        || bitmap.bmHeight <= 0 || bitmap.bmHeight > 1024)
        return {};
    // The HWND owns this HICON; only GetIconInfo's copied bitmaps are destroyed.
    return QIcon(QPixmap::fromImage(QImage::fromHICON(icon)));
}
#endif
