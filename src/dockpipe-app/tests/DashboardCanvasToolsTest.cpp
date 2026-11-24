#include "ContextStore.h"
#include "DashboardFilePreview.h"
#include "DashboardPage.h"
#include "DashboardRepositoryPanel.h"
#include "DashboardStore.h"
#include "DashboardWidgetFactory.h"
#include "DashboardFilesWidget.h"
#include "DashboardGitPreview.h"
#include "WorkspaceFileModel.h"
#include "WorkspaceFilesController.h"
#include "WorkspaceMachineMenu.h"
#include "FileEditorWorkspace.h"
#include "DashboardWorkspace.h"
#include "ProjectFilesWidget.h"
#include "ProjectGitWidget.h"
#include "SessionManager.h"
#include "Theme.h"

#include <QApplication>
#include <QCryptographicHash>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QJsonObject>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QLockFile>
#include <QMessageBox>
#include <QPlainTextEdit>
#include <QPixmap>
#include <QPushButton>
#include <QTemporaryDir>
#include <QScopeGuard>
#include <QTest>
#include <QThread>
#include <QTimer>
#include <QTableWidget>
#include <QTextDocument>
#include <memory>
#include <cstdio>

namespace {
bool appendMachineFixtureCall(const QJsonObject &call)
{
    const QString path = qEnvironmentVariable("DOCKPIPE_DASHBOARD_MACHINE_LOG");
    QLockFile lock(path + ".lock");
    if (!lock.tryLock(3000))
        return false;

    QFile log(path);
    const QByteArray line = QJsonDocument(call).toJson(QJsonDocument::Compact) + '\n';
    return log.open(QIODevice::WriteOnly | QIODevice::Append) && log.write(line) == line.size() && log.flush();
}

int runMachineFixture(int argc, char **argv)
{
    QStringList arguments;
    for (int index = 1; index < argc; ++index)
        arguments.append(QString::fromLocal8Bit(argv[index]));
    const bool preparation = arguments.value(1) == "feature-prepare";
    const bool remote = arguments.value(0) == "remote"
        && (preparation || arguments.value(1) == "feature-call");
    if (!remote && arguments.value(0) != "files")
        return appendMachineFixtureCall({{"unexpected_local", arguments.value(0)}}) ? 3 : 2;
    const int requestIndex = arguments.indexOf("--request");
    if (requestIndex < 0)
        return 4;
    QFile input;
    const QString requestPath = arguments.value(requestIndex + 1);
    if (requestPath == "-") {
        if (!input.open(stdin, QIODevice::ReadOnly))
            return 5;
    } else {
        input.setFileName(requestPath);
        if (!input.open(QIODevice::ReadOnly))
            return 5;
    }
    auto request = QJsonDocument::fromJson(input.readAll()).object();
    if (!appendMachineFixtureCall({{"remote", remote}, {"request", request}}))
        return 2;
    if (remote && preparation) {
        QJsonObject reference;
        for (const auto &key : {"schema", "target", "root", "action", "deadline"})
            reference.insert(key, request.value(key));
        reference.insert("id", QString(64, 'f'));
        const auto bytes = QJsonDocument(reference).toJson(QJsonDocument::Compact);
        std::fwrite(bytes.constData(), 1, size_t(bytes.size()), stdout);
        return 0;
    }
    const QString node = remote ? request.value("target").toObject().value("node").toString() : QStringLiteral("local");
    const QString action = remote ? request.value("action").toString()
        : "files." + request.value("request").toObject().value("action").toString();
    const auto payload = remote ? request.value("payload").toObject() : request.value("request").toObject();
    QJsonObject value;
    if (action == "files.list") {
        value = {{"schema", "dockpipe.files/v1"}, {"path", payload.value("path")}, {"revision", QString(64, 'a')},
            {"entries", QJsonArray{QJsonObject{{"name", node + "-remote.txt"}, {"kind", "file"}, {"size_bytes", "9"}}}}};
    } else if (action == "files.read") {
        const QByteArray bytes = ("remote from " + node + '\n').toUtf8();
        value = {{"schema", "dockpipe.files/v1"}, {"path", payload.value("path")},
            {"revision", QString::fromLatin1(QCryptographicHash::hash(bytes, QCryptographicHash::Sha256).toHex())},
            {"content", QString::fromLatin1(bytes.toBase64())}};
    } else if (action == "git.status") {
        value = {{"root", "/"}, {"branch", node}, {"snapshot", node + "-snapshot"},
            {"changes", QJsonArray{QJsonObject{{"path", "remote.txt"}, {"status", " M"}}}}};
    } else if (action == "git.compare") {
        value = {{"review", QJsonObject{{"path", "remote.txt"}, {"patch", "@@ -1 +1 @@\n-before\n+after\n"}}}};
    } else if (action == "containers.snapshot") {
        value = {{"available", true}, {"containers", QJsonArray{QJsonObject{{"Names", node}, {"State", "running"}, {"Image", "fixture"}}}}};
    } else {
        return 6;
    }
    QJsonObject output = value;
    if (remote) {
        request.insert("id", QString(64, 'f'));
        output = {{"request", request}, {"status", "success"},
            {"result", QJsonObject{{"status", "success"}, {"payload", value}}}};
    }
    const auto bytes = QJsonDocument(output).toJson(QJsonDocument::Compact);
    std::fwrite(bytes.constData(), 1, size_t(bytes.size()), stdout);
    return 0;
}

QJsonObject machineCatalog(const QString &node, QChar identity)
{
    return {{"target", QJsonObject{{"node", node}, {"fingerprint", QString(64, identity)},
        {"generation", QString(64, QChar(identity.unicode() + 1))}}}, {"docker_read", true},
        {"roots", QJsonArray{QJsonObject{{"id", node + "-root"}, {"name", node + " project"}}}}};
}

class MachineFixtureEnvironment final {
public:
    explicit MachineFixtureEnvironment(const QString &directory)
    {
        for (const auto &name : {QByteArray("DOCKPIPE_BIN"), QByteArray("DOCKPIPE_DASHBOARD_MACHINE_LOG"), QByteArray("PATH")})
            m_saved.append({name, qgetenv(name.constData()), qEnvironmentVariableIsSet(name.constData())});
        qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
        qputenv("DOCKPIPE_DASHBOARD_MACHINE_LOG", (directory + "/requests.jsonl").toUtf8());
        qputenv("PATH", directory.toUtf8() + QDir::listSeparator().toLatin1() + qgetenv("PATH"));
#ifdef Q_OS_WIN
        ready = QFile::copy(QCoreApplication::applicationFilePath(), directory + "/docker.exe");
#else
        ready = QFile::link(QCoreApplication::applicationFilePath(), directory + "/docker");
#endif
    }

