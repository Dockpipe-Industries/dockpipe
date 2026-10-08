#include "PackageManagerDialog.h"

#include "DockpipeChoices.h"
#include "LauncherSettings.h"
#include "PackageCommand.h"

#include <QComboBox>
#include <QDir>
#include <QHeaderView>
#include <QJsonObject>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QSplitter>
#include <QTabWidget>
#include <QTableWidget>
#include <QTextBrowser>
#include <QUrl>
#include <QVBoxLayout>

namespace {
QTableWidget *packageTable()
{
    auto *table = new QTableWidget;
    table->setColumnCount(5);
    table->setHorizontalHeaderLabels({QObject::tr("Name"), QObject::tr("Version"), QObject::tr("Kind"), QObject::tr("Source"), QObject::tr("Status")});
    table->setSelectionBehavior(QAbstractItemView::SelectRows);
    table->setSelectionMode(QAbstractItemView::SingleSelection);
    table->setEditTriggers(QAbstractItemView::NoEditTriggers);
    table->setAlternatingRowColors(true);
    table->verticalHeader()->setVisible(false);
    table->horizontalHeader()->setSectionResizeMode(0, QHeaderView::Stretch);
    table->horizontalHeader()->setStretchLastSection(true);
    return table;
}

QString remoteManifest(QString remote)
{
    QUrl url(remote);
    if (url.path().isEmpty() || url.path() == QStringLiteral("/"))
        url.setPath(QStringLiteral("/packages/latest.json"));
    return url.toString();
}

void populate(QTableWidget *table, const QJsonArray &packages, const QString &filter, const QString &status)
{
    const QSignalBlocker blocker(table);
    table->setRowCount(0);
    for (const auto &value : packages) {
        const auto record = value.toObject();
        QStringList searchable;
        for (const auto &field : {"name", "title", "description", "kind", "provider", "capability", "version", "source"})
            searchable.append(record.value(QLatin1String(field)).toString());
        if (!searchable.join(QLatin1Char(' ')).contains(filter, Qt::CaseInsensitive))
            continue;
        const int row = table->rowCount();
        table->insertRow(row);
        QString title = record.value(QStringLiteral("title")).toString();
        if (title.isEmpty())
            title = record.value(QStringLiteral("name")).toString();
        auto *item = new QTableWidgetItem(title);
        item->setData(Qt::UserRole, record);
        table->setItem(row, 0, item);
        table->setItem(row, 1, new QTableWidgetItem(record.value(QStringLiteral("version")).toString()));
        table->setItem(row, 2, new QTableWidgetItem(record.value(QStringLiteral("kind")).toString()));
        table->setItem(row, 3, new QTableWidgetItem(record.value(QStringLiteral("source")).toString()));
        table->setItem(row, 4, new QTableWidgetItem(record.value(QStringLiteral("status")).toString(status)));
    }
    if (table->rowCount() > 0)
        table->selectRow(0);
}
} // namespace

