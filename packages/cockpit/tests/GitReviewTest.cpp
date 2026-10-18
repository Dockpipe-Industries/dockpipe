#include "GitReviewWidget.h"
#include "CockpitWidget.h"
#include "GitReviewModel.h"
#include "GitReviewProgress.h"
#include "GitReviewWorld.h"
#include "WorkspaceFlightView.h"
#include "SourceEditor.h"
#include <QApplication>
#include <QComboBox>
#include <QElapsedTimer>
#include <QFile>
#include <QJsonArray>
#include <QLabel>
#include <QListWidget>
#include <QPlainTextEdit>
#include <QProcess>
#include <QPushButton>
#include <QScreen>
#include <QTemporaryDir>
#include <QThread>
#include <cstdio>

#define VERIFY(value) do { if (!(value)) { std::fprintf(stderr, "Git review assertion failed at %d: %s\n", __LINE__, #value); return 1; } } while (false)

namespace {
void idle(int milliseconds)
{
    QElapsedTimer elapsed;
    elapsed.start();
    while (elapsed.elapsed() < milliseconds) {
        QApplication::processEvents();
        QThread::msleep(2);
    }
}
bool settled(GitReviewWidget &widget)
{
    QElapsedTimer elapsed;
    elapsed.start();
    do {
        idle(10);
    } while (widget.busy() && elapsed.elapsed() < 15000);
    if (widget.busy() || widget.model().root.isEmpty())
        std::fprintf(stderr, "%s\n", qPrintable(widget.findChild<QLabel *>("gitReviewStatus")->text()));
    return !widget.busy() && !widget.model().root.isEmpty();
}
bool write(const QString &path, const QByteArray &content)
{
    QFile file(path);
    return file.open(QIODevice::WriteOnly) && file.write(content) == content.size();
}
bool git(const QString &root, const QStringList &arguments)
{
    // Git is used only to prepare and inspect this disposable test repository.
    QProcess process;
    process.start("git", QStringList{"-C", root} + arguments);
    if (!process.waitForFinished(10000) || process.exitCode() != 0) {
        std::fprintf(stderr, "%s\n", process.readAllStandardError().constData());
        return false;
    }
    return true;
}
}

