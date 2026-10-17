#include "FlightAppBillboards.h"
#include "WorkspaceFlightView.h"
#include "CanvasIcons.h"
#include "CockpitHudPanel.h"
#include <QSettings>
#include <QDebug>
#include <QDialog>
#include <QDialogButtonBox>
#include <QEvent>
#include <QLabel>
#include <QListWidget>
#include <QMessageBox>
#include <QPushButton>
#include <QVBoxLayout>

QVector<DesktopAppChoice> FlightAppBillboards::chooseWindows(QWidget *parent)
{
    QString error;
    const auto available = DesktopAppWindow::availableWindows(error);
    if (!error.isEmpty()) {
        QMessageBox::information(parent, tr("App billboards · Experimental"), error);
        return {};
    }
    QDialog dialog(parent);
    dialog.setWindowTitle(tr("App billboards · Experimental"));
    dialog.resize(580, 440);
    auto *layout = new QVBoxLayout(&dialog);
    auto *explanation = new QLabel(tr("Choose up to three open windows to stream into this flight. Shoot a billboard to bring its live app into a front-facing control panel inside Dockpipe. Mouse and keyboard go to that app while connected.\n\nPreviews stay on this machine. Return to flight restores the app to the desktop. Some apps may not support embedding."));
    explanation->setWordWrap(true);
    layout->addWidget(explanation);
    auto *list = new QListWidget;
    list->setSelectionMode(QAbstractItemView::NoSelection);
    for (const auto &choice : available) {
        auto *item = new QListWidgetItem(CanvasIcons::icon(CanvasIcons::Symbol::Code), choice.title, list);
        item->setFlags(item->flags() | Qt::ItemIsUserCheckable);
        item->setCheckState(Qt::Unchecked);
    }
    layout->addWidget(list, 1);
    auto *hint = new QLabel(tr("0 of 3 selected · Reduced-rate previews · No selection is remembered"));
    layout->addWidget(hint);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Ok | QDialogButtonBox::Cancel);
    buttons->button(QDialogButtonBox::Ok)->setText(tr("Enter with selected apps"));
    buttons->button(QDialogButtonBox::Ok)->setEnabled(false);
    layout->addWidget(buttons);
    connect(list, &QListWidget::itemChanged, &dialog, [list, hint, buttons]() {
        int count = 0;
        for (int index = 0; index < list->count(); ++index)
            count += list->item(index)->checkState() == Qt::Checked;
        hint->setText(tr("%1 of 3 selected · Reduced-rate previews · No selection is remembered").arg(count));
        buttons->button(QDialogButtonBox::Ok)->setEnabled(count > 0 && count <= 3);
    });
    connect(buttons, &QDialogButtonBox::accepted, &dialog, &QDialog::accept);
    connect(buttons, &QDialogButtonBox::rejected, &dialog, &QDialog::reject);
    if (dialog.exec() != QDialog::Accepted)
        return {};
    QVector<DesktopAppChoice> chosen;
    for (int index = 0; index < list->count(); ++index) {
        if (list->item(index)->checkState() == Qt::Checked)
            chosen.append(available[index]);
    }
    return chosen;
}

FlightAppBillboards::FlightAppBillboards(WorkspaceFlightView *flight, const QVector<DesktopAppChoice> &choices)
    : QObject(flight), m_flight(flight)
{
    if (choices.size() > 3)
        return;
    for (const auto &choice : choices) {
        m_windows.push_back(std::make_unique<DesktopAppWindow>(choice));
        m_frames.append({choice.title, {}, tr("Connecting…")});
        m_failures.append(0);
    }
    flight->installEventFilter(this);
    // Two captures per second TOTAL, round-robin. At most 10.6 MiB of retained
    // preview pixels, no background work or queued frames while flight is hidden.
    m_timer.setInterval(500);
    connect(&m_timer, &QTimer::timeout, this, [this]() { refresh(); });
    m_connectTimer.setInterval(25);
    connect(&m_connectTimer, &QTimer::timeout, this, [this]() { finishControl(); });
    if (!m_windows.empty())
        m_timer.start();
}

FlightAppBillboards::~FlightAppBillboards()
{
    stopSharing(false);
}