    ~MachineFixtureEnvironment()
    {
        for (const auto &entry : m_saved) {
            if (entry.existed)
                qputenv(entry.name.constData(), entry.value);
            else
                qunsetenv(entry.name.constData());
        }
    }

    bool ready = false;

private:
    struct Entry {
        QByteArray name;
        QByteArray value;
        bool existed;
    };
    QVector<Entry> m_saved;
};

int runGitFixture(int argc, char **argv)
{
    if (argc != 4 || QByteArray(argv[2]) != "--request")
        return 2;
    QFile input(QString::fromLocal8Bit(argv[3]));
    if (!input.open(QIODevice::ReadOnly))
        return 3;
    const auto request = QJsonDocument::fromJson(input.readAll()).object();
    const QString root = request.value("workdir").toString();
    QFile log(qEnvironmentVariable("DOCKPIPE_CANVAS_GIT_FIXTURE_LOG"));
    if (!log.open(QIODevice::WriteOnly | QIODevice::Append))
        return 4;
    log.write(QJsonDocument(request).toJson(QJsonDocument::Compact) + '\n');
    log.close();
    QThread::msleep(150);
    if (QFileInfo::exists(root + "/fail"))
        return 5;
    const QString path = QFileInfo(root).fileName() + ".cpp";
    QJsonObject reply;
    if (request.value("action") == "status") {
        reply = {{"root", root}, {"branch", "fixture"}, {"snapshot", root},
            {"changes", QJsonArray{QJsonObject{{"path", path}, {"status", " M"}}}}};
    } else if (request.value("action") == "compare" && request.value("snapshot") == root) {
        reply = {{"review", QJsonObject{{"path", path}, {"patch", "@@ -1 +1 @@\n-before\n+after\n"}}}};
    } else {
        return 6;
    }
    const auto bytes = QJsonDocument(reply).toJson(QJsonDocument::Compact);
    std::fwrite(bytes.constData(), 1, size_t(bytes.size()), stdout);
    return 0;
}

DashboardDefinition emptyDashboard()
{
    DashboardDefinition definition;
    definition.id = DashboardStore::newId();
    definition.name = "Canvas tools fixture";
    return definition;
}

bool writeFile(const QString &path, const QByteArray &content)
{
    QFile file(path);
    return file.open(QIODevice::WriteOnly) && file.write(content) == content.size();
}

DashboardRepositoryPanel *repositoryPanel(DashboardPage &page, const QString &id)
{
    auto *frame = page.findChild<QWidget *>("dashboardPanel_" + id);
    return frame ? frame->findChild<DashboardRepositoryPanel *>() : nullptr;
}

void cancelNextConfirmation(bool &observed)
{
    QTimer::singleShot(0, [&observed]() {
        auto *confirmation = qobject_cast<QMessageBox *>(QApplication::activeModalWidget());
        observed = confirmation != nullptr;
        if (confirmation)
            confirmation->done(QMessageBox::Cancel);
    });
}
}

