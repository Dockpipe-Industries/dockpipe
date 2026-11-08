#include "DashboardFilePreview.h"
#include "DashboardGitPreview.h"
#include "FileEditorWorkspace.h"
#include "GitDiffWidget.h"
#include "ProjectFilesWidget.h"
#include "SourceEditor.h"

#include <QAbstractButton>
#include <QApplication>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLabel>
#include <QMessageBox>
#include <QPushButton>
#include <QTemporaryDir>
#include <QTest>
#include <QThread>
#include <QTimer>
#include <cstdio>

namespace {
bool writeBytes(const QString &path, const QByteArray &bytes)
{
    QFile file(path);
    return file.open(QIODevice::WriteOnly | QIODevice::Truncate) && file.write(bytes) == bytes.size();
}

QByteArray readBytes(const QString &path)
{
    QFile file(path);
    return file.open(QIODevice::ReadOnly) ? file.readAll() : QByteArray();
}

class ScopedEnvironment {
public:
    ScopedEnvironment(const char *name, const QByteArray &value)
        : m_name(name), m_previous(qgetenv(name)), m_wasSet(qEnvironmentVariableIsSet(name))
    {
        qputenv(m_name, value);
    }

    ~ScopedEnvironment()
    {
        if (m_wasSet)
            qputenv(m_name, m_previous);
        else
            qunsetenv(m_name);
    }

private:
    const char *m_name;
    QByteArray m_previous;
    bool m_wasSet;
};

int runGitFixture(int argc, char **argv)
{
    if (argc != 4 || QByteArray(argv[2]) != "--request")
        return 2;
    const auto request = QJsonDocument::fromJson(readBytes(QString::fromLocal8Bit(argv[3]))).object();
    QFile log(qEnvironmentVariable("DOCKPIPE_PREVIEW_FIXTURE_LOG"));
    if (!log.open(QIODevice::WriteOnly | QIODevice::Append))
        return 3;
    log.write(QJsonDocument(request).toJson(QJsonDocument::Compact) + '\n');
    log.close();
    if (QFileInfo::exists(qEnvironmentVariable("DOCKPIPE_PREVIEW_FIXTURE_PAUSE")))
        QThread::sleep(10);

    QJsonObject reply;
    const QString action = request.value("action").toString();
    if (action == "status") {
        reply.insert("snapshot", "preview-fixture-snapshot");
    } else if (action == "compare" && request.value("snapshot") == "preview-fixture-snapshot") {
        const QString path = request.value("paths").toArray().first().toString();
        const QString comparison = request.value("comparison").toString();
        reply.insert("review", QJsonObject{{"path", path}, {"before_label", "Index"},
            {"after_label", comparison}, {"patch", QString("@@ -1 +1 @@\n-before\n+%1 %2\n").arg(comparison, path)}});
    } else {
        return 4;
    }
    const auto bytes = QJsonDocument(reply).toJson(QJsonDocument::Compact);
    std::fwrite(bytes.constData(), 1, size_t(bytes.size()), stdout);
    return 0;
}
}

class DashboardPreviewTest final : public QObject {
    Q_OBJECT
private slots:
    void fileUsesScopedEditorAndSaveGuards();
    void fileRejectsOutsideLinksBinaryAndLargeInput();
    void gitLoadsScopedReadOnlyComparisons();
    void gitCancelsHiddenRequestAndCanReopen();
    void gitReportsUnavailableContext();
};

