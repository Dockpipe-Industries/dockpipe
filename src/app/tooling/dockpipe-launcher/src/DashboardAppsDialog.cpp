#include "DashboardAppsDialog.h"
#include "CanvasIcons.h"
#include "DesktopWindowWidget.h"

#include <QDialogButtonBox>
#include <QFutureWatcher>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QPushButton>
#include <QVBoxLayout>
#include <QtConcurrent>

namespace {
struct ApplicationInventory {
    QVector<DesktopApplicationEntry> applications;
    QString error;
};
}

DashboardAppsDialog::DashboardAppsDialog(const QVector<WorkflowMeta> &workflows, QWidget *parent,
    bool allowDesktopApps)
    : QDialog(parent), m_workflows(workflows)
{
    setObjectName("dashboardAppsDialog");
    setWindowTitle(tr("Open an app"));
    setAttribute(Qt::WA_DeleteOnClose);
    resize(550, 570);
    auto *layout = new QVBoxLayout(this);
    m_search = new QLineEdit;
    m_search->setObjectName("dashboardAppSearch");
    m_search->setPlaceholderText(allowDesktopApps ? tr("Find a desktop app or DockPipe app…") : tr("Find a DockPipe app…"));
    m_search->setMaxLength(180);
    layout->addWidget(m_search);
    m_list = new QListWidget;
    m_list->setObjectName("dashboardAppList");
    layout->addWidget(m_list, 1);
    auto *hint = new QLabel(allowDesktopApps
        ? (DesktopWindowWidget::automaticAcquisitionUnavailableReason().isEmpty()
            ? tr("Apps open in Tiling when their window can be identified. Ambiguous windows require a selection.")
            : tr("Apps open on your desktop, then use the system window picker where supported."))
        : tr("DockPipe apps run through their existing workflows. Desktop windows are available in tiling mode."));
    hint->setWordWrap(true);
    layout->addWidget(hint);
    m_status = new QLabel(tr("Looking for installed applications…"));
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    layout->addWidget(m_status);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Open | QDialogButtonBox::Close);
    m_launch = buttons->button(QDialogButtonBox::Open);
    layout->addWidget(buttons);
    connect(buttons, &QDialogButtonBox::accepted, this, &DashboardAppsDialog::launchSelected);
    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
    connect(m_list, &QListWidget::itemDoubleClicked, this, &DashboardAppsDialog::launchSelected);
    connect(m_list, &QListWidget::currentRowChanged, this, [this]() {
        m_launch->setEnabled(!m_launching && m_list->currentItem());
    });
    connect(m_search, &QLineEdit::textChanged, this, &DashboardAppsDialog::rebuildList);
    rebuildList();
    if (!allowDesktopApps) {
        m_status->setText(tr("Choose a DockPipe app to open."));
        return;
    }

    // Inventory is bounded and only requested when the picker opens. Filesystem
    // scanning and platform launch dispatch never block the dashboard UI thread.
    auto *inventory = new QFutureWatcher<ApplicationInventory>(this);
    connect(inventory, &QFutureWatcher<ApplicationInventory>::finished, this, [this, inventory]() {
        const auto result = inventory->result();
        inventory->deleteLater();
        m_applications = result.applications;
        m_status->setText(result.error.isEmpty() ? tr("Choose an app to open.") : result.error);
        rebuildList();
    });
    inventory->setFuture(QtConcurrent::run([]() {
        ApplicationInventory result;
        result.applications = DesktopApplicationCatalog::enumerate(&result.error);
        return result;
    }));
}

void DashboardAppsDialog::rebuildList()
{
    if (m_launching)
        return;
    m_list->clear();
    const QString search = m_search->text().trimmed();
    for (const auto &workflow : m_workflows) {
        if (workflow.category.compare("app", Qt::CaseInsensitive) != 0)
            continue;
        const QString name = workflow.displayName.isEmpty() ? workflow.workflowId : workflow.displayName;
        if (!(name + ' ' + workflow.workflowId).contains(search, Qt::CaseInsensitive))
            continue;
        auto *item = new QListWidgetItem(CanvasIcons::icon(CanvasIcons::Symbol::Workflow), name + tr(" · DockPipe app"), m_list);
        item->setData(Qt::UserRole, "workflow");
        item->setData(Qt::UserRole + 1, workflow.workflowId);
    }
    for (int index = 0; index < m_applications.size(); ++index) {
        const auto &application = m_applications[index];
        if (!application.name.contains(search, Qt::CaseInsensitive))
            continue;
        if (!m_applicationIcons.contains(index)) {
            QIcon icon = DesktopApplicationCatalog::applicationIcon(application);
            if (icon.isNull())
                icon = CanvasIcons::icon(CanvasIcons::Symbol::Monitor);
            m_applicationIcons.insert(index, icon);
        }
        const QIcon icon = m_applicationIcons.value(index);
        auto *item = new QListWidgetItem(icon, application.name, m_list);
        item->setToolTip(application.name);
        item->setData(Qt::UserRole, "desktop");
        item->setData(Qt::UserRole + 1, index);
    }
    if (m_list->count())
        m_list->setCurrentRow(0);
    m_launch->setEnabled(m_list->currentItem());
}

void DashboardAppsDialog::launchSelected()
{
    const auto *item = m_list->currentItem();
    if (!item || m_launching)
        return;
    if (item->data(Qt::UserRole) == "workflow") {
        emit workflowRequested(item->data(Qt::UserRole + 1).toString());
        accept();
        return;
    }
    const auto application = m_applications.at(item->data(Qt::UserRole + 1).toInt());
    m_launching = true;
    emit desktopApplicationRequested(application);
    accept();
}
