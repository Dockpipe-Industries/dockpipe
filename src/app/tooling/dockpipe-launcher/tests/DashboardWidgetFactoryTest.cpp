#include "ContextStore.h"
#include "DashboardWidgetFactory.h"
#include "DashboardLogWidget.h"
#include "SessionManager.h"
#include "TerminalHub.h"
#include "TerminalWidget.h"

#include <QAction>
#include <QApplication>
#include <QCoreApplication>
#include <QComboBox>
#include <QDir>
#include <QPlainTextEdit>
#include <QTextCursor>
#include <QElapsedTimer>
#include <QFile>
#include <QJsonDocument>
#include <QMetaMethod>
#include <QTimer>
#include <QVBoxLayout>
#include <cstdio>
#ifdef Q_OS_UNIX
#include <cerrno>
#include <csignal>
#endif
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QProgressBar>
#include <QPushButton>
#include <QSignalSpy>
#include <QSet>
#include <QTableWidget>
#include <QTemporaryDir>
#include <QTest>
#include <QTreeView>
#include <memory>

class DashboardWidgetFactoryTest final : public QObject {
    Q_OBJECT
private slots:
    void catalogAndCompactWorkflowActions();
    void boundedSnapshotViewsAndFailure();
    void filesRequireProjectAndLogsAreScoped();
    void duplicateWidgetsShareObservationReaders_data();
    void duplicateWidgetsShareObservationReaders();
    void hiddenAndDestroyedWidgetsReleaseObservationReaders();
    void unchangedLogTailPreservesSelectionAndRunModel();
    void terminalHasContextActionsWithoutInnerChrome();
};

void DashboardWidgetFactoryTest::terminalHasContextActionsWithoutInnerChrome()
{
    SessionManager sessions;
    ContextStore store;
    DashboardWidgetFactory factory(sessions, store);
    std::unique_ptr<QWidget> panel(factory.create("terminal"));
    auto *terminal = panel->findChild<TerminalWidget *>();
    auto *footer = panel->findChild<QWidget *>("dashboardWidgetFooter");
    QVERIFY(terminal && footer);
    QVERIFY(footer->isHidden());
    QVERIFY(terminal->findChild<QWidget *>("terminalToolbar")->isHidden());
    QVERIFY(factory.terminals().sessionIds().isEmpty());
    auto *open = terminal->findChild<QAction *>("terminalOpenFullViewAction");
    QVERIFY(open);
    QSignalSpy requested(&factory, &DashboardWidgetFactory::openFullView);
    open->trigger();
    QCOMPARE(requested.size(), 1);
    QCOMPARE(requested.first().first().toString(), QString("terminal"));
    panel.reset();
    QVERIFY(factory.terminals().sessionIds().isEmpty());
}

void DashboardWidgetFactoryTest::catalogAndCompactWorkflowActions()
{
    SessionManager sessions;
    ContextStore store;
    DashboardWidgetFactory factory(sessions, store);
    QSet<QString> kinds;
    for (const auto &entry : factory.catalog()) {
        QVERIFY(!entry.id.isEmpty());
        QVERIFY(!entry.title.isEmpty());
        QVERIFY(!entry.description.isEmpty());
        QVERIFY(!kinds.contains(entry.id));
        kinds.insert(entry.id);
    }
    QCOMPARE(kinds.size(), 11);
    QVERIFY(!factory.create("unknown"));
    QVector<WorkflowMeta> workflows;
    for (int index = 0; index < 60; ++index) {
        WorkflowMeta workflow;
        workflow.workflowId = QString("build-%1").arg(index);
        workflow.displayName = QString("Build %1").arg(index);
        workflows.append(workflow);
    }
    factory.setWorkflows(workflows);
    std::unique_ptr<QWidget> widget(factory.create("workflows"));
    auto *list = widget->findChild<QListWidget *>("dashboardWorkflowList");
    auto *search = widget->findChild<QLineEdit *>("dashboardWorkflowSearch");
    auto *launch = widget->findChild<QPushButton *>("dashboardLaunchWorkflow");
    QCOMPARE(list->count(), 40);
    QSignalSpy launchSpy(&factory, &DashboardWidgetFactory::launchWorkflow);
    search->setText("build-59");
    QCOMPARE(list->count(), 1);
    launch->click();
    QCOMPARE(launchSpy.size(), 1);
    QCOMPARE(launchSpy[0][0].toString(), QString("build-59"));
    search->setText("missing");
    QVERIFY(!launch->isEnabled());
    QSignalSpy openSpy(&factory, &DashboardWidgetFactory::openFullView);
    widget->findChild<QPushButton *>("dashboardOpenFullView")->click();
    QCOMPARE(openSpy[0][0].toString(), QString("workflows"));
    search->clear();
    QCOMPARE(list->count(), 40);
    factory.setWorkdir("/different-project");
    QCOMPARE(list->count(), 0);
    QVERIFY(!launch->isEnabled());
}

