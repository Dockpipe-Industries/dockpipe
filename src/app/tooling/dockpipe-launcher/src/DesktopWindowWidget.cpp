#include "DesktopWindowWidget.h"
#include "DesktopAppWindow.h"
#include "DesktopApplicationCatalog.h"

#include <QComboBox>
#include <QDialog>
#include <QDialogButtonBox>
#include <QElapsedTimer>
#include <QGuiApplication>
#include <QIcon>
#include <QHideEvent>
#include <QShowEvent>
#include <QLabel>
#include <QPointer>
#include <QAction>
#include <QTimer>
#include <QVBoxLayout>
#include <QFutureWatcher>
#include <QSet>
#include <QMap>
#include <QtConcurrent>

#ifdef DOCKPIPE_PORTAL_WINDOW_CAPTURE
#include "PortalWindowCapture.h"
#endif
#ifdef DOCKPIPE_MAC_WINDOW_CAPTURE
#include "MacWindowCapture.h"
#endif
#include <QPainter>

namespace {
QMap<QString, QPointer<DesktopWindowWidget>> processDisplayOwners;

struct ApplicationLaunchResult {
    bool success = false;
    QString error;
};

class CapturePreview final : public QWidget {
public:
    explicit CapturePreview(QWidget *parent) : QWidget(parent) {}
    void setFrame(const QImage &frame)
    {
        m_frame = frame;
        update();
    }
protected:
    void paintEvent(QPaintEvent *) override
    {
        QPainter painter(this);
        painter.fillRect(rect(), QColor("#101d28"));
        if (m_frame.isNull())
            return;
        const QSize size = m_frame.size().scaled(this->size(), Qt::KeepAspectRatio);
        const QRect target(QPoint((width() - size.width()) / 2, (height() - size.height()) / 2), size);
        painter.drawImage(target, m_frame);
    }
private:
    QImage m_frame;
};
}


struct DesktopWindowWidget::State {
    qint64 displayProcessId = 0;
    QString displayStartToken;
    QTimer *displayTimer = nullptr;
    QElapsedTimer displayDeadline;
    QAction *choose = nullptr;
    QIcon applicationIcon;
    QLabel *status = nullptr;
    QWidget *surface = nullptr;
    QVBoxLayout *surfaceLayout = nullptr;
    QTimer *nativeTimer = nullptr;
    QTimer *hiddenTimer = nullptr;
    QElapsedTimer nativeDeadline;
    QPointer<QDialog> picker;
    std::unique_ptr<DesktopAppWindow> native;
    QPointer<QWidget> nativeContainer;
    bool attached = false;
    bool closeRequested = false;
    bool releasing = false;
    QTimer *launchTimer = nullptr;
    QElapsedTimer launchDeadline;
    DesktopApplicationEntry launchingApplication;
    QSet<quint64> windowsBeforeLaunch;
    quint64 launchGeneration = 0;
    bool launchAccepted = false;
    bool launching = false;
#ifdef DOCKPIPE_PORTAL_WINDOW_CAPTURE
    PortalWindowCapture *capture = nullptr;
#endif
#ifdef DOCKPIPE_MAC_WINDOW_CAPTURE
    MacWindowCapture *macCapture = nullptr;
#endif
    CapturePreview *preview = nullptr;
};

