#include "DashboardWidgetFactory.h"

#include "DashboardLogWidget.h"
#include "DashboardFilesWidget.h"
#include <QDesktopServices>
#include <QMessageBox>
#include <QUrl>
#include "DockpipeChoices.h"
#include "TerminalHub.h"
#include "TerminalWidget.h"
#include "TerminalPreferences.h"
#include "SendInputWidget.h"

#include <QEvent>
#include <QJsonDocument>
#include <QPushButton>
#include <QTimer>
#include <QVBoxLayout>
#include <QWidget>

DashboardWidgetFactory::DashboardWidgetFactory(SessionManager &sessions, ContextStore &store, QObject *parent)
    : QObject(parent), m_sessions(sessions), m_store(store), m_terminals(new TerminalHub(this)), m_snapshotCommand(this),
      m_containerCommand(this), m_machineCommand(this), m_gitCommand(this), m_timer(new QTimer(this))
{
    // One reader per source serves all copies of that widget. Hidden dashboard
    // panels retain state but do not spawn background observation processes.
    m_timer->setObjectName("dashboardSnapshotPoll");
    m_timer->setInterval(4000);
    connect(m_timer, &QTimer::timeout, this, [this]() {
        refreshSnapshot();
        refreshContainers();
    });
    m_timer->start();
    auto *slowTimer = new QTimer(this);
    slowTimer->setObjectName("dashboardInventoryPoll");
    slowTimer->setInterval(30000);
    connect(slowTimer, &QTimer::timeout, this, [this]() {
        refreshGit();
        refreshMachines();
    });
    slowTimer->start();
}

QVector<DashboardWidgetDescriptor> DashboardWidgetFactory::catalog()
{
    return {
        {"system", tr("System monitor"), tr("Live CPU, memory, disk and network measurements for this computer.")},
        {"workflows", tr("Workflow launcher"), tr("Find and launch workflows using the normal DockPipe configuration flow.")},
        {"executions", tr("Recent executions"), tr("The latest 12 managed operations and their current state.")},
        {"logs", tr("Run logs"), tr("A live, bounded tail of workflows launched in this desktop session.")},
        {"containers", tr("Containers"), tr("Local Docker container names, states and images; management stays in Containers.")},
        {"files", tr("File manager"), tr("Browse project files in Canvas or the filesystem from Home in Tiling. Change either scope in dashboard settings.")},
        {"git", tr("Git changes"), tr("Current branch and a compact list of changes from the DockPipe Git service.")},
        {"machines", tr("Machines"), tr("Managed machine access states from the existing remote service.")},
        {"browser", tr("Browser"), tr("A live browser with shared settings, tabs, downloads and extensions.")},
        {"terminal", tr("Terminal"), tr("A shell that starts when its panel is first shown. Closing the panel stops its shell.")},
        {"send-input", tr("Send input"), tr("Send text to an explicitly selected terminal session.")}
    };
}

QWidget *DashboardWidgetFactory::wrap(const QString &kind, const QString &pageKey, QWidget *content, QWidget *parent)
{
    auto *widget = new QWidget(parent);
    widget->setObjectName("dashboardWidget_" + kind);
    widget->setProperty("dashboardKind", kind);
    if (kind == "terminal") {
        const auto updateBackground = [widget]() {
            const TerminalSettings settings = TerminalPreferences::instance()->settings();
            widget->setProperty("canvasPanelTransparent", settings.backgroundTarget == "canvas" && TerminalPreferences::renderingOpacity(settings) < 100);
        };
        connect(TerminalPreferences::instance(), &TerminalPreferences::changed, widget, updateBackground);
        updateBackground();
    }
    widget->setMinimumSize(260, 180);
    auto *layout = new QVBoxLayout(widget);
    layout->setContentsMargins(10, 10, 10, 8);
    layout->setSpacing(8);
    if (kind == "terminal") {
        layout->setContentsMargins(0, 0, 0, 0);
        layout->setSpacing(0);
    }
    layout->addWidget(content, 1);
    auto *footerWidget = new QWidget;
    footerWidget->setObjectName("dashboardWidgetFooter");
    auto *footer = new QHBoxLayout(footerWidget);
    footer->setContentsMargins(0, 0, 0, 0);
    if (kind != "browser" && kind != "files" && kind != "terminal" && kind != "send-input") {
        auto *refreshButton = new QPushButton(tr("Refresh"));
        refreshButton->setObjectName("dashboardWidgetRefresh");
        connect(refreshButton, &QPushButton::clicked, this, &DashboardWidgetFactory::refresh);
        footer->addWidget(refreshButton);
    }
    footer->addStretch();
    auto *open = new QPushButton(tr("Open full view"));
    open->setObjectName("dashboardOpenFullView");
    if (auto *files = qobject_cast<DashboardFilesWidget *>(content)) {
        open->setText(tr("Open folder"));
        open->setToolTip(tr("Open this folder in your system file manager"));
        connect(open, &QPushButton::clicked, files, [files]() {
            const QString directory = files->currentDirectory();
            if (!directory.isEmpty() && !QDesktopServices::openUrl(QUrl::fromLocalFile(directory)))
                QMessageBox::warning(files, tr("Open folder"), tr("The system could not open this folder."));
        });
    } else {
        connect(open, &QPushButton::clicked, this, [this, pageKey]() { emit openFullView(pageKey); });
    }
    footer->addWidget(open);
    layout->addWidget(footerWidget);
    if (kind == "terminal")
        footerWidget->hide();
    widget->installEventFilter(this);
    m_widgets.append(widget);
    connect(widget, &QObject::destroyed, this, [this]() {
        m_widgets.removeIf([](const QPointer<QWidget> &entry) { return entry.isNull(); });
    });
    return widget;
}