class DashboardCanvasToolsTest final : public QObject {
    Q_OBJECT
private slots:
    void independentRepositoriesDuplicateAndPersist();
    void requestsStayOnOriginatingPage();
    void cancelledPreviewClosePreservesDraft();
    void unavailableRepositoryAndFilePreserveContext();
    void commitDraftSurvivesCancelledAggregateLeave();
    void closingPreviewRemovesSavedContext();
    void gitFullViewsLoadIndependentRepositoriesAndPreserveDrafts();
    void gitFullViewFailedLoadRemainsUnknown();
    void machineOverridesPersistWithoutWorkerAdoption();
    void remotePreviewsStayPinnedAndDraftGateIsPure();
};

void DashboardCanvasToolsTest::gitFullViewsLoadIndependentRepositoriesAndPreserveDrafts()
{
    QTemporaryDir firstDirectory;
    QTemporaryDir secondDirectory;
    QTemporaryDir fixture;
    const QByteArray previousBinary = qgetenv("DOCKPIPE_BIN");
    const bool binaryWasSet = qEnvironmentVariableIsSet("DOCKPIPE_BIN");
    const auto restoreEnvironment = qScopeGuard([&]() {
        if (binaryWasSet)
            qputenv("DOCKPIPE_BIN", previousBinary);
        else
            qunsetenv("DOCKPIPE_BIN");
        qunsetenv("DOCKPIPE_CANVAS_GIT_FIXTURE_LOG");
    });
    qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
    const QString logPath = fixture.filePath("requests.jsonl");
    qputenv("DOCKPIPE_CANVAS_GIT_FIXTURE_LOG", logPath.toUtf8());
    const QString firstRoot = QFileInfo(firstDirectory.path()).canonicalFilePath();
    const QString secondRoot = QFileInfo(secondDirectory.path()).canonicalFilePath();
    DashboardRepositoryPanel first("git-full", firstRoot);
    DashboardRepositoryPanel second("git-full", secondRoot);
    auto *firstGit = first.findChild<ProjectGitWidget *>();
    auto *secondGit = second.findChild<ProjectGitWidget *>();
    QVERIFY(firstGit && secondGit);
    QVERIFY(firstGit->busy() && secondGit->busy());
    QVERIFY(!first.setRepository(secondRoot));
    QCOMPARE(first.repository(), firstRoot);
    QCOMPARE(firstGit->findChild<QLabel *>("gitChangedCount")->text(), QString::fromUtf8("—"));
    QTRY_VERIFY(!firstGit->busy() && !secondGit->busy());
    auto *firstFiles = firstGit->findChild<QListWidget *>("gitChangedFiles");
    auto *secondFiles = secondGit->findChild<QListWidget *>("gitChangedFiles");
    QCOMPARE(firstFiles->count(), 1);
    QCOMPARE(secondFiles->count(), 1);
    QCOMPARE(firstFiles->item(0)->text(), QFileInfo(firstRoot).fileName() + ".cpp");
    QCOMPARE(secondFiles->item(0)->text(), QFileInfo(secondRoot).fileName() + ".cpp");

    auto *message = firstGit->findChild<QLineEdit *>("gitCommitMessage");
    message->setText("Keep this draft");
    QFile log(logPath);
    QVERIFY(log.open(QIODevice::ReadOnly));
    const auto originalRequests = log.readAll();
    QVERIFY(first.restoreContext(first.context()));
    first.show();
    first.hide();
    first.show();
    QTest::qWait(200);
    QCOMPARE(message->text(), QString("Keep this draft"));
    log.seek(0);
    QCOMPARE(log.readAll(), originalRequests);
    message->clear();

    QVERIFY(first.setRepository(secondRoot));
    QTRY_VERIFY(!firstGit->busy());
    QCOMPARE(firstFiles->item(0)->text(), QFileInfo(secondRoot).fileName() + ".cpp");
    DashboardRepositoryPanel restored("git-full", {});
    QVERIFY(restored.restoreContext({{"root", firstRoot}}));
    auto *restoredGit = restored.findChild<ProjectGitWidget *>();
    QTRY_VERIFY(!restoredGit->busy());
    QCOMPARE(restoredGit->findChild<QListWidget *>("gitChangedFiles")->item(0)->text(),
        QFileInfo(firstRoot).fileName() + ".cpp");
}