PackageManagerDialog::PackageManagerDialog(const QString &hintWorkdir, QWidget *parent)
    : QDialog(parent), m_hintWorkdir(hintWorkdir)
{
    setWindowTitle(tr("Packages"));
    resize(1060, 680);
    if (m_hintWorkdir.isEmpty())
        m_hintWorkdir = QDir::homePath();
    m_localCommand = new PackageCommand(this);
    m_remoteCommand = new PackageCommand(this);
    auto *layout = new QVBoxLayout(this);
    auto *intro = new QLabel(tr("Install packages from Marketplace. Manage your packages in Installed."));
    intro->setWordWrap(true);
    layout->addWidget(intro);
    auto *remoteRow = new QHBoxLayout;
    remoteRow->addWidget(new QLabel(tr("Remote")));
    m_remote = new QComboBox;
    m_remote->addItems(LauncherSettings::current().packageRemotes);
    remoteRow->addWidget(m_remote, 1);
    m_refresh = new QPushButton(tr("Refresh"));
    m_refresh->setObjectName(QStringLiteral("refreshPackages"));
    remoteRow->addWidget(m_refresh);
    layout->addLayout(remoteRow);
    m_search = new QLineEdit;
    m_search->setPlaceholderText(tr("Search packages…"));
    layout->addWidget(m_search);
    auto *splitter = new QSplitter;
    m_tabs = new QTabWidget;
    m_installedTable = packageTable();
    m_installedTable->setObjectName(QStringLiteral("installedPackages"));
    m_marketplaceTable = packageTable();
    m_marketplaceTable->setObjectName(QStringLiteral("marketplacePackages"));
    m_tabs->addTab(m_installedTable, tr("Installed"));
    m_tabs->addTab(m_marketplaceTable, tr("Marketplace"));
    splitter->addWidget(m_tabs);
    m_details = new QTextBrowser;
    splitter->addWidget(m_details);
    splitter->setSizes({680, 340});
    layout->addWidget(splitter, 1);
    m_localStatus = new QLabel;
    m_localStatus->setWordWrap(true);
    m_localStatus->setTextFormat(Qt::PlainText);
    layout->addWidget(m_localStatus);
    m_status = new QLabel;
    m_status->setObjectName(QStringLiteral("packageStatus"));
    m_status->setWordWrap(true);
    m_status->setTextFormat(Qt::PlainText);
    layout->addWidget(m_status);
    m_actionHint = new QLabel;
    m_actionHint->setWordWrap(true);
    layout->addWidget(m_actionHint);
    auto *actions = new QHBoxLayout;
    actions->addStretch();
    m_cancel = new QPushButton(tr("Cancel operation"));
    m_cancel->setObjectName(QStringLiteral("cancelPackageOperation"));
    m_install = new QPushButton(tr("Install"));
    m_uninstall = new QPushButton(tr("Uninstall"));
    m_uninstall->setObjectName(QStringLiteral("uninstallPackage"));
    m_install->setObjectName(QStringLiteral("installPackage"));
    actions->addWidget(m_cancel);
    actions->addWidget(m_install);
    actions->addWidget(m_uninstall);
    layout->addLayout(actions);

    connect(m_search, &QLineEdit::textChanged, this, &PackageManagerDialog::applyFilter);
    connect(m_tabs, &QTabWidget::currentChanged, this, &PackageManagerDialog::refreshDetails);
    connect(m_installedTable, &QTableWidget::itemSelectionChanged, this, &PackageManagerDialog::refreshDetails);
    connect(m_marketplaceTable, &QTableWidget::itemSelectionChanged, this, &PackageManagerDialog::refreshDetails);
    connect(m_remote, &QComboBox::currentIndexChanged, this, &PackageManagerDialog::loadRemote);
    connect(m_refresh, &QPushButton::clicked, this, [this]() { loadInstalled(); loadRemote(); });
    connect(m_install, &QPushButton::clicked, this, &PackageManagerDialog::installSelected);
    connect(m_uninstall, &QPushButton::clicked, this, &PackageManagerDialog::uninstallSelected);
    connect(m_cancel, &QPushButton::clicked, this, [this]() {
        m_cancelling = true;
        m_status->setText(tr("Cancelling…"));
        m_localCommand->cancel();
        m_remoteCommand->cancel();
        updateButtons();
    });
    connect(m_localCommand, &PackageCommand::completed, this, [this](const QJsonObject &result) {
        m_inventoryReady = true;
        m_installed = result.value(QStringLiteral("packages")).toArray();
        m_installRoot = result.value(QStringLiteral("install_root")).toString();
        QStringList warnings;
        for (const auto &warning : result.value(QStringLiteral("warnings")).toArray())
            warnings.append(warning.toString());
        m_localStatus->setText(warnings.isEmpty() ? tr("Install location: %1").arg(m_installRoot) : warnings.join(QLatin1Char('\n')).left(2000));
        applyFilter();
    });
    connect(m_localCommand, &PackageCommand::failed, this, [this](const QString &error) {
        m_localStatus->setText(error);
        updateButtons();
    });
    connect(m_remoteCommand, &PackageCommand::completed, this, [this](const QJsonObject &result) {
        if (m_operation != Operation::Catalog) {
            const bool installed = m_operation == Operation::Install;
            m_operation = Operation::Catalog;
            m_status->setText((installed ? tr("Installed: %1") : tr("Uninstalled: %1. Package data and settings were kept."))
                                 .arg(result.value(QStringLiteral("path")).toString()));
            loadInstalled();
        } else {
            m_manifest = result.value(QStringLiteral("manifest")).toString();
            m_available = result.value(QStringLiteral("packages")).toArray();
            for (int index = 0; index < m_available.size(); ++index) {
                auto record = m_available[index].toObject();
                record.insert(QStringLiteral("source"), m_remote->currentText());
                m_available[index] = record;
            }
            m_status->setText(tr("%1 packages for %2").arg(m_available.size()).arg(result.value(QStringLiteral("platform")).toString()));
        }
        applyFilter();
    });
    connect(m_remoteCommand, &PackageCommand::failed, this, [this](const QString &error) {
        const bool changedPackages = m_operation != Operation::Catalog;
        m_operation = Operation::Catalog;
        m_status->setText(error);
        if (changedPackages)
            loadInstalled();
        updateButtons();
    });
    loadInstalled();
    loadRemote();
}