DesktopWindowWidget::DesktopWindowWidget(QWidget *parent)
    : QWidget(parent), m_state(std::make_unique<State>())
{
    setObjectName("desktopWindowWidget");
    setProperty("dashboardExternalApp", true);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->setSpacing(0);
    m_state->choose = new QAction(tr("Choose window…"), this);
    m_state->choose->setObjectName("desktopWindowChoose");
    setContextMenuPolicy(Qt::ActionsContextMenu);
    addAction(m_state->choose);

    m_state->status = new QLabel(this);
    m_state->status->setObjectName("desktopWindowStatus");
    m_state->status->setWordWrap(true);
    m_state->status->setMargin(8);
    m_state->status->setTextFormat(Qt::PlainText);
    layout->addWidget(m_state->status);
    m_state->surface = new QWidget(this);
    m_state->surface->setMinimumSize(160, 100);
    m_state->surfaceLayout = new QVBoxLayout(m_state->surface);
    m_state->surfaceLayout->setContentsMargins(0, 0, 0, 0);
    layout->addWidget(m_state->surface, 1);

    m_state->nativeTimer = new QTimer(this);
    m_state->nativeTimer->setObjectName("desktopNativeMonitor");
    m_state->nativeTimer->setInterval(50);
    connect(m_state->nativeTimer, &QTimer::timeout, this, &DesktopWindowWidget::advanceNativeWindow);
    m_state->displayTimer = new QTimer(this);
    m_state->displayTimer->setInterval(100);
    connect(m_state->displayTimer, &QTimer::timeout, this, &DesktopWindowWidget::advanceProcessDisplay);
    m_state->launchTimer = new QTimer(this);
    m_state->launchTimer->setObjectName("desktopApplicationDiscovery");
    m_state->launchTimer->setInterval(50);
    connect(m_state->launchTimer, &QTimer::timeout, this, &DesktopWindowWidget::findLaunchedWindow);
    m_state->hiddenTimer = new QTimer(this);
    m_state->hiddenTimer->setSingleShot(true);
    m_state->hiddenTimer->setInterval(250);
    connect(m_state->hiddenTimer, &QTimer::timeout, this, &DesktopWindowWidget::updateCaptureVisibility);
    connect(m_state->choose, &QAction::triggered, this, &DesktopWindowWidget::chooseWindow);
    connect(qApp, &QCoreApplication::aboutToQuit, this, &DesktopWindowWidget::releaseWindow);
    releaseWindow();
}

DesktopWindowWidget::~DesktopWindowWidget()
{
    disconnect(this, &DesktopWindowWidget::windowChanged, nullptr, nullptr);
    disconnect(this, &DesktopWindowWidget::applicationIconChanged, nullptr, nullptr);
    // Foreign windows must be detached while the native QWidget ancestors still
    // exist. QWidget's child destruction happens after this destructor body.
    releaseWindow();
}

bool DesktopWindowWidget::requestClose(QString &error)
{
    if (!m_state->native) {
        error = tr("This shared surface does not support closing its OS window.");
        return false;
    }
    if (!m_state->native->requestClose(error))
        return false;
    m_state->closeRequested = true;
    m_state->nativeTimer->start();
    return true;
}

bool DesktopWindowWidget::hasInteractiveWindow() const
{
    return m_state->native && m_state->native->controlAttached();
}

void DesktopWindowWidget::setApplicationHint(const QString &name)
{
    if (!m_state->attached && !m_state->preview)
        m_state->status->setText(tr("%1 launched. Right-click to choose its window.").arg(name.left(180)));
}

