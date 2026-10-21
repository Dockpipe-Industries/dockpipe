#include "DesktopAppWindow.h"
#include "DesktopWindowWidget.h"
#include "DashboardWorkspace.h"

#include <QApplication>
#include <QComboBox>
#include <QDialog>
#include <QElapsedTimer>
#include <QLabel>
#include <QLineEdit>
#include <QProcess>
#include <QPointer>
#include <QThread>
#include <QTimer>
#include <QVBoxLayout>
#include <cstdio>
#include <functional>

#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <windows.h>
#include "WindowsTestDesktop.h"

#define CHECK(condition) do { if (!(condition)) { std::fprintf(stderr, "Windows embedding assertion failed at line %d: %s\n", __LINE__, #condition); return __LINE__; } } while (false)

namespace {
void reportPhase(const char *phase)
{
    std::fprintf(stderr, "Windows embedding phase: pid=%lu tick=%llu %s\n",
        GetCurrentProcessId(), static_cast<unsigned long long>(GetTickCount64()), phase);
    std::fflush(stderr);
}

bool waitFor(const std::function<bool()> &condition)
{
    QElapsedTimer deadline;
    deadline.start();
    do {
        QApplication::processEvents();
        if (condition())
            return true;
        QThread::msleep(10);
    } while (deadline.elapsed() < 5000);
    return false;
}

class NativeFixtureWindows final {
public:
    ~NativeFixtureWindows()
    {
        for (auto iterator = m_windows.crbegin(); iterator != m_windows.crend(); ++iterator) {
            if (IsWindow(*iterator))
                DestroyWindow(*iterator);
        }
    }

    HWND add(const WCHAR *title, DWORD extendedStyle, HWND owner, int show)
    {
        std::fprintf(stderr, "Windows fixture native window: tick=%llu before CreateWindowEx title=%ls show=%d\n",
            static_cast<unsigned long long>(GetTickCount64()), title, show);
        std::fflush(stderr);
        const HWND window = CreateWindowExW(extendedStyle, L"STATIC", title,
            WS_OVERLAPPEDWINDOW, 100, 100, 480, 320, owner, nullptr,
            GetModuleHandleW(nullptr), nullptr);
        const DWORD creationError = window ? ERROR_SUCCESS : GetLastError();
        std::fprintf(stderr, "Windows fixture native window: tick=%llu after CreateWindowEx title=%ls hwnd=%p error=%lu\n",
            static_cast<unsigned long long>(GetTickCount64()), title,
            static_cast<void *>(window), creationError);
        std::fflush(stderr);
        if (window) {
            m_windows.append(window);
            reportPhase("before native ShowWindow");
            ShowWindow(window, show);
            reportPhase("after native ShowWindow");
        }
        return window;
    }

private:
    QVector<HWND> m_windows;
};

QByteArray fixtureValue(const QByteArray &output, const QByteArray &name)
{
    for (const QByteArray &line : output.split('\n')) {
        if (line.startsWith(name + ':')) {
            QByteArray value = line.mid(name.size() + 1);
            if (value.endsWith('\r'))
                value.chop(1);
            return value;
        }
    }
    return {};
}

quint64 fixtureWindowId(const QByteArray &output, const QByteArray &name)
{
    return fixtureValue(output, name).toULongLong();
}

class FixtureProcess final : public QProcess {
public:
    ~FixtureProcess() override
    {
        if (state() == NotRunning)
            return;
        terminate();
        if (!waitForFinished(3000)) {
            kill();
            waitForFinished(1000);
        }
    }
};

struct FixtureIdentity {
    DWORD session = 0;
    TOKEN_ELEVATION elevation{};
    DWORD appContainer = 0;
    alignas(TOKEN_USER) BYTE user[sizeof(TOKEN_USER) + SECURITY_MAX_SID_SIZE]{};

