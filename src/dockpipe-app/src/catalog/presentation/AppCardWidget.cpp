#include "AppCardWidget.h"

#include <QHBoxLayout>
#include <QIcon>
#include <QLabel>
#include <QPushButton>
#include <QVBoxLayout>

AppCardWidget::AppCardWidget(const WorkflowMeta &app, const QIcon &icon, bool compact,
                             bool running, bool launching, QWidget *parent)
    : QFrame(parent)
{
    setObjectName(QStringLiteral("appCard"));
    auto *root = new QVBoxLayout(this);
    root->setContentsMargins(18, 16, 18, 16);
    root->setSpacing(10);
    auto *heading = new QHBoxLayout;
    heading->setSpacing(12);
    auto *glyph = new QLabel;
    const int iconSize = compact ? 32 : 40;
    glyph->setPixmap(icon.pixmap(iconSize, iconSize));
    glyph->setFixedSize(iconSize, iconSize);
    heading->addWidget(glyph);
    auto *title = new QLabel(app.displayName.isEmpty() ? app.workflowId : app.displayName);
    title->setTextFormat(Qt::PlainText);
    title->setObjectName(QStringLiteral("cardTitle"));
    title->setWordWrap(true);
    auto *identity = new QVBoxLayout;
    identity->setSpacing(2);
    identity->addWidget(title);
    if (!app.sourceLabel().isEmpty()) {
        auto *provenance = new QLabel(app.sourceLabel());
        provenance->setObjectName("cardSource");
        provenance->setTextFormat(Qt::PlainText);
        provenance->setToolTip(app.sourcePath);
        identity->addWidget(provenance);
    }
    heading->addLayout(identity, 1);
    if (running || launching) {
        auto *status = new QLabel(launching ? tr("Starting") : tr("Running"));
        status->setObjectName(QStringLiteral("cardStatus"));
        heading->addWidget(status, 0, Qt::AlignVCenter);
    }
    root->addLayout(heading);
    if (!compact) {
        auto *description = new QLabel(app.description.isEmpty() ? tr("Open this app in your workspace.") : app.description);
        description->setTextFormat(Qt::PlainText);
        description->setObjectName(QStringLiteral("cardDescription"));
        description->setWordWrap(true);
        description->setMaximumHeight(42);
        description->setToolTip(app.description);
        root->addWidget(description);
    }
    auto *actions = new QHBoxLayout;
    auto *configure = new QPushButton(tr("Configure"));
    configure->setObjectName(QStringLiteral("quietButton"));
    configure->setAccessibleName(tr("Configure %1").arg(app.displayName));
    auto *launch = new QPushButton(launching ? tr("Starting…") : tr("Open app"));
    launch->setObjectName(QStringLiteral("cardLaunchButton"));
    launch->setEnabled(!launching && app.supported);
    launch->setToolTip(app.catalogDescription());
    launch->setAccessibleName(tr("Open %1").arg(app.displayName));
    connect(configure, &QPushButton::clicked, this, &AppCardWidget::configureRequested);
    connect(launch, &QPushButton::clicked, this, &AppCardWidget::launchRequested);
    actions->addWidget(configure);
    if (app.cloneable) {
        auto *clone = new QPushButton(tr("Clone"));
        clone->setToolTip(tr("Clone to workflows"));
        clone->setAccessibleName(tr("Clone %1 to workflows").arg(app.displayName));
        clone->setObjectName("cloneAppWorkflow");
        connect(clone, &QPushButton::clicked, this, &AppCardWidget::cloneRequested);
        actions->addWidget(clone);
    }
    actions->addStretch();
    actions->addWidget(launch);
    if (compact) {
        heading->addLayout(actions);
    } else {
        root->addStretch();
        root->addLayout(actions);
    }
}
