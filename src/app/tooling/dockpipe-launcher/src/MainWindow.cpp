#include "MainWindow.h"

#include "BasicModeWidget.h"
#include "RemoteWidget.h"
#include "ActivityWidget.h"
#include "RemoteRunDialog.h"
#include <QComboBox>
#include "ContextRowWidget.h"
#include "DockpipeChoices.h"
#include "DockerObservabilityWidget.h"
#include "GitHelper.h"
#include "LogViewerDialog.h"
#include "PackageManagerDialog.h"
#include "LauncherEnvironment.h"
#include "PromptDialog.h"
#include "SettingsDialog.h"
#include "WorkflowLaunchDialog.h"
#include "WorkflowCatalog.h"

#include <QActionGroup>
#include <QApplication>
#include <QCloseEvent>
#include <QCoreApplication>
#include <QDesktopServices>
#include <QDir>
#include <QDirIterator>
#include <QDialog>
#include <QFileDialog>
#include <QFileInfo>
#include <QFile>
#include <QFrame>
#include <QHBoxLayout>
#include <QIcon>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QPlainTextEdit>
#include <QMenu>
#include <QMenuBar>
#include <QMessageBox>
#include <QProcess>
#include <QProcessEnvironment>
#include <QPushButton>
#include <QScrollBar>
#include <QSet>
#include <QSplitter>
#include <QStackedWidget>
#include <QStatusBar>
#include <QTimer>
#include <QUrl>
#include <QVBoxLayout>
#include <QFontDatabase>
#include <QGuiApplication>
#include <QRegularExpression>
#include <QtConcurrent>

#include <functional>

namespace {

QString statusLabel(SessionManager &sm, const QString &id, bool *runningOut, bool *failedOut)
{
    *runningOut = sm.isRunning(id);
    if (*runningOut) {
        *failedOut = false;
        return QObject::tr("Running");
    }
    const SessionInfo si = sm.info(id);
    if (si.status == SessionStatus::Failed) {
        *failedOut = true;
        return QObject::tr("Failed");
    }
    *failedOut = false;
    return QObject::tr("Ready");
}

bool contextMatchesFilter(const Context &c, const QString &filter)
{
    const QString needle = filter.trimmed().toCaseFolded();
    if (needle.isEmpty())
        return true;

    const QString haystack = QStringList{
                                 c.label,
                                 c.workdir,
                                 c.workflow,
                                 c.workflowFile,
                                 c.resolver,
                                 c.strategy,
                                 c.runtime,
                                 c.dockpipeBinary,
                                 c.envFile,
                                 c.id,
                             }
                                 .join(QLatin1Char('\n'))
                                 .toCaseFolded();
    return haystack.contains(needle);
}

QString contextDisplayKey(const Context &c)
{
    return QStringList{
        QDir::cleanPath(c.workdir),
        c.workflow,
        c.workflowFile,
        c.label,
        c.resolver,
        c.runtime,
    }.join(QLatin1Char('\x1f'));
}

QString shellQuote(QString s)
{
    if (s.isEmpty())
        return QStringLiteral("''");
    s.replace(QLatin1Char('\''), QStringLiteral("'\"'\"'"));
    if (s.contains(QRegularExpression(QStringLiteral("[\\s\"'`$&|;<>()\\[\\]{}*!?\\\\]"))))
        return QStringLiteral("'") + s + QStringLiteral("'");
    return s;
}

QMap<QString, QString> parseEnvAssignments(const QStringList &lines)
{
    QMap<QString, QString> out;
    for (const QString &raw : lines) {
        const QString line = raw.trimmed();
        if (line.isEmpty() || line.startsWith(QLatin1Char('#')))
            continue;
        const int idx = line.indexOf(QLatin1Char('='));
        if (idx <= 0)
            continue;
        const QString key = line.left(idx).trimmed();
        const QString value = line.mid(idx + 1);
        if (!key.isEmpty())
            out.insert(key, value);
    }
    return out;
}

QStringList formatEnvAssignments(const QMap<QString, QString> &values)
{
    QStringList out;
    for (auto it = values.begin(); it != values.end(); ++it) {
        if (it.key().trimmed().isEmpty() || it.value().trimmed().isEmpty())
            continue;
        out.append(it.key() + QStringLiteral("=") + it.value());
    }
    return out;
}

QSet<QString> workflowModeledEnvNames(const WorkflowMeta &meta)
{
    QSet<QString> out;
    std::function<void(const QVector<WorkflowInputMeta> &)> walk = [&](const QVector<WorkflowInputMeta> &inputs) {
        for (const WorkflowInputMeta &input : inputs) {
            const QString envName = input.envName.trimmed().toUpper();
            if (!envName.isEmpty())
                out.insert(envName);
            if (!input.children.isEmpty())
                walk(input.children);
        }
    };
    walk(meta.inputs);
    return out;
}

QSet<QString> workflowModeledEnvPrefixes(const WorkflowMeta &meta)
{
    QSet<QString> out;
    std::function<void(const QVector<WorkflowInputMeta> &)> walk = [&](const QVector<WorkflowInputMeta> &inputs) {
        for (const WorkflowInputMeta &input : inputs) {
            const QString envName = input.envName.trimmed().toUpper();
            const int lastUnderscore = envName.lastIndexOf(QLatin1Char('_'));
            if (lastUnderscore > 0)
                out.insert(envName.left(lastUnderscore + 1));
            if (!input.children.isEmpty())
                walk(input.children);
        }
    };
    walk(meta.inputs);
    return out;
}

bool workflowNeedsConfigPromptInputs(const QVector<WorkflowInputMeta> &inputs, const QMap<QString, QString> &currentValues)
{
    for (const WorkflowInputMeta &input : inputs) {
        const QString envName = input.envName.trimmed().toUpper();
        if (!envName.isEmpty()) {
            if (!currentValues.value(envName).trimmed().isEmpty())
                continue;
            if (!input.defaultValue.trimmed().isEmpty())
                continue;
            return true;
        }
        if (!input.children.isEmpty() && workflowNeedsConfigPromptInputs(input.children, currentValues))
            return true;
    }
    return false;
}

QMap<QString, QString> pruneStaleWorkflowValues(const WorkflowMeta &meta, const QMap<QString, QString> &currentValues)
{
    if (meta.inputs.isEmpty())
        return currentValues;
    const QSet<QString> modeledNames = workflowModeledEnvNames(meta);
    const QSet<QString> modeledPrefixes = workflowModeledEnvPrefixes(meta);
    QMap<QString, QString> out = currentValues;
    for (auto it = out.begin(); it != out.end();) {
        const QString key = it.key().trimmed().toUpper();
        bool inModeledNamespace = false;
        for (const QString &prefix : modeledPrefixes) {
            if (key.startsWith(prefix)) {
                inModeledNamespace = true;
                break;
            }
        }
        if (inModeledNamespace && !modeledNames.contains(key))
            it = out.erase(it);
        else
            ++it;
    }
    return out;
}

bool workflowNeedsConfigPrompt(const WorkflowMeta &meta, const QMap<QString, QString> &currentValues)
{
    return workflowNeedsConfigPromptInputs(meta.inputs, currentValues);
}

QVector<WorkflowMeta> appWorkflowsFromCatalog(const WorkflowCatalogData &catalog)
{
    QVector<WorkflowMeta> out;
    QStringList seenDisplayNames;
    for (const WorkflowMeta &m : catalog.workflows) {
        if (m.category.compare(QStringLiteral("app"), Qt::CaseInsensitive) != 0)
            continue;
        const QString displayKey = m.displayName.trimmed().toLower();
        if (!displayKey.isEmpty() && seenDisplayNames.contains(displayKey))
            continue;
        out.append(m);
        if (!displayKey.isEmpty())
            seenDisplayNames.append(displayKey);
    }
    return out;
}

QVector<Context> contextsFromCatalog(const QString &workdir, const QString &repoRoot, const WorkflowCatalogData &catalog)
{
    Q_UNUSED(repoRoot);
    const QString clean = QDir::cleanPath(workdir);

    QString baseLabel = QFileInfo(clean).fileName();
    const QString gitRoot = GitHelper::repoRoot(clean);
    if (!gitRoot.isEmpty())
        baseLabel = QFileInfo(gitRoot).fileName() + QStringLiteral(" — ") + QFileInfo(clean).fileName();

    QVector<Context> out;
    if (catalog.workflows.isEmpty()) {
        Context c = Context::createNew();
        c.workdir = clean;
        c.label = baseLabel;
        c.workflow = QStringLiteral("vscode");
        c.dockpipeBinary = DockpipeChoices::preferredDockpipeBinary(clean);
        out.append(c);
        return out;
    }

    out.reserve(catalog.workflows.size());
    for (const WorkflowMeta &wf : catalog.workflows) {
        if (wf.workflowId.trimmed().isEmpty())
            continue;
        Context c = Context::createNew();
        c.workdir = clean;
        c.label = wf.displayName.isEmpty() ? wf.workflowId : wf.displayName;
        c.workflow = wf.workflowId;
        c.dockpipeBinary = DockpipeChoices::preferredDockpipeBinary(clean);
        out.append(c);
    }
    return out;
}

WorkflowCatalogData fastWorkflowShellCatalog(const QString &workdir)
{
    WorkflowCatalogData catalog;
    const QString clean = QDir::cleanPath(workdir);
    const QString repoRoot = DockpipeChoices::findRepoRoot(clean);
    if (repoRoot.isEmpty())
        return catalog;

    QSet<QString> seen;
    auto yamlScalarFromText = [](const QString &text, const QString &key) {
        const QRegularExpression re(QStringLiteral("(?m)^\\s*%1\\s*:\\s*([^#\\r\\n]+)").arg(QRegularExpression::escape(key)));
        const QRegularExpressionMatch match = re.match(text);
        if (!match.hasMatch())
            return QString();
        QString value = match.captured(1).trimmed();
        if ((value.startsWith(QLatin1Char('"')) && value.endsWith(QLatin1Char('"'))) ||
            (value.startsWith(QLatin1Char('\'')) && value.endsWith(QLatin1Char('\'')))) {
            value = value.mid(1, value.size() - 2);
        }
        return value.trimmed();
    };

    auto addWorkflow = [&](const QString &workflowId, const QString &configPath) {
        const QString id = workflowId.trimmed();
        if (id.isEmpty() || seen.contains(id))
            return;
        seen.insert(id);
        WorkflowMeta meta;
        meta.workflowId = id;
        meta.displayName = id;
        meta.configPath = configPath;
        QFile config(configPath);
        if (config.open(QIODevice::ReadOnly | QIODevice::Text)) {
            const QString text = QString::fromUtf8(config.readAll());
            meta.category = yamlScalarFromText(text, QStringLiteral("category"));
            meta.description = yamlScalarFromText(text, QStringLiteral("description"));
            const QString displayName = yamlScalarFromText(text, QStringLiteral("display_name"));
            if (!displayName.isEmpty())
                meta.displayName = displayName;
            const QString icon = yamlScalarFromText(text, QStringLiteral("icon"));
            if (!icon.isEmpty()) {
                const QFileInfo iconInfo(icon);
                meta.iconPath = iconInfo.isAbsolute() ? QDir::cleanPath(icon)
                                                      : QDir(QFileInfo(configPath).absolutePath()).filePath(icon);
            }
        }
        catalog.workflows.append(meta);
    };

    auto scanConfigs = [&](const QString &root) {
        if (!QFileInfo::exists(root))
            return;
        QDirIterator it(root, QStringList{QStringLiteral("config.yml")}, QDir::Files, QDirIterator::Subdirectories);
        while (it.hasNext()) {
            const QString configPath = QDir::cleanPath(it.next());
            const QString id = QFileInfo(QFileInfo(configPath).absolutePath()).fileName();
            addWorkflow(id, configPath);
        }
    };

    scanConfigs(QDir(repoRoot).filePath(QStringLiteral("workflows")));
    scanConfigs(QDir(repoRoot).filePath(QStringLiteral("src/core/workflows")));
    scanConfigs(QDir(repoRoot).filePath(QStringLiteral("packages")));

    std::sort(catalog.workflows.begin(), catalog.workflows.end(), [](const WorkflowMeta &a, const WorkflowMeta &b) {
        return a.workflowId.localeAwareCompare(b.workflowId) < 0;
    });
    return catalog;
}

} // namespace