void DashboardPreviewTest::fileUsesScopedEditorAndSaveGuards()
{
    QTemporaryDir root;
    const QString path = root.filePath("notes.txt");
    QVERIFY(writeBytes(path, "original\n"));
    DashboardFilePreview preview(root.path(), "notes.txt");
    preview.show();
    QCOMPARE(preview.rootPath(), QFileInfo(root.path()).canonicalFilePath());
    QTRY_COMPARE(preview.currentPath(), QFileInfo(path).canonicalFilePath());
    auto *files = preview.findChild<ProjectFilesWidget *>();
    auto *workspace = preview.findChild<FileEditorWorkspace *>();
    QVERIFY(files && workspace && workspace->activeEditor());
    QVERIFY(preview.findChild<QWidget *>("filesExplorerPanel")->isHidden());
    auto *editor = workspace->activeEditor();
    QVERIFY(!editor->isReadOnly());
    editor->selectAll();
    editor->insertPlainText("draft\n");
    QVERIFY(preview.hasUnsavedChanges());

    QTimer answer;
    connect(&answer, &QTimer::timeout, []() {
        for (auto *widget : QApplication::topLevelWidgets()) {
            auto *dialog = qobject_cast<QMessageBox *>(widget);
            if (dialog && dialog->isVisible())
                dialog->button(QMessageBox::Cancel)->click();
        }
    });
    answer.start(10);
    int leaveResult = -1;
    preview.confirmLeaveAsync([&leaveResult](bool accepted) { leaveResult = accepted ? 1 : 0; });
    QTRY_COMPARE(leaveResult, 0);
    answer.stop();
    QCOMPARE(editor->toPlainText(), QString("draft\n"));
    int saveResult = -1;
    files->saveFileAsync([&saveResult](bool accepted) { saveResult = accepted ? 1 : 0; });
    QTRY_COMPARE(saveResult, 1);
    QCOMPARE(readBytes(path), QByteArray("draft\n"));
    QVERIFY(!preview.hasUnsavedChanges());

    editor->insertPlainText("unsaved");
    QVERIFY(writeBytes(path, "changed externally\n"));
    saveResult = -1;
    files->saveFileAsync([&saveResult](bool accepted) { saveResult = accepted ? 1 : 0; });
    QTRY_COMPARE(saveResult, 0);
    QCOMPARE(readBytes(path), QByteArray("changed externally\n"));
    QVERIFY(preview.hasUnsavedChanges());
}

void DashboardPreviewTest::fileRejectsOutsideLinksBinaryAndLargeInput()
{
    QTemporaryDir root;
    QTemporaryDir outside;
    QVERIFY(writeBytes(outside.filePath("private.txt"), "outside\n"));
    DashboardFilePreview escaped(root.path(), outside.filePath("private.txt"));
    auto *escapedFiles = escaped.findChild<ProjectFilesWidget *>();
    QVERIFY(escapedFiles);
    QTRY_VERIFY(!escapedFiles->filesOperationPending());
    QVERIFY(escaped.currentPath().isEmpty());
    QVERIFY(writeBytes(root.filePath("binary"), QByteArray("a\0b", 3)));
    DashboardFilePreview binary(root.path(), "binary");
    auto *binaryFiles = binary.findChild<ProjectFilesWidget *>();
    QVERIFY(binaryFiles);
    QTRY_VERIFY(!binaryFiles->filesOperationPending());
    QVERIFY(binary.currentPath().isEmpty());
    QVERIFY(writeBytes(root.filePath("large"), QByteArray(1024 * 1024 + 1, 'x')));
    DashboardFilePreview large(root.path(), "large");
    auto *largeFiles = large.findChild<ProjectFilesWidget *>();
    QVERIFY(largeFiles);
    QTRY_VERIFY(!largeFiles->filesOperationPending());
    QVERIFY(large.currentPath().isEmpty());
#ifdef Q_OS_UNIX
    QVERIFY(QFile::link(outside.filePath("private.txt"), root.filePath("linked.txt")));
    DashboardFilePreview linked(root.path(), "linked.txt");
    auto *linkedFiles = linked.findChild<ProjectFilesWidget *>();
    QVERIFY(linkedFiles);
    QTRY_VERIFY(!linkedFiles->filesOperationPending());
    QVERIFY(linked.currentPath().isEmpty());
#endif
}