void DesktopWindowWidget::launchApplication(const DesktopApplicationEntry &application)
{
    releaseWindow();
    const quint64 generation = m_state->launchGeneration;
    m_state->launchingApplication = application;
    m_state->launching = true;
    m_state->choose->setEnabled(false);
    setApplicationIcon(DesktopApplicationCatalog::applicationIcon(application));
    m_state->status->setText(tr("Opening %1 in Tiling…").arg(application.name));
    const bool canIdentifyWindow = automaticAcquisitionUnavailableReason().isEmpty()
        && application.hasDeclaredWindowClass;
    if (canIdentifyWindow) {
        QString error;
        const auto choices = availableWindows(error);
        if (!error.isEmpty()) {
            fail(tr("Could not inspect app windows before launch: %1 No app was launched.").arg(error));
            return;
        }
        QVector<DesktopAppChoice> existing;
        for (const auto &choice : choices) {
            m_state->windowsBeforeLaunch.insert(choice.id);
            if (choice.applicationClass.compare(application.windowClass, Qt::CaseInsensitive) == 0)
                existing.append(choice);
        }
        if (existing.size() == 1) {
            // Dock activation brings an existing app into this workspace. Do
            // not dispatch another launch and then race its new window against
            // the old one (particularly browsers with a shared process).
            if (!acquireWindow(existing.first().id, error))
                fail(error);
            else
                advanceNativeWindow();
            return;
        }
        if (existing.size() > 1) {
            m_state->launching = false;
            m_state->choose->setEnabled(true);
            m_state->status->setText(tr("Choose which %1 window to bring into Tiling.").arg(application.name));
            chooseWindow();
            return;
        }
        // Arm discovery before dispatch, including applications that delegate
        // their new window to an already running browser process.
        m_state->launchDeadline.start();
        m_state->launchTimer->start();
    }
    auto *launch = new QFutureWatcher<ApplicationLaunchResult>(this);
    connect(launch, &QFutureWatcher<ApplicationLaunchResult>::finished, this,
        [this, launch, generation, canIdentifyWindow]() {
        const auto result = launch->result();
        launch->deleteLater();
        if (generation != m_state->launchGeneration)
            return;
        if (!result.success) {
            fail(result.error);
            return;
        }
        m_state->launchAccepted = true;
        if (canIdentifyWindow) {
            findLaunchedWindow();
        } else {
            m_state->launching = false;
            m_state->choose->setEnabled(true);
            m_state->status->setText(tr("Choose the app's window to finish opening it in Tiling."));
            chooseWindow();
        }
    });
    launch->setFuture(QtConcurrent::run([application]() {
        ApplicationLaunchResult result;
        result.success = DesktopApplicationCatalog::launch(application, result.error);
        return result;
    }));
}

void DesktopWindowWidget::findLaunchedWindow()
{
    if (!m_state->launching)
        return;
    if (m_state->launchDeadline.elapsed() >= 15000) {
        fail(tr("Could not identify a single window for %1. The app may still be starting; try opening it from the dock again.")
            .arg(m_state->launchingApplication.name));
        return;
    }
    if (!m_state->launchAccepted)
        return;
    QString error;
    const auto choices = availableWindows(error);
    QVector<DesktopAppChoice> newWindows;
    for (const auto &choice : choices) {
        if (choice.applicationClass.compare(m_state->launchingApplication.windowClass, Qt::CaseInsensitive) != 0)
            continue;
        if (!m_state->windowsBeforeLaunch.contains(choice.id))
            newWindows.append(choice);
    }
    if (newWindows.size() != 1)
        return;
    const quint64 candidate = newWindows.first().id;
    const QIcon icon = m_state->applicationIcon;
    if (!acquireWindow(candidate, error)) {
        fail(tr("Could not bring the app into Tiling: %1").arg(error));
        return;
    }
    if (m_state->applicationIcon.isNull())
        setApplicationIcon(icon);
    // Some window managers acknowledge withdrawal immediately. Avoid adding a
    // full monitor interval before the first attachment attempt.
    advanceNativeWindow();
}

QString DesktopWindowWidget::automaticAcquisitionUnavailableReason()
{
    if (QGuiApplication::platformName() == "cocoa")
        return tr("macOS requires you to choose each shared window. Automatic window acquisition is unavailable; sharing is view-only.");
    if (QGuiApplication::platformName().startsWith("wayland"))
        return tr("Wayland requires you to choose each shared window through the desktop portal. Automatic window acquisition is unavailable; sharing is view-only.");
    return DesktopAppWindow::unavailableReason();
}

QVector<DesktopAppChoice> DesktopWindowWidget::availableWindows(QString &error)
{
    error = automaticAcquisitionUnavailableReason();
    if (!error.isEmpty())
        return {};
    return DesktopAppWindow::availableWindows(error);
}

bool DesktopWindowWidget::acquireWindow(quint64 nativeId, QString &error)
{
    return acquireWindow(nativeId, error, 0, {});
}