MainWindow::MainWindow(QWidget *parent) : QMainWindow(parent), m_sessions(this)
{
    const QString version = QCoreApplication::applicationVersion();
    setWindowTitle(version.startsWith("dev+") ? tr("Dockpipe Launcher — %1").arg(version)
                                             : tr("Dockpipe Launcher"));
    setWindowIcon(QGuiApplication::windowIcon());
    resize(1180, 780);

    m_settings.load();
    applyGlobalRootDefault(m_settings.globalRootOverride);
    const QStringList args = QCoreApplication::arguments();
    const bool startHome = args.contains(QStringLiteral("--start-home"));
    m_workflowCatalogWatcher = new QFutureWatcher<AsyncWorkflowCatalogResult>(this);

    connect(m_workflowCatalogWatcher, &QFutureWatcher<AsyncWorkflowCatalogResult>::finished, this, [this]() {
        const AsyncWorkflowCatalogResult result = m_workflowCatalogWatcher->result();
        m_workflowCatalogRunningWorkdir.clear();
        const QString requested = QDir::cleanPath(m_workflowCatalogRequestedWorkdir);
        const QString currentProject = QDir::cleanPath(m_settings.projectFolder);
        if (result.workdir == requested && result.workdir == currentProject)
            applyWorkflowCatalogResult(result);
        if (m_workflowCatalogRefreshPending || requested != result.workdir) {
            m_workflowCatalogRefreshPending = false;
            startWorkflowCatalogDiscovery();
        }
    });

    setupMenuBar();

    auto *central = new QWidget(this);
    central->setObjectName(QStringLiteral("mainCentral"));
    auto *outer = new QVBoxLayout(central);
    outer->setContentsMargins(0, 0, 0, 0);
    outer->setSpacing(0);

    m_stack = new QStackedWidget;
    m_basicWidget = new BasicModeWidget(this);
    m_advancedPage = new QWidget;
    setupAdvancedPage(m_advancedPage);

    m_stack->addWidget(m_basicWidget);
    m_stack->addWidget(m_advancedPage);
    m_remoteWidget = new RemoteWidget;
    m_activityWidget = new ActivityWidget(m_sessions, m_store);
    m_stack->addWidget(m_remoteWidget);
    m_stack->addWidget(m_activityWidget);
    m_dockerWidget = new DockerObservabilityWidget;
    m_stack->addWidget(m_dockerWidget);

    auto *shell = new QHBoxLayout;
    shell->setSpacing(0);
    auto *rail = new QFrame;
    rail->setObjectName(QStringLiteral("navigationRail"));
    rail->setFixedWidth(196);
    auto *railLayout = new QVBoxLayout(rail);
    railLayout->setContentsMargins(16, 24, 16, 16);
    railLayout->setSpacing(12);
    auto *brand = new QLabel(tr("Dockpipe"));
    brand->setObjectName(QStringLiteral("appTitle"));
    railLayout->addWidget(brand);
    auto *tagline = new QLabel(tr("Your work. Anywhere."));
    tagline->setObjectName(QStringLiteral("appSubtitle"));
    railLayout->addWidget(tagline);
    m_navigation = new QListWidget;
    m_navigation->setObjectName(QStringLiteral("workspaceNavigation"));
    m_navigation->addItems({tr("Apps"), tr("Workflows"), tr("Machines"), tr("Activity"), tr("Docker")});
    m_navigation->setSpacing(6);
    m_navigation->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    for (int i = 0; i < m_navigation->count(); ++i)
        m_navigation->item(i)->setSizeHint(QSize(140, 42));
    railLayout->addWidget(m_navigation, 1);
    auto *packages = new QPushButton(tr("Packages"));
    auto *settings = new QPushButton(tr("Settings"));
    railLayout->addWidget(packages);
    railLayout->addWidget(settings);
    connect(packages, &QPushButton::clicked, this, &MainWindow::onManagePackages);
    connect(settings, &QPushButton::clicked, this, &MainWindow::onOpenSettings);
    shell->addWidget(rail);
    auto *workspace = new QVBoxLayout;
    workspace->setContentsMargins(0, 0, 0, 0);
    workspace->setSpacing(0);
    auto *workspaceBar = new QFrame;
    workspaceBar->setObjectName(QStringLiteral("workspaceBar"));
    auto *workspaceLayout = new QHBoxLayout(workspaceBar);
    workspaceLayout->setContentsMargins(28, 12, 28, 12);
    workspaceLayout->setSpacing(12);
    auto *workspaceLabel = new QLabel(tr("WORKSPACE"));
    workspaceLabel->setObjectName(QStringLiteral("eyebrow"));
    workspaceLayout->addWidget(workspaceLabel);
    m_workspaceButton = new QPushButton;
    m_workspaceButton->setObjectName(QStringLiteral("workspaceSelector"));
    m_workspaceButton->setMaximumWidth(280);
    auto *workspaceMenu = new QMenu(m_workspaceButton);
    connect(workspaceMenu, &QMenu::aboutToShow, this, [this, workspaceMenu]() {
        workspaceMenu->clear();
        workspaceMenu->addAction(tr("Open folder…"), this, &MainWindow::onFileOpenProject);
        if (!m_settings.recentProjectFolders.isEmpty())
            workspaceMenu->addSeparator();
        for (const QString &folder : m_settings.recentProjectFolders) {
            auto *action = workspaceMenu->addAction(QDir::toNativeSeparators(folder));
            connect(action, &QAction::triggered, this, [this, folder]() { onBasicOpenRecent(folder); });
        }
    });
    m_workspaceButton->setMenu(workspaceMenu);
    workspaceLayout->addWidget(m_workspaceButton);
    m_workspacePath = new QLabel;
    m_workspacePath->setTextFormat(Qt::PlainText);
    m_workspacePath->setObjectName(QStringLiteral("workspacePath"));
    m_workspacePath->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    workspaceLayout->addWidget(m_workspacePath, 1);
    workspace->addWidget(workspaceBar);
    workspace->addWidget(m_stack, 1);
    shell->addLayout(workspace, 1);
    outer->addLayout(shell, 1);
    connect(m_navigation, &QListWidget::currentRowChanged, this, [this](int page) {
        if (page < 0)
            return;
        m_stack->setCurrentIndex(page);
        m_dockerWidget->setActive(page == 4);
        m_actBasic->setChecked(page == 0);
        m_actAdvanced->setChecked(page == 1);
        m_actIcons->setEnabled(page == 0);
        m_actList->setEnabled(page == 0);
        if (page == 0) {
            m_settings.uiMode = QStringLiteral("basic");
            if (!m_settings.projectFolder.isEmpty())
                m_basicWidget->showWorkspacePage();
            updateBasicPage();
        } else if (page == 1) {
            m_settings.uiMode = QStringLiteral("advanced");
            startAdvancedContextDiscovery();
        } else if (page == 2) {
            m_remoteWidget->setWorkdir(m_settings.projectFolder);
            m_remoteWidget->refresh();
        } else if (page == 3) {
            m_activityWidget->setWorkdir(m_settings.projectFolder);
            m_activityWidget->refreshLocal();
        }
    });
    connect(m_remoteWidget, &RemoteWidget::nodesChanged, this, [this](const QStringList &nodes) {
        const QString selected = m_runTarget->currentData().toString();
        m_runTarget->clear();
        m_runTarget->addItem(tr("This computer"), QString());
        for (const auto &node : nodes)
            m_runTarget->addItem(node, node);
        const int index = m_runTarget->findData(selected);
        m_runTarget->setCurrentIndex(index < 0 ? 0 : index);
    });
    connect(m_stack, &QStackedWidget::currentChanged, this, [this](int index) {
        if (m_navigation->currentRow() != index)
            m_navigation->setCurrentRow(index);
    });

    setCentralWidget(central);

    setupDisclaimerBar();

    connect(m_basicWidget, &BasicModeWidget::packagesRequested, this, &MainWindow::onManagePackages);
    connect(m_basicWidget, &BasicModeWidget::openProjectRequested, this, &MainWindow::onFileOpenProject);
    connect(m_basicWidget, &BasicModeWidget::refreshAppsRequested, this, &MainWindow::onRefreshAppList);
    connect(m_basicWidget, &BasicModeWidget::launchRequested, this, &MainWindow::onBasicLaunch);
    connect(m_basicWidget, &BasicModeWidget::configureRequested, this, &MainWindow::onBasicConfigure);
    connect(m_basicWidget, &BasicModeWidget::recentProjectSelected, this, &MainWindow::onBasicOpenRecent);
    connect(m_basicWidget, &BasicModeWidget::continueLastRequested, this, &MainWindow::onBasicContinueLast);
    connect(m_basicWidget, &BasicModeWidget::backToHomeRequested, this, &MainWindow::onBasicBackHome);

    connect(&m_sessions, &SessionManager::sessionStarted, this, &MainWindow::onSessionChanged);
    connect(&m_sessions, &SessionManager::sessionReady, this, &MainWindow::onSessionChanged);
    connect(&m_sessions, &SessionManager::sessionStopped, this, &MainWindow::onSessionChanged);
    connect(&m_sessions, &SessionManager::sessionFailed, this, &MainWindow::onSessionChanged);
    connect(&m_sessions, &SessionManager::sessionOutput, this, &MainWindow::onSessionOutput);
    connect(&m_sessions, &SessionManager::sessionPrompt, this, &MainWindow::onSessionPrompt);
    connect(&m_sessions, &SessionManager::sessionFailed, this,
            [this](const QString &, const QString &err) { QMessageBox::warning(this, tr("Dockpipe Launcher"), err); });
    QTimer::singleShot(0, this, [this, startHome]() {
        m_store.load();
        setupTray();
        if (startHome) {
            activateHome();
            QTimer::singleShot(200, this, [this]() {
                rebuildAdvancedContextList();
                refreshInlineConsole();
            });
            return;
        }
        rebuildUi();
        applyUiMode();
    });
}

