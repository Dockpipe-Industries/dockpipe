#include "NotificationBell.h"

#include "CanvasIcons.h"
#include "RemoteAccessMonitor.h"

#include <QAction>
#include <QDialog>
#include <QDialogButtonBox>
#include <QFrame>
#include <QFontMetrics>
#include <QHBoxLayout>
#include <QLabel>
#include <QMenu>
#include <QPushButton>
#include <QStyle>
#include <QTimer>
#include <QVBoxLayout>
#include <QWidgetAction>

namespace {
void addHeading(QMenu &menu, const QString &text)
{
    auto *label = new QLabel(text);
    label->setObjectName("notificationSection");
    auto *action = new QWidgetAction(&menu);
    action->setDefaultWidget(label);
    menu.addAction(action);
}

QString readableStatus(const RemoteAccessMonitor &monitor)
{
    const QString detail = monitor.detail();
    if (detail.size() <= 600)
        return detail;
    return detail.left(600) + QChar(0x2026);
}
}

NotificationBell::NotificationBell(RemoteAccessMonitor &monitor, QWidget *parent)
    : QToolButton(parent), m_monitor(monitor)
{
    setObjectName("notificationBell");
    setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Bell));
    setIconSize(QSize(18, 18));
    setToolButtonStyle(Qt::ToolButtonTextBesideIcon);
    setPopupMode(QToolButton::InstantPopup);
    setCursor(Qt::ArrowCursor);
    auto *notifications = new QMenu(this);
    notifications->setObjectName("applicationNotifications");
    setMenu(notifications);
    connect(notifications, &QMenu::aboutToShow, this, [this, notifications]() {
        notifications->clear();
        addHeading(*notifications, tr("REMOTE ACCESS"));
        auto *description = new QLabel(readableStatus(m_monitor));
        description->setObjectName("remoteNotificationDetail");
        m_notificationDetail = description;
        description->setTextFormat(Qt::PlainText);
        description->setWordWrap(true);
        description->setFixedWidth(300);
        auto *content = new QWidgetAction(notifications);
        content->setDefaultWidget(description);
        notifications->addAction(content);
        notifications->addSeparator();
        m_unlockAction = notifications->addAction(tr("Unlock remote access…"), this, &NotificationBell::showUnlockPrompt);
        m_unlockAction->setVisible(m_monitor.state() == RemoteAccessMonitor::State::Locked);
        m_unlockAction->setEnabled(!m_monitor.busy());
        notifications->addAction(tr("Security settings…"), this, &NotificationBell::securitySettingsRequested);
        notifications->addAction(tr("Check again"), &m_monitor, &RemoteAccessMonitor::refresh);
        addHeading(*notifications, tr("WORKFLOW ACTIVITY"));
        if (m_workflows.isEmpty()) {
            auto *empty = notifications->addAction(tr("No completed workflows in this app session"));
            empty->setEnabled(false);
        }
        const int shown = qMin(8, int(m_workflows.size()));
        for (int index = 0; index < shown; ++index) {
            const WorkflowNotice notice = m_workflows[index];
            QString outcome = tr("Completed");
            if (notice.outcome == WorkflowOutcome::Failed)
                outcome = tr("Failed");
            else if (notice.outcome == WorkflowOutcome::Stopped)
                outcome = tr("Stopped");
            const QString title = fontMetrics().elidedText(notice.title, Qt::ElideRight, 240).replace("&", "&&");
            auto *action = notifications->addAction(
                (notice.unread ? QString::fromUtf8("● ") : QString())
                + title + tr(" — %1 · %2").arg(outcome, notice.time.toLocalTime().toString("HH:mm")));
            action->setObjectName("workflowNotification");
            action->setToolTip(notice.title + " · " + outcome);
            connect(action, &QAction::triggered, this, [this, notice]() {
                for (auto &stored : m_workflows) {
                    if (stored.id == notice.id)
                        stored.unread = false;
                }
                updateState();
                if (notice.remoteJob.isEmpty())
                    emit workflowLogsRequested(notice.title, notice.logPath);
                else
                    emit remoteJobRequested(notice.remoteJob);
            });
        }
        notifications->addAction(tr("Workflow history…"), this, &NotificationBell::workflowHistoryRequested);
        if (unreadCount() > 0) {
            notifications->addAction(tr("Mark all as read"), this, [this]() {
                for (auto &notice : m_workflows)
                    notice.unread = false;
                updateState();
            });
        }
    });
    connect(&m_monitor, &RemoteAccessMonitor::changed, this, &NotificationBell::updateState);
    connect(&m_monitor, &RemoteAccessMonitor::unlockFinished, this, [this](bool unlocked) {
        m_unlockRequested = false;
        if (unlocked && m_prompt)
            m_prompt->accept();
        updateState();
    });
    updateState();
}

int NotificationBell::unreadCount() const
{
    int count = 0;
    for (const auto &notice : m_workflows) {
        if (notice.unread)
            ++count;
    }
    return count;
}

void NotificationBell::notifyRemoteWorkflow(const QString &title, const QString &jobId, WorkflowOutcome outcome)
{
    notifyWorkflow(title, QString(), outcome);
    m_workflows.first().remoteJob = jobId;
}

void NotificationBell::notifyWorkflow(const QString &title, const QString &logPath, WorkflowOutcome outcome)
{
    m_workflows.prepend({++m_nextId, title, logPath, outcome, QDateTime::currentDateTimeUtc(), true});
    // The bell is a bounded activity inbox; retained run history remains canonical.
    if (m_workflows.size() > 50)
        m_workflows.removeLast();
    updateState();
}

