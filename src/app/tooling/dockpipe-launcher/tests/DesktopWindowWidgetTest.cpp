#include "DesktopAppWindow.h"
#include "DesktopWindowWidget.h"
#include "DashboardWorkspace.h"

#include <QApplication>
#include <QAction>
#include <QComboBox>
#include <QDialog>
#include <QElapsedTimer>
#include <QLabel>
#include <QProcess>
#include <QPointer>
#include <QPixmap>
#include <QPushButton>
#include <QThread>
#include <QTimer>
#include <cstdio>
#include <functional>

#define CHECK(condition) do { if (!(condition)) { std::fprintf(stderr, "Desktop widget assertion failed at line %d: %s\n", __LINE__, #condition); return __LINE__; } } while (false)

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
    DashboardWorkspace workspace("native-test");
    workspace.resize(1100, 760);
    QPointer<DesktopWindowWidget> widget = new DesktopWindowWidget;
    CHECK(workspace.addPanel("native", "External window", widget));
    CHECK(workspace.addPanel("other", "Other panel", new QLabel("Retained panel")));
    workspace.show();
    CHECK(widget->findChildren<QPushButton *>().isEmpty());
    CHECK(widget->contextMenuPolicy() == Qt::ActionsContextMenu);
    auto *choose = widget->findChild<QAction *>("desktopWindowChoose");
    CHECK(widget->actions().contains(choose));
    CHECK(!widget->findChild<QPushButton *>("desktopWindowRelease"));
    auto *status = widget->findChild<QLabel *>("desktopWindowStatus");
    CHECK(choose && choose->isEnabled() && status);
    CHECK(!widget->findChild<QDialog *>("desktopWindowPicker"));

    if (!app.arguments().contains("--native")) {
        // Unsupported platforms fail visibly and acquire no foreign window.
        CHECK(!DesktopAppWindow::unavailableReason().isEmpty());
        QString error;
        CHECK(!DesktopWindowWidget::automaticAcquisitionUnavailableReason().isEmpty());
        CHECK(DesktopWindowWidget::availableWindows(error).isEmpty() && !error.isEmpty());
        CHECK(!widget->acquireWindow(123, error) && !error.isEmpty());
        choose->trigger();
        CHECK(!status->text().isEmpty() && !status->isHidden());
        CHECK(!widget->hasInteractiveWindow());
        CHECK(!widget->findChild<QDialog *>("desktopWindowPicker"));
        widget->releaseWindow();
        widget->hide();
        widget->show();
        CHECK(choose->isEnabled() && !widget->hasInteractiveWindow());
        return 0;
    }

    if (!DesktopAppWindow::unavailableReason().isEmpty())
        return 77;
    FixtureProcess fixture;
    fixture.setProcessChannelMode(QProcess::ForwardedErrorChannel);
    fixture.start(QCoreApplication::applicationDirPath() + "/desktop-app-window-test", {"--fixture"});
    CHECK(fixture.waitForStarted(3000));
    QByteArray output;
    CHECK(waitFor([&] {
        output += fixture.readAllStandardOutput();
        return output.startsWith("WINDOW:") && output.contains('\n');
    }));
    const quint64 id = output.split('\n').first().mid(7).toULongLong();
    CHECK(id);
    DesktopAppWindow selected({id, "Isolated test window", {}});
    CHECK(selected.alive());
    CHECK(!selected.applicationIcon().isNull());
    CHECK(selected.applicationIcon().pixmap(32, 32).toImage().pixelColor(16, 16) == QColor("#35b77c"));
    CHECK(waitFor([&] {
        QString error;
        for (const auto &choice : DesktopAppWindow::availableWindows(error)) {
            if (choice.id == id)
                return true;
        }
        return false;
    }));

    choose->trigger();
    auto *picker = widget->findChild<QDialog *>("desktopWindowPicker");
    CHECK(picker && !choose->isEnabled());
    picker->reject();
    CHECK(choose->isEnabled());
    QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
    CHECK(!widget->hasInteractiveWindow() && selected.alive());

    for (int cycle = 0; cycle < 2; ++cycle) {
        QString acquisitionError;
        CHECK(!widget->acquireWindow(quint64(workspace.winId()), acquisitionError));
        CHECK(!acquisitionError.isEmpty());
        CHECK(!widget->acquiredWindowId());
        if (cycle == 0) {
            choose->trigger();
            picker = widget->findChild<QDialog *>("desktopWindowPicker");
            CHECK(picker);
            auto *choices = picker->findChild<QComboBox *>("desktopWindowChoices");
            CHECK(choices);
            const int index = choices->findData(QVariant::fromValue(id));
            CHECK(index >= 0);
            choices->setCurrentIndex(index);
            picker->accept();
        } else {
            CHECK(widget->acquireWindow(id, acquisitionError));
            CHECK(acquisitionError.isEmpty());
        }
        CHECK(waitFor([&] { return widget->hasInteractiveWindow(); }));
        CHECK(widget->acquiredWindowId() == id);
        CHECK(!widget->acquiredWindowTitle().isEmpty());
        CHECK(!widget->applicationIcon().isNull());
        CHECK(!choose->isEnabled() && status->isHidden());
        workspace.setMode("tiling");
        CHECK(waitFor([&] { return widget->hasInteractiveWindow() && selected.alive(); }));
        CHECK(workspace.dockPanel("native", "other", "tab"));
        CHECK(waitFor([&] { return widget->hasInteractiveWindow() && selected.alive(); }));
        CHECK(workspace.floatPanel("native"));
        CHECK(waitFor([&] { return widget->hasInteractiveWindow() && selected.alive(); }));
        workspace.setMode("canvas");
        CHECK(waitFor([&] { return widget->hasInteractiveWindow() && selected.alive(); }));
        auto *monitor = widget->findChild<QTimer *>("desktopNativeMonitor");
        CHECK(monitor);
        widget->hide();
        CHECK(waitFor([&] { return !monitor->isActive(); }));
        CHECK(selected.alive());
        widget->show();
        CHECK(waitFor([&] { return monitor->isActive() && widget->hasInteractiveWindow(); }));
        CHECK(widget->hasInteractiveWindow() && !choose->isEnabled() && status->isHidden());
        if (cycle == 0) {
            widget->releaseWindow();
            CHECK(choose->isEnabled() && !widget->hasInteractiveWindow());
        } else {
            workspace.removePanel("native");
        }
        CHECK(selected.alive());
        CHECK(waitFor([&] {
            QString error;
            for (const auto &choice : DesktopAppWindow::availableWindows(error)) {
                if (choice.id == id)
                    return true;
            }
            return false;
        }));
        QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
    }
    return 0;
}