PackageManagerDialog::~PackageManagerDialog()
{
    // Stop children while the dialog state still exists; no callbacks during destruction.
    m_localCommand->disconnect(this);
    m_remoteCommand->disconnect(this);
    delete m_localCommand;
    delete m_remoteCommand;
}

void PackageManagerDialog::run(PackageCommand *command, const QStringList &arguments)
{
    command->start(DockpipeChoices::preferredDockpipeBinary(m_hintWorkdir), arguments, m_hintWorkdir);
    updateButtons();
}

void PackageManagerDialog::loadInstalled()
{
    if (m_localCommand->busy())
        return;
    m_inventoryReady = false;
    run(m_localCommand, {QStringLiteral("package"), QStringLiteral("list"), QStringLiteral("--format"), QStringLiteral("json"), QStringLiteral("--workdir"), m_hintWorkdir});
}

void PackageManagerDialog::loadRemote()
{
    if (m_remoteCommand->busy())
        return;
    m_available = {};
    m_manifest.clear();
    applyFilter();
    if (m_remote->currentText().isEmpty()) {
        m_status->setText(tr("Add a package remote in Settings to browse Marketplace."));
        return;
    }
    m_status->setText(tr("Loading remote catalog…"));
    run(m_remoteCommand, {QStringLiteral("package"), QStringLiteral("catalog"), QStringLiteral("--remote"), remoteManifest(m_remote->currentText())});
}

QJsonObject PackageManagerDialog::selection() const
{
    auto *table = m_tabs->currentIndex() == 0 ? m_installedTable : m_marketplaceTable;
    const auto *item = table->item(table->currentRow(), 0);
    return item ? item->data(Qt::UserRole).toJsonObject() : QJsonObject{};
}

void PackageManagerDialog::installSelected()
{
    const auto record = selection();
    if (!m_inventoryReady || record.isEmpty() || m_manifest.isEmpty() || m_remoteCommand->busy() || m_localCommand->busy()
        || m_tabs->currentIndex() != 1 || versionInstalled(record))
        return;
    m_operation = Operation::Install;
    m_status->setText(tr("Downloading and verifying %1…").arg(record.value(QStringLiteral("name")).toString()));
    run(m_remoteCommand, {QStringLiteral("package"), QStringLiteral("install"), QStringLiteral("--remote"), m_manifest,
        QStringLiteral("--kind"), record.value(QStringLiteral("kind")).toString(),
        QStringLiteral("--name"), record.value(QStringLiteral("name")).toString(),
        QStringLiteral("--sha256"), record.value(QStringLiteral("sha256")).toString()});
}

void PackageManagerDialog::uninstallSelected()
{
    const auto record = selection();
    if (!m_inventoryReady || m_tabs->currentIndex() != 0 || !record.value(QStringLiteral("removable")).toBool()
        || m_remoteCommand->busy() || m_localCommand->busy())
        return;
    m_operation = Operation::Uninstall;
    m_status->setText(tr("Uninstalling %1…").arg(record.value(QStringLiteral("name")).toString()));
    run(m_remoteCommand, {QStringLiteral("package"), QStringLiteral("uninstall"),
                         QStringLiteral("--path"), record.value(QStringLiteral("path")).toString()});
}