void MainWindow::setupDisclaimerBar()
{
    if (m_settings.thirdPartyDisclaimerDismissed || m_disclaimerContainer)
        return;

    auto *wrap = new QWidget;
    auto *lay = new QHBoxLayout(wrap);
    lay->setContentsMargins(4, 0, 4, 0);
    lay->setSpacing(8);

    auto *disclaimer = new QLabel(
        tr("Notice: Dockpipe Launcher does not distribute third-party applications. Dockpipe workflows run on "
           "your machine; install tools from official vendor or distribution channels and accept each "
           "publisher’s terms."));
    disclaimer->setObjectName(QStringLiteral("disclaimerWatermark"));
    disclaimer->setWordWrap(true);
    disclaimer->setAlignment(Qt::AlignLeft | Qt::AlignVCenter);

    auto *dismiss = new QPushButton(tr("Dismiss"));
    dismiss->setObjectName(QStringLiteral("disclaimerDismiss"));
    dismiss->setCursor(Qt::PointingHandCursor);
    connect(dismiss, &QPushButton::clicked, this, &MainWindow::onDismissThirdPartyDisclaimer);

    lay->addWidget(disclaimer, 1);
    lay->addWidget(dismiss, 0, Qt::AlignTop);

    m_disclaimerContainer = wrap;
    statusBar()->addWidget(wrap, 1);
}

void MainWindow::onDismissThirdPartyDisclaimer()
{
    m_settings.thirdPartyDisclaimerDismissed = true;
    m_settings.save();
    if (!m_disclaimerContainer)
        return;
    statusBar()->removeWidget(m_disclaimerContainer);
    QWidget *w = m_disclaimerContainer;
    m_disclaimerContainer = nullptr;
    w->deleteLater();
}

void MainWindow::onRestoreThirdPartyDisclaimer()
{
    m_settings.thirdPartyDisclaimerDismissed = false;
    m_settings.save();
    setupDisclaimerBar();
}

void MainWindow::setupMenuBar()
{
    QMenu *file = menuBar()->addMenu(tr("File"));
    file->addAction(tr("Open project folder…"), this, &MainWindow::onFileOpenProject, QKeySequence::Open);
    file->addAction(tr("Refresh app list"), this, &MainWindow::onRefreshAppList, QKeySequence::Refresh);
    file->addSeparator();
    file->addAction(tr("Quit"), qApp, &QApplication::quit, QKeySequence::Quit);

    QMenu *view = menuBar()->addMenu(tr("View"));
    auto *modeGroup = new QActionGroup(this);
    m_actBasic = view->addAction(tr("Apps"));
    m_actBasic->setCheckable(true);
    modeGroup->addAction(m_actBasic);
    m_actAdvanced = view->addAction(tr("Workflows"));
    m_actAdvanced->setCheckable(true);
    modeGroup->addAction(m_actAdvanced);
    connect(m_actBasic, &QAction::triggered, this, &MainWindow::onViewBasic);
    connect(m_actAdvanced, &QAction::triggered, this, &MainWindow::onViewAdvanced);

    QMenu *settingsMenu = menuBar()->addMenu(tr("Settings"));
    settingsMenu->addAction(tr("Preferences…"), this, &MainWindow::onOpenSettings);

    QMenu *packagesMenu = menuBar()->addMenu(tr("Packages"));
    packagesMenu->addAction(tr("Manage Packages…"), this, &MainWindow::onManagePackages);

    QMenu *help = menuBar()->addMenu(tr("Help"));
    help->addAction(tr("About Dockpipe Launcher…"), this, &MainWindow::onAbout);
    help->addSeparator();
    help->addAction(tr("Show notice in status bar again"), this, &MainWindow::onRestoreThirdPartyDisclaimer);
    help->addAction(tr("Third-party software notice…"), this, [this]() {
        QMessageBox::information(
            this, tr("Third-party software"),
            tr("Dockpipe Launcher is a launcher for dockpipe workflows. It does not ship or bundle third-party "
               "applications.\n\n"
               "If a workflow needs external tools, you install them yourself from official sources. Use of "
               "those products is subject to their respective licensors’ terms."));
    });

    view->addSeparator();
    auto *viewGroup = new QActionGroup(this);
    m_actIcons = view->addAction(tr("Icon grid"));
    m_actIcons->setCheckable(true);
    viewGroup->addAction(m_actIcons);
    m_actList = view->addAction(tr("Compact list"));
    m_actList->setCheckable(true);
    viewGroup->addAction(m_actList);
    connect(m_actIcons, &QAction::triggered, this, &MainWindow::onViewIconGrid);
    connect(m_actList, &QAction::triggered, this, &MainWindow::onViewCompactList);
}

void MainWindow::onAbout()
{
    QMessageBox box(this);
    box.setWindowTitle(tr("About Dockpipe Launcher"));
    box.setIcon(QMessageBox::Information);
    box.setTextFormat(Qt::RichText);
    box.setTextInteractionFlags(Qt::TextBrowserInteraction);
    box.setText(
        tr("<h3>Dockpipe Launcher</h3>"
           "<p>Version: %1</p>"
           "<p>Dockpipe Launcher is the desktop shell and local-first workspace surface for Dockpipe workflows.</p>"
           "<p><a href=\"https://dockpipe.com\">dockpipe.com</a></p>")
            .arg(QCoreApplication::applicationVersion().toHtmlEscaped()));
    box.setStandardButtons(QMessageBox::Ok);
    box.exec();
}