void DashboardWidgetFactoryTest::boundedSnapshotViewsAndFailure()
{
    SessionManager sessions;
    ContextStore store;
    DashboardWidgetFactory factory(sessions, store);
    std::unique_ptr<QWidget> system(factory.create("system"));
    std::unique_ptr<QWidget> executions(factory.create("executions"));
    std::unique_ptr<QWidget> containers(factory.create("containers"));
    std::unique_ptr<QWidget> git(factory.create("git"));
    std::unique_ptr<QWidget> machines(factory.create("machines"));
    QJsonArray records;
    for (int index = 0; index < 30; ++index) {
        records.append(QJsonObject{{"workflow", QString("Build %1").arg(index)}, {"state", "running"},
            {"started_at", QString("2026-01-01T12:00:%1Z").arg(index, 2, 10, QLatin1Char('0'))}});
    }
    const QJsonObject resources{{"memory_total", 1000}, {"memory_available", 250},
        {"swap_total", 0}, {"swap_free", 0}, {"disk_total", 1000}, {"disk_available", 900}};
    emit factory.snapshotChanged({{"executions", records}, {"system", resources}}, {});
    auto *table = executions->findChild<QTableWidget *>("dashboardExecutions");
    QCOMPARE(table->rowCount(), 12);
    QCOMPARE(table->item(0, 0)->text(), QString("Build 29"));
    auto *memory = system->findChild<QLabel *>("systemMetric1");
    QVERIFY(memory->text().contains("750"));
    emit factory.snapshotChanged({}, "Unavailable fixture");
    QCOMPARE(table->rowCount(), 0);
    QCOMPARE(memory->text(), QString("Unavailable"));
    QCOMPARE(executions->findChild<QLabel *>("dashboardWidgetStatus")->text(), QString("Unavailable fixture"));

    QJsonArray inventory;
    for (int index = 0; index < 25; ++index)
        inventory.append(QJsonObject{{"Names", QString("Container %1").arg(index)}, {"State", "running"}, {"Image", "fixture"}});
    emit factory.containersChanged(inventory, {});
    auto *containerTable = containers->findChild<QTableWidget *>("dashboardContainers");
    QCOMPARE(containerTable->rowCount(), 20);
    QVERIFY(containers->findChild<QLabel *>("dashboardWidgetStatus")->text().contains("20 of 25"));
    emit factory.containersChanged({}, "Docker unavailable");
    QCOMPARE(containerTable->rowCount(), 0);

    emit factory.gitChanged({{"branch", "feature"}, {"changes", QJsonArray{QJsonObject{{"status", "M"}, {"path", "sample.cpp"}}}}}, {});
    QCOMPARE(git->findChild<QLabel *>("dashboardGitBranch")->text(), QString("feature"));
    QCOMPARE(git->findChild<QListWidget *>("dashboardGitChanges")->count(), 1);
    emit factory.gitChanged({}, "Git unavailable");
    QCOMPARE(git->findChild<QListWidget *>("dashboardGitChanges")->count(), 0);
    QVERIFY(git->findChild<QLabel *>("dashboardGitBranch")->text().isEmpty());

    emit factory.machinesChanged({QJsonObject{{"node", "test-machine"}, {"status", "paired"}}}, {});
    QCOMPARE(machines->findChild<QTableWidget *>("dashboardMachines")->item(0, 1)->text(), QString("paired"));
    emit factory.machinesChanged({}, "Machine unavailable");
    QCOMPARE(machines->findChild<QTableWidget *>("dashboardMachines")->rowCount(), 0);
}