bool DesktopWindowWidget::acquireWindow(quint64 nativeId, QString &error, qint64 expectedPid, const QString &startToken)
{
    error.clear();
    if (!nativeId) {
        error = tr("Choose a valid desktop window.");
        return false;
    }
    if (acquiredWindowId() == nativeId && m_state->native->alive()
        && (!expectedPid || m_state->native->matchesProcess(expectedPid, startToken)))
        return true;
    error = automaticAcquisitionUnavailableReason();
    if (!error.isEmpty())
        return false;
    // Recheck this live selection, including all native eligibility checks,
    // without repeating a whole-desktop process scan for every incoming tile.
    const auto choices = DesktopAppWindow::availableWindows(error, nativeId);
    if (!error.isEmpty())
        return false;
    for (const auto &choice : choices) {
        if (choice.id != nativeId)
            continue;
        releaseWindow();
        m_state->native = std::make_unique<DesktopAppWindow>(choice);
        if (expectedPid && !m_state->native->matchesProcess(expectedPid, startToken)) {
            error = tr("The display process identity changed. Refresh its package status before attaching.");
            releaseWindow();
            return false;
        }
        if (!m_state->native->prepareControl(error)) {
            fail(error);
            return false;
        }
        const QIcon nativeIcon = m_state->native->applicationIcon();
        if (!nativeIcon.isNull())
            setApplicationIcon(nativeIcon);
        m_state->choose->setEnabled(false);
        m_state->status->show();
        m_state->status->setText(tr("Opening %1…").arg(choice.title));
        m_state->surface->show();
        m_state->nativeDeadline.start();
        m_state->nativeTimer->setInterval(50);
        m_state->nativeTimer->start();
        emit windowChanged();
        return true;
    }
    error = tr("This desktop window is no longer available or cannot be embedded.");
    return false;
}

void DesktopWindowWidget::setProcessDisplay(qint64 processId, const QString &startToken)
{
    if (m_state->displayProcessId == processId && m_state->displayStartToken == startToken)
        return;
    releaseWindow();
    m_state->displayProcessId = processId;
    m_state->displayStartToken = startToken;
    m_state->choose->setEnabled(processId <= 0);
    if (processId > 0 && isVisible()) {
        m_state->displayDeadline.restart();
        m_state->displayTimer->start();
        advanceProcessDisplay();
    }
}

void DesktopWindowWidget::advanceProcessDisplay()
{
    if (!isVisible() || m_state->displayProcessId <= 0) {
        m_state->displayTimer->stop();
        return;
    }
    if (m_state->native) {
        m_state->displayTimer->stop();
        return;
    }
    const QString key = QString::number(m_state->displayProcessId) + ':' + m_state->displayStartToken;
    const auto previous = processDisplayOwners.value(key);
    if (previous && previous != this)
        previous->releaseWindow();
    QString error;
    if (acquireProcessWindow(m_state->displayProcessId, m_state->displayStartToken, error)) {
        processDisplayOwners.insert(key, this);
        m_state->displayTimer->stop();
        return;
    }
    if (m_state->displayDeadline.elapsed() >= 5000) {
        m_state->displayTimer->stop();
        m_state->status->setText(error);
        emit windowUnavailable(error);
    } else {
        m_state->displayTimer->start();
        m_state->status->setText(tr("Waiting for the verified display window…"));
    }
}

bool DesktopWindowWidget::acquireProcessWindow(qint64 processId, const QString &startToken, QString &error)
{
#ifdef Q_OS_WIN
    Q_UNUSED(processId);
    Q_UNUSED(startToken);
    error = tr("Package display attachment requires public kernel process identities, which are unavailable on Windows hosts.");
    return false;
#elif !defined(DOCKPIPE_X11_RESOURCE_IDENTITIES)
    Q_UNUSED(processId);
    Q_UNUSED(startToken);
    error = tr("This build cannot verify native display ownership. Package displays require X11 XRes process identities.");
    return false;
#else
    const auto choices = availableWindows(error);
    if (!error.isEmpty())
        return false;
    const auto id = processWindowId(choices, processId, startToken, error);
    return id && acquireWindow(id, error, processId, startToken);
#endif
}