QWidget *NotificationBell::createMachinesNotice(QWidget *parent)
{
    auto *notice = new QFrame(parent);
    notice->setObjectName("remoteAccessNotice");
    auto *row = new QHBoxLayout(notice);
    row->setContentsMargins(20, 12, 20, 12);
    row->setSpacing(16);
    m_noticeText = new QLabel(notice);
    m_noticeText->setTextFormat(Qt::PlainText);
    m_noticeText->setWordWrap(true);
    row->addWidget(m_noticeText, 1);
    m_noticeAction = new QPushButton(notice);
    m_noticeAction->setObjectName("remoteNoticeAction");
    row->addWidget(m_noticeAction);
    connect(m_noticeAction, &QPushButton::clicked, this, [this]() {
        if (m_monitor.state() == RemoteAccessMonitor::State::Locked)
            showUnlockPrompt();
        else
            emit securitySettingsRequested();
    });
    m_notice = notice;
    updateState();
    return notice;
}

void NotificationBell::machinesEntered()
{
    m_machinesActive = true;
    m_visitPending = true;
    m_monitor.refresh();
}

void NotificationBell::machinesLeft()
{
    m_machinesActive = false;
    m_visitPending = false;
}

void NotificationBell::updateState()
{
    using State = RemoteAccessMonitor::State;
    const State state = m_monitor.state();
    const bool locked = state == State::Locked;
    const bool unavailable = state == State::Unavailable;
    QString label = tr("Notifications");
    if (locked)
        label = tr("Remote locked");
    else if (unavailable)
        label = tr("Remote unavailable");
    if (unreadCount() > 0)
        label += QStringLiteral(" · %1").arg(unreadCount());
    setText(label);
    setAccessibleName(label + ". " + readableStatus(m_monitor));
    setToolTip(readableStatus(m_monitor));
    setProperty("needsAttention", locked || unavailable || unreadCount() > 0);
    style()->unpolish(this);
    style()->polish(this);
    if (m_notificationDetail)
        m_notificationDetail->setText(readableStatus(m_monitor));
    if (m_unlockAction) {
        m_unlockAction->setVisible(locked);
        m_unlockAction->setEnabled(!m_monitor.busy());
    }
    if (m_notice) {
        m_notice->setVisible(locked || unavailable);
        m_noticeText->setText(locked
            ? tr("Remote access is locked. Unlock to manage your machines and use remote workflows.")
            : readableStatus(m_monitor));
        m_noticeAction->setText(locked ? tr("Unlock…") : tr("Security settings…"));
        m_noticeAction->setEnabled(!m_monitor.busy());
    }
    if (m_prompt) {
        m_promptDetail->setText(readableStatus(m_monitor));
        m_promptUnlock->setEnabled(locked && !m_monitor.busy());
        m_promptUnlock->setText(m_unlockRequested ? tr("Authenticating…") : tr("Unlock remote access"));
        if (state == State::Unlocked)
            m_prompt->accept();
    }
    // Consume one visit only after the fresh, non-interactive status check ends.
    if (m_visitPending && !m_monitor.busy() && state != State::Checking) {
        m_visitPending = false;
        if (locked)
            QTimer::singleShot(0, this, [this]() {
                if (m_machinesActive)
                    showUnlockPrompt();
            });
    }
}

void NotificationBell::showUnlockPrompt()
{
    if (m_monitor.state() != RemoteAccessMonitor::State::Locked)
        return;
    if (m_prompt) {
        m_prompt->raise();
        return;
    }
    auto *dialog = new QDialog(window());
    dialog->setObjectName("remoteUnlockDialog");
    dialog->setWindowTitle(tr("Unlock remote access"));
    dialog->setWindowModality(Qt::WindowModal);
    dialog->setAttribute(Qt::WA_DeleteOnClose);
    dialog->setFixedWidth(460);
    auto *column = new QVBoxLayout(dialog);
    column->setContentsMargins(24, 24, 24, 24);
    column->setSpacing(16);
    auto *message = new QLabel(tr("Authenticate with your computer to manage machines and run workflows remotely."));
    message->setTextFormat(Qt::PlainText);
    message->setWordWrap(true);
    column->addWidget(message);
    m_promptDetail = new QLabel(readableStatus(m_monitor));
    m_promptDetail->setObjectName("remoteUnlockDetail");
    m_promptDetail->setTextFormat(Qt::PlainText);
    m_promptDetail->setWordWrap(true);
    column->addWidget(m_promptDetail);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Cancel);
    m_promptUnlock = buttons->addButton(tr("Unlock remote access"), QDialogButtonBox::AcceptRole);
    m_promptUnlock->setObjectName("remoteUnlockConfirm");
    connect(m_promptUnlock, &QPushButton::clicked, this, [this]() {
        if (m_monitor.busy())
            return;
        m_unlockRequested = true;
        m_monitor.unlock();
        updateState();
    });
    connect(buttons, &QDialogButtonBox::rejected, dialog, &QDialog::reject);
    connect(dialog, &QDialog::rejected, this, [this]() {
        if (m_unlockRequested)
            m_monitor.cancelUnlock();
    });
    connect(dialog, &QDialog::finished, this, [this]() {
        m_prompt.clear();
        m_unlockRequested = false;
    });
    column->addWidget(buttons);
    m_prompt = dialog;
    updateState();
    dialog->open();
}
