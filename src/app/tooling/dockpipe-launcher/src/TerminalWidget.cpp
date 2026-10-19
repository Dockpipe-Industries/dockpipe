#include "TerminalWidget.h"
#include "TerminalHub.h"
#include "TerminalBridge.h"
#include "TerminalPreferences.h"
#include "TerminalSettingsDialog.h"
#include "CanvasIcons.h"

#include <QComboBox>
#include <QAction>
#include <QApplication>
#include <QClipboard>
#include <QJsonArray>
#include <QJsonDocument>
#include <QPointer>
#include <QPainter>
#include <QShortcut>
#include <QHideEvent>
#include <QHBoxLayout>
#include <QLabel>
#include <QPushButton>
#include <QShowEvent>
#include <QSignalBlocker>
#include <QTimer>
#include <QVBoxLayout>
#include <QWebChannel>
#include <QWebEnginePage>
#include <QWebEngineProfile>
#include <QWebEngineSettings>
#include <QWebEngineUrlRequestInterceptor>
#include <QWebEngineUrlRequestInfo>
#include <QWebEngineView>

static void initializeTerminalResources()
{
    Q_INIT_RESOURCE(terminal);
}

namespace {
// The privileged channel is never exposed to arbitrary web content.
class TerminalRequests final : public QWebEngineUrlRequestInterceptor {
public:
    using QWebEngineUrlRequestInterceptor::QWebEngineUrlRequestInterceptor;
    void interceptRequest(QWebEngineUrlRequestInfo &info) override
    {
        const QUrl url = info.requestUrl();
        const bool allowed = url.scheme() == "qrc"
            && (url.path().startsWith("/terminal/") || url.path() == "/qtwebchannel/qwebchannel.js");
        info.block(!allowed);
    }
};

class TerminalPage final : public QWebEnginePage {
public:
    TerminalPage(QWebEngineProfile *profile, QObject *parent) : QWebEnginePage(profile, parent) {}
protected:
    bool acceptNavigationRequest(const QUrl &url, NavigationType, bool isMainFrame) override
    {
        return isMainFrame && url == QUrl("qrc:/terminal/index.html");
    }
    QWebEnginePage *createWindow(WebWindowType) override { return nullptr; }
};
}

TerminalWidget::TerminalWidget(TerminalHub &hub, const QString &workdir, QWidget *parent, Mode mode)
    : QWidget(parent), m_hub(hub), m_hubGuard(&hub), m_mode(mode), m_workdir(workdir)
{
    setObjectName("terminalWidget");
    setMinimumSize(260, 180);
    auto *layout = new QVBoxLayout(this);
    const bool dashboard = mode == Mode::Dashboard;
    const int margin = dashboard ? 0 : 8;
    layout->setContentsMargins(margin, margin, margin, margin);
    if (dashboard)
        layout->setSpacing(0);
    auto *toolbarWidget = new QWidget;
    toolbarWidget->setObjectName("terminalToolbar");
    auto *toolbar = new QHBoxLayout(toolbarWidget);
    toolbar->setContentsMargins(0, 0, 0, 0);
    m_sessions = new QComboBox;
    m_sessions->setObjectName("terminalSessions");
    m_sessions->setSizeAdjustPolicy(QComboBox::AdjustToMinimumContentsLengthWithIcon);
    m_sessions->setMinimumContentsLength(8);
    m_sessions->setAccessibleName(tr("Terminal session"));
    toolbar->addWidget(m_sessions, 1);
    auto *create = new QPushButton(tr("New terminal"));
    create->setObjectName("terminalNew");
    toolbar->addWidget(create);
    m_close = new QPushButton(tr("Close"));
    m_close->setObjectName("terminalClose");
    m_close->setToolTip(tr("Stop and close the selected shell for all views"));
    toolbar->addWidget(m_close);
    auto *settingsButton = new QPushButton;
    settingsButton->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Properties));
    settingsButton->setObjectName("terminalSettings");
    settingsButton->setAccessibleName(tr("Terminal settings"));
    settingsButton->setToolTip(tr("Terminal settings"));
    settingsButton->setFixedSize(32, 32);
    toolbar->addWidget(settingsButton);
    connect(settingsButton, &QPushButton::clicked, this, &TerminalWidget::showSettings);
    connect(TerminalPreferences::instance(), &TerminalPreferences::changed, this, &TerminalWidget::applyAppearance);
    layout->addWidget(toolbarWidget);
    toolbarWidget->setVisible(!dashboard);
    m_status = new QLabel(tr("Open a terminal or select an existing session."));
    m_status->setObjectName("terminalStatus");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    layout->addWidget(m_status);
    m_status->setVisible(!dashboard);
    m_transparencyHint = new QLabel(tr("Restart DockPipe to enable desktop transparency. Your desktop must support compositing."));
    m_transparencyHint->setObjectName("terminalTransparencyHint");
    m_transparencyHint->setWordWrap(true);
    m_transparencyHint->hide();
    layout->addWidget(m_transparencyHint);
    layout->addStretch(1);
    m_flushTimer = new QTimer(this);
    m_flushTimer->setSingleShot(true);
    m_flushTimer->setInterval(16);
    connect(m_flushTimer, &QTimer::timeout, this, &TerminalWidget::flushOutput);
    connect(create, &QPushButton::clicked, this, [this]() {
        const QString id = m_hub.createSession(m_workdir);
        if (id.isEmpty()) {
            showStatus(tr("Eight terminals are already open. Close one before opening another."));
            return;
        }
        selectSession(id);
    });
    connect(m_close, &QPushButton::clicked, this, [this]() { m_hub.closeSession(m_sessionId); });
    connect(m_sessions, &QComboBox::currentIndexChanged, this, [this]() {
        if (m_mode == Mode::Managed)
            selectSession(m_sessions->currentData().toString());
    });
    connect(&m_hub, &TerminalHub::sessionsChanged, this, &TerminalWidget::refreshSessions);
    connect(&m_hub, &TerminalHub::outputAvailable, this, [this](const QString &id) {
        if (id == m_sessionId)
            scheduleOutput();
    });
    if (dashboard) {
        setContextMenuPolicy(Qt::ActionsContextMenu);
        auto *settingsAction = new QAction(tr("Terminal settings…"), this);
        settingsAction->setObjectName("terminalSettingsAction");
        addAction(settingsAction);
        connect(settingsAction, &QAction::triggered, this, &TerminalWidget::showSettings);
        auto *open = new QAction(tr("Open full view"), this);
        open->setObjectName("terminalOpenFullViewAction");
        addAction(open);
        connect(open, &QAction::triggered, this, &TerminalWidget::openFullViewRequested);
        m_restart = new QAction(tr("Start terminal"), this);
        m_restart->setObjectName("terminalStartAction");
        addAction(m_restart);
        connect(m_restart, &QAction::triggered, this, &TerminalWidget::startDashboardSession);
    }
    refreshSessions();
    applyAppearance();
}