quint64 DesktopWindowWidget::processWindowId(const QVector<DesktopAppChoice> &choices,
                                           qint64 processId, const QString &startToken, QString &error)
{
    error.clear();
    QVector<DesktopAppChoice> matching;
    if (processId > 0 && !startToken.isEmpty()) {
        for (const auto &choice : choices) {
            if (choice.processId == processId && choice.processStartToken == startToken)
                matching.append(choice);
        }
    }
    if (matching.size() != 1) {
        error = matching.isEmpty()
            ? tr("No eligible window matches the live display process. Refresh its package status.")
            : tr("The display process has several eligible windows. A single display window is required.");
        return 0;
    }
    return matching.first().id;
}

quint64 DesktopWindowWidget::acquiredWindowId() const
{
    return m_state->native ? m_state->native->choice().id : 0;
}

QString DesktopWindowWidget::acquiredWindowTitle() const
{
    return m_state->native ? m_state->native->title() : QString();
}

QString DesktopWindowWidget::acquiredApplicationClass() const
{
    return m_state->native ? m_state->native->choice().applicationClass : QString();
}

QIcon DesktopWindowWidget::applicationIcon() const
{
    return m_state->applicationIcon;
}

void DesktopWindowWidget::setApplicationIcon(const QIcon &icon)
{
    if (m_state->applicationIcon.cacheKey() == icon.cacheKey())
        return;
    m_state->applicationIcon = icon;
    emit applicationIconChanged(icon);
}

void DesktopWindowWidget::chooseWindow()
{
    if (m_state->picker) {
        m_state->picker->raise();
        m_state->picker->activateWindow();
        return;
    }
    if (QGuiApplication::platformName() == "cocoa")
        startMacCapture();
    else if (QGuiApplication::platformName().startsWith("wayland"))
        startPortalCapture();
    else
        chooseNativeWindow();
}

void DesktopWindowWidget::chooseNativeWindow()
{
    QString error;
    const auto choices = DesktopAppWindow::availableWindows(error);
    if (!error.isEmpty() || choices.isEmpty()) {
        fail(error.isEmpty() ? tr("No available desktop windows.") : error);
        return;
    }

    auto *dialog = new QDialog(this);
    dialog->setObjectName("desktopWindowPicker");
    dialog->setWindowTitle(tr("Choose a desktop window"));
    dialog->setAttribute(Qt::WA_DeleteOnClose);
    auto *layout = new QVBoxLayout(dialog);
    auto *explanation = new QLabel(tr("The selected window will move into this dashboard. With app management enabled, closing its panel asks the app to close. Otherwise it returns to the desktop."), dialog);
    explanation->setWordWrap(true);
    layout->addWidget(explanation);
    auto *windows = new QComboBox(dialog);
    windows->setObjectName("desktopWindowChoices");
    for (const auto &choice : choices)
        windows->addItem(choice.title, QVariant::fromValue(choice.id));
    layout->addWidget(windows);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Ok | QDialogButtonBox::Cancel, dialog);
    connect(buttons, &QDialogButtonBox::accepted, dialog, &QDialog::accept);
    connect(buttons, &QDialogButtonBox::rejected, dialog, &QDialog::reject);
    layout->addWidget(buttons);
    m_state->picker = dialog;
    m_state->choose->setEnabled(false);
    connect(dialog, &QDialog::finished, this, [this, dialog](int result) {
        if (m_state->picker == dialog)
            m_state->picker = nullptr;
        if (!m_state->native && !m_state->preview) {
            m_state->choose->setEnabled(true);
            if (result == QDialog::Rejected && !m_state->releasing)
                emit windowUnavailable({});
        }
    });
    connect(dialog, &QDialog::accepted, this, [this, choices, windows] {
        const int index = windows->currentIndex();
        if (index < 0 || index >= choices.size()) {
            fail(tr("No desktop window was selected."));
            return;
        }
        const auto choice = choices[index];
        m_state->picker = nullptr;
        QString error;
        if (!acquireWindow(choice.id, error))
            fail(error);
    });
    dialog->resize(440, dialog->sizeHint().height());
    dialog->open();
}