void MainWindow::setupAdvancedPage(QWidget *page)
{
    auto *root = new QVBoxLayout(page);
    root->setSpacing(14);
    root->setContentsMargins(28, 24, 28, 24);

    auto *header = new QFrame;
    header->setObjectName(QStringLiteral("workflowHeader"));
    auto *headLay = new QVBoxLayout(header);
    headLay->setSpacing(12);
    headLay->setContentsMargins(0, 0, 0, 0);

    auto *title = new QLabel(tr("Workflows"));
    title->setObjectName(QStringLiteral("appTitle"));
    auto *subtitle = new QLabel(tr("Choose what to run, choose a machine, and follow its progress in Activity."));
    subtitle->setObjectName(QStringLiteral("appSubtitle"));
    subtitle->setWordWrap(true);
    headLay->addWidget(title);
    headLay->addWidget(subtitle);

    auto *primaryRow = new QHBoxLayout;
    primaryRow->setSpacing(8);
    auto addPrimary = [this, primaryRow](const QString &text, void (MainWindow::*slot)(), const char *objName) {
        auto *b = new QPushButton(text);
        b->setObjectName(QString::fromUtf8(objName));
        connect(b, &QPushButton::clicked, this, slot);
        primaryRow->addWidget(b);
    };
    auto *targetLabel = new QLabel(tr("Run on"));
    primaryRow->addWidget(targetLabel);
    m_runTarget = new QComboBox;
    m_runTarget->setObjectName(QStringLiteral("runTarget"));
    m_runTarget->addItem(tr("This computer"), QString());
    m_runTarget->setMinimumWidth(180);
    primaryRow->addWidget(m_runTarget);
    addPrimary(tr("Run selected"), &MainWindow::onLaunch, "primaryButton");
    addPrimary(tr("Stop"), &MainWindow::onStop, "quietButton");
    addPrimary(tr("Logs"), &MainWindow::onOpenLogs, "quietButton");
    primaryRow->addStretch(1);
    headLay->addLayout(primaryRow);

    root->addWidget(header);
    auto *contextsPage = new QWidget(page);
    auto *contextsRoot = new QVBoxLayout(contextsPage);
    contextsRoot->setContentsMargins(0, 0, 0, 0);
    contextsRoot->setSpacing(14);

    m_hint = new QLabel(tr("Workflows below are discovered from the current project folder. Right-click a row for actions."));
    m_hint->setObjectName(QStringLiteral("hintText"));
    m_hint->setWordWrap(true);
    contextsRoot->addWidget(m_hint);

    m_search = new QLineEdit(page);
    m_search->setObjectName(QStringLiteral("surfaceSearch"));
    m_search->setClearButtonEnabled(true);
    m_search->setPlaceholderText(tr("Search workflows by label, folder, workflow, resolver…"));
    connect(m_search, &QLineEdit::textChanged, this, &MainWindow::onAdvancedSearchChanged);
    contextsRoot->addWidget(m_search);

    m_advancedSearchTimer = new QTimer(this);
    m_advancedSearchTimer->setSingleShot(true);
    m_advancedSearchTimer->setInterval(120);
    connect(m_advancedSearchTimer, &QTimer::timeout, this, &MainWindow::applyAdvancedContextFilter);

    m_basicLaunchingTimer = new QTimer(this);
    m_basicLaunchingTimer->setInterval(1000);
    connect(m_basicLaunchingTimer, &QTimer::timeout, this, [this]() {
        if (m_basicLaunchingContextId.isEmpty()) {
            m_basicLaunchingTimer->stop();
            return;
        }
        const bool stillRunning = m_sessions.isRunning(m_basicLaunchingContextId);
        const SessionInfo si = m_sessions.info(m_basicLaunchingContextId);
        if (!stillRunning || si.ready || si.status == SessionStatus::Failed || si.status == SessionStatus::Stopped) {
            m_basicLaunchingContextId.clear();
            m_basicLaunchingWorkflowId.clear();
            m_basicLaunchingTimer->stop();
            refreshSessionUi();
        }
    });

    auto *splitter = new QSplitter(Qt::Vertical, page);
    splitter->setChildrenCollapsible(false);

    auto *listPanel = new QFrame;
    listPanel->setObjectName(QStringLiteral("listPanel"));
    auto *listOuter = new QVBoxLayout(listPanel);
    listOuter->setContentsMargins(0, 0, 0, 0);

    m_list = new QListWidget(page);
    m_list->setFrameShape(QFrame::NoFrame);
    m_list->setSpacing(2);
    m_list->setUniformItemSizes(true);
    m_list->setContextMenuPolicy(Qt::CustomContextMenu);
    listOuter->addWidget(m_list, 1);

    m_emptyState = new QFrame;
    m_emptyState->setObjectName(QStringLiteral("emptyState"));
    auto *emptyLay = new QVBoxLayout(m_emptyState);
    emptyLay->setContentsMargins(28, 36, 28, 36);
    emptyLay->setSpacing(8);
    m_emptyTitle = new QLabel(tr("No workflows yet"));
    m_emptyTitle->setObjectName(QStringLiteral("emptyTitle"));
    m_emptyTitle->setAlignment(Qt::AlignCenter);
    m_emptyBody = new QLabel(
        tr("Open a project folder with File → Open project folder."));
    m_emptyBody->setObjectName(QStringLiteral("emptyBody"));
    m_emptyBody->setWordWrap(true);
    m_emptyBody->setAlignment(Qt::AlignCenter);
    emptyLay->addWidget(m_emptyTitle);
    emptyLay->addWidget(m_emptyBody);

    listOuter->addWidget(m_emptyState, 1);
    listOuter->addWidget(m_list, 1);
    m_emptyState->hide();
    m_list->hide();

    splitter->addWidget(listPanel);

    auto *consolePanel = new QFrame;
    consolePanel->setObjectName(QStringLiteral("inlineConsolePanel"));
    auto *consoleLay = new QVBoxLayout(consolePanel);
    consoleLay->setContentsMargins(12, 12, 12, 12);
    consoleLay->setSpacing(8);

    m_consoleTitle = new QLabel(tr("Run output"));
    m_consoleTitle->setObjectName(QStringLiteral("consoleTitle"));
    m_consoleMeta = new QLabel(tr("Select a workflow row, then launch it to see output here."));
    m_consoleMeta->setObjectName(QStringLiteral("consoleMeta"));
    m_consoleMeta->setWordWrap(true);

    m_console = new QPlainTextEdit(consolePanel);
    m_console->setObjectName(QStringLiteral("inlineConsole"));
    m_console->setReadOnly(true);
    m_console->setPlaceholderText(tr("Command output will appear here."));
    m_console->setLineWrapMode(QPlainTextEdit::NoWrap);
    m_console->setMinimumHeight(120);
    m_console->setFont(QFontDatabase::systemFont(QFontDatabase::FixedFont));

    consoleLay->addWidget(m_consoleTitle);
    consoleLay->addWidget(m_consoleMeta);
    consoleLay->addWidget(m_console, 1);
    splitter->addWidget(consolePanel);
    splitter->setStretchFactor(0, 3);
    splitter->setStretchFactor(1, 2);
    splitter->setSizes({360, 240});
    contextsRoot->addWidget(splitter, 1);

    root->addWidget(contextsPage, 1);

    connect(m_list, &QListWidget::customContextMenuRequested, this, [this](const QPoint &p) {
        if (QListWidgetItem *it = m_list->itemAt(p))
            applyContextMenu(it, m_list->mapToGlobal(p));
    });
    connect(m_list, &QListWidget::itemSelectionChanged, this, &MainWindow::onAdvancedSelectionChanged);
}

void MainWindow::applyUiMode()
{
    m_actBasic->blockSignals(true);
    m_actAdvanced->blockSignals(true);
    m_actIcons->blockSignals(true);
    m_actList->blockSignals(true);

    m_stack->setCurrentIndex(m_settings.isAdvanced() ? 1 : 0);
    m_navigation->setCurrentRow(m_stack->currentIndex());
    m_actBasic->setChecked(!m_settings.isAdvanced());
    m_actAdvanced->setChecked(m_settings.isAdvanced());
    const bool basic = !m_settings.isAdvanced();
    m_actIcons->setEnabled(basic);
    m_actList->setEnabled(basic);
    m_actIcons->setChecked(basic && m_settings.isBasicIcons());
    m_actList->setChecked(basic && !m_settings.isBasicIcons());
    m_basicWidget->setViewIconMode(m_settings.isBasicIcons());
    m_basicWidget->setRecentProjects(m_settings.recentProjectFolders);
    m_basicWidget->setContinueLastVisible(!m_settings.projectFolder.isEmpty()
                                           || !m_settings.recentProjectFolders.isEmpty());
    if (basic) {
        if (m_settings.projectFolder.isEmpty())
            m_basicWidget->showHomePage();
        else
            m_basicWidget->showWorkspacePage();
    }

    m_actBasic->blockSignals(false);
    m_actAdvanced->blockSignals(false);
    m_actIcons->blockSignals(false);
    m_actList->blockSignals(false);

    rebuildAdvancedContextList();
    updateBasicPage();
    refreshInlineConsole();
}