TerminalWidget::~TerminalWidget()
{
    m_flushTimer->stop();
    if (m_hubGuard) {
        disconnect(m_hubGuard, nullptr, this, nullptr);
        if (!m_ownedSessionId.isEmpty())
            m_hubGuard->closeSession(m_ownedSessionId);
    }
}

void TerminalWidget::startDashboardSession()
{
    if (m_mode != Mode::Dashboard || !m_hubGuard || m_hub.running(m_ownedSessionId))
        return;
    m_dashboardStarted = true;
    if (!m_ownedSessionId.isEmpty())
        m_hub.closeSession(m_ownedSessionId);
    m_ownedSessionId = m_hub.createSession(m_workdir);
    if (m_ownedSessionId.isEmpty()) {
        showStatus(tr("Eight terminals are already open. Close one, then choose Start terminal from the context menu."));
        return;
    }
    selectSession(m_ownedSessionId);
}

void TerminalWidget::showSettings()
{
    TerminalSettingsDialog dialog(window());
    dialog.exec();
}

void TerminalWidget::showStatus(const QString &message, bool issue)
{
    m_status->setText(message);
    m_status->setVisible(m_mode == Mode::Managed || issue);
}

void TerminalWidget::setWorkdir(const QString &workdir) { m_workdir = workdir; }

void TerminalWidget::refreshSessions()
{
    QSignalBlocker blocker(m_sessions);
    m_sessions->clear();
    m_sessions->addItem(tr("Choose a terminal…"), QString());
    for (const QString &id : m_hub.sessionIds()) {
        QString label = m_hub.title(id);
        if (!m_hub.running(id))
            label += tr(" (stopped)");
        m_sessions->addItem(label, id);
    }
    if (!m_sessionId.isEmpty() && !m_hub.sessionIds().contains(m_sessionId))
        selectSession({});
    m_sessions->setCurrentIndex(qMax(0, m_sessions->findData(m_sessionId)));
    m_close->setEnabled(!m_sessionId.isEmpty());
    if (m_restart)
        m_restart->setEnabled(!m_hub.running(m_ownedSessionId));
    if (!m_sessionId.isEmpty()) {
        const QString error = m_hub.error(m_sessionId);
        QString message = error;
        if (message.isEmpty()) {
            if (m_hub.running(m_sessionId)) {
                message = tr("Interactive shell · Ctrl+Shift+C/V to copy/paste");
            } else if (m_mode == Mode::Dashboard) {
                message = tr("Shell exited. Choose Start terminal from the context menu to open another shell.");
            } else {
                message = tr("Shell exited. Open a new terminal to continue.");
            }
        }
        showStatus(message, !error.isEmpty() || !m_hub.running(m_sessionId));
    }
}