void DesktopWindowWidget::advanceNativeWindow()
{
    if (!m_state->native)
        return;
    if (!m_state->native->alive()) {
        fail(m_state->closeRequested ? QString() : tr("The selected window closed."));
        return;
    }
    if (!isVisible()) {
        // Retain ownership while hidden, including an in-progress withdrawal.
        // Resume attachment only when there is a visible host for the window.
        m_state->nativeTimer->stop();
        return;
    }
    if (m_state->attached) {
        m_state->native->syncControlGeometry();
        return;
    }
    // Withdrawal/attachment is asynchronous, but must never hide an app forever.
    if (m_state->nativeDeadline.elapsed() > 3000) {
        fail(tr("The desktop could not attach this window. It has been released."));
        return;
    }
    if (!m_state->nativeContainer) {
        if (!m_state->native->controlReady())
            return;
        QString error;
        m_state->nativeContainer = m_state->native->takeControl(m_state->surface, error);
        if (!m_state->nativeContainer) {
            fail(error);
            return;
        }
        m_state->surfaceLayout->addWidget(m_state->nativeContainer);
        m_state->nativeContainer->show();
    }
    if (!m_state->native->controlAttached())
        return;
    m_state->attached = true;
    m_state->nativeTimer->setInterval(100);
    if (!isVisible())
        m_state->hiddenTimer->start();
    m_state->native->focusControl();
    m_state->status->hide();
    m_state->surface->show();
    emit windowChanged();
}

void DesktopWindowWidget::startPortalCapture()
{
#ifdef DOCKPIPE_PORTAL_WINDOW_CAPTURE
    releaseWindow();
    m_state->choose->setEnabled(false);
    m_state->preview = new CapturePreview(m_state->surface);
    m_state->surfaceLayout->addWidget(m_state->preview);
    m_state->surface->show();
    m_state->capture = new PortalWindowCapture(this);
    connect(m_state->capture, &PortalWindowCapture::frameReady,
        this, [this](const QImage &frame) {
            if (!m_state->preview)
                return;
            m_state->preview->setFrame(frame);
            m_state->status->hide();
            m_state->surface->setToolTip(tr("Window sharing is view-only. Close this panel to stop sharing."));
        });
    connect(m_state->capture, &PortalWindowCapture::statusChanged, m_state->status, &QLabel::setText);
    connect(m_state->capture, &PortalWindowCapture::failed, this, &DesktopWindowWidget::fail);
    connect(m_state->capture, &PortalWindowCapture::cancelled, this, [this] {
        releaseWindow();
        m_state->status->setText(tr("Window sharing cancelled."));
        emit windowUnavailable({});
    });
    m_state->capture->start();
    updateCaptureVisibility();
#else
    fail(tr("Wayland window sharing is unavailable in this build. It needs the desktop portal backend, GStreamer with PipeWire and JPEG plugins, and a window-sharing portal. Native Wayland windows cannot be embedded for input."));
#endif
}

void DesktopWindowWidget::startMacCapture()
{
#ifdef DOCKPIPE_MAC_WINDOW_CAPTURE
    releaseWindow();
    m_state->choose->setEnabled(false);
    m_state->preview = new CapturePreview(m_state->surface);
    m_state->surfaceLayout->addWidget(m_state->preview);
    m_state->surface->show();
    m_state->macCapture = new MacWindowCapture(this);
    connect(m_state->macCapture, &MacWindowCapture::frameReady,
        this, [this](const QImage &frame) {
            if (!m_state->preview)
                return;
            m_state->preview->setFrame(frame);
            m_state->status->hide();
            m_state->surface->setToolTip(tr("Window sharing is view-only. Close this panel to stop sharing."));
        });
    connect(m_state->macCapture, &MacWindowCapture::statusChanged, m_state->status, &QLabel::setText);
    connect(m_state->macCapture, &MacWindowCapture::failed, this, &DesktopWindowWidget::fail);
    connect(m_state->macCapture, &MacWindowCapture::cancelled, this, [this] {
        releaseWindow();
        m_state->status->setText(tr("Window sharing cancelled."));
        emit windowUnavailable({});
    });
    m_state->macCapture->start();
    updateCaptureVisibility();
#else
    fail(tr("macOS window sharing is unavailable in this build. It requires the macOS 14+ ScreenCaptureKit backend."));
#endif
}