void DashboardCanvasToolsTest::gitFullViewFailedLoadRemainsUnknown()
{
    QTemporaryDir directory;
    const QByteArray previousBinary = qgetenv("DOCKPIPE_BIN");
    const bool binaryWasSet = qEnvironmentVariableIsSet("DOCKPIPE_BIN");
    const auto restoreEnvironment = qScopeGuard([&]() {
        if (binaryWasSet)
            qputenv("DOCKPIPE_BIN", previousBinary);
        else
            qunsetenv("DOCKPIPE_BIN");
        qunsetenv("DOCKPIPE_CANVAS_GIT_FIXTURE_LOG");
    });
    qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
    qputenv("DOCKPIPE_CANVAS_GIT_FIXTURE_LOG", directory.filePath("requests.jsonl").toUtf8());
    QVERIFY(writeFile(directory.filePath("fail"), "fail"));
    DashboardRepositoryPanel panel("git-full", directory.path());
    auto *git = panel.findChild<ProjectGitWidget *>();
    QTRY_VERIFY(!git->busy());
    QVERIFY(git->findChild<QLabel *>("gitStatus")->text().contains("failed"));
    QCOMPARE(git->findChild<QLabel *>("gitChangedCount")->text(), QString::fromUtf8("—"));
    QVERIFY(git->findChild<QLabel *>("gitEmptyTitle")->text() != "All caught up");
    QVERIFY(QFile::remove(directory.filePath("fail")));
    git->refresh();
    QTRY_VERIFY(!git->busy());
    QCOMPARE(git->findChild<QLabel *>("gitChangedCount")->text(), QString("1"));
    QVERIFY(writeFile(directory.filePath("fail"), "fail"));
    git->refresh();
    QTRY_VERIFY(!git->busy());
    QCOMPARE(git->findChild<QLabel *>("gitChangedCount")->text(), QString::fromUtf8("—"));
}

void DashboardCanvasToolsTest::independentRepositoriesDuplicateAndPersist()
{
    QTemporaryDir firstDirectory;
    QTemporaryDir secondDirectory;
    QVERIFY(firstDirectory.isValid() && secondDirectory.isValid());
    const QString firstRoot = QFileInfo(firstDirectory.path()).canonicalFilePath();
    const QString secondRoot = QFileInfo(secondDirectory.path()).canonicalFilePath();
    const QString document = secondRoot + "/document.txt";
    QVERIFY(writeFile(document, "Persistent panel context\n"));
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    factory.setWorkdir(firstRoot);
    DashboardPage page(emptyDashboard(), &factory);
    QVERIFY(page.addWidgetKind("files-full"));
    QVERIFY(page.addWidgetKind("files-full"));
    const auto initial = page.snapshot();
    auto *first = repositoryPanel(page, initial.panels.at(0).id);
    auto *second = repositoryPanel(page, initial.panels.at(1).id);
    QVERIFY(first && second && first != second);
    QCOMPARE(first->repository(), firstRoot);
    QCOMPARE(second->repository(), firstRoot);
    QVERIFY(second->setRepository(secondRoot));
    auto *secondFiles = second->findChild<ProjectFilesWidget *>();
    QVERIFY(secondFiles);
    secondFiles->openFileAsync(document);
    QTRY_COMPARE(secondFiles->currentPath(), document);
    QCOMPARE(first->repository(), firstRoot);
    factory.setWorkdir(secondRoot);
    QCOMPARE(first->repository(), firstRoot);
    QCOMPARE(second->repository(), secondRoot);

    auto *duplicate = second->findChild<QPushButton *>("dashboardDuplicateView");
    QVERIFY(duplicate);
    duplicate->click();
    QCOMPARE(page.panelCount(), 3);
    const auto duplicated = page.snapshot();
    auto *third = repositoryPanel(page, duplicated.panels.last().id);
    QVERIFY(third && third != second);
    QTRY_COMPARE(third->context(), second->context());
    QVERIFY(third->setRepository(firstRoot));
    QCOMPARE(second->repository(), secondRoot);
    QCOMPARE(secondFiles->currentPath(), document);

    const QString imageDirectory = qEnvironmentVariable("DOCKPIPE_CANVAS_TOOLS_IMAGES");
    if (!imageDirectory.isEmpty()) {
        QVERIFY(QDir().mkpath(imageDirectory));
        page.resize(1600, 1000);
        page.show();
        QTest::qWait(100);
        QVERIFY(page.grab().save(QDir(imageDirectory).filePath("independent-files-panels.png")));
        page.hide();
    }

    const auto saved = page.snapshot();
    const auto panelContexts = saved.layout.value("panelContexts").toObject();
    QCOMPARE(panelContexts.value(initial.panels.at(0).id).toObject().value("root").toString(), firstRoot);
    QCOMPARE(panelContexts.value(initial.panels.at(1).id).toObject().value("path").toString(), document);
    DashboardStore store;
    QVector<DashboardDefinition> definitions;
    QString error;
    QVERIFY2(store.load(definitions, error), qPrintable(error));
    QVERIFY2(store.save({saved}, error), qPrintable(error));
    DashboardStore reader;
    QVERIFY2(reader.load(definitions, error), qPrintable(error));
    QCOMPARE(definitions.size(), 1);
    QCOMPARE(definitions.first().layout.value("panelContexts"), saved.layout.value("panelContexts"));
    factory.setWorkdir(secondRoot);
    DashboardPage restored(definitions.first(), &factory);
    auto *restoredFirst = repositoryPanel(restored, initial.panels.at(0).id);
    auto *restoredSecond = repositoryPanel(restored, initial.panels.at(1).id);
    auto *restoredThird = repositoryPanel(restored, duplicated.panels.last().id);
    QVERIFY(restoredFirst && restoredSecond && restoredThird);
    QCOMPARE(restoredFirst->repository(), firstRoot);
    QCOMPARE(restoredSecond->repository(), secondRoot);
    QTRY_COMPARE(restoredSecond->findChild<ProjectFilesWidget *>()->currentPath(), document);
    QCOMPARE(restoredThird->repository(), firstRoot);
}

