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

#define CHECK(condition) do { if (!(condition)) { std::fprintf(stderr, "Windows embedding assertion failed at line %d: %s\n", __LINE__, #condition); return __LINE__; } } while (false)

namespace {
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
        const HWND window = CreateWindowExW(extendedStyle, L"STATIC", title,
            WS_OVERLAPPEDWINDOW, 100, 100, 480, 320, owner, nullptr,
            GetModuleHandleW(nullptr), nullptr);
        if (window) {
            m_windows.append(window);
            ShowWindow(window, show);
        }
        return window;
    }

private:
    QVector<HWND> m_windows;
};

quint64 fixtureWindowId(const QByteArray &output, const QByteArray &name)
{
    for (const QByteArray &line : output.split('\n')) {
        if (line.startsWith(name + ':'))
            return line.mid(name.size() + 1).toULongLong();
    }
    return 0;
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
}

int main(int argc, char **argv)
{
    QApplication app(argc, argv);
    if (!DesktopAppWindow::unavailableReason().isEmpty())
        return 77;
    if (app.arguments().contains("--fixture")) {
        QWidget window;
        window.setWindowTitle("DockPipe isolated Windows embedding fixture");
        window.resize(560, 360);
        auto *layout = new QVBoxLayout(&window);
        layout->addWidget(new QLabel("Only this disposable test window may be embedded", &window));
        layout->addWidget(new QLineEdit("Native keyboard input", &window));
        window.show();
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
        std::printf("WINDOW:%llu\nMINIMIZED:%llu\nOWNED:%llu\nOWNER:%llu\nDIALOG:%llu\nTOOL:%llu\nEXTRA:%llu\nREADY\n",
            static_cast<unsigned long long>(window.winId()),
            static_cast<unsigned long long>(reinterpret_cast<quintptr>(minimized)),
            static_cast<unsigned long long>(reinterpret_cast<quintptr>(ownedApp)),
            static_cast<unsigned long long>(reinterpret_cast<quintptr>(owner)),
            static_cast<unsigned long long>(reinterpret_cast<quintptr>(dialog)),
            static_cast<unsigned long long>(reinterpret_cast<quintptr>(tool)),
            static_cast<unsigned long long>(reinterpret_cast<quintptr>(extra)));
        std::fflush(stdout);
        QTimer::singleShot(60000, &app, &QApplication::quit);
        return app.exec();
    }

    FixtureProcess fixture;
    fixture.setProcessChannelMode(QProcess::ForwardedErrorChannel);
    fixture.start(app.applicationFilePath(), {"--fixture"});
    CHECK(fixture.waitForStarted(3000));
    QByteArray output;
    const bool fixtureReady = waitFor([&] {
        output += fixture.readAllStandardOutput();
        // Windows CRT text output uses CRLF; QProcess preserves the raw bytes.
        return output.startsWith("WINDOW:")
            && (output.contains("\nREADY\r\n") || output.contains("\nREADY\n"));
    });
    if (!fixtureReady) {
        std::fprintf(stderr, "Fixture readiness failed: state=%d exitStatus=%d exitCode=%d stdoutBytes=%lld stdoutHex=%s\n",
            static_cast<int>(fixture.state()), static_cast<int>(fixture.exitStatus()),
            fixture.exitCode(), static_cast<long long>(output.size()),
            output.left(4096).toHex(' ').constData());
    }
    CHECK(fixtureReady);
    const quint64 id = fixtureWindowId(output, "WINDOW");
    CHECK(id);
    const HWND window = reinterpret_cast<HWND>(static_cast<quintptr>(id));
    const HWND originalParent = GetParent(window);
    const LONG_PTR originalStyle = GetWindowLongPtrW(window, GWL_STYLE);
    const LONG_PTR originalExtendedStyle = GetWindowLongPtrW(window, GWL_EXSTYLE);
    RECT originalRectangle{};
    CHECK(GetWindowRect(window, &originalRectangle));
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