void MainWindow::onViewBasic()
{
    m_settings.uiMode = QStringLiteral("basic");
    m_settings.save();
    applyUiMode();
}

void MainWindow::onViewAdvanced()
{
    m_settings.uiMode = QStringLiteral("advanced");
    m_settings.save();
    applyUiMode();
}

void MainWindow::onViewIconGrid()
{
    m_settings.basicView = QStringLiteral("icons");
    m_settings.save();
    m_basicWidget->setViewIconMode(true);
    m_actIcons->setChecked(true);
    m_actList->setChecked(false);
}

void MainWindow::onViewCompactList()
{
    m_settings.basicView = QStringLiteral("list");
    m_settings.save();
    m_basicWidget->setViewIconMode(false);
    m_actIcons->setChecked(false);
    m_actList->setChecked(true);
}

void MainWindow::onOpenSettings()
{
    SettingsDialog dialog(m_settings, this);
    if (dialog.exec() != QDialog::Accepted)
        return;
    m_settings = dialog.updatedSettings();
    m_settings.save();
    applyGlobalRootDefault(m_settings.globalRootOverride);
    rebuildUi();
}

void MainWindow::onManagePackages()
{
    PackageManagerDialog dialog(m_settings.projectFolder, this);
    dialog.exec();
    onRefreshAppList();
}

void MainWindow::onFileOpenProject()
{
    const QString d = QFileDialog::getExistingDirectory(this, tr("Open project folder"), m_settings.projectFolder);
    if (d.isEmpty())
        return;
    m_settings.projectFolder = QDir::cleanPath(d);
    m_settings.addRecentProject(m_settings.projectFolder);
    m_settings.save();
    m_basicWidget->setRecentProjects(m_settings.recentProjectFolders);
    m_basicWidget->setContinueLastVisible(true);
    m_basicWidget->showWorkspacePage();
    rebuildUi();
}

void MainWindow::onBasicBackHome()
{
    m_basicWidget->setRecentProjects(m_settings.recentProjectFolders);
    m_basicWidget->setContinueLastVisible(!m_settings.projectFolder.isEmpty()
                                           || !m_settings.recentProjectFolders.isEmpty());
    m_basicWidget->showHomePage();
}

void MainWindow::onBasicOpenRecent(const QString &absPath)
{
    if (absPath.isEmpty())
        return;
    m_settings.projectFolder = QDir::cleanPath(absPath);
    m_settings.addRecentProject(m_settings.projectFolder);
    m_settings.save();
    m_basicWidget->setRecentProjects(m_settings.recentProjectFolders);
    m_basicWidget->setContinueLastVisible(true);
    m_basicWidget->showWorkspacePage();
    rebuildUi();
}

void MainWindow::onBasicContinueLast()
{
    QString folder = m_settings.projectFolder;
    if (folder.isEmpty() && !m_settings.recentProjectFolders.isEmpty())
        folder = m_settings.recentProjectFolders.first();
    if (folder.isEmpty())
        return;
    m_settings.projectFolder = QDir::cleanPath(folder);
    m_settings.addRecentProject(m_settings.projectFolder);
    m_settings.save();
    m_basicWidget->setRecentProjects(m_settings.recentProjectFolders);
    m_basicWidget->setContinueLastVisible(true);
    m_basicWidget->showWorkspacePage();
    rebuildUi();
}

void MainWindow::activateHome()
{
    m_settings.uiMode = QStringLiteral("basic");
    m_settings.projectFolder.clear();
    m_settings.save();
    m_basicWidget->setRecentProjects(m_settings.recentProjectFolders);
    m_basicWidget->setContinueLastVisible(!m_settings.recentProjectFolders.isEmpty());
    m_basicWidget->showHomePage();
    applyUiMode();
    show();
    if (isMinimized())
        showNormal();
    raise();
    activateWindow();
}

void MainWindow::onRefreshAppList()
{
    startWorkflowCatalogDiscovery();
    refreshInlineConsole();
}

void MainWindow::updateBasicPage()
{
    const QString folder = QDir::toNativeSeparators(m_settings.projectFolder);
    const QString name = QFileInfo(m_settings.projectFolder).fileName();
    QString workspaceName = name.isEmpty() ? folder : name;
    if (folder.isEmpty())
        workspaceName = tr("Open a project");
    m_workspaceButton->setText(workspaceName);
    m_workspaceButton->setToolTip(folder);
    m_workspacePath->setText(folder);
    m_workspacePath->setToolTip(folder);
    m_remoteWidget->setWorkdir(m_settings.projectFolder);
    m_activityWidget->setWorkdir(m_settings.projectFolder);
    if (m_settings.projectFolder.isEmpty()) {
        m_basicAppsLoading = false;
        m_basicApps.clear();
        m_basicWidget->setApps({});
        m_basicWidget->setAppDiscoveryLoading(false);
        m_basicWidget->setRunningByWorkflow({});
        m_basicWidget->clearLaunchingWorkflow();
        return;
    }
    m_basicWidget->setApps(m_basicApps);
    m_basicWidget->setAppDiscoveryLoading(m_basicAppsLoading);

    QHash<QString, bool> run;
    if (!m_settings.projectFolder.isEmpty()) {
        const QString wd = QDir::cleanPath(m_settings.projectFolder);
        for (const Context &c : m_store.contexts) {
            if (QDir::cleanPath(c.workdir) != wd)
                continue;
            if (m_sessions.isRunning(c.id))
                run.insert(c.workflow, true);
        }
    }
    m_basicWidget->setRunningByWorkflow(run);
    if (!m_basicLaunchingWorkflowId.isEmpty()) {
        if (m_basicLaunchingTimer && !m_basicLaunchingTimer->isActive())
            m_basicLaunchingTimer->start();
        QString label = m_basicLaunchingWorkflowId;
        for (const WorkflowMeta &meta : m_basicApps) {
            QString metaWorkflowId = meta.workflowId;
            if (metaWorkflowId == QStringLiteral("pipeon") || metaWorkflowId == QStringLiteral("Pipeon"))
                metaWorkflowId = QStringLiteral("pipeon-dev-stack");
            if (metaWorkflowId == m_basicLaunchingWorkflowId) {
                label = meta.displayName;
                break;
            }
        }
        m_basicWidget->setLaunchingWorkflow(m_basicLaunchingWorkflowId, label);
    } else {
        if (m_basicLaunchingTimer)
            m_basicLaunchingTimer->stop();
        m_basicWidget->clearLaunchingWorkflow();
    }
}

void MainWindow::refreshSessionUi()
{
    applyAdvancedContextFilter();
    updateBasicPage();
    refreshInlineConsole();
}

void MainWindow::onBasicLaunch(const QString &workflowId)
{
    if (m_settings.projectFolder.isEmpty()) {
        QMessageBox::information(this, tr("Dockpipe Launcher"),
                                 tr("Choose a project folder first (File → Open project folder, or Choose folder…)."));
        return;
    }
    m_settings.addRecentProject(m_settings.projectFolder);
    m_settings.save();
    m_basicWidget->setRecentProjects(m_settings.recentProjectFolders);
    QString effectiveWorkflowId = workflowId;
    if (effectiveWorkflowId == QStringLiteral("pipeon") || effectiveWorkflowId == QStringLiteral("Pipeon"))
        effectiveWorkflowId = QStringLiteral("pipeon-dev-stack");
    WorkflowMeta meta;
    for (const WorkflowMeta &candidate : m_basicApps) {
        QString metaWorkflowId = candidate.workflowId;
        if (metaWorkflowId == QStringLiteral("pipeon") || metaWorkflowId == QStringLiteral("Pipeon"))
            metaWorkflowId = QStringLiteral("pipeon-dev-stack");
        if (metaWorkflowId == effectiveWorkflowId) {
            meta = candidate;
            m_basicLaunchingWorkflowId = effectiveWorkflowId;
            break;
        }
    }
    if (m_basicLaunchingWorkflowId.isEmpty())
        m_basicLaunchingWorkflowId = effectiveWorkflowId;
    updateBasicPage();

    QTimer::singleShot(0, this, [this, effectiveWorkflowId, meta]() {
        Context *c = ensureBasicWorkflowContext(effectiveWorkflowId);
        if (!c) {
            m_basicLaunchingContextId.clear();
            m_basicLaunchingWorkflowId.clear();
            updateBasicPage();
            return;
        }
        if (!configureContextForWorkflow(*c, meta, false)) {
            m_basicLaunchingContextId.clear();
            m_basicLaunchingWorkflowId.clear();
            updateBasicPage();
            return;
        }
        m_basicLaunchingContextId = c->id;
        if (m_sessions.launch(*c, ContextStore::logsDir()))
            refreshSessionUi();
        else if (!m_sessions.isRunning(c->id)) {
            m_basicLaunchingContextId.clear();
            m_basicLaunchingWorkflowId.clear();
            updateBasicPage();
            QMessageBox::warning(this, tr("Dockpipe Launcher"), tr("Could not start dockpipe (see stderr)."));
        }
    });
}