void DashboardCanvasToolsTest::requestsStayOnOriginatingPage()
{
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString root = QFileInfo(directory.path()).canonicalFilePath();
    const QString document = root + "/document.txt";
    QVERIFY(writeFile(document, "Scoped preview\n"));
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    factory.setWorkdir(root);
    DashboardPage first(emptyDashboard(), &factory);
    DashboardPage second(emptyDashboard(), &factory);
    auto *firstSource = new QWidget(&first);
    auto *secondSource = new QWidget(&second);
    QWidget unrelated;
    factory.filePreviewRequested(firstSource, root, document);
    QCOMPARE(first.panelCount(), 1);
    QCOMPARE(second.panelCount(), 0);
    factory.filePreviewRequested(firstSource, root, document);
    QCOMPARE(first.panelCount(), 1);
    auto *preview = first.findChild<DashboardFilePreview *>();
    QVERIFY(preview);
    QCOMPARE(preview->rootPath(), root);
    QTRY_COMPARE(preview->currentPath(), document);
    factory.filePreviewRequested(secondSource, root, document);
    QCOMPARE(second.panelCount(), 1);
    factory.filePreviewRequested(&unrelated, root, document);
    QCOMPARE(first.panelCount(), 1);
    QCOMPARE(second.panelCount(), 1);

    // Hidden previews do not start the CLI. Distinct staged selections retain
    // their exact originating context without touching repository state.
    factory.gitPreviewRequested(firstSource, root, "document.txt", false);
    factory.gitPreviewRequested(firstSource, root, "document.txt", true);
    QCOMPARE(first.panelCount(), 3);
    QCOMPARE(second.panelCount(), 1);
    const auto snapshot = first.snapshot();
    const auto panelContexts = snapshot.layout.value("panelContexts").toObject();
    const auto unstaged = panelContexts.value(snapshot.panels.at(1).id).toObject();
    const auto staged = panelContexts.value(snapshot.panels.at(2).id).toObject();
    QCOMPARE(unstaged.value("root").toString(), root);
    QCOMPARE(unstaged.value("path").toString(), QString("document.txt"));
    QVERIFY(!unstaged.value("staged").toBool());
    QVERIFY(staged.value("staged").toBool());

    factory.canvasViewRequested(secondSource, "files-full", root);
    QCOMPARE(first.panelCount(), 3);
    QCOMPARE(second.panelCount(), 2);
    auto *full = repositoryPanel(second, second.snapshot().panels.last().id);
    QVERIFY(full);
    QCOMPARE(full->repository(), root);
}

void DashboardCanvasToolsTest::cancelledPreviewClosePreservesDraft()
{
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString root = QFileInfo(directory.path()).canonicalFilePath();
    const QString document = root + "/document.txt";
    const QByteArray original("Original bytes\n");
    QVERIFY(writeFile(document, original));
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    factory.setWorkdir(root);
    DashboardPage page(emptyDashboard(), &factory);
    auto *source = new QWidget(&page);
    factory.filePreviewRequested(source, root, document);
    auto *preview = page.findChild<DashboardFilePreview *>();
    QVERIFY(preview);
    auto *editor = preview->findChild<QPlainTextEdit *>("fileTextEditor");
    QVERIFY(editor);
    QTRY_COMPARE(preview->currentPath(), document);
    editor->insertPlainText("Unsaved draft: ");
    QVERIFY(preview->hasUnsavedChanges());
    const QString draft = editor->toPlainText();
    auto *canvas = page.findChild<DashboardWorkspace *>("dashboardCanvasWorkspace");
    QVERIFY(canvas);
    const QString id = page.snapshot().panels.first().id;
    bool closePrompt = false;
    cancelNextConfirmation(closePrompt);
    canvas->panelCloseRequested(id);
    QVERIFY(closePrompt);
    QCOMPARE(page.panelCount(), 1);
    QCOMPARE(page.findChild<DashboardFilePreview *>(), preview);
    QCOMPARE(editor->toPlainText(), draft);
    QVERIFY(preview->hasUnsavedChanges());
    bool leavePrompt = false;
    cancelNextConfirmation(leavePrompt);
    int leaveResult = -1;
    page.confirmLeaveAsync([&leaveResult](bool accepted) { leaveResult = accepted ? 1 : 0; });
    QTRY_COMPARE(leaveResult, 0);
    QVERIFY(leavePrompt);
    QCOMPARE(editor->toPlainText(), draft);
    QFile file(document);
    QVERIFY(file.open(QIODevice::ReadOnly));
    QCOMPARE(file.readAll(), original);
}