    bool read(DWORD processId)
    {
        HANDLE process = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, processId);
        if (!process) {
            std::fprintf(stderr, "Fixture OpenProcess failed: error=%lu\n", GetLastError());
            return false;
        }
        HANDLE token = nullptr;
        if (!OpenProcessToken(process, TOKEN_QUERY, &token)) {
            const DWORD error = GetLastError();
            CloseHandle(process);
            std::fprintf(stderr, "Fixture OpenProcessToken failed: error=%lu\n", error);
            return false;
        }
        DWORD length = 0;
        const bool valid = ProcessIdToSessionId(processId, &session)
            && GetTokenInformation(token, TokenElevation, &elevation, sizeof(elevation), &length)
            && GetTokenInformation(token, TokenIsAppContainer, &appContainer, sizeof(appContainer), &length)
            && GetTokenInformation(token, TokenUser, user, sizeof(user), &length);
        const DWORD error = valid ? ERROR_SUCCESS : GetLastError();
        CloseHandle(token);
        CloseHandle(process);
        if (!valid)
            std::fprintf(stderr, "Fixture identity query failed: error=%lu\n", error);
        return valid;
    }

    PSID sid() const
    {
        return reinterpret_cast<const TOKEN_USER *>(user)->User.Sid;
    }
};
}