void MainWindow::onBasicConfigure(const QString &workflowId)
{
    if (workflowId.trimmed().isEmpty() || m_settings.projectFolder.trimmed().isEmpty())
        return;
    const WorkflowMeta meta = findWorkflowMeta(m_settings.projectFolder, workflowId, QString());
    Context *c = ensureBasicWorkflowContext(workflowId);
    if (!c)
        return;
    configureContextForWorkflow(*c, meta, true);
}

Context *MainWindow::ensureBasicWorkflowContext(const QString &workflowId)
{
    Context *c = findContext(m_settings.projectFolder, workflowId, QString());
    if (!c) {
        Context nc = Context::createNew();
        nc.workdir = m_settings.projectFolder;
        nc.workflow = workflowId;
        nc.dockpipeBinary = DockpipeChoices::preferredDockpipeBinary(m_settings.projectFolder);
        nc.label = QFileInfo(m_settings.projectFolder).fileName() + QStringLiteral(" — ") + workflowId;
        m_store.contexts.append(nc);
        m_store.save();
        c = &m_store.contexts.last();
    } else if (c->dockpipeBinary.trimmed().isEmpty() || c->dockpipeBinary.trimmed() == QStringLiteral("dockpipe")) {
        c->dockpipeBinary = DockpipeChoices::preferredDockpipeBinary(m_settings.projectFolder);
        m_store.save();
    }
    return c;
}

WorkflowMeta MainWindow::findWorkflowMeta(const QString &workdir, const QString &workflowId, const QString &workflowFile) const
{
    if (!workflowId.trimmed().isEmpty()) {
        for (const WorkflowMeta &meta : m_basicApps) {
            QString metaWorkflowId = meta.workflowId;
            if (metaWorkflowId == QStringLiteral("pipeon") || metaWorkflowId == QStringLiteral("Pipeon"))
                metaWorkflowId = QStringLiteral("pipeon-dev-stack");
            if (metaWorkflowId == workflowId)
                return meta;
        }
    }
    const QString repo = DockpipeChoices::findRepoRoot(workdir);
    const QVector<WorkflowMeta> all = WorkflowCatalog::discoverAll(repo, workdir);
    for (const WorkflowMeta &meta : all) {
        if (!workflowFile.trimmed().isEmpty() && QDir::cleanPath(meta.configPath) == QDir::cleanPath(workflowFile))
            return meta;
        if (!workflowId.trimmed().isEmpty() && meta.workflowId == workflowId)
            return meta;
    }
    return WorkflowMeta{};
}

bool MainWindow::configureContextForWorkflow(Context &ctx, const WorkflowMeta &meta, bool forceDialog)
{
    if (meta.workflowId.trimmed().isEmpty())
        return false;
    if (meta.inputs.isEmpty()) {
        if (forceDialog)
            return openWorkflowConfig(meta);
        return true;
    }
    const QMap<QString, QString> currentValues = parseEnvAssignments(ctx.extraDockpipeEnv);
    const QMap<QString, QString> prunedValues = pruneStaleWorkflowValues(meta, currentValues);
    if (prunedValues != currentValues) {
        ctx.extraDockpipeEnv = formatEnvAssignments(prunedValues);
        m_store.save();
    }
    if (!forceDialog && !workflowNeedsConfigPrompt(meta, prunedValues))
        return true;
    WorkflowLaunchDialog dialog(meta, prunedValues, this);
    if (dialog.exec() != QDialog::Accepted)
        return false;
    QMap<QString, QString> merged = prunedValues;
    const QMap<QString, QString> edited = dialog.values();
    for (auto it = edited.begin(); it != edited.end(); ++it) {
        if (it.value().trimmed().isEmpty())
            merged.remove(it.key());
        else
            merged.insert(it.key(), it.value());
    }
    ctx.extraDockpipeEnv = formatEnvAssignments(merged);
    m_store.save();
    return true;
}

bool MainWindow::openWorkflowConfig(const WorkflowMeta &meta)
{
    const QString configPath = QDir::cleanPath(meta.configPath);
    if (configPath.isEmpty()) {
        QMessageBox::information(this, tr("Dockpipe Launcher"),
                                 tr("This workflow does not expose launcher-managed settings."));
        return false;
    }
    if (!QFileInfo::exists(configPath)) {
        QMessageBox::warning(this, tr("Dockpipe Launcher"),
                             tr("The workflow file could not be found:\n%1").arg(QDir::toNativeSeparators(configPath)));
        return false;
    }
    if (!QDesktopServices::openUrl(QUrl::fromLocalFile(configPath))) {
        QMessageBox::warning(this, tr("Dockpipe Launcher"),
                             tr("Could not open the workflow file:\n%1").arg(QDir::toNativeSeparators(configPath)));
        return false;
    }
    return true;
}

void MainWindow::setupTray()
{
    m_tray = new QSystemTrayIcon(QGuiApplication::windowIcon(), this);
    m_tray->setToolTip(tr("Dockpipe Launcher"));
    auto *menu = new QMenu(this);
    menu->addAction(tr("Show"), this, [this]() { show(); raise(); activateWindow(); });
    menu->addSeparator();
    menu->addAction(tr("Quit"), qApp, &QApplication::quit);
    m_tray->setContextMenu(menu);
    connect(m_tray, &QSystemTrayIcon::activated, this, &MainWindow::onTrayActivate);
    m_tray->show();
}

void MainWindow::onTrayActivate(QSystemTrayIcon::ActivationReason reason)
{
    if (reason == QSystemTrayIcon::Trigger || reason == QSystemTrayIcon::DoubleClick) {
        if (isVisible())
            hide();
        else {
            show();
            raise();
            activateWindow();
        }
    }
}

void MainWindow::closeEvent(QCloseEvent *event)
{
    if (m_tray && m_tray->isVisible()) {
        hide();
        event->ignore();
        return;
    }
    QMainWindow::closeEvent(event);
}

void MainWindow::clearContextList()
{
    while (m_list->count() > 0) {
        QListWidgetItem *it = m_list->item(0);
        QWidget *w = m_list->itemWidget(it);
        m_list->removeItemWidget(it);
        delete w;
        delete m_list->takeItem(0);
    }
}

void MainWindow::rebuildAdvancedContextList()
{
    const bool hasProject = !m_settings.projectFolder.isEmpty();
    if (!hasProject) {
        m_advancedDiscoveryLoading = false;
        m_advancedSourceContexts.clear();
        applyAdvancedContextFilter();
        return;
    }
    startAdvancedContextDiscovery();
}

void MainWindow::applyAdvancedContextFilter()
{
    QString selectedKey;
    if (Context *current = currentAdvancedDisplayContext())
        selectedKey = contextDisplayKey(*current);

    if (m_list)
        m_list->setUpdatesEnabled(false);
    clearContextList();
    m_advancedContexts.clear();

    const QString filter = m_search ? m_search->text() : QString();
    int visibleCount = 0;
    const bool hasProject = !m_settings.projectFolder.isEmpty();
    const bool noProjectRows = m_advancedSourceContexts.isEmpty();

    if (m_emptyTitle && m_emptyBody) {
        if (!hasProject) {
            m_emptyTitle->setText(tr("No project selected"));
            m_emptyBody->setText(tr("Open a project folder with File → Open project folder."));
        } else if (m_advancedDiscoveryLoading) {
            m_emptyTitle->setText(tr("Loading workflows"));
            m_emptyBody->setText(tr("Dockpipe is discovering workflows for the current project folder."));
        } else if (noProjectRows) {
            m_emptyTitle->setText(tr("No workflows found"));
            m_emptyBody->setText(
                tr("No Dockpipe workflows were discovered for the current project folder."));
        } else {
            m_emptyTitle->setText(tr("No matching workflows"));
            m_emptyBody->setText(tr("Try a different search, or clear the filter to show every workflow."));
        }
    }

    int restoreRow = -1;
    for (Context c : m_advancedSourceContexts) {
        if (Context *stored = findStoredContextForDisplay(c)) {
            c = *stored;
        } else if (c.dockpipeBinary.trimmed().isEmpty()) {
            c.dockpipeBinary = DockpipeChoices::preferredDockpipeBinary(c.workdir);
        }
        if (!contextMatchesFilter(c, filter))
            continue;
        ++visibleCount;
        m_advancedContexts.append(c);

        bool running = false;
        bool failed = false;
        QString st = tr("Ready");
        if (Context *stored = findStoredContextForDisplay(c))
            st = statusLabel(m_sessions, stored->id, &running, &failed);

        auto *item = new QListWidgetItem;
        item->setData(Qt::UserRole, m_advancedContexts.size() - 1);
        item->setSizeHint(QSize(0, 76));
        m_list->addItem(item);
        if (!selectedKey.isEmpty() && contextDisplayKey(c) == selectedKey)
            restoreRow = m_list->count() - 1;

        auto *row = new ContextRowWidget(c, st, running, failed, m_list);
        m_list->setItemWidget(item, row);
    }

    const bool empty = visibleCount == 0;
    m_emptyState->setVisible(empty);
    m_list->setVisible(!empty);
    if (restoreRow >= 0)
        m_list->setCurrentRow(restoreRow);
    if (m_list)
        m_list->setUpdatesEnabled(true);
}

