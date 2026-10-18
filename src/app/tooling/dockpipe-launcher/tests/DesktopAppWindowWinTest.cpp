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
        std::printf("WINDOW:%llu\n", static_cast<unsigned long long>(window.winId()));
        std::fflush(stdout);
        QTimer::singleShot(30000, &app, &QApplication::quit);
        return app.exec();
    }

    FixtureProcess fixture;
    fixture.setProcessChannelMode(QProcess::ForwardedErrorChannel);
    fixture.start(app.applicationFilePath(), {"--fixture"});
    CHECK(fixture.waitForStarted(3000));
    QByteArray output;
    CHECK(waitFor([&] {
        output += fixture.readAllStandardOutput();
        return output.startsWith("WINDOW:") && output.contains('\n');
    }));
    const quint64 id = output.split('\n').first().mid(7).toULongLong();
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