void DashboardCanvasToolsTest::unavailableRepositoryAndFilePreserveContext()
{
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString root = QFileInfo(directory.path()).canonicalFilePath();
    const QString document = root + "/document.txt";
    QVERIFY(writeFile(document, "Retained document\n"));
    DashboardRepositoryPanel panel("files-full", root);
    auto *files = panel.findChild<ProjectFilesWidget *>();
    QVERIFY(files);
    files->openFileAsync(document);
    QTRY_COMPARE(files->currentPath(), document);
    const auto original = panel.context();
    QVERIFY(!panel.setRepository(root + "/missing"));
    QCOMPARE(panel.context(), original);
    auto *error = panel.findChild<QLabel *>("dashboardRepositoryError");
    QVERIFY(error && !error->isHidden() && !error->text().isEmpty());
    QVERIFY(!panel.restoreContext({{"root", root}, {"path", root + "/missing.txt"}}));
    QCOMPARE(panel.context(), original);
}

void DashboardCanvasToolsTest::commitDraftSurvivesCancelledAggregateLeave()
{
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    DashboardRepositoryPanel panel("git-full", {});
    auto *message = panel.findChild<QLineEdit *>("gitCommitMessage");
    QVERIFY(message);
    const QString draft = "Keep this commit draft until the operation completes";
    message->setText(draft);
    bool cancelledLeave = false;
    cancelNextConfirmation(cancelledLeave);
    QVERIFY(!panel.confirmLeave());
    QVERIFY(cancelledLeave);
    QCOMPARE(message->text(), draft);
    bool discardedLeave = false;
    QTimer::singleShot(0, [&discardedLeave]() {
        auto *confirmation = qobject_cast<QMessageBox *>(QApplication::activeModalWidget());
        discardedLeave = confirmation != nullptr;
        if (confirmation)
            confirmation->done(QMessageBox::Discard);
    });
    QVERIFY(panel.confirmLeave());
    QVERIFY(discardedLeave);
    QCOMPARE(message->text(), draft);
    bool cancelledSwitch = false;
    cancelNextConfirmation(cancelledSwitch);
    QVERIFY(!panel.setRepository(directory.path()));
    QVERIFY(cancelledSwitch);
    QVERIFY(panel.repository().isEmpty());
    QCOMPARE(message->text(), draft);
}

void DashboardCanvasToolsTest::closingPreviewRemovesSavedContext()
{
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString root = QFileInfo(directory.path()).canonicalFilePath();
    const QString document = root + "/document.txt";
    QVERIFY(writeFile(document, "Clean preview\n"));
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    factory.setWorkdir(root);
    DashboardPage page(emptyDashboard(), &factory);
    auto *source = new QWidget(&page);
    factory.filePreviewRequested(source, root, document);
    QCOMPARE(page.panelCount(), 1);
    auto *preview = page.findChild<DashboardFilePreview *>();
    QVERIFY(preview);
    QTRY_COMPARE(preview->currentPath(), document);
    const QString id = page.snapshot().panels.first().id;
    QVERIFY(page.snapshot().layout.value("panelContexts").toObject().contains(id));
    auto *workspace = page.findChild<DashboardWorkspace *>("dashboardCanvasWorkspace");
    QVERIFY(workspace);
    workspace->panelCloseRequested(id);
    QTRY_COMPARE(page.panelCount(), 0);
    const auto closed = page.snapshot();
    QVERIFY(!closed.layout.value("panelContexts").toObject().contains(id));
    QVERIFY(page.applyOptions(closed.options));
    QString error;
    QVERIFY2(DashboardStore::validate({page.snapshot()}, error), qPrintable(error));
}