void MainWindow::rebuildUi()
{
    startWorkflowCatalogDiscovery();
    updateBasicPage();
    refreshInlineConsole();
}

void MainWindow::onSessionChanged()
{
    if (!m_basicLaunchingContextId.isEmpty()) {
        const bool stillRunning = m_sessions.isRunning(m_basicLaunchingContextId);
        const SessionInfo si = m_sessions.info(m_basicLaunchingContextId);
        if (!stillRunning || si.ready || si.status == SessionStatus::Failed || si.status == SessionStatus::Stopped) {
            m_basicLaunchingContextId.clear();
            m_basicLaunchingWorkflowId.clear();
        }
    }
    refreshSessionUi();
}

void MainWindow::startAdvancedContextDiscovery()
{
    startWorkflowCatalogDiscovery();
}

void MainWindow::startBasicAppDiscovery()
{
    startWorkflowCatalogDiscovery();
}

void MainWindow::startWorkflowCatalogDiscovery()
{
    const QString workdir = QDir::cleanPath(m_settings.projectFolder);
    if (workdir.isEmpty()) {
        m_workflowCatalogRequestedWorkdir.clear();
        m_workflowCatalogRunningWorkdir.clear();
        m_workflowCatalogRefreshPending = false;
        m_advancedDiscoveryLoading = false;
        m_basicAppsLoading = false;
        m_advancedSourceContexts.clear();
        m_basicApps.clear();
        applyAdvancedContextFilter();
        updateBasicPage();
        return;
    }

    const QString previousRequested = QDir::cleanPath(m_workflowCatalogRequestedWorkdir);
    const bool projectChanged = previousRequested != workdir;
    if (projectChanged) {
        m_remoteWidget->setResolvers({});
        m_remoteWidget->setWorkdir(workdir);
    }
    m_workflowCatalogRequestedWorkdir = workdir;
    m_advancedDiscoveryRequestedWorkdir = workdir;
    m_basicAppsRequestedWorkdir = workdir;

    if (m_workflowCatalogWatcher && m_workflowCatalogWatcher->isRunning()) {
        m_advancedDiscoveryLoading = true;
        m_basicAppsLoading = true;
        applyAdvancedContextFilter();
        updateBasicPage();
        if (QDir::cleanPath(m_workflowCatalogRunningWorkdir) != workdir)
            m_workflowCatalogRefreshPending = true;
        return;
    }

    m_workflowCatalogRefreshPending = false;
    m_advancedDiscoveryLoading = true;
    m_basicAppsLoading = true;
    m_workflowCatalogRunningWorkdir = workdir;
    m_advancedDiscoveryRunningWorkdir = workdir;
    m_basicAppsRunningWorkdir = workdir;
    if (projectChanged) {
        m_advancedSourceContexts.clear();
        m_basicApps.clear();
    }
    if (projectChanged || m_advancedSourceContexts.isEmpty()) {
        const WorkflowCatalogData shellCatalog = fastWorkflowShellCatalog(workdir);
        if (!shellCatalog.workflows.isEmpty()) {
            m_advancedSourceContexts = contextsFromCatalog(workdir, QString(), shellCatalog);
            m_basicApps = appWorkflowsFromCatalog(shellCatalog);
        }
    }
    applyAdvancedContextFilter();
    updateBasicPage();

    if (m_workflowCatalogWatcher) {
        m_workflowCatalogWatcher->setFuture(QtConcurrent::run([workdir]() {
            AsyncWorkflowCatalogResult result;
            result.workdir = workdir;
            result.repoRoot = DockpipeChoices::findRepoRoot(workdir);
            result.catalog = WorkflowCatalog::discoverCatalog(workdir);
            return result;
        }));
    }
}

void MainWindow::applyWorkflowCatalogResult(const AsyncWorkflowCatalogResult &result)
{
    WorkflowCatalogData catalog = result.catalog;
    if (catalog.workflows.isEmpty()) {
        const WorkflowCatalogData fallback = fastWorkflowShellCatalog(result.workdir);
        if (!fallback.workflows.isEmpty())
            catalog.workflows = fallback.workflows;
    }
    m_remoteWidget->setResolvers(catalog.resolverDetails);
    m_advancedSourceContexts = contextsFromCatalog(result.workdir, result.repoRoot, catalog);
    m_basicApps = appWorkflowsFromCatalog(catalog);
    m_advancedDiscoveryLoading = false;
    m_basicAppsLoading = false;
    m_advancedDiscoveryRunningWorkdir.clear();
    m_basicAppsRunningWorkdir.clear();
    applyAdvancedContextFilter();
    updateBasicPage();
}

void MainWindow::onAdvancedSearchChanged(const QString &)
{
    if (m_advancedSearchTimer)
        m_advancedSearchTimer->start();
    else
        applyAdvancedContextFilter();
}

void MainWindow::onAdvancedSelectionChanged()
{
    refreshInlineConsole();
}

void MainWindow::onSessionOutput(const QString &contextId, const QString &text)
{
    if (contextId != m_consoleContextId)
        return;
    appendInlineConsole(text);
}

void MainWindow::onSessionPrompt(const QString &contextId, const QString &payload)
{
    const QJsonDocument doc = QJsonDocument::fromJson(payload.toUtf8());
    if (!doc.isObject()) {
        QMessageBox::warning(this, tr("Dockpipe Launcher"), tr("Received an invalid Dockpipe prompt payload."));
        m_sessions.sendInput(contextId, QString());
        return;
    }

    const QJsonObject obj = doc.object();
    const QString type = obj.value(QStringLiteral("type")).toString();
    const QString title = obj.value(QStringLiteral("title")).toString(tr("Dockpipe Prompt"));
    const QString message = obj.value(QStringLiteral("message")).toString();
    const QString defaultValue = obj.value(QStringLiteral("default")).toString();
    const QString intent = obj.value(QStringLiteral("intent")).toString();
    const QString automationGroup = obj.value(QStringLiteral("automation_group")).toString();
    const QString pathMode = obj.value(QStringLiteral("path_mode")).toString(QStringLiteral("open-file"));
    const QString fileFilter = obj.value(QStringLiteral("file_filter")).toString();
    const QString baseDir = obj.value(QStringLiteral("base_dir")).toString();
    const QString resourceMode = obj.value(QStringLiteral("resource_mode")).toString(QStringLiteral("select"));
    const QString resourceSelection = obj.value(QStringLiteral("resource_selection")).toString(QStringLiteral("single"));
    const QString resourceKind = obj.value(QStringLiteral("resource_kind")).toString(QStringLiteral("file"));
    const bool sensitive = obj.value(QStringLiteral("sensitive")).toBool(false);
    const bool mustExist = obj.value(QStringLiteral("must_exist")).toBool(false);

    QStringList items;
    const QJsonArray options = obj.value(QStringLiteral("options")).toArray();
    for (const QJsonValue &value : options) {
        const QString option = value.toString();
        if (!option.isEmpty())
            items.append(option);
    }
    QStringList filters;
    const QJsonArray filterValues = obj.value(QStringLiteral("filters")).toArray();
    for (const QJsonValue &value : filterValues) {
        const QString filter = value.toString();
        if (!filter.isEmpty())
            filters.append(filter);
    }
    if (filters.isEmpty() && !fileFilter.isEmpty())
        filters = fileFilter.split(QStringLiteral(";;"), Qt::SkipEmptyParts);

    QString response = defaultValue;
    if (type == QStringLiteral("confirm") || type == QStringLiteral("input") ||
        type == QStringLiteral("choice") || type == QStringLiteral("file") || type == QStringLiteral("resource")) {
        PromptDialog dialog({type, title, message, defaultValue, intent, automationGroup, pathMode, fileFilter, baseDir,
                             resourceMode, resourceSelection, resourceKind, filters, items, sensitive, mustExist},
                            this);
        dialog.exec();
        response = dialog.response();
    } else {
        QMessageBox::information(this, tr("Dockpipe Launcher"),
                                 tr("Unsupported prompt type from Dockpipe: %1").arg(type));
    }

    m_sessions.sendInput(contextId, response);
}

QListWidgetItem *MainWindow::currentItem()
{
    return m_list->currentItem();
}

Context *MainWindow::findContext(const QString &workdir, const QString &workflow, const QString &workflowFile)
{
    const QString wd = QDir::cleanPath(workdir);
    for (Context &c : m_store.contexts) {
        if (QDir::cleanPath(c.workdir) != wd)
            continue;
        if (c.workflow != workflow)
            continue;
        if (c.workflowFile != workflowFile)
            continue;
        return &c;
    }
    return nullptr;
}

Context *MainWindow::findStoredContextForDisplay(const Context &display)
{
    return findContext(display.workdir, display.workflow, display.workflowFile);
}

Context *MainWindow::ensureStoredContextForDisplay(const Context &display)
{
    if (Context *existing = findStoredContextForDisplay(display))
        return existing;

    Context stored = display;
    if (stored.id.trimmed().isEmpty())
        stored.id = Context::createNew().id;
    if (stored.dockpipeBinary.trimmed().isEmpty())
        stored.dockpipeBinary = DockpipeChoices::preferredDockpipeBinary(stored.workdir);
    m_store.contexts.append(stored);
    m_store.save();
    return &m_store.contexts.last();
}

