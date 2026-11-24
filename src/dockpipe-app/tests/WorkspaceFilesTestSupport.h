#pragma once

#include "ProjectFilesWidget.h"
#include <QEventLoop>
#include <QPointer>
#include <QTimer>
#include <memory>

namespace FilesTest {
// Event-loop waits belong to fixtures only. Production callers use continuations.
inline bool await(const std::function<void(ProjectFilesWidget::Completion)> &operation)
{
    struct State {
        bool done = false;
        bool success = false;
    };
    auto state = std::make_shared<State>();
    QEventLoop loop;
    const QPointer<QEventLoop> owner(&loop);
    QTimer timer;
    timer.setSingleShot(true);
    QObject::connect(&timer, &QTimer::timeout, &loop, &QEventLoop::quit);
    timer.start(5000);
    operation([state, owner](bool success) {
        state->done = true;
        state->success = success;
        if (owner)
            owner->quit();
    });
    if (!state->done)
        loop.exec();
    return state->done && state->success;
}

inline bool open(ProjectFilesWidget &files, const QString &path)
{
    return await([&](auto completion) { files.openFileAsync(path, completion); });
}
inline bool create(ProjectFilesWidget &files, const QString &path)
{
    return await([&](auto completion) { files.createFileAsync(path, completion); });
}
inline bool rename(ProjectFilesWidget &files, const QString &path)
{
    return await([&](auto completion) { files.renameFileAsync(path, completion); });
}
inline bool setRoot(ProjectFilesWidget &files, const QString &path)
{
    return await([&](auto completion) { files.setRootAsync(path, completion); });
}
inline bool save(ProjectFilesWidget &files)
{
    return await([&](auto completion) { files.saveFileAsync(completion); });
}
inline bool saveAll(ProjectFilesWidget &files)
{
    return await([&](auto completion) { files.saveAllFilesAsync(completion); });
}
inline bool reload(ProjectFilesWidget &files)
{
    return await([&](auto completion) { files.reloadFileAsync(completion); });
}
inline bool remove(ProjectFilesWidget &files)
{
    return await([&](auto completion) { files.deleteFileAsync(completion); });
}
}