void DashboardCanvasToolsTest::machineOverridesPersistWithoutWorkerAdoption()
{
    QTemporaryDir fixture;
    MachineFixtureEnvironment environment(fixture.path());
    QVERIFY(environment.ready);
    const auto first = machineCatalog("worker-a", 'a');
    const auto second = machineCatalog("worker-b", 'b');
    const auto third = machineCatalog("worker-c", 'c');
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    factory.setWorkdir(fixture.path());
    factory.setMachineCatalog({first, second, third});
    QString error;
    QVERIFY2(factory.setSharedMachineContext(first, "worker-a-root", &error), qPrintable(error));
    DashboardPage page(emptyDashboard(), &factory);
    QVERIFY(page.addWidgetKind("files"));
    QVERIFY(page.addWidgetKind("files"));
    QVERIFY(page.addWidgetKind("git"));
    QVERIFY(page.addWidgetKind("containers"));
    page.resize(1200, 850);
    page.show();
    auto browsers = page.findChildren<DashboardFilesWidget *>();
    QCOMPARE(browsers.size(), 2);
    auto *following = browsers[0];
    auto *overridden = browsers[1];
    QTRY_COMPARE(following->currentDirectory(), QString("/"));
    QTRY_COMPARE(overridden->currentDirectory(), QString("/"));
    auto *git = page.findChild<QWidget *>("dashboardWidget_git");
    auto *containers = page.findChild<QWidget *>("dashboardWidget_containers");
    QVERIFY(git && containers);
    QTRY_COMPARE(git->findChild<QLabel *>("dashboardGitBranch")->text(), QString("worker-a"));
    QTRY_COMPARE(containers->findChild<QTableWidget *>("dashboardContainers")->rowCount(), 1);
    const QJsonObject override{{"mode", "remote"}, {"target", second.value("target")}, {"rootId", "worker-b-root"}};
    QVERIFY2(factory.setWidgetMachineContext(overridden, override, &error), qPrintable(error));
    QVERIFY2(factory.setWidgetMachineContext(git, override, &error), qPrintable(error));
    QTRY_COMPARE(overridden->findChild<WorkspaceFileModel *>()->index(0, 0).data().toString(), QString("worker-b-remote.txt"));
    QTRY_COMPARE(git->findChild<QLabel *>("dashboardGitBranch")->text(), QString("worker-b"));
    QVERIFY(page.canChangeSharedMachineContext(&error));
    QVERIFY(factory.setSharedMachineContext(third, "worker-c-root", &error));
    QVERIFY(page.setSharedMachineContext(third, "worker-c-root", &error));
    QTRY_COMPARE(following->findChild<WorkspaceFileModel *>()->index(0, 0).data().toString(), QString("worker-c-remote.txt"));
    QCOMPARE(overridden->machineTarget(), second.value("target").toObject());
    QCOMPARE(git->findChild<QLabel *>("dashboardGitBranch")->text(), QString("worker-b"));
    QTRY_COMPARE(containers->findChild<QTableWidget *>("dashboardContainers")->rowCount(), 1);
    QCOMPARE(containers->findChild<QTableWidget *>("dashboardContainers")->item(0, 0)->text(), QString("worker-c"));
    const QString logPath = fixture.filePath("requests.jsonl");
    QByteArray calls;
    {
        QLockFile lock(logPath + ".lock");
        QVERIFY(lock.tryLock(3000));
        QFile log(logPath);
        QVERIFY(log.open(QIODevice::ReadOnly));
        calls = log.readAll();
    }
    QVERIFY(!calls.isEmpty());
    for (const auto &line : calls.split('\n')) {
        if (line.isEmpty())
            continue;
        const auto document = QJsonDocument::fromJson(line);
        QVERIFY(document.isObject());
        const auto call = document.object();
        QVERIFY(!call.contains("unexpected_local"));
        QVERIFY(call.value("remote").toBool());
        const auto request = call.value("request").toObject();
        const QString action = request.value("action").toString();
        const QString node = request.value("target").toObject().value("node").toString();
        if (action == "containers.snapshot")
            QVERIFY(!request.contains("root"));
        else
            QCOMPARE(request.value("root").toString(), node + "-root");
        QVERIFY(!request.value("payload").toObject().contains("workdir"));
    }

    // Catalog refresh cannot silently replace a pinned worker generation.
    auto replacement = second;
    auto replacementTarget = replacement.value("target").toObject();
    replacementTarget.insert("generation", QString(64, 'f'));
    replacement.insert("target", replacementTarget);
    page.setMachineCatalog({first, replacement, third});
    QCOMPARE(factory.widgetMachineContext(git).value("target"), second.value("target"));
    QVERIFY(git->findChild<WorkspaceMachineMenu *>("dashboardWidgetMachine")->text().contains("Unavailable"));
    QVERIFY(factory.setWidgetMachineContext(overridden, {{"mode", "local"}}, &error));
    QTRY_COMPARE(overridden->currentDirectory(), fixture.path());
    QVERIFY(overridden->machineTarget().isEmpty());
    const auto saved = page.snapshot();
    QVERIFY2(DashboardStore::validate({saved}, error), qPrintable(error));
    DashboardPage restored(saved, &factory);
    auto *restoredGit = restored.findChild<QWidget *>("dashboardWidget_git");
    QVERIFY(restoredGit);
    QCOMPARE(factory.widgetMachineContext(restoredGit).value("target"), second.value("target"));
    QCOMPARE(factory.widgetMachineContext(restoredGit).value("rootId").toString(), QString("worker-b-root"));
    const auto restoredBrowsers = restored.findChildren<DashboardFilesWidget *>();
    QCOMPARE(restoredBrowsers.size(), 2);
    QTRY_VERIFY(restoredBrowsers[0]->machineTarget() != restoredBrowsers[1]->machineTarget());
}