bool PackageManagerDialog::versionInstalled(const QJsonObject &record) const
{
    for (const auto &value : m_installed) {
        const auto installed = value.toObject();
        if (installed.value(QStringLiteral("name")) == record.value(QStringLiteral("name"))
            && installed.value(QStringLiteral("kind")) == record.value(QStringLiteral("kind"))
            && installed.value(QStringLiteral("version")) == record.value(QStringLiteral("version")))
            return true;
    }
    return false;
}

void PackageManagerDialog::applyFilter()
{
    populate(m_installedTable, m_installed, m_search->text(), tr("Installed"));
    QJsonArray available = m_available;
    for (int index = 0; index < available.size(); ++index) {
        auto remote = available[index].toObject();
        if (versionInstalled(remote))
            remote.insert(QStringLiteral("status"), tr("Installed"));
        available[index] = remote;
    }
    populate(m_marketplaceTable, available, m_search->text(), tr("Available"));
    m_tabs->setTabText(0, tr("Installed (%1)").arg(m_installedTable->rowCount()));
    m_tabs->setTabText(1, tr("Marketplace (%1)").arg(m_marketplaceTable->rowCount()));
    refreshDetails();
}

void PackageManagerDialog::refreshDetails()
{
    const auto record = selection();
    QStringList lines;
    for (const auto &field : {"name", "version", "kind", "description", "provider", "capability", "source", "path", "sha256"}) {
        const auto value = record.value(QLatin1String(field)).toString();
        if (!value.isEmpty())
            lines.append(QStringLiteral("%1: %2").arg(QLatin1String(field), value));
    }
    if (m_tabs->currentIndex() == 1 && !record.isEmpty() && !versionInstalled(record)) {
        lines.append(tr("Installs this package only. Required resolvers must also be installed. Existing project packages can take precedence."));
        lines.append(tr("A user copy of the same filename will be replaced after verification."));
    }
    m_details->setPlainText(lines.join(QStringLiteral("\n\n")));
    updateButtons();
}

void PackageManagerDialog::updateButtons()
{
    const bool busy = m_remoteCommand->busy() || m_localCommand->busy();
    if (!busy)
        m_cancelling = false;
    const bool marketplace = m_tabs->currentIndex() == 1;
    const auto record = selection();
    const bool installed = versionInstalled(record);
    m_remote->setEnabled(!busy);
    m_refresh->setEnabled(!busy);
    m_cancel->setVisible(busy);
    m_cancel->setEnabled(busy && !m_cancelling);
    m_install->setVisible(marketplace);
    m_install->setText(installed ? tr("Installed") : tr("Install"));
    m_install->setEnabled(m_inventoryReady && !busy && marketplace && !m_manifest.isEmpty() && !record.isEmpty() && !installed);
    m_uninstall->setVisible(!marketplace && !record.isEmpty());
    m_uninstall->setEnabled(m_inventoryReady && !busy && record.value(QStringLiteral("removable")).toBool());
    QString hint;
    if (!marketplace && !record.isEmpty()) {
        if (record.value(QStringLiteral("kind")).toString() == QStringLiteral("core"))
            hint = tr("Core is required by DockPipe and cannot be uninstalled here.");
        else if (record.value(QStringLiteral("source")).toString() == QStringLiteral("System"))
            hint = tr("This package is managed by your DockPipe installer and cannot be uninstalled here.");
        else if (!record.value(QStringLiteral("removable")).toBool())
            hint = tr("This package is managed outside the user store and cannot be uninstalled here.");
        else
            hint = tr("Uninstall removes this package. Its data and settings are kept.");
    } else if (marketplace && installed) {
        hint = tr("This version is already installed. Manage it from the Installed tab.");
    }
    m_actionHint->setText(hint);
    m_actionHint->setVisible(!hint.isEmpty());
}