Context *MainWindow::currentAdvancedDisplayContext()
{
    QListWidgetItem *it = currentItem();
    if (!it)
        return nullptr;
    bool ok = false;
    const int index = it->data(Qt::UserRole).toInt(&ok);
    if (!ok || index < 0 || index >= m_advancedContexts.size())
        return nullptr;
    return &m_advancedContexts[index];
}

Context *MainWindow::currentContext()
{
    Context *display = currentAdvancedDisplayContext();
    if (!display)
        return nullptr;
    return findStoredContextForDisplay(*display);
}

bool MainWindow::hasContext(const Context &c) const
{
    const QString wd = QDir::cleanPath(c.workdir);
    for (const Context &ex : m_store.contexts) {
        if (QDir::cleanPath(ex.workdir) != wd)
            continue;
        if (ex.workflow == c.workflow && ex.workflowFile == c.workflowFile)
            return true;
    }
    return false;
}

void MainWindow::onLaunch()
{
    Context *display = currentAdvancedDisplayContext();
    if (!display)
        return;
    Context *c = ensureStoredContextForDisplay(*display);
    const WorkflowMeta meta = findWorkflowMeta(c->workdir, c->workflow, c->workflowFile);
    const QString node = m_runTarget->currentData().toString();
    if (!node.isEmpty()) {
        Context remoteContext = *c;
        if (remoteContext.workflowFile.isEmpty())
            remoteContext.workflowFile = meta.configPath;
        if (remoteContext.workflowFile.isEmpty()) {
            QMessageBox::information(this, tr("Workflow source required"), tr("Remote delivery requires a discovered workflow source file."));
            return;
        }
        if (!c->envFile.isEmpty() || !c->extraDockpipeEnv.isEmpty() || !c->resolver.isEmpty() || !c->runtime.isEmpty() || !c->strategy.isEmpty()) {
            QMessageBox::information(this, tr("Local launch overrides"),
                tr("This launch has local environment or execution overrides. Remote delivery uses the workflow YAML. "
                   "Clear local overrides and put non-secret execution settings in the workflow before sending it."));
            return;
        }
        RemoteRunDialog dialog(remoteContext, node, this);
        if (dialog.exec() == QDialog::Accepted) {
            m_navigation->setCurrentRow(3);
            m_activityWidget->refreshRemote();
        }
        return;
    }
    if (!configureContextForWorkflow(*c, meta, false))
        return;
    if (m_sessions.launch(*c, ContextStore::logsDir()))
        refreshSessionUi();
    else if (!m_sessions.isRunning(c->id))
        QMessageBox::warning(this, tr("Dockpipe Launcher"), tr("Could not start dockpipe (see stderr)."));
}

void MainWindow::onRelaunch()
{
    Context *display = currentAdvancedDisplayContext();
    if (!display)
        return;
    Context *c = ensureStoredContextForDisplay(*display);
    if (m_sessions.isRunning(c->id))
        m_sessions.stop(c->id);
    QTimer::singleShot(400, this, [this]() { onLaunch(); });
}

void MainWindow::onStop()
{
    Context *c = currentContext();
    if (!c)
        return;
    m_sessions.stop(c->id);
    refreshSessionUi();
}

void MainWindow::onStopAllForRepo()
{
    QString path = m_settings.projectFolder;
    if (path.isEmpty()) {
        if (Context *display = currentAdvancedDisplayContext())
            path = display->workdir;
    }
    if (path.isEmpty())
        return;
    const QString root = GitHelper::repoRoot(path);
    if (root.isEmpty()) {
        if (Context *c = currentContext())
            m_sessions.stop(c->id);
        refreshSessionUi();
        return;
    }
    for (const Context &x : m_store.contexts) {
        const QString xr = GitHelper::repoRoot(x.workdir);
        if (xr == root && m_sessions.isRunning(x.id))
            m_sessions.stop(x.id);
    }
    refreshSessionUi();
}

void MainWindow::onOpenLogs()
{
    Context *c = currentContext();
    if (!c) {
        const QString wd = QDir::cleanPath(m_settings.projectFolder);
        for (int i = m_store.contexts.size() - 1; i >= 0; --i) {
            if (QDir::cleanPath(m_store.contexts[i].workdir) == wd) {
                c = &m_store.contexts[i];
                break;
            }
        }
    }
    if (!c) {
        QMessageBox::information(this, tr("Dockpipe Launcher"), tr("No logs yet for this project."));
        return;
    }
    const SessionInfo si = m_sessions.info(c->id);
    QString path = si.logPath;
    if (path.isEmpty()) {
        const QDir logsRoot(ContextStore::logsDir());
        const QStringList matches = logsRoot.entryList({c->id + QStringLiteral("-*.log")}, QDir::Files, QDir::Time);
        if (!matches.isEmpty())
            path = logsRoot.filePath(matches.first());
    }

    LogViewerDialog dialog(c->label.isEmpty() ? tr("Session logs") : tr("%1 logs").arg(c->label),
                           path,
                           currentContextCommandLine(),
                           m_sessions.isRunning(c->id),
                           this);
    dialog.exec();
}

void MainWindow::onOpenFolder()
{
    const QString path = m_settings.projectFolder.isEmpty()
                             ? (currentAdvancedDisplayContext() ? currentAdvancedDisplayContext()->workdir : QString())
                             : m_settings.projectFolder;
    if (path.isEmpty())
        return;
    QDesktopServices::openUrl(QUrl::fromLocalFile(path));
}

void MainWindow::applyContextMenu(QListWidgetItem *, const QPoint &globalPos)
{
    QMenu menu(this);
    menu.addAction(tr("Launch"), this, &MainWindow::onLaunch);
    menu.addAction(tr("Workflow settings…"), this, [this]() {
        Context *display = currentAdvancedDisplayContext();
        if (!display)
            return;
        Context *c = ensureStoredContextForDisplay(*display);
        const WorkflowMeta meta = findWorkflowMeta(c->workdir, c->workflow, c->workflowFile);
        configureContextForWorkflow(*c, meta, true);
    });
    menu.addAction(tr("Relaunch"), this, &MainWindow::onRelaunch);
    menu.addAction(tr("Stop"), this, &MainWindow::onStop);
    menu.addAction(tr("Stop all for repo"), this, &MainWindow::onStopAllForRepo);
    menu.addSeparator();
    menu.addAction(tr("Open logs"), this, &MainWindow::onOpenLogs);
    menu.addAction(tr("Open folder"), this, &MainWindow::onOpenFolder);
    menu.exec(globalPos);
}

void MainWindow::refreshInlineConsole()
{
    if (!m_console || !m_consoleTitle || !m_consoleMeta)
        return;

    Context *display = currentAdvancedDisplayContext();
    Context *c = currentContext();
    if (!display && !c) {
        m_consoleContextId.clear();
        m_consoleTitle->setText(tr("Run output"));
        m_consoleMeta->setText(tr("Select a workflow row, then launch it to see output here."));
        m_console->setPlainText(QString());
        return;
    }

    const Context *metaContext = c ? c : display;
    m_consoleContextId = c ? c->id : QString();
    m_consoleTitle->setText(metaContext->label.isEmpty() ? tr("Run output") : metaContext->label);
    m_consoleMeta->setText(currentContextCommandLine());

    const SessionInfo si = c ? m_sessions.info(c->id) : SessionInfo{};
    QString text;
    if (!si.logPath.isEmpty()) {
        QFile f(si.logPath);
        if (f.open(QIODevice::ReadOnly | QIODevice::Text))
            text = QString::fromLocal8Bit(f.readAll());
    }
    if (text.isEmpty()) {
        text = tr("# No output yet.\n# Launch this row to run dockpipe inline here.");
    }
    m_console->setPlainText(text);
    auto *bar = m_console->verticalScrollBar();
    if (bar)
        bar->setValue(bar->maximum());
}

void MainWindow::appendInlineConsole(const QString &text)
{
    if (!m_console || text.isEmpty())
        return;
    m_console->moveCursor(QTextCursor::End);
    m_console->insertPlainText(text);
    auto *bar = m_console->verticalScrollBar();
    if (bar)
        bar->setValue(bar->maximum());
}

QString MainWindow::currentContextCommandLine() const
{
    Context *display = const_cast<MainWindow *>(this)->currentAdvancedDisplayContext();
    if (!display)
        return tr("Select a workflow row, then launch it to see output here.");
    const Context *c = const_cast<MainWindow *>(this)->currentContext();
    if (!c)
        c = display;

    SessionInfo si = m_sessions.info(c->id);
    QString program = si.program;
    QStringList args = si.arguments;
    if (program.isEmpty()) {
        program = c->dockpipeBinary.trimmed();
        if (program.isEmpty())
            program = DockpipeChoices::preferredDockpipeBinary(c->workdir);
        args = SessionManager::dockpipeArguments(*c);
    }

    QStringList parts;
    parts.append(shellQuote(program));
    for (const QString &arg : args)
        parts.append(shellQuote(arg));
    return parts.join(QLatin1Char(' '));
}