void TerminalWidget::selectSession(const QString &id)
{
    if (m_sessionId == id)
        return;
    m_sessionId = id;
    m_cursor = 0;
    ++m_generation;
    m_inFlight = false;
    m_reset = true;
    if (!id.isEmpty())
        ensureRenderer();
    if (m_view)
        m_view->setVisible(!id.isEmpty());
    if (id.isEmpty())
        showStatus(m_mode == Mode::Dashboard
            ? tr("Terminal closed. Choose Start terminal from the context menu to open another shell.")
            : tr("Open a terminal or select an existing session."));
    refreshSessions();
    scheduleOutput();
}

void TerminalWidget::ensureRenderer()
{
    if (m_view)
        return;
    initializeTerminalResources();
    m_view = new QWebEngineView(this);
    m_view->setObjectName("terminalDisplay");
    m_view->setContextMenuPolicy(Qt::NoContextMenu);
    if (m_mode == Mode::Dashboard) {
        m_view->setContextMenuPolicy(Qt::ActionsContextMenu);
        m_view->addActions(actions());
    }
    auto *profile = new QWebEngineProfile(this);
    auto *requests = new TerminalRequests(profile);
    profile->setUrlRequestInterceptor(requests);
    auto *page = new TerminalPage(profile, m_view);
    m_view->setPage(page);
    applyAppearance();
    auto *settings = page->settings();
    settings->setAttribute(QWebEngineSettings::LocalContentCanAccessFileUrls, false);
    settings->setAttribute(QWebEngineSettings::LocalContentCanAccessRemoteUrls, false);
    settings->setAttribute(QWebEngineSettings::JavascriptCanAccessClipboard, false);
    settings->setAttribute(QWebEngineSettings::JavascriptCanOpenWindows, false);
    auto *channel = new QWebChannel(page);
    auto *bridge = new TerminalBridge(channel);
    connect(bridge, &TerminalBridge::ready, this, &TerminalWidget::rendererReady);
    connect(bridge, &TerminalBridge::input, this, &TerminalWidget::terminalInput);
    connect(bridge, &TerminalBridge::sizeChanged, this, &TerminalWidget::terminalSize);
    connect(bridge, &TerminalBridge::acknowledged, this, &TerminalWidget::outputRendered);
    connect(this, &TerminalWidget::renderOutput, bridge, &TerminalBridge::renderOutput);
    connect(this, &TerminalWidget::appearanceChanged, bridge, &TerminalBridge::appearanceChanged);
    channel->registerObject("terminalBridge", bridge);
    page->setWebChannel(channel);
    auto *column = qobject_cast<QVBoxLayout *>(layout());
    delete column->takeAt(column->count() - 1);
    column->addWidget(m_view, 1);
    connect(m_view, &QWebEngineView::renderProcessTerminated, this, [this]() {
        m_ready = false;
        m_inFlight = false;
        showStatus(tr("Terminal display stopped. The shell is still available from another view."));
    });
    auto *copy = new QShortcut(QKeySequence(Qt::CTRL | Qt::SHIFT | Qt::Key_C), m_view);
    copy->setContext(Qt::WidgetWithChildrenShortcut);
    connect(copy, &QShortcut::activated, this, [this]() {
        QPointer<TerminalWidget> guard(this);
        m_view->page()->runJavaScript("window.dockpipeTerminal.getSelection()", [guard](const QVariant &text) {
            if (guard && !text.toString().isEmpty())
                QApplication::clipboard()->setText(text.toString());
        });
    });
    auto *paste = new QShortcut(QKeySequence(Qt::CTRL | Qt::SHIFT | Qt::Key_V), m_view);
    paste->setContext(Qt::WidgetWithChildrenShortcut);
    connect(paste, &QShortcut::activated, this, [this]() {
        const QString text = QApplication::clipboard()->text();
        if (text.toUtf8().size() > 64 * 1024) {
            showStatus(tr("Paste is limited to 64 KiB."));
            return;
        }
        const QByteArray argument = QJsonDocument(QJsonArray{text}).toJson(QJsonDocument::Compact);
        m_view->page()->runJavaScript("window.dockpipeTerminal.paste(" + QString::fromUtf8(argument) + "[0])");
    });
    m_view->load(QUrl("qrc:/terminal/index.html"));
}

void TerminalWidget::rendererReady()
{
    m_ready = true;
    setProperty("terminalRendererReady", true);
    m_inFlight = false;
    m_reset = true;
    applyAppearance();
    scheduleOutput();
}