void DashboardPreviewTest::gitLoadsScopedReadOnlyComparisons()
{
    QTemporaryDir root;
    ScopedEnvironment binary("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
    ScopedEnvironment log("DOCKPIPE_PREVIEW_FIXTURE_LOG", root.filePath("requests.jsonl").toUtf8());
    DashboardGitPreview unstaged(root.path(), "src/first.cpp", false);
    DashboardGitPreview staged(root.path(), "src/second.cpp", true);
    unstaged.show();
    staged.show();
    auto *firstAfter = unstaged.findChild<QPlainTextEdit *>("gitDiffAfterEditor");
    auto *secondAfter = staged.findChild<QPlainTextEdit *>("gitDiffAfterEditor");
    QVERIFY(firstAfter && secondAfter);
    QTRY_VERIFY(firstAfter->toPlainText().contains("unstaged src/first.cpp"));
    QTRY_VERIFY(secondAfter->toPlainText().contains("staged src/second.cpp"));
    QVERIFY(firstAfter->isReadOnly() && secondAfter->isReadOnly());
    QCOMPARE(staged.rootPath(), QFileInfo(root.path()).canonicalFilePath());
    QCOMPARE(staged.currentPath(), QString("src/second.cpp"));
    QVERIFY(staged.isStaged());
    const auto requests = readBytes(root.filePath("requests.jsonl")).trimmed().split('\n');
    QCOMPARE(requests.size(), 4);
    for (const auto &bytes : requests) {
        const auto request = QJsonDocument::fromJson(bytes).object();
        const auto action = request.value("action").toString();
        QVERIFY(action == "status" || action == "compare");
        QCOMPARE(request.value("workdir").toString(), staged.rootPath());
    }
}

void DashboardPreviewTest::gitCancelsHiddenRequestAndCanReopen()
{
    QTemporaryDir root;
    const QString pausePath = root.filePath("pause");
    QVERIFY(writeBytes(pausePath, "pause"));
    ScopedEnvironment binary("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
    ScopedEnvironment log("DOCKPIPE_PREVIEW_FIXTURE_LOG", root.filePath("requests.jsonl").toUtf8());
    ScopedEnvironment pause("DOCKPIPE_PREVIEW_FIXTURE_PAUSE", pausePath.toUtf8());
    DashboardGitPreview preview(root.path(), "first.cpp", false);
    preview.show();
    QTRY_VERIFY(!readBytes(root.filePath("requests.jsonl")).isEmpty());
    preview.hide();
    QVERIFY(QFile::remove(pausePath));
    preview.show();
    auto *after = preview.findChild<QPlainTextEdit *>("gitDiffAfterEditor");
    QTRY_VERIFY(after->toPlainText().contains("unstaged first.cpp"));
}

void DashboardPreviewTest::gitReportsUnavailableContext()
{
    QTemporaryDir root;
    DashboardGitPreview missing(root.filePath("missing"), "first.cpp", false);
    missing.refresh();
    QVERIFY(missing.findChild<QLabel *>("dashboardGitPreviewStatus")->text().contains("unavailable"));
    DashboardGitPreview escaped(root.path(), "../outside.cpp", false);
    escaped.refresh();
    QVERIFY(escaped.findChild<QLabel *>("dashboardGitPreviewStatus")->text().contains("inside"));
    ScopedEnvironment binary("DOCKPIPE_BIN", root.filePath("missing-cli").toUtf8());
    DashboardGitPreview failed(root.path(), "first.cpp", false);
    failed.show();
    QTRY_VERIFY(failed.findChild<QLabel *>("dashboardGitPreviewStatus")->text().contains("DOCKPIPE_BIN"));
    QVERIFY(failed.findChild<QPlainTextEdit *>("gitDiffAfterEditor")->toPlainText().isEmpty());
}

int main(int argc, char **argv)
{
    if (argc > 1 && QByteArray(argv[1]) == "git" && qEnvironmentVariableIsSet("DOCKPIPE_PREVIEW_FIXTURE_LOG"))
        return runGitFixture(argc, argv);
    QTemporaryDir configuration;
    qputenv("XDG_CONFIG_HOME", configuration.path().toUtf8());
    QApplication app(argc, argv);
    DashboardPreviewTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "DashboardPreviewTest.moc"