void DashboardCanvasToolsTest::remotePreviewsStayPinnedAndDraftGateIsPure()
{
    QTemporaryDir fixture;
    MachineFixtureEnvironment environment(fixture.path());
    QVERIFY(environment.ready);
    const auto first = machineCatalog("worker-a", 'a');
    const auto second = machineCatalog("worker-b", 'b');
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    factory.setWorkdir(fixture.path());
    factory.setMachineCatalog({first, second});
    QString error;
    QVERIFY(factory.setSharedMachineContext(first, "worker-a-root", &error));
    DashboardPage page(emptyDashboard(), &factory);
    QVERIFY(page.addWidgetKind("files"));
    auto *source = page.findChild<DashboardFilesWidget *>();
    QVERIFY(source);
    QTRY_COMPARE(source->currentDirectory(), QString("/"));
    factory.filePreviewRequested(source, "/", "/worker-a-remote.txt");
    auto *preview = page.findChild<DashboardFilePreview *>();
    QVERIFY(preview);
    QTRY_COMPARE(preview->currentPath(), QString("/worker-a-remote.txt"));
    auto *previewFiles = preview->findChild<ProjectFilesWidget *>();
    auto *editor = previewFiles->findChild<QPlainTextEdit *>("fileTextEditor");
    QVERIFY(editor);
    QVERIFY(editor->toPlainText().contains("worker-a"));
    editor->insertPlainText("retained preview draft\n");
    const auto *previewOwner = WorkspaceFilesController::get(previewFiles);
    QCOMPARE(previewOwner->reader->context().nodeId, QString("worker-a"));
    QVERIFY(!preview->findChild<WorkspaceMachineMenu *>("dashboardPanelMachine")->isEnabled());
    QVERIFY(page.canChangeSharedMachineContext(&error));
    QVERIFY(factory.setSharedMachineContext(second, "worker-b-root", &error));
    QVERIFY(page.setSharedMachineContext(second, "worker-b-root", &error));
    QCOMPARE(previewOwner->reader->context().nodeId, QString("worker-a"));
    QVERIFY(previewFiles->hasUnsavedChanges());
    QVERIFY(editor->toPlainText().contains("retained preview draft"));
    {
        const auto saved = page.snapshot();
        const auto savedContexts = saved.layout.value("panelContexts").toObject();
        for (const auto &panel : saved.panels) {
            if (panel.kind != "file-preview")
                continue;
            const auto machine = savedContexts.value(panel.id).toObject().value("machine").toObject();
            QCOMPARE(machine.value("mode").toString(), QString("remote"));
            QCOMPARE(machine.value("target"), first.value("target"));
            QCOMPARE(machine.value("rootId").toString(), QString("worker-a-root"));
        }
        DashboardPage reloaded(saved, &factory);
        auto *pinned = reloaded.findChild<DashboardFilePreview *>();
        QVERIFY(pinned);
        QTRY_COMPARE(pinned->currentPath(), QString("/worker-a-remote.txt"));
        QCOMPARE(WorkspaceFilesController::get(pinned->findChild<ProjectFilesWidget *>())->reader->context().nodeId,
            QString("worker-a"));
    }

    QVERIFY(page.addWidgetKind("files-full"));
    auto *full = page.findChild<DashboardRepositoryPanel *>();
    QVERIFY(full);
    auto *files = full->findChild<ProjectFilesWidget *>();
    QVERIFY(files);
    files->openFileAsync("/support.txt");
    QTRY_COMPARE(files->currentPath(), QString("/support.txt"));
    files->findChild<QPlainTextEdit *>("fileTextEditor")->insertPlainText("keep full-view draft\n");
    QVERIFY(!page.canChangeSharedMachineContext(&error));
    QVERIFY(!error.isEmpty());
    QVERIFY(!QApplication::activeModalWidget());
    QVERIFY(!page.setSharedMachineContext(first, "worker-a-root", &error));
    QCOMPARE(factory.machineTarget(), second.value("target").toObject());
    QCOMPARE(WorkspaceFilesController::get(files)->reader->context().nodeId, QString("worker-b"));
    QVERIFY(files->hasUnsavedChanges());
    QVERIFY(!QApplication::activeModalWidget());
}

int main(int argc, char **argv)
{
    if (argc > 1 && qEnvironmentVariableIsSet("DOCKPIPE_DASHBOARD_MACHINE_LOG"))
        return runMachineFixture(argc, argv);
    if (argc > 1 && QByteArray(argv[1]) == "git" && qEnvironmentVariableIsSet("DOCKPIPE_CANVAS_GIT_FIXTURE_LOG"))
        return runGitFixture(argc, argv);
    QTemporaryDir profile;
    if (!profile.isValid())
        return 1;
    for (const auto *name : {"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "APPDATA", "LOCALAPPDATA"})
        qputenv(name, profile.path().toUtf8());
    QApplication application(argc, argv);
    application.setOrganizationName("DockpipeCanvasToolsTest");
    application.setApplicationName("CanvasToolsFixture");
    if (!qEnvironmentVariable("DOCKPIPE_CANVAS_TOOLS_IMAGES").isEmpty()
        || !qEnvironmentVariable("DOCKPIPE_TEST_THEME").isEmpty()) {
        application.setStyle("Fusion");
        applyDockpipeLauncherTheme(application);
        const QString theme = qEnvironmentVariable("DOCKPIPE_TEST_THEME");
        if (theme == "light")
            previewDockpipeLauncherTheme(application, LauncherThemeMode::Light);
        else if (theme == "dark")
            previewDockpipeLauncherTheme(application, LauncherThemeMode::Dark);
    }
    DashboardCanvasToolsTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "DashboardCanvasToolsTest.moc"