void TerminalWidget::terminalInput(const QString &text, int generation)
{
    if (generation != m_generation || m_sessionId.isEmpty())
        return;
    if (text.toUtf8().size() > 64 * 1024 || !m_hub.sendBytes(m_sessionId, text.toUtf8()))
        showStatus(tr("Input was not accepted. Check that the selected shell is running."));
}

void TerminalWidget::terminalSize(int columns, int rows, int generation)
{
    if (isVisible() && generation == m_generation)
        m_hub.resize(m_sessionId, qBound(2, columns, 500), qBound(2, rows, 200));
}

void TerminalWidget::outputRendered(int generation)
{
    if (generation != m_generation)
        return;
    m_inFlight = false;
    scheduleOutput();
}

void TerminalWidget::scheduleOutput()
{
    if (isVisible() && m_ready && !m_inFlight && !m_sessionId.isEmpty() && !m_flushTimer->isActive())
        m_flushTimer->start();
}

void TerminalWidget::flushOutput()
{
    if (!isVisible() || !m_ready || m_inFlight || m_sessionId.isEmpty())
        return;
    bool truncated = false;
    const QByteArray bytes = m_hub.readOutput(m_sessionId, m_cursor, truncated);
    if (bytes.isEmpty() && !m_reset)
        return;
    if (truncated)
        showStatus(tr("Output exceeded the 256 KiB replay window; older output was omitted."));
    m_inFlight = true;
    emit renderOutput(QString::fromLatin1(bytes.toBase64()), m_reset || truncated, m_generation);
    m_reset = false;
}

void TerminalWidget::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    if (m_mode == Mode::Dashboard && !m_dashboardStarted)
        startDashboardSession();
    applyAppearance();
    scheduleOutput();
}

void TerminalWidget::hideEvent(QHideEvent *event)
{
    m_flushTimer->stop();
    QWidget::hideEvent(event);
}

void TerminalWidget::applyAppearance()
{
    const TerminalSettings settings = TerminalPreferences::instance()->settings();
    const int opacity = TerminalPreferences::renderingOpacity(settings);
    m_desktopBackground = settings.backgroundTarget == "desktop" && opacity < 100;
    setProperty("canvasPanelTransparent", settings.backgroundTarget == "canvas" && opacity < 100);
    const bool needsCompositorRestart = opacity != settings.opacity;
    m_transparencyHint->setText(needsCompositorRestart
        ? tr("Restart DockPipe to enable terminal transparency with compatible browser rendering.")
        : tr("Restart DockPipe to enable desktop transparency. Your desktop must support compositing."));
    m_transparencyHint->setVisible(needsCompositorRestart
        || (m_desktopBackground && !window()->testAttribute(Qt::WA_TranslucentBackground)));
    update();
    const QJsonObject theme = TerminalPreferences::theme(settings.theme);
    if (m_view) {
        // Qt 6.2 chooses the WebEngine alpha surface when the renderer starts.
        // Keep it available throughout a software-rendered session; the terminal
        // DOM still paints an opaque background when the requested opacity is 100%.
        const bool softwareRendering = qEnvironmentVariable("QT_QUICK_BACKEND") == "software";
        m_view->page()->setBackgroundColor(softwareRendering
            ? QColor(Qt::transparent) : QColor(theme.value("background").toString()));
    }
    if (!m_ready)
        return;
    QJsonObject appearance;
    appearance.insert("theme", theme);
    appearance.insert("fontFamily", settings.fontFamily);
    appearance.insert("fontSize", settings.fontSize);
    appearance.insert("cursorStyle", settings.cursorStyle);
    appearance.insert("cursorBlink", settings.cursorBlink);
    appearance.insert("scrollback", settings.scrollback);
    appearance.insert("opacity", opacity);
    appearance.insert("scanlines", settings.scanlines);
    appearance.insert("glow", settings.glow);
    appearance.insert("glowStrength", settings.glowStrength);
    appearance.insert("glowSpread", settings.glowSpread);
    emit appearanceChanged(appearance);
}

void TerminalWidget::paintEvent(QPaintEvent *event)
{
    QWidget::paintEvent(event);
    if (!m_desktopBackground || !m_view || !m_view->isVisible()
        || !window()->testAttribute(Qt::WA_TranslucentBackground))
        return;
    // Clear only the terminal viewport in the shared backing store. The browser
    // paints its translucent background and fully opaque text over this region.
    // Source composition removes ancestor panel fills without fading controls.
    QPainter painter(this);
    painter.setCompositionMode(QPainter::CompositionMode_Source);
    painter.fillRect(m_view->geometry(), Qt::transparent);
}