QWidget *DashboardWidgetFactory::create(const QString &kind, QWidget *parent, const QString &fileScope)
{
    QWidget *content = nullptr;
    QString page = kind;
    if (kind == "system") {
        content = createSystem(nullptr);
        page = "executions";
    } else if (kind == "executions") {
        content = createExecutions(nullptr);
    } else if (kind == "workflows") {
        content = createWorkflows(nullptr);
    } else if (kind == "containers") {
        content = createContainers(nullptr);
    } else if (kind == "logs") {
        auto *logs = new DashboardLogWidget(m_sessions, m_store);
        logs->setWorkdir(m_workdir);
        connect(this, &DashboardWidgetFactory::workdirChanged, logs, &DashboardLogWidget::setWorkdir);
        content = logs;
        page = "logs";
    } else if (kind == "files") {
        content = createFiles(nullptr, fileScope);
    } else if (kind == "git") {
        content = createGit(nullptr);
    } else if (kind == "machines") {
        content = createMachines(nullptr);
    } else if (kind == "browser") {
        content = createBrowser(nullptr);
    } else if (kind == "terminal") {
        auto *terminal = new TerminalWidget(*m_terminals, m_workdir, nullptr, TerminalWidget::Mode::Dashboard);
        connect(this, &DashboardWidgetFactory::workdirChanged, terminal, &TerminalWidget::setWorkdir);
        connect(terminal, &TerminalWidget::openFullViewRequested, this, [this]() { emit openFullView("terminal"); });
        content = terminal;
    } else if (kind == "send-input") {
        content = new SendInputWidget(*m_terminals);
        page = "terminal";
    } else {
        return nullptr;
    }
    return wrap(kind, page, content, parent);
}

void DashboardWidgetFactory::setWorkdir(const QString &workdir)
{
    if (m_workdir == workdir)
        return;
    m_workdir = workdir;
    m_workflows.clear();
    emit workflowsChanged();
    ++m_generation;
    m_snapshotCommand.cancel();
    m_gitCommand.cancel();
    m_machineCommand.cancel();
    emit workdirChanged(workdir);
    emit snapshotChanged({}, tr("Waiting for this project's measurements…"));
    emit gitChanged({}, tr("Waiting for this project's Git status…"));
    QTimer::singleShot(0, this, &DashboardWidgetFactory::refresh);
}

void DashboardWidgetFactory::setWorkflows(const QVector<WorkflowMeta> &workflows)
{
    m_workflows = workflows;
    emit workflowsChanged();
}

bool DashboardWidgetFactory::hasVisibleKind(const QStringList &kinds) const
{
    for (const auto &widget : m_widgets) {
        if (widget && widget->isVisible() && kinds.contains(widget->property("dashboardKind").toString()))
            return true;
    }
    return false;
}

bool DashboardWidgetFactory::eventFilter(QObject *watched, QEvent *event)
{
    if (event->type() == QEvent::Show)
        QTimer::singleShot(0, this, &DashboardWidgetFactory::refresh);
    if (event->type() == QEvent::Hide) {
        QTimer::singleShot(0, this, [this]() {
            if (!hasVisibleKind({"system", "executions"}))
                m_snapshotCommand.cancel();
            if (!hasVisibleKind({"containers"}))
                m_containerCommand.cancel();
            if (!hasVisibleKind({"git"}))
                m_gitCommand.cancel();
            if (!hasVisibleKind({"machines"}))
                m_machineCommand.cancel();
        });
    }
    return QObject::eventFilter(watched, event);
}