void DesktopWindowWidget::releaseWindow()
{
    for (auto owner = processDisplayOwners.begin(); owner != processDisplayOwners.end();) {
        if (!owner.value() || owner.value() == this)
            owner = processDisplayOwners.erase(owner);
        else
            ++owner;
    }
    m_state->displayTimer->stop();
    ++m_state->launchGeneration;
    m_state->launchTimer->stop();
    m_state->launching = false;
    m_state->launchAccepted = false;
    m_state->windowsBeforeLaunch.clear();
    const bool hadWindow = m_state->native || m_state->preview;
    m_state->releasing = true;
    m_state->hiddenTimer->stop();
    m_state->nativeTimer->stop();
    if (m_state->picker)
        m_state->picker->reject();
    m_state->picker = nullptr;
    m_state->native.reset();
    m_state->nativeContainer = nullptr;
    m_state->attached = false;
    m_state->closeRequested = false;
#ifdef DOCKPIPE_PORTAL_WINDOW_CAPTURE
    if (m_state->capture) {
        m_state->capture->disconnect(this);
        m_state->capture->stop();
        m_state->capture->deleteLater();
        m_state->capture = nullptr;
    }
#endif
#ifdef DOCKPIPE_MAC_WINDOW_CAPTURE
    if (m_state->macCapture) {
        m_state->macCapture->disconnect(this);
        m_state->macCapture->stop();
        m_state->macCapture->deleteLater();
        m_state->macCapture = nullptr;
    }
#endif
    if (m_state->preview) {
        m_state->preview->hide();
        m_state->surfaceLayout->removeWidget(m_state->preview);
        m_state->preview->deleteLater();
        m_state->preview = nullptr;
    }
    m_state->choose->setEnabled(m_state->displayProcessId <= 0);
    m_state->status->show();
    m_state->surface->hide();
    if (QGuiApplication::platformName() == "cocoa")
        m_state->status->setText(tr("Right-click to choose a desktop window. macOS requires its sharing picker; sharing is view-only."));
    else if (QGuiApplication::platformName().startsWith("wayland"))
        m_state->status->setText(tr("Right-click to choose a desktop window. Wayland requires its sharing picker; sharing is view-only."));
    else
        m_state->status->setText(tr("Right-click to choose a desktop window."));
    m_state->releasing = false;
    if (hadWindow) {
        setApplicationIcon({});
        emit windowChanged();
    }
}

void DesktopWindowWidget::fail(const QString &message)
{
    releaseWindow();
    m_state->status->setText(message);
    emit windowUnavailable(message);
}

void DesktopWindowWidget::hideEvent(QHideEvent *event)
{
    QWidget::hideEvent(event);
    if (m_state->displayProcessId > 0) {
        releaseWindow();
        return;
    }
    // Mode changes briefly hide/reparent panels. Preserve their sessions without
    // repeatedly suspending and resuming the native capture backend.
    m_state->hiddenTimer->start();
}

void DesktopWindowWidget::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    if (m_state->displayProcessId > 0) {
        m_state->displayDeadline.restart();
        m_state->displayTimer->start();
        advanceProcessDisplay();
        return;
    }
    m_state->hiddenTimer->stop();
    updateCaptureVisibility();
}

void DesktopWindowWidget::updateCaptureVisibility()
{
    const bool visible = isVisible();
#ifdef DOCKPIPE_PORTAL_WINDOW_CAPTURE
    if (m_state->capture)
        m_state->capture->setPaused(!visible);
#endif
#ifdef DOCKPIPE_MAC_WINDOW_CAPTURE
    if (m_state->macCapture)
        m_state->macCapture->setVisible(visible);
#endif
    if (m_state->native) {
        if (visible) {
            if (!m_state->attached)
                m_state->nativeDeadline.restart();
            m_state->nativeTimer->start();
        } else {
            m_state->nativeTimer->stop();
        }
    }
}