void DashboardWidgetFactoryTest::filesRequireProjectAndLogsAreScoped()
{
    SessionManager sessions;
    ContextStore store;
    DashboardWidgetFactory factory(sessions, store);
    std::unique_ptr<QWidget> files(factory.create("files"));
    auto *tree = files->findChild<QTreeView *>("dashboardFiles");
    QVERIFY(tree->isHidden());
    QTemporaryDir project;
    factory.setWorkdir(project.path());
    QVERIFY(!tree->isHidden());
    QVERIFY(tree->rootIndex().isValid());
    factory.setWorkdir({});
    QVERIFY(tree->isHidden());
    std::unique_ptr<QWidget> logs(factory.create("logs"));
    QVERIFY(logs->findChild<QLabel *>()->text().contains("No desktop session logs"));
    // The widgets were never shown: construction and project changes must not
    // launch observation commands or manufacture sessions.
    QCOMPARE(store.contexts.size(), 0);
}

namespace {
class ScopedEnvironment final {
public:
    ScopedEnvironment(const char *name, const QByteArray &value)
        : m_name(name), m_previous(qgetenv(name)), m_wasSet(qEnvironmentVariableIsSet(name))
    {
        qputenv(m_name.constData(), value);
    }
    ~ScopedEnvironment()
    {
        if (m_wasSet)
            qputenv(m_name.constData(), m_previous);
        else
            qunsetenv(m_name.constData());
    }
private:
    QByteArray m_name;
    QByteArray m_previous;
    bool m_wasSet;
};

QList<qint64> helperPids(const QString &directory)
{
    QFile file(directory + "/helpers");
    if (!file.open(QIODevice::ReadOnly))
        return {};
    QList<qint64> pids;
    for (const auto &line : file.readAll().split('\n')) {
        bool valid = false;
        const qint64 pid = line.toLongLong(&valid);
        if (valid && pid > 0)
            pids.append(pid);
    }
    return pids;
}

bool helperAlive(qint64 pid)
{
#ifdef Q_OS_UNIX
    return ::kill(pid_t(pid), 0) == 0 || errno == EPERM;
#else
    Q_UNUSED(pid);
    return false;
#endif
}

int runObservationHelper(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    const QString directory = qEnvironmentVariable("DOCKPIPE_DASHBOARD_READER_FIXTURE");
    QFile starts(directory + "/helpers");
    if (directory.isEmpty() || !starts.open(QIODevice::WriteOnly | QIODevice::Append))
        return 30;
    starts.write(QByteArray::number(QCoreApplication::applicationPid()) + '\n');
    starts.close();
    QFile mode(directory + "/mode");
    if (mode.open(QIODevice::ReadOnly) && mode.readAll() == "hold") {
        // A real owned process stays alive until the reader cancels it. Its
        // independent deadline prevents a failed test from leaking a helper.
        QTimer::singleShot(15000, &application, [&application]() { application.exit(31); });
        return application.exec();
    }
    const QString command = application.arguments().value(1);
    QTimer::singleShot(100, &application, [&application, command]() {
        QJsonDocument document;
        if (command == "remote") {
            document = QJsonDocument(QJsonArray{});
        } else if (command == "git") {
            document = QJsonDocument(QJsonObject{{"branch", "fixture"}, {"changes", QJsonArray{}}});
        } else if (command == "container") {
            document = QJsonDocument(QJsonObject{{"Names", "fixture"}, {"State", "running"}, {"Image", "fixture"}});
        } else {
            document = QJsonDocument(QJsonObject{{"schema", "dockpipe.execution_snapshot.v1"},
                {"executions", QJsonArray{}}, {"processes", QJsonArray{}}});
        }
        QByteArray bytes = document.toJson(QJsonDocument::Compact);
        if (command == "--workflow") {
            bytes.clear();
            for (int line = 0; line < 600; ++line)
                bytes += "fixture line " + QByteArray::number(line) + '\n';
        }
        std::fwrite(bytes.constData(), 1, size_t(bytes.size()), stdout);
        std::fflush(stdout);
        application.exit(0);
    });
    return application.exec();
}
}