void DashboardWidgetFactory::refresh()
{
    refreshSnapshot();
    refreshContainers();
    refreshGit();
    refreshMachines();
    emit workflowsChanged();
    for (const auto &widget : m_widgets) {
        if (widget && widget->isVisible()) {
            if (auto *logs = widget->findChild<DashboardLogWidget *>())
                logs->refresh();
        }
    }
}

void DashboardWidgetFactory::refreshSnapshot()
{
    if (m_snapshotCommand.busy() || !hasVisibleKind({"system", "executions"}))
        return;
    QStringList args{"runs", "observe", "--json", "--resources"};
    if (m_workdir.isEmpty())
        args << "--global";
    else
        args << "--workdir" << m_workdir;
    const quint64 generation = m_generation;
    m_snapshotCommand.run(DockpipeChoices::preferredDockpipeBinary(m_workdir), args,
        [this, generation](bool ok, const QByteArray &bytes) {
            if (generation != m_generation) {
                QTimer::singleShot(0, this, &DashboardWidgetFactory::refresh);
                return;
            }
            const auto document = QJsonDocument::fromJson(bytes);
            const auto snapshot = document.object();
            if (!ok || snapshot.value("schema").toString() != "dockpipe.execution_snapshot.v1"
                || !snapshot.value("executions").isArray() || snapshot.value("executions").toArray().size() > 256) {
                emit snapshotChanged({}, tr("Monitoring unavailable. Check the CLI and machine permissions."));
                return;
            }
            emit snapshotChanged(snapshot, {});
        }, 5000);
}

void DashboardWidgetFactory::refreshContainers()
{
    if (m_containerCommand.busy() || !hasVisibleKind({"containers"}))
        return;
    // This is the same read-only Docker inventory contract as Containers.
    m_containerCommand.run("docker", {"container", "ls", "--all", "--format", "{{json .}}"},
        [this](bool ok, const QByteArray &bytes) {
            if (!ok) {
                emit containersChanged({}, tr("Docker inventory unavailable. Check Docker access in Containers."));
                return;
            }
            QJsonArray containers;
            // Display only a compact window into inventory, with an explicit
            // count. The command transport bounds the complete response bytes.
            for (const auto &line : bytes.split('\n')) {
                if (line.trimmed().isEmpty())
                    continue;
                const auto document = QJsonDocument::fromJson(line);
                if (!document.isObject()) {
                    emit containersChanged({}, tr("Docker returned an invalid inventory."));
                    return;
                }
                if (containers.size() >= 4096) {
                    emit containersChanged({}, tr("Docker inventory exceeds the dashboard limit of 4096 containers. Use Containers for the full inventory."));
                    return;
                }
                containers.append(document.object());
            }
            emit containersChanged(containers, {});
        }, 5000);
}

void DashboardWidgetFactory::refreshGit()
{
    if (m_gitCommand.busy() || !hasVisibleKind({"git"}))
        return;
    if (m_workdir.isEmpty()) {
        emit gitChanged({}, tr("Open a project to see Git changes."));
        return;
    }
    const quint64 generation = m_generation;
    m_gitCommand.run(m_workdir, {{"action", "status"}},
        [this, generation](bool ok, const QJsonObject &snapshot, const QString &) {
            if (generation != m_generation) {
                QTimer::singleShot(0, this, &DashboardWidgetFactory::refresh);
                return;
            }
            emit gitChanged(ok ? snapshot : QJsonObject{}, ok ? QString{} : tr("Git status unavailable. Open Git for details."));
        });
}

void DashboardWidgetFactory::refreshMachines()
{
    if (m_machineCommand.busy() || !hasVisibleKind({"machines"}))
        return;
    const quint64 generation = m_generation;
    m_machineCommand.run(DockpipeChoices::preferredDockpipeBinary(m_workdir), {"remote", "nodes"},
        [this, generation](bool ok, const QByteArray &bytes) {
            if (generation != m_generation) {
                QTimer::singleShot(0, this, &DashboardWidgetFactory::refresh);
                return;
            }
            const auto document = QJsonDocument::fromJson(bytes);
            if (!ok || !document.isArray()) {
                emit machinesChanged({}, tr("Machine inventory unavailable. Configure remote access in Machines."));
                return;
            }
            if (document.array().size() > 4096) {
                emit machinesChanged({}, tr("Machine inventory exceeds the dashboard limit of 4096 entries. Use Machines for the full inventory."));
                return;
            }
            emit machinesChanged(document.array(), {});
        }, 5000);
}