void FlightAppBillboards::refresh()
{
    if (!m_flight->isVisible())
        return;
    if (m_active >= 0) {
        if (m_connectTimer.isActive())
            return;
        if (!m_windows[m_active]->alive())
            failControl(tr("The app window closed. Flight has resumed."));
        else if (!m_windows[m_active]->syncControlGeometry())
            failControl(tr("The app left the control panel and was restored to the desktop. Preview remains available."));
        return;
    }
    if (m_flight->editorActive() || m_windows.empty())
        return;
    int index = -1;
    for (int attempt = 0; attempt < int(m_windows.size()); ++attempt) {
        const int candidate = m_next;
        m_next = (m_next + 1) % int(m_windows.size());
        if (!m_visibilityFiltered || m_visibleStreams.contains(candidate)) {
            index = candidate;
            break;
        }
    }
    if (index < 0)
        return;
    if (m_failures[index] >= 3)
        return;
    auto &frame = m_frames[index];
    QElapsedTimer captureTimer;
    captureTimer.start();
    frame.image = m_windows[index]->capture(frame.status);
    m_lastCaptureNanoseconds = captureTimer.nsecsElapsed();
    ++m_captureCount;
    if (frame.image.isNull()) {
        ++m_failures[index];
        if (m_failures[index] == 3)
            frame.status += tr(" · Re-enter flight to reconnect.");
    } else {
        m_failures[index] = 0;
    }
    m_flight->viewport()->update();
}

void FlightAppBillboards::control(int index)
{
    if (index < 0 || index >= int(m_windows.size()) || m_active >= 0 || m_flight->editorActive())
        return;
    m_flight->setEditorActive(true);
    // Give the foreign app its own native HUD surface. Do not promote the flight
    // view and its ancestors to native windows or replace their GL viewport:
    // both invalidate composition when an accelerated app is detached again.
    m_restoreHardware = m_flight->hardwareAccelerationRequested();
    auto *panel = new CockpitHudPanel(tr("APP LINK / %1").arg(m_frames[index].title), m_flight->window());
    panel->setAttribute(Qt::WA_DontCreateNativeAncestors);
    panel->setAttribute(Qt::WA_NativeWindow);
    panel->setProperty("flightOverlay", true);
    panel->setObjectName("billboardControlPanel");
    panel->setArrangementEnabled(true);
    auto *layout = panel->body();
    auto *fullscreen = panel->fullscreenButton();
    fullscreen->setObjectName("billboardFullscreen");
    fullscreen->setFixedSize(150, 32);
    connect(panel, &CockpitHudPanel::fullscreenRequested, this, [this]() {
        m_flight->setFlightFullscreen(!m_flight->flightFullscreen());
        positionPanel();
    });
    auto *back = panel->closeButton();
    back->setObjectName("billboardReleaseControl");
    back->setText(tr("Return to flight"));
    back->setFixedSize(150, 32);
    connect(panel, &CockpitHudPanel::closeRequested, this, [this]() { releaseControl(); });
    QSettings settings;
    m_panelPlacement = settings.value("cockpit/appPanel", m_panelPlacement).toRectF();
    connect(panel, &CockpitHudPanel::placementChanged, this, [this, panel]() {
        if (!m_flight->flightFullscreen()) {
            m_panelPlacement = panel->placement();
            QSettings().setValue("cockpit/appPanel", m_panelPlacement);
        }
    });
    auto *hint = new QLabel(tr("Live app controls · Flight paused · Use Return to flight above to disconnect"));
    hint->setWordWrap(true);
    layout->addWidget(hint);
    m_panel = panel;
    panel->installEventFilter(this);
    m_active = index;
    m_timer.start();
    m_controlCreated = false;
    m_frames[index].controlError.clear();
    m_flight->setFlightStatus({});
    qInfo("App control: preparing native window handoff");
    positionPanel();
    panel->show();
    panel->raise();
    auto *connecting = new QLabel(tr("Connecting to the selected app…"));
    connecting->setObjectName("billboardConnecting");
    layout->addWidget(connecting, 1, Qt::AlignCenter);
    QString error;
    if (!m_windows[index]->prepareControl(error)) {
        failControl(error);
        return;
    }
    m_connectElapsed.start();
    m_connectTimer.start();
}

