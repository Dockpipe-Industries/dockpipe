#include "DesktopPermissionsWidget.h"
#include <QApplication>
#include <QHideEvent>
#include <QHBoxLayout>
#include <QLabel>
#include <QPushButton>
#include <QShowEvent>
#include <QTimer>
#include <QVBoxLayout>
#include <utility>

DesktopPermissionsWidget::DesktopPermissionsWidget(QWidget *parent,
    std::unique_ptr<DesktopPermissionAccess> access)
    : QGroupBox(tr("macOS permissions"), parent), access_(std::move(access))
{
    Q_ASSERT(access_);
    setObjectName(QStringLiteral("desktopPermissions"));
    auto *layout = new QVBoxLayout(this);
    auto *description = new QLabel(tr("Permissions belong to Dockpipe and are managed by macOS. Request access when you use a feature that needs it. VM displays and Dockpipe's own panels do not need these permissions."));
    description->setWordWrap(true);
    layout->addWidget(description);
    rows_ = {{
        addPermission(DesktopPermission::ScreenCapture, QStringLiteral("screenCapture"),
            tr("Screen capture"), tr("Capture content from other Mac apps.")),
        addPermission(DesktopPermission::Accessibility, QStringLiteral("accessibility"),
            tr("Accessibility"), tr("Allow supported desktop automation to control other Mac apps."))
    }};
    error_ = new QLabel;
    error_->setWordWrap(true);
    layout->addWidget(error_);
    auto *note = new QLabel(tr("Status updates when you return from System Settings. macOS may require restarting Dockpipe after a change."));
    note->setWordWrap(true);
    layout->addWidget(note);
    timer_ = new QTimer(this);
    timer_->setInterval(1000);
    connect(timer_, &QTimer::timeout, this, &DesktopPermissionsWidget::refresh);
    connect(qApp, &QGuiApplication::applicationStateChanged, this, [this](Qt::ApplicationState state) {
        if (state == Qt::ApplicationActive && isVisible())
            refresh();
    });
    refresh();
}

DesktopPermissionsWidget::PermissionRow DesktopPermissionsWidget::addPermission(
    DesktopPermission permission, const QString &name, const QString &title, const QString &description)
{
    auto *container = new QWidget;
    auto *row = new QHBoxLayout(container);
    row->setContentsMargins(0, 4, 0, 4);
    auto *text = new QVBoxLayout;
    auto *label = new QLabel(title);
    auto *detail = new QLabel(description);
    detail->setWordWrap(true);
    text->addWidget(label);
    text->addWidget(detail);
    row->addLayout(text, 1);
    auto *status = new QLabel;
    status->setObjectName(name + QStringLiteral("Status"));
    auto *request = new QPushButton(tr("Request access"));
    request->setObjectName(name + QStringLiteral("Request"));
    auto *settings = new QPushButton(tr("Open System Settings"));
    settings->setObjectName(name + QStringLiteral("Settings"));
    row->addWidget(status);
    row->addWidget(request);
    row->addWidget(settings);
    static_cast<QVBoxLayout *>(layout())->addWidget(container);
    connect(request, &QPushButton::clicked, this, [this, permission] {
        error_->clear();
        access_->request(permission);
        refresh();
    });
    connect(settings, &QPushButton::clicked, this, [this, permission] {
        error_->clear();
        if (!access_->openSettings(permission))
            error_->setText(tr("Could not open System Settings. Open Privacy & Security from the Apple menu."));
    });
    return {permission, status, request, settings};
}

void DesktopPermissionsWidget::refresh()
{
    for (const auto &row : rows_) {
        const auto state = access_->state(row.permission);
        if (state == DesktopPermissionState::Allowed)
            row.status->setText(tr("Allowed"));
        else if (state == DesktopPermissionState::NotAllowed)
            row.status->setText(tr("Not allowed"));
        else
            row.status->setText(tr("Unavailable"));
        row.request->setEnabled(state == DesktopPermissionState::NotAllowed);
        row.settings->setEnabled(state != DesktopPermissionState::Unavailable);
    }
}

void DesktopPermissionsWidget::showEvent(QShowEvent *event)
{
    QGroupBox::showEvent(event);
    refresh();
    timer_->start();
}

void DesktopPermissionsWidget::hideEvent(QHideEvent *event)
{
    timer_->stop();
    QGroupBox::hideEvent(event);
}
