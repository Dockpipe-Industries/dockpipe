#include "DependencyChangeDialog.h"
#include "DockpipeChoices.h"

#include <QApplication>
#include <QElapsedTimer>
#include <QFile>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QTemporaryDir>
#include <QThread>
#include <cstdio>
#include <functional>

#define CHECK(value) do { if (!(value)) { std::fprintf(stderr, "Dependency dialog assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

// The production selector is independently covered by operator-dockpipe-binary.
QString DockpipeChoices::operatorDockpipeBinary(QString *)
{
    return qEnvironmentVariable("DOCKPIPE_TEST_CLI");
}

static bool waitUntil(const std::function<bool()> &predicate)
{
    QElapsedTimer timer;
    timer.start();
    while (timer.elapsed() < 5000) {
        QApplication::processEvents();
        if (predicate())
            return true;
        QThread::msleep(10);
    }
    return false;
}

int main(int argc, char **argv)
{
    QApplication application(argc, argv);
    QTemporaryDir directory;
    CHECK(directory.isValid());
    QFile cli(directory.filePath("fixture-cli"));
    CHECK(cli.open(QIODevice::WriteOnly));
    cli.write(R"SH(#!/bin/sh
if [ "$1" != dependency ]; then exit 1; fi
printf '%s\n' "$@" > "$DOCKPIPE_TEST_ARGS"
printf '%s\n' '{"result":{"schema":"dockpipe.dependency.change/v1","status":"ready","message":"Verified fixture"},"apply_approval":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","recovery_approval":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}'
)SH");
    cli.close();
    CHECK(cli.setPermissions(QFileDevice::ReadOwner | QFileDevice::WriteOwner | QFileDevice::ExeOwner));
    qputenv("DOCKPIPE_TEST_CLI", cli.fileName().toUtf8());
    const QString argsPath = directory.filePath("arguments");
    qputenv("DOCKPIPE_TEST_ARGS", argsPath.toUtf8());
    const QString sourcePath = directory.filePath("request.json");
    QFile source(sourcePath);
    CHECK(source.open(QIODevice::WriteOnly));
    source.write("{\"target\":\"reviewed-original\"}");
    source.close();

    DependencyChangeDialog dialog(directory.path());
    auto *apply = dialog.findChild<QPushButton *>("dependencyApply");
    auto *recover = dialog.findChild<QPushButton *>("dependencyRecover");
    auto *review = dialog.findChild<QPushButton *>("dependencyReviewUpdate");
    auto *reviewRecovery = dialog.findChild<QPushButton *>("dependencyReviewRecovery");
    CHECK(apply && recover && review && reviewRecovery);
    CHECK(!apply->isEnabled() && !recover->isEnabled() && !review->isEnabled());
    dialog.loadRequest(sourcePath);
    CHECK(review->isEnabled());
    // Mutating the original after selection cannot change the reviewed snapshot.
    CHECK(source.open(QIODevice::WriteOnly | QIODevice::Truncate));
    source.write("{\"target\":\"changed-on-disk\"}");
    source.close();
    review->click();
    CHECK(waitUntil([&]() { return apply->isEnabled(); }));
    CHECK(!recover->isEnabled());
    QFile arguments(argsPath);
    CHECK(arguments.open(QIODevice::ReadOnly));
    const QByteArray reviewArguments = arguments.readAll();
    CHECK(reviewArguments.contains("dependency\nupdate\n") && reviewArguments.contains("--dry-run"));
    CHECK(!reviewArguments.contains(sourcePath.toUtf8()));
    const auto *output = dialog.findChild<QPlainTextEdit *>();
    CHECK(output->toPlainText().contains("reviewed-original"));
    CHECK(!output->toPlainText().contains("changed-on-disk"));
    apply->click();
    CHECK(waitUntil([&]() { return review->isEnabled(); }));
    CHECK(!apply->isEnabled());
    arguments.close();
    CHECK(arguments.open(QIODevice::ReadOnly));
    CHECK(arguments.readAll().contains("--approve\naaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"));
    reviewRecovery->click();
    CHECK(waitUntil([&]() { return recover->isEnabled(); }));
    CHECK(!apply->isEnabled());
    return 0;
}