void DashboardWidgetFactoryTest::duplicateWidgetsShareObservationReaders_data()
{
    QTest::addColumn<QString>("kind");
    QTest::newRow("executions") << QString("executions");
    QTest::newRow("system") << QString("system");
    QTest::newRow("git") << QString("git");
    QTest::newRow("machines") << QString("machines");
#ifdef Q_OS_UNIX
    QTest::newRow("containers") << QString("containers");
#endif
}

void DashboardWidgetFactoryTest::duplicateWidgetsShareObservationReaders()
{
    QFETCH(QString, kind);
    QTemporaryDir fixture;
    QVERIFY(fixture.isValid());
    ScopedEnvironment binary("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
    ScopedEnvironment state("DOCKPIPE_DASHBOARD_READER_FIXTURE", fixture.path().toUtf8());
    ScopedEnvironment path("PATH", fixture.path().toUtf8() + QDir::listSeparator().toLatin1() + qgetenv("PATH"));
    if (kind == "containers")
        QVERIFY(QFile::link(QCoreApplication::applicationFilePath(), fixture.path() + "/docker"));
    SessionManager sessions;
    ContextStore store;
    DashboardWidgetFactory factory(sessions, store);
    factory.setWorkdir(fixture.path());
    QWidget dashboard;
    auto *layout = new QVBoxLayout(&dashboard);
    for (int index = 0; index < 8; ++index)
        layout->addWidget(factory.create(kind, &dashboard));
    QMetaMethod signal = QMetaMethod::fromSignal(&DashboardWidgetFactory::snapshotChanged);
    if (kind == "git")
        signal = QMetaMethod::fromSignal(&DashboardWidgetFactory::gitChanged);
    else if (kind == "machines")
        signal = QMetaMethod::fromSignal(&DashboardWidgetFactory::machinesChanged);
    else if (kind == "containers")
        signal = QMetaMethod::fromSignal(&DashboardWidgetFactory::containersChanged);
    QSignalSpy observations(&factory, signal);
    dashboard.resize(420, 500);
    dashboard.show();
    QTRY_COMPARE_WITH_TIMEOUT(observations.size(), 1, 3000);
    QCOMPARE(helperPids(fixture.path()).size(), 1);
    QVERIFY(observations[0][1].toString().isEmpty());
    const QString timerName = kind == "git" || kind == "machines" ? "dashboardInventoryPoll" : "dashboardSnapshotPoll";
    auto *poll = factory.findChild<QTimer *>(timerName);
    QVERIFY(poll);
    // Exercise actual timer delivery without waiting four seconds per tick.
    QVERIFY(QMetaObject::invokeMethod(poll, "timeout", Qt::DirectConnection));
    for (int index = 0; index < 8; ++index)
        factory.refresh();
    QTRY_COMPARE_WITH_TIMEOUT(observations.size(), 2, 3000);
    QCOMPARE(helperPids(fixture.path()).size(), 2);
    QVERIFY(observations[1][1].toString().isEmpty());
    for (auto *table : dashboard.findChildren<QTableWidget *>("dashboardExecutions"))
        QCOMPARE(table->rowCount(), 0);
}

void DashboardWidgetFactoryTest::hiddenAndDestroyedWidgetsReleaseObservationReaders()
{
#ifndef Q_OS_UNIX
    QSKIP("Process identity checks in this regression target Unix desktop hosts.");
#else
    QTemporaryDir fixture;
    QVERIFY(fixture.isValid());
    QFile mode(fixture.path() + "/mode");
    QVERIFY(mode.open(QIODevice::WriteOnly));
    QCOMPARE(mode.write("hold"), qint64(4));
    mode.close();
    ScopedEnvironment binary("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
    ScopedEnvironment state("DOCKPIPE_DASHBOARD_READER_FIXTURE", fixture.path().toUtf8());
    SessionManager sessions;
    ContextStore store;
    auto factory = std::make_unique<DashboardWidgetFactory>(sessions, store);
    QWidget dashboard;
    auto *layout = new QVBoxLayout(&dashboard);
    layout->addWidget(factory->create("executions", &dashboard));
    dashboard.show();
    QTRY_COMPARE_WITH_TIMEOUT(helperPids(fixture.path()).size(), 1, 3000);
    const qint64 first = helperPids(fixture.path()).first();
    QVERIFY(helperAlive(first));
    dashboard.hide();
    QTRY_VERIFY_WITH_TIMEOUT(!helperAlive(first), 3000);
    auto *poll = factory->findChild<QTimer *>("dashboardSnapshotPoll");
    QVERIFY(poll);
    for (int index = 0; index < 8; ++index) {
        QVERIFY(QMetaObject::invokeMethod(poll, "timeout", Qt::DirectConnection));
        factory->refresh();
    }
    QTest::qWait(150);
    QCOMPARE(helperPids(fixture.path()).size(), 1);
    dashboard.show();
    QTRY_COMPARE_WITH_TIMEOUT(helperPids(fixture.path()).size(), 2, 3000);
    const qint64 second = helperPids(fixture.path()).last();
    QVERIFY(helperAlive(second));
    QElapsedTimer shutdown;
    shutdown.start();
    factory.reset();
    QVERIFY2(shutdown.elapsed() < 2500, "Reader shutdown exceeded its terminate/kill budget");
    QVERIFY(!helperAlive(second));
#endif
}

void DashboardWidgetFactoryTest::unchangedLogTailPreservesSelectionAndRunModel()
{
    QTemporaryDir fixture;
    QVERIFY(fixture.isValid());
    ScopedEnvironment state("DOCKPIPE_DASHBOARD_READER_FIXTURE", fixture.path().toUtf8());
    SessionManager sessions;
    ContextStore store;
    Context context = Context::createNew();
    context.label = "Fixture logs";
    context.workdir = fixture.path();
    context.workflow = "dashboard-log-fixture";
    context.dockpipeBinary = QCoreApplication::applicationFilePath();
    store.contexts.append(context);
    QVERIFY(sessions.launch(context, fixture.path()));
    QTRY_VERIFY_WITH_TIMEOUT(!sessions.isRunning(context.id), 3000);
    DashboardLogWidget logs(sessions, store);
    logs.setWorkdir(fixture.path());
    auto *editor = logs.findChild<QPlainTextEdit *>("dashboardLogTail");
    auto *runs = logs.findChild<QComboBox *>("dashboardLogRun");
    QVERIFY(editor->toPlainText().contains("fixture line 599"));
    QVERIFY(editor->document()->blockCount() <= 400);
    QTextCursor selection = editor->textCursor();
    selection.setPosition(0);
    selection.movePosition(QTextCursor::Right, QTextCursor::KeepAnchor, 7);
    editor->setTextCursor(selection);
    const QString selected = editor->textCursor().selectedText();
    QSignalSpy textChanged(editor, &QPlainTextEdit::textChanged);
    QSignalSpy modelReset(runs->model(), &QAbstractItemModel::modelReset);
    for (int refresh = 0; refresh < 5; ++refresh)
        logs.refresh();
    QCOMPARE(textChanged.size(), 0);
    QCOMPARE(modelReset.size(), 0);
    QCOMPARE(editor->textCursor().selectedText(), selected);
}

int main(int argc, char **argv)
{
    if (argc > 1 && qEnvironmentVariableIsSet("DOCKPIPE_DASHBOARD_READER_FIXTURE")) {
        const QByteArray command(argv[1]);
        if (command == "runs" || command == "remote" || command == "git" || command == "container" || command == "--workflow")
            return runObservationHelper(argc, argv);
    }
    QTemporaryDir config;
    qputenv("XDG_CONFIG_HOME", config.path().toUtf8());
    QApplication application(argc, argv);
    DashboardWidgetFactoryTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "DashboardWidgetFactoryTest.moc"