int main(int argc, char **argv)
{
    reportPhase("before QApplication");
    QApplication app(argc, argv);
    reportPhase("after QApplication");
    if (!DesktopAppWindow::unavailableReason().isEmpty())
        return 77;
    std::wstring desktopName;
    const DWORD desktopError = WindowsTestDesktop::currentName(desktopName);
    if (desktopError != ERROR_SUCCESS)
        std::fprintf(stderr, "Windows fixture desktop query failed: error=%lu\n", desktopError);
    CHECK(desktopError == ERROR_SUCCESS);
    const QByteArray desktopUtf8 = QString::fromStdWString(desktopName).toUtf8();
    if (app.arguments().contains("--fixture")) {
        reportPhase("fixture window creation");
        QWidget window;
        window.setWindowTitle("DockPipe isolated Windows embedding fixture");
        window.resize(560, 360);
        auto *layout = new QVBoxLayout(&window);
        layout->addWidget(new QLabel("Only this disposable test window may be embedded", &window));
        layout->addWidget(new QLineEdit("Native keyboard input", &window));
        reportPhase("before Qt fixture show");
        window.show();
        reportPhase("after Qt fixture show");
        NativeFixtureWindows windows;
        const HWND owner = windows.add(L"Hidden fixture owner", 0, nullptr, SW_HIDE);
        const HWND minimized = windows.add(L"Minimized fixture app", WS_EX_APPWINDOW, nullptr, SW_SHOWMINNOACTIVE);
        const HWND ownedApp = windows.add(L"Owned fixture app", WS_EX_APPWINDOW, owner, SW_SHOWNOACTIVATE);
        const HWND dialog = windows.add(L"Owned fixture dialog", WS_EX_APPWINDOW,
            reinterpret_cast<HWND>(window.winId()), SW_SHOWNOACTIVATE);
        const HWND tool = windows.add(L"Fixture tool window", WS_EX_APPWINDOW | WS_EX_TOOLWINDOW,
            nullptr, SW_SHOWNOACTIVATE);
        const HWND extra = windows.add(L"Fourth ordinary fixture window", 0, nullptr, SW_SHOWNOACTIVATE);
        CHECK(owner && minimized && ownedApp && dialog && tool && extra);
        reportPhase("fixture native windows ready");
        std::printf("WINDOW:%llu\nMINIMIZED:%llu\nOWNED:%llu\nOWNER:%llu\nDIALOG:%llu\nTOOL:%llu\nEXTRA:%llu\nDESKTOP:%s\nREADY\n",
            static_cast<unsigned long long>(window.winId()),
            static_cast<unsigned long long>(reinterpret_cast<quintptr>(minimized)),
            static_cast<unsigned long long>(reinterpret_cast<quintptr>(ownedApp)),
            static_cast<unsigned long long>(reinterpret_cast<quintptr>(owner)),
            static_cast<unsigned long long>(reinterpret_cast<quintptr>(dialog)),
            static_cast<unsigned long long>(reinterpret_cast<quintptr>(tool)),
            static_cast<unsigned long long>(reinterpret_cast<quintptr>(extra)),
            desktopUtf8.constData());
        std::fflush(stdout);
        QTimer::singleShot(60000, &app, &QApplication::quit);
        return app.exec();
    }

    FixtureProcess fixture;
    fixture.setProcessChannelMode(QProcess::ForwardedErrorChannel);
    fixture.setCreateProcessArgumentsModifier([&desktopName](QProcess::CreateProcessArguments *arguments) {
        const DWORD originalFlags = arguments->flags;
        // This GUI fixture communicates through Qt's inherited pipes and needs
        // no console. Its CREATE_NO_WINDOW path failed in KERNELBASE startup.
        // Avoid console allocation while retaining QProcess and job ownership.
        arguments->flags = (arguments->flags & ~CREATE_NO_WINDOW) | DETACHED_PROCESS;
        std::fprintf(stderr,
            "Windows fixture launch: flags=0x%08lx originalFlags=0x%08lx inheritHandles=%d originalDesktop=%ls selectedDesktop=%ls console=%p\n",
            static_cast<unsigned long>(arguments->flags), static_cast<unsigned long>(originalFlags),
            arguments->inheritHandles,
            arguments->startupInfo->lpDesktop ? arguments->startupInfo->lpDesktop : L"(implicit)",
            desktopName.c_str(), static_cast<void *>(GetConsoleWindow()));
        // The outer test launcher uses this same explicit interactive context.
        // Keep storage alive through start() and preserve Qt's other arguments.
        arguments->startupInfo->lpDesktop = desktopName.data();
    });
    reportPhase("before fixture start");
    fixture.start(app.applicationFilePath(), {"--fixture"});
    reportPhase("before waitForStarted");
    CHECK(fixture.waitForStarted(3000));
    reportPhase("before fixture readiness");
    QByteArray output;
    const bool fixtureReady = waitFor([&] {
        output += fixture.readAllStandardOutput();
        // Windows CRT text output uses CRLF; QProcess preserves the raw bytes.
        return output.startsWith("WINDOW:")
            && (output.contains("\nREADY\r\n") || output.contains("\nREADY\n"));
    });
    if (!fixtureReady) {
        std::fprintf(stderr, "Fixture readiness failed: state=%d exitStatus=%d exitCode=%d exitHex=0x%08lx processError=%d error=%s stdoutBytes=%lld stdoutHex=%s\n",
            static_cast<int>(fixture.state()), static_cast<int>(fixture.exitStatus()),
            fixture.exitCode(), static_cast<unsigned long>(static_cast<DWORD>(fixture.exitCode())),
            static_cast<int>(fixture.error()), fixture.errorString().toUtf8().constData(),
            static_cast<long long>(output.size()),
            output.left(4096).toHex(' ').constData());
    }
    CHECK(fixtureReady);
    CHECK(fixtureValue(output, "DESKTOP") == desktopUtf8);
    reportPhase("fixture readiness complete");
    const quint64 id = fixtureWindowId(output, "WINDOW");
    CHECK(id);
    const HWND window = reinterpret_cast<HWND>(static_cast<quintptr>(id));
    const HWND originalParent = GetParent(window);
    const LONG_PTR originalStyle = GetWindowLongPtrW(window, GWL_STYLE);
    const LONG_PTR originalExtendedStyle = GetWindowLongPtrW(window, GWL_EXSTYLE);
    RECT originalRectangle{};
    CHECK(GetWindowRect(window, &originalRectangle));
    DWORD fixtureProcessId = 0;
    CHECK(GetWindowThreadProcessId(window, &fixtureProcessId));
    CHECK(fixtureProcessId == static_cast<DWORD>(fixture.processId()));
    FixtureIdentity currentIdentity;
    FixtureIdentity fixtureIdentity;
    CHECK(currentIdentity.read(GetCurrentProcessId()));
    CHECK(fixtureIdentity.read(fixtureProcessId));
    const bool sameUser = EqualSid(currentIdentity.sid(), fixtureIdentity.sid());
    std::fprintf(stderr,
        "Windows embedding identity: parentElevated=%lu childElevated=%lu childAppContainer=%lu sameUser=%d sameSession=%d\n",
        currentIdentity.elevation.TokenIsElevated, fixtureIdentity.elevation.TokenIsElevated,
        fixtureIdentity.appContainer, sameUser, currentIdentity.session == fixtureIdentity.session);
    CHECK(!currentIdentity.elevation.TokenIsElevated);
    CHECK(!fixtureIdentity.elevation.TokenIsElevated && !fixtureIdentity.appContainer);
    CHECK(sameUser && currentIdentity.session == fixtureIdentity.session);
    DesktopAppWindow selected({id, "Isolated Windows fixture", {}});
    CHECK(selected.alive());
    CHECK(waitFor([&] {
        QString error;
        for (const auto &choice : DesktopAppWindow::availableWindows(error)) {
            if (choice.id == id)
                return true;
        }
        return false;
    }));

    const quint64 minimizedId = fixtureWindowId(output, "MINIMIZED");
    const quint64 ownedId = fixtureWindowId(output, "OWNED");
    const quint64 dialogId = fixtureWindowId(output, "DIALOG");
    const quint64 toolId = fixtureWindowId(output, "TOOL");
    const quint64 extraId = fixtureWindowId(output, "EXTRA");
    const HWND originalOwner = reinterpret_cast<HWND>(static_cast<quintptr>(fixtureWindowId(output, "OWNER")));
    CHECK(minimizedId && ownedId && dialogId && toolId && extraId && originalOwner);
    QString discoveryError;
    const auto choices = DesktopAppWindow::availableWindows(discoveryError);
    CHECK(discoveryError.isEmpty());
    const auto contains = [&choices](quint64 target) {
        for (const auto &choice : choices) {
            if (choice.id == target)
                return true;
        }
        return false;
    };
    // Four ordinary HWNDs from one process prove the picker has no three-app
    // or one-window-per-PID limit. The minimized one remains discoverable.
    CHECK(contains(id) && contains(minimizedId) && contains(ownedId) && contains(extraId));
    CHECK(!contains(dialogId) && !contains(toolId));
    const auto targeted = DesktopAppWindow::availableWindows(discoveryError, ownedId);
    CHECK(discoveryError.isEmpty() && targeted.size() == 1 && targeted.first().id == ownedId);
    CHECK(DesktopAppWindow::availableWindows(discoveryError, dialogId).isEmpty());
    CHECK(!discoveryError.isEmpty());
    DesktopAppWindow dialogWindow({dialogId, "Rejected dialog", {}});
    CHECK(!dialogWindow.prepareControl(discoveryError));
    CHECK(!discoveryError.isEmpty());
    CHECK(GetWindow(reinterpret_cast<HWND>(static_cast<quintptr>(dialogId)), GW_OWNER)
        == reinterpret_cast<HWND>(static_cast<quintptr>(id)));

    {
        DesktopAppWindow prepared({id, "Cancelled fixture preflight", {}});
        CHECK(prepared.prepareControl(discoveryError));
        CHECK(ShowWindowAsync(window, SW_SHOWMINNOACTIVE));
        CHECK(waitFor([&] { return IsIconic(window); }));
        prepared.releaseControl();
        // No embedding happened, so cancellation must preserve the user's
        // newer desktop state rather than replaying the preflight snapshot.
        CHECK(IsIconic(window));
        CHECK(ShowWindowAsync(window, SW_SHOWNOACTIVATE));
        CHECK(waitFor([&] { return !IsIconic(window); }));
    }

    for (quint64 target : {minimizedId, ownedId}) {
        const HWND targetWindow = reinterpret_cast<HWND>(static_cast<quintptr>(target));
        const LONG_PTR style = GetWindowLongPtrW(targetWindow, GWL_STYLE);
        const LONG_PTR extendedStyle = GetWindowLongPtrW(targetWindow, GWL_EXSTYLE);
        const HWND owner = GetWindow(targetWindow, GW_OWNER);
        WINDOWPLACEMENT placement{sizeof(WINDOWPLACEMENT)};
        CHECK(GetWindowPlacement(targetWindow, &placement));
        const bool minimized = IsIconic(targetWindow);
        CHECK(minimized == (target == minimizedId));
        CHECK(target != ownedId || owner == originalOwner);
        DesktopAppWindow attachment({target, "Disposable fixture window", {}});
        CHECK(attachment.prepareControl(discoveryError));
        // Preparing or cancelling must not restore/minimize/activate an app.
        CHECK(bool(IsIconic(targetWindow)) == minimized);
        QWidget host;
        host.resize(620, 400);
        auto *layout = new QVBoxLayout(&host);
        auto *container = attachment.takeControl(&host, discoveryError);
        CHECK(container && discoveryError.isEmpty());
        layout->addWidget(container);
        host.show();
        CHECK(waitFor([&] { return attachment.controlAttached() && !IsIconic(targetWindow); }));
        attachment.releaseControl();
        CHECK(waitFor([&] {
            return attachment.alive() && GetWindow(targetWindow, GW_OWNER) == owner
                && bool(IsIconic(targetWindow)) == minimized
                && GetWindowLongPtrW(targetWindow, GWL_STYLE) == style
                && GetWindowLongPtrW(targetWindow, GWL_EXSTYLE) == extendedStyle;
        }));
        WINDOWPLACEMENT restored{sizeof(WINDOWPLACEMENT)};
        CHECK(GetWindowPlacement(targetWindow, &restored));
        CHECK(EqualRect(&placement.rcNormalPosition, &restored.rcNormalPosition));
        CHECK(placement.showCmd == restored.showCmd);
    }

    {
        const HWND targetWindow = reinterpret_cast<HWND>(static_cast<quintptr>(ownedId));
        DesktopAppWindow attachment({ownedId, "Disposable owner-lifecycle fixture", {}});
        CHECK(attachment.prepareControl(discoveryError));
        QWidget host;
        host.resize(620, 400);
        auto *layout = new QVBoxLayout(&host);
        auto *container = attachment.takeControl(&host, discoveryError);
        CHECK(container && discoveryError.isEmpty());
        layout->addWidget(container);
        host.show();
        CHECK(waitFor([&] { return attachment.controlAttached(); }));
        // Destroy only this disposable hidden owner after detachment. Its app
        // HWND survives and must be releasable without an endless recovery host.
        CHECK(PostMessageW(originalOwner, WM_CLOSE, 0, 0));
        CHECK(waitFor([&] { return !IsWindow(originalOwner); }));
        CHECK(attachment.alive());
        attachment.releaseControl();
        CHECK(waitFor([&] {
            return attachment.alive() && !GetWindow(targetWindow, GW_OWNER)
                && !(GetWindowLongPtrW(targetWindow, GWL_STYLE) & WS_CHILD);
        }));
        CHECK(!attachment.controlReady());
    }

    DashboardWorkspace workspace("native-test");
    workspace.resize(1100, 760);
    QPointer<DesktopWindowWidget> widget = new DesktopWindowWidget;
    CHECK(workspace.addPanel("native", "External window", widget));
    CHECK(workspace.addPanel("other", "Other panel", new QLabel("Retained panel")));
    workspace.show();
    widget->chooseWindow();
    auto *picker = widget->findChild<QDialog *>("desktopWindowPicker");
    CHECK(picker);
    picker->reject();
    QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
    CHECK(GetParent(window) == originalParent);

    for (int cycle = 0; cycle < 2; ++cycle) {
        widget->chooseWindow();
        picker = widget->findChild<QDialog *>("desktopWindowPicker");
        CHECK(picker);
        auto *choices = picker->findChild<QComboBox *>("desktopWindowChoices");
        CHECK(choices);
        const int index = choices->findData(QVariant::fromValue(id));
        CHECK(index >= 0);
        choices->setCurrentIndex(index);
        picker->accept();
        auto *status = widget->findChild<QLabel *>("desktopWindowStatus");
        CHECK(status);
        CHECK(waitFor([&] { return widget->hasInteractiveWindow(); }));
        CHECK(status->isHidden());
        workspace.setMode("tiling");
        CHECK(waitFor([&] { return widget->hasInteractiveWindow() && selected.alive(); }));
        CHECK(workspace.dockPanel("native", "other", "tab"));
        CHECK(waitFor([&] { return widget->hasInteractiveWindow() && selected.alive(); }));
        CHECK(workspace.floatPanel("native"));
        CHECK(waitFor([&] { return widget->hasInteractiveWindow() && selected.alive(); }));
        workspace.setMode("canvas");
        CHECK(waitFor([&] { return widget->hasInteractiveWindow() && selected.alive(); }));
        CHECK(widget->hasInteractiveWindow());
        if (cycle == 0)
            widget->releaseWindow();
        else
            workspace.removePanel("native");
        CHECK(waitFor([&] {
            return selected.alive() && GetParent(window) == originalParent
                && GetWindowLongPtrW(window, GWL_STYLE) == originalStyle
                && GetWindowLongPtrW(window, GWL_EXSTYLE) == originalExtendedStyle;
        }));
        RECT restored{};
        CHECK(GetWindowRect(window, &restored));
        CHECK(EqualRect(&originalRectangle, &restored));
        QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
    }
    return 0;
}