int gitReviewTest()
{
    QImage frame(32, 16, QImage::Format_ARGB32_Premultiplied);
    frame.fill(Qt::black);
    QImage content(20, 28, QImage::Format_ARGB32_Premultiplied);
    for (int row = 0; row < content.height(); ++row)
        for (int column = 0; column < content.width(); ++column)
            content.setPixelColor(column, row, QColor(row * 8, column * 8, 20));
    const ScrollingPreview scrolling(frame, content, QRect(4, 4, 20, 8));
    VERIFY(scrolling.scrolling() && scrolling.offsetAt(-1) == 0 && scrolling.offsetAt(3000) == 0);
    VERIFY(scrolling.offsetAt(4000) == 10 && scrolling.offsetAt(5000) == 20);
    VERIFY(scrolling.offsetAt(7999) == 20 && scrolling.offsetAt(8000) == 0);
    VERIFY(scrolling.imageAt(4000).pixelColor(4, 4) == content.pixelColor(0, 10));
    VERIFY(scrolling.imageAt(4000).pixelColor(0, 0) == Qt::black);
    VERIFY(scrolling.imageAt(0) == scrolling.imageAt(8000));
    const ScrollingPreview fitting(frame, content.copy(0, 0, 20, 8), QRect(4, 4, 20, 8));
    VERIFY(!fitting.scrolling() && fitting.imageAt(0) == fitting.imageAt(90000));
    QImage oversized(20, ScrollingPreview::maximumHeight + 1, QImage::Format_ARGB32_Premultiplied);
    const ScrollingPreview bounded(frame, oversized, QRect(4, 4, 20, 8));
    VERIFY(!bounded.scrolling() && bounded.imageAt(4000) == frame);

    GitReviewDiff longPreview;
    GitReviewHunk longHunk;
    for (int line = 0; line < 180; ++line)
        longHunk.rows.append({QString("int before_%1 = 1;").arg(line), QString("int after_%1 = 2;").arg(line), line + 1, line + 1, true, true});
    longPreview.hunks.append(longHunk);
    const auto codePreview = GitReviewWorld::codePreview({"sample.cpp"}, &longPreview, 0, false);
    VERIFY(codePreview.scrolling() && codePreview.imageAt(0) != codePreview.imageAt(5000));
    VERIFY(codePreview.imageAt(0).copy(0, 0, 512, 36) == codePreview.imageAt(5000).copy(0, 0, 512, 36));
    VERIFY(codePreview.imageAt(0).copy(0, 292, 512, 28) == codePreview.imageAt(5000).copy(0, 292, 512, 28));
    const QString previewCapture = qEnvironmentVariable("DOCKPIPE_GIT_CAPTURE");
    if (!previewCapture.isEmpty()) {
        VERIFY(codePreview.imageAt(0).save(previewCapture + "-text-start.png"));
        VERIFY(codePreview.imageAt(8000).save(previewCapture + "-text-scrolled.png"));
    }
    const QString digest(64, 'a');
    QJsonObject comparison{{"path", "engine.cpp"}, {"revision", digest}, {"comparison", "unstaged"},
        {"before_label", "Index"}, {"after_label", "Working tree"},
        {"patch", "@@ -8,4 +8,5 @@ function\n context\n-old\n+new\n+extra\n tail\n last\n\\ No newline at end of file\n"}};
    GitReviewDiff diff;
    QString error;
    VERIFY(GitReviewDiff::parse(comparison, diff, error));
    VERIFY(diff.hunks.size() == 1 && diff.hunks[0].rows.size() == 5);
    VERIFY(diff.hunks[0].rows[1].beforeLine == 9 && diff.hunks[0].rows[1].afterLine == 9);
    VERIFY(diff.hunks[0].rows[2].beforeLine == 0 && diff.hunks[0].rows[2].afterLine == 10);
    VERIFY(diff.hunks[0].rows[3].beforeLine == 10 && diff.hunks[0].rows[3].afterLine == 11);
    VERIFY(diff.hunks[0].additions == 2 && diff.hunks[0].deletions == 1 && diff.notice.contains("final newline"));
    comparison["patch"] = "@@ -1,2 +1,2 @@\n-only\n+one\n";
    VERIFY(!GitReviewDiff::parse(comparison, diff, error));
    comparison["patch"] = "@@ -0,0 +1 @@\n+" + QString(17000, 'x') + "\n";
    VERIFY(GitReviewDiff::parse(comparison, diff, error) && diff.limited && diff.hunks.isEmpty());

    GitReviewProgress progress;
    progress.load("isolated-model-fixture");
    GitReviewFile mixed{"engine.cpp", {}, "MM", digest};
    progress.mark(mixed, GitComparison::Combined, 0);
    VERIFY(!progress.reviewed(mixed));
    progress.mark(mixed, GitComparison::Unstaged, 0);
    VERIFY(!progress.reviewed(mixed));
    progress.mark(mixed, GitComparison::Staged, 2);
    VERIFY(progress.reviewed(mixed));
    GitReviewProgress resumed;
    resumed.load("isolated-model-fixture");
    VERIFY(resumed.reviewed(mixed) && resumed.lastHunk() == 2);
    mixed.revision = QString(64, 'b');
    VERIFY(resumed.stale(mixed) && !resumed.reviewed(mixed));

    GitReviewModel large;
    for (int index = 0; index < GitReviewModel::maximumFiles; ++index)
        large.files.append({QString("group-%1/file-%2.cpp").arg(index / 40).arg(index), {}, " M", digest});
    for (int first : {0, 4, 1000, 2044}) {
        const auto section = GitReviewWorld::build(large, resumed, first);
        if (!section.world.success)
            std::fprintf(stderr, "World: %s\n", qPrintable(section.world.diagnostic));
        VERIFY(section.world.success);
        VERIFY(section.filePositions.size() == 4 && section.plan.displays.size() == 9);
        VERIFY(section.world.interior.solids.size() <= 40);
        VERIFY(section.filePositions == GitReviewWorld::build(large, resumed, first).filePositions);
    }

    const QString cli = qEnvironmentVariable("DOCKPIPE_BIN");
    if (cli.isEmpty() || !QFile::exists(cli)) {
        std::fprintf(stderr, "Set DOCKPIPE_BIN to the matching built CLI for the integration fixture.\n");
        return 77;
    }
    QTemporaryDir repository;
    VERIFY(repository.isValid());
    const bool stress = qEnvironmentVariableIsSet("DOCKPIPE_GIT_STRESS");
    const int fileCount = stress ? GitReviewModel::maximumFiles : 15;
    VERIFY(git(repository.path(), {"init", "-b", "main"}));
    VERIFY(git(repository.path(), {"config", "user.name", "Review Fixture"}));
    VERIFY(git(repository.path(), {"config", "user.email", "fixture@example.invalid"}));
    for (int index = 0; index < fileCount; ++index) {
        const int lines = index == fileCount - 1 ? 64 : 1;
        VERIFY(write(repository.filePath(QString("file-%1.cpp").arg(index, 4, 10, QChar('0'))), QByteArray("int value = 1;\n").repeated(lines)));
    }
    VERIFY(git(repository.path(), {"add", "."}));
    VERIFY(git(repository.path(), {"commit", "-m", "Fixture baseline"}));
    for (int index = 0; index < fileCount; ++index) {
        const int lines = index == fileCount - 1 ? 64 : 1;
        VERIFY(write(repository.filePath(QString("file-%1.cpp").arg(index, 4, 10, QChar('0'))), QByteArray("int value = 2;\n").repeated(lines)));
    }
    VERIFY(git(repository.path(), {"add", "file-0000.cpp"}));
    VERIFY(write(repository.filePath("file-0000.cpp"), "int value = 3;\n"));

    const bool hardware = QGuiApplication::platformName() != "offscreen";
    GitReviewWidget widget(hardware);
    widget.resize(1500, 950);
    if (hardware)
        widget.setWindowFlag(Qt::WindowStaysOnTopHint);
    widget.setWorkdir(repository.path());
    widget.show();
    VERIFY(settled(widget));
    VERIFY(!widget.model().files.isEmpty());
    std::printf("Git changed files=%lld\n", static_cast<long long>(widget.model().files.size()));
    // The hub can stay open indefinitely. Hidden GL views must not fail their
    // initialization watchdog before a user even enters the review rooms.
    idle(350);
    widget.beginReview();
    VERIFY(settled(widget) && widget.spatial() && widget.scene()->walking());
    VERIFY(widget.model().files.size() == fileCount);
    VERIFY(widget.diff().hunks.size() == 1);
    VERIFY(widget.diff().hunks[0].rows[0].before == "int value = 2;");
    VERIFY(widget.diff().hunks[0].rows[0].after == "int value = 3;");
    if (hardware) {
        widget.scene()->setFlightFullscreen(true);
        QVector<double> draws;
        QVector<double> intervals;
        QElapsedTimer frameClock;
        QElapsedTimer walkingClock;
        frameClock.start();
        walkingClock.start();
        auto lastFrame = widget.scene()->renderedFrames();
        widget.scene()->setPilotInput({0, 0, 1}, {}, true);
        while (walkingClock.elapsed() < 7000) {
            QApplication::processEvents();
            if (widget.scene()->renderedFrames() != lastFrame) {
                draws.append(widget.scene()->lastDrawNanoseconds() / 1e6);
                intervals.append(frameClock.restart());
                lastFrame = widget.scene()->renderedFrames();
            }
            QThread::msleep(1);
        }
        widget.scene()->setPilotInput({}, {}, false);
        widget.scene()->setFlightFullscreen(false);
        std::sort(draws.begin(), draws.end());
        std::sort(intervals.begin(), intervals.end());
        VERIFY(!draws.isEmpty());
        VERIFY(!widget.scene()->rendererDescription().contains("initialization failed"));
        std::printf("Git walking frames=%lld draw_p95_ms=%.3f interval_p95_ms=%.3f worst_ms=%.3f renderer=%s\n",
            static_cast<long long>(draws.size()), draws[draws.size() * 95 / 100],
            intervals[intervals.size() * 95 / 100], intervals.last(), qPrintable(widget.scene()->rendererDescription()));
        VERIFY(settled(widget));
        widget.jumpToFile(0);
        VERIFY(settled(widget));
    }
    // Stress profiling uses only the disposable fixture repository.
    if (stress)
        return 0;
    const QString capture = qEnvironmentVariable("DOCKPIPE_GIT_CAPTURE");
    auto screenshot = [&](const QString &suffix) {
        idle(100);
        if (capture.isEmpty())
            return true;
        const auto image = hardware ? widget.screen()->grabWindow(widget.winId()) : widget.grab();
        return image.save(capture + suffix);
    };
    VERIFY(screenshot("-space.png"));
    widget.scene()->destinationActivated("display:before-0", false);
    VERIFY(!widget.spatial());
    auto *before = widget.findChild<QPlainTextEdit *>("gitReviewBefore");
    auto *after = widget.findChild<QPlainTextEdit *>("gitReviewAfter");
    VERIFY(before && after);
    VERIFY(before->toPlainText() == "int value = 2;" && after->toPlainText() == "int value = 3;");
    if (hardware) {
        QWidget focusTarget;
        focusTarget.setWindowTitle("DockPipe focus regression fixture");
        focusTarget.resize(320, 180);
        for (int cycle = 0; cycle < 8; ++cycle) {
            widget.returnToSpace();
            widget.scene()->setFlightFullscreen(true);
            widget.scene()->setPilotInput({0, 0, 1}, {}, false);
            idle(80);
            focusTarget.show();
            focusTarget.raise();
            focusTarget.activateWindow();
            idle(80);
            focusTarget.hide();
            widget.raise();
            widget.activateWindow();
            widget.scene()->setFlightFullscreen(false);
            widget.jumpToFile(cycle % fileCount);
            VERIFY(settled(widget));
            widget.inspectHunk(0);
            idle(80);
            VERIFY(!before->toPlainText().isEmpty() && !after->toPlainText().isEmpty());
        }
        widget.jumpToFile(0);
        VERIFY(settled(widget));
        widget.inspectHunk(0);
        std::puts("Git fullscreen/focus/inspection transitions passed.");
    }
    widget.findChild<QPushButton *>("gitMarkReviewed")->click();
    VERIFY(!widget.findChild<QListWidget *>("gitReviewFiles")->item(0)->text().startsWith("Reviewed"));
    auto *selector = widget.findChild<QComboBox *>("gitReviewComparison");
    selector->setCurrentIndex(1);
    QMetaObject::invokeMethod(selector, "activated", Q_ARG(int, 1));
    VERIFY(settled(widget));
    VERIFY(before->toPlainText() == "int value = 1;" && after->toPlainText() == "int value = 2;");
    widget.findChild<QPushButton *>("gitMarkReviewed")->click();
    VERIFY(widget.findChild<QListWidget *>("gitReviewFiles")->item(0)->text().startsWith("Reviewed"));
    auto *notes = widget.findChild<QPlainTextEdit *>("gitReviewNotes");
    notes->insertPlainText("Check the boundary condition.");
    VERIFY(screenshot("-focus.png"));
    const auto position = widget.scene()->cameraPosition();
    widget.returnToSpace();
    VERIFY(widget.spatial() && widget.scene()->cameraPosition() == position);
    widget.inspectHunk(0);
    VERIFY(notes->toPlainText() == "Check the boundary condition.");

    widget.jumpToFile(fileCount - 1);
    VERIFY(settled(widget) && widget.spatial());
    idle(400);
    const auto stationaryFrames = widget.scene()->renderedFrames();
    idle(3400);
    VERIFY(widget.scene()->renderedFrames() > stationaryFrames + 3);

    // Every changed file is reachable. Stage through the actual shared runtime,
    // then review that exact index revision; staged and working states stay distinct.
    for (int index = 0; index < 15; ++index) {
        widget.jumpToFile(index);
        VERIFY(settled(widget) && widget.currentFile() == index);
        VERIFY(widget.diff().path == widget.model().files[index].path);
        widget.inspectHunk(0);
        widget.findChild<QPushButton *>("gitStageFile")->click();
        VERIFY(settled(widget));
        selector->setCurrentIndex(1);
        QMetaObject::invokeMethod(selector, "activated", Q_ARG(int, 1));
        VERIFY(settled(widget));
        widget.findChild<QPushButton *>("gitMarkReviewed")->click();
        VERIFY(widget.findChild<QListWidget *>("gitReviewFiles")->item(index)->text().startsWith("Reviewed"));
    }
    widget.showSummary();
    VERIFY(screenshot("-summary.png"));
    for (int index = 0; index < 15; ++index)
        VERIFY(widget.findChild<QListWidget *>("gitReviewFiles")->item(index)->text().startsWith("Reviewed"));
    VERIFY(git(repository.path(), {"diff", "--quiet"}));
    VERIFY(write(repository.filePath("file-0014.cpp"), "int value = 4;\n"));
    widget.refresh();
    VERIFY(settled(widget));
    VERIFY(widget.findChild<QListWidget *>("gitReviewFiles")->item(14)->text().startsWith("STALE"));
    widget.jumpToFile(0);
    VERIFY(settled(widget));
    widget.returnToSpace();
    // Crossing the fourth chamber's exit streams the next section, not a loop
    // back to file one. This uses the same proximity path as walking/controller input.
    widget.scene()->setSurfaceSpawn({-180, 1650 + 560});
    widget.scene()->setPilotInput({0, 0, 1}, {}, false);
    idle(150);
    VERIFY(settled(widget) && widget.currentFile() == 4);
    VERIFY(widget.scene()->moving());
    const auto continuedPosition = widget.scene()->cameraPosition();
    idle(100);
    VERIFY(widget.scene()->cameraPosition() != continuedPosition);
    widget.scene()->setPilotInput({}, {}, false);
    widget.hide();
    idle(50);
    const auto hiddenFrames = widget.scene()->renderedFrames();
    idle(250);
    VERIFY(widget.scene()->renderedFrames() == hiddenFrames && !widget.scene()->moving());
    {
        CockpitWidget cockpit;
        auto settings = cockpit.settings();
        settings.enabled = true;
        settings.appControl = false;
        settings.controller = false;
        settings.hardwareAcceleration = false;
        cockpit.applySettings(settings);
        cockpit.setWorkspace(repository.path());
        cockpit.resize(1200, 800);
        cockpit.show();
        cockpit.visit("git");
        auto *review = cockpit.findChild<GitReviewWidget *>();
        VERIFY(review && settled(*review) && review->model().root == repository.path());
        review->jumpToFile(7);
        VERIFY(settled(*review));
        cockpit.returnToOrbit();
        VERIFY(!review->isVisible() && cockpit.flight()->orbit());
        cockpit.visit("git");
        VERIFY(review->isVisible() && review->currentFile() == 7);
    }
    std::printf("Git review: parser, stale progress, bounded worlds and all 15 real changed files passed.\n");
    return 0;
}