void FlightAppBillboards::finishControl()
{
    if (m_active < 0)
        return;
    const int index = m_active;
    if (!m_windows[index]->alive()) {
        failControl(tr("The app window closed during connection. Flight has resumed."));
        return;
    }
    if (m_controlCreated) {
        if (m_windows[index]->syncControlGeometry()) {
            m_connectTimer.stop();
            m_windows[index]->focusControl();
            qInfo("App control: native window attachment confirmed");
        } else if (m_connectElapsed.elapsed() >= 2000) {
            failControl(tr("The app could not stay inside the control panel. Its desktop window was restored; preview remains available."));
        }
        return;
    }
    if (!m_windows[index]->controlReady()) {
        if (m_connectElapsed.elapsed() >= 2000 || !m_windows[index]->alive()) {
            failControl(tr("The desktop did not release this app within 2 seconds. The window was restored."));
        }
        return;
    }
    QString error;
    QWidget *container = m_windows[index]->takeControl(m_panel, error);
    if (!container) {
        failControl(error);
        return;
    }
    delete m_panel->findChild<QLabel *>("billboardConnecting");
    static_cast<CockpitHudPanel *>(m_panel)->body()->addWidget(container, 1);
    container->show();
    container->raise();
    container->setFocus();
    m_controlCreated = true;
}

void FlightAppBillboards::failControl(const QString &message)
{
    if (m_active < 0)
        return;
    m_frames[m_active].controlError = message;
    qWarning().noquote() << "App control:" << message;
    releaseControl();
    m_flight->setFlightStatus(message);
}

void FlightAppBillboards::releaseControl(bool resumeRenderer)
{
    m_connectTimer.stop();
    if (m_active < 0)
        return;
    m_windows[m_active]->releaseControl();
    m_failures[m_active] = 0;
    m_active = -1;
    if (m_visibilityFiltered && m_visibleStreams.isEmpty())
        m_timer.stop();
    m_controlCreated = false;
    delete m_panel;
    m_panel = nullptr;
    m_flight->setEditorActive(false);
    // Hiding/destruction must not create a new GL viewport, but the user's
    // renderer choice must survive for the next Show event.
    m_flight->setHardwareAcceleration(m_restoreHardware, !resumeRenderer);
    m_flight->viewport()->update();
    if (resumeRenderer && m_flight->isVisible())
        m_flight->window()->activateWindow();
    m_flight->setFocus();
}

void FlightAppBillboards::stopSharing(bool resumeRenderer)
{
    m_timer.stop();
    releaseControl(resumeRenderer);
    m_windows.clear();
    m_frames.clear();
    m_failures.clear();
    m_flight->viewport()->update();
}

void FlightAppBillboards::positionPanel()
{
    if (!m_panel)
        return;
    const bool fullscreen = m_flight->flightFullscreen();
    auto *panel = static_cast<CockpitHudPanel *>(m_panel);
    panel->setPlacementBounds(QRect(m_flight->mapTo(panel->parentWidget(), QPoint()), m_flight->size()));
    panel->place(fullscreen ? QRectF(0, 0, 1, 1) : m_panelPlacement);
    m_panel->findChild<QPushButton *>("billboardFullscreen")->setText(
        fullscreen ? tr("Exit fullscreen") : tr("Fullscreen"));
    if (m_controlCreated) {
        m_panel->layout()->activate();
        m_windows[m_active]->syncControlGeometry();
    }
}

bool FlightAppBillboards::eventFilter(QObject *object, QEvent *event)
{
    if (object == m_panel && m_controlCreated
        && (event->type() == QEvent::Move || event->type() == QEvent::Resize)) {
        m_panel->layout()->activate();
        m_windows[m_active]->syncControlGeometry();
    }
    if (object == m_flight) {
        if (event->type() == QEvent::Resize || event->type() == QEvent::Move)
            positionPanel();
        else if (event->type() == QEvent::Hide)
            stopSharing(false);
    }
    return QObject::eventFilter(object, event);
}

void FlightAppBillboards::setVisibleStreams(const QSet<int> &indices)
{
    m_visibilityFiltered = true;
    m_visibleStreams = indices;
    if (m_active < 0 && indices.isEmpty())
        m_timer.stop();
    else if (!m_windows.empty() && !m_timer.isActive())
        m_timer.start();
}
