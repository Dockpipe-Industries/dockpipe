#include "EnvironmentsPage.h"

#include "CanvasIcons.h"
#include "DockpipeChoices.h"
#include "ResourceCanvas.h"

#include <QComboBox>
#include <QCheckBox>
#include <QDateTime>
#include <QDialog>
#include <QDialogButtonBox>
#include <QDesktopServices>
#include <QDir>
#include <QFileDialog>
#include <QFileInfo>
#include <QFormLayout>
#include <QFrame>
#include <QGridLayout>
#include <QHeaderView>
#include <QJsonObject>
#include <QLabel>
#include <QLineEdit>
#include <QLocale>
#include <QProcess>
#include <QPushButton>
#include <QRegularExpression>
#include <QSignalBlocker>
#include <QSizePolicy>
#include <QSpinBox>
#include <QSplitter>
#include <QStackedWidget>
#include <QTabBar>
#include <QTabWidget>
#include <QTreeWidget>
#include <QUrl>
#include <QVBoxLayout>
#include <algorithm>

namespace {

constexpr qsizetype maxExplicitFiles = 2048;
const QString localHostId = QStringLiteral("@local");
const QString virtualMachinePrefix = QStringLiteral("vm:");
// Retain observations for at most three intervals of the VM page's existing
// five-second poll, measured from the start of the provider's observation.
constexpr qint64 maxVmObservationAgeMs = 15000;

qint64 remainingVmObservation(qint64 observedAt)
{
    const auto now = QDateTime::currentMSecsSinceEpoch();
    if (observedAt <= 0 || observedAt > now || now - observedAt >= maxVmObservationAgeMs)
        return 0;
    return maxVmObservationAgeMs - (now - observedAt);
}

QLabel *definitionDetails()
{
    auto *label = new QLabel;
    label->setTextFormat(Qt::PlainText);
    label->setTextInteractionFlags(Qt::TextSelectableByMouse);
    label->setWordWrap(true);
    label->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    return label;
}

QString definitionName(const EnvironmentDefinition &definition)
{
    return definition.name.isEmpty() ? QFileInfo(definition.path).fileName() : definition.name;
}

bool yamlDefinition(const QString &path)
{
    const auto extension = QFileInfo(path).suffix();
    return extension.compare(QStringLiteral("yaml"), Qt::CaseInsensitive) == 0
        || extension.compare(QStringLiteral("yml"), Qt::CaseInsensitive) == 0;
}

}

EnvironmentsPage::EnvironmentsPage(QWidget *parent)
    : QWidget(parent), m_client(this)
{
    setObjectName(QStringLiteral("environmentsPage"));
    auto *frame = new QVBoxLayout(this);
    frame->setContentsMargins(0, 0, 0, 0);
    m_back = new QPushButton(tr("← Environments"));
    m_back->hide();
    frame->addWidget(m_back, 0, Qt::AlignLeft);
    m_sections = new QStackedWidget(this);
    frame->addWidget(m_sections, 1);
    m_inventory = new QWidget(m_sections);
    m_sections->addWidget(m_inventory);

    auto *layout = new QVBoxLayout(m_inventory);
    layout->setContentsMargins(0, 0, 0, 0);

    auto *toolbar = new QHBoxLayout;
    toolbar->setContentsMargins(20, 12, 20, 0);
    auto *tabs = new QTabBar;
    tabs->setObjectName(QStringLiteral("environmentViewTabs"));
    tabs->setDrawBase(false);
    tabs->addTab(tr("Canvas"));
    tabs->addTab(tr("List"));
    toolbar->addWidget(tabs);
    toolbar->addStretch(1);
    auto *refreshButton = new QPushButton(tr("Refresh"));
    toolbar->addWidget(refreshButton);
    auto *add = new QPushButton(tr("Add Environment"));
    add->setObjectName(QStringLiteral("primaryButton"));
    toolbar->addWidget(add);
    layout->addLayout(toolbar);

    m_status = new QLabel;
    m_status->setObjectName(QStringLiteral("environmentDefinitionsStatus"));
    m_status->setTextFormat(Qt::PlainText);
    m_status->setContentsMargins(20, 0, 20, 0);
    m_status->setWordWrap(true);
    m_status->hide();
    layout->addWidget(m_status);

    m_views = new QStackedWidget(this);
    m_canvas = new ResourceCanvas(QStringLiteral("environments"), tr("Environments"),
        tr("Hosts, virtual machines, and containers"), buildInspector(true), m_views);
    m_canvas->setDetailsLabel(tr("Details"));
    m_canvas->setCompactDetails(true, 300);
    m_views->addWidget(m_canvas);

    auto *table = new QWidget;
    auto *tableLayout = new QVBoxLayout(table);
    tableLayout->setContentsMargins(20, 12, 20, 20);
    auto *title = new QLabel(tr("Environments"));
    title->setObjectName(QStringLiteral("sectionTitle"));
    tableLayout->addWidget(title);
    auto *splitter = new QSplitter(Qt::Horizontal, table);
    splitter->setChildrenCollapsible(false);
    m_list = new QTreeWidget(splitter);
    m_list->setObjectName(QStringLiteral("environmentDefinitionsList"));
    m_list->setRootIsDecorated(false);
    m_list->setUniformRowHeights(true);
    m_list->setAlternatingRowColors(true);
    m_list->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_list->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_list->setTextElideMode(Qt::ElideRight);
    m_list->setHeaderLabels({tr("Name"), tr("Type"), tr("Location"), tr("Status")});
    m_list->header()->setDefaultAlignment(Qt::AlignLeft | Qt::AlignVCenter);
    m_list->header()->setStretchLastSection(false);
    m_list->header()->setSectionResizeMode(0, QHeaderView::Stretch);
    for (const int column : {1, 2, 3})
        m_list->header()->setSectionResizeMode(column, QHeaderView::ResizeToContents);

    auto *listInspector = buildInspector(false);
    listInspector->setMinimumWidth(220);
    splitter->addWidget(listInspector);
    splitter->setStretchFactor(0, 3);
    splitter->setStretchFactor(1, 1);
    splitter->setSizes({760, 280});
    tableLayout->addWidget(splitter, 1);
    m_views->addWidget(table);
    layout->addWidget(m_views, 1);

    connect(tabs, &QTabBar::currentChanged, this, [this](int index) {
        m_views->setCurrentIndex(index);
        if (m_views->currentWidget() == m_canvas && !m_selectedPath.isEmpty()) {
            const QSignalBlocker block(m_canvas);
            m_canvas->focusResource(m_selectedPath);
        }
    });
    connect(m_list, &QTreeWidget::currentItemChanged, this, [this](QTreeWidgetItem *item) {
        selectDefinition(item ? item->data(0, Qt::UserRole).toString() : QString());
    });
    connect(m_canvas, &ResourceCanvas::resourceSelected, this, &EnvironmentsPage::selectDefinition);
    connect(add, &QPushButton::clicked, this, &EnvironmentsPage::addEnvironment);
    connect(refreshButton, &QPushButton::clicked, this, &EnvironmentsPage::refresh);
    connect(m_back, &QPushButton::clicked, this, [this]() { showControls(m_inventory); });
    connect(&m_client, &EnvironmentDefinitionsClient::loaded, this, &EnvironmentsPage::showDefinitions);
    connect(&m_client, &EnvironmentDefinitionsClient::failed, this, [this](const QString &error) {
        for (auto &definition : m_definitions) {
            if (!definition.runtimeState.isEmpty()) {
                definition.runtimeState = QStringLiteral("unverified");
                definition.runtimeRunId.clear();
                definition.runtimeContainerId.clear();
            }
        }
        renderDefinitions();
        emit containerTargetsChanged();
        m_status->setText(error);
        m_status->show();
    });
    m_inventoryRefresh.setInterval(5000);
    connect(&m_inventoryRefresh, &QTimer::timeout, this, [this]() {
        if (!m_client.isLoading()) {
            m_client.load(m_workdir, m_files);
            if (m_active)
                emit runtimeInventoryRequested();
        }
    });
    m_vmExpiry.setSingleShot(true);
    m_vmExpiry.setTimerType(Qt::PreciseTimer);
    connect(&m_vmExpiry, &QTimer::timeout, this, [this]() {
        setVmObservations({}, 0);
    });
    selectDefinition({});
}

void EnvironmentsPage::setWorkspace(const QString &workdir)
{
    if (m_workdir == workdir)
        return;

    m_workdir = workdir;
    m_files.clear();
    m_client.cancel();
    m_vmExpiry.stop();
    m_vmStates.clear();
    m_virtualMachineInventory = {};
    m_vmObservedAt = 0;
    showDefinitions({});
    if (m_active || m_terminalInventoryActive)
        refresh();
}

void EnvironmentsPage::setActive(bool active)
{
    if (m_active == active)
        return;

    m_active = active;
    if (m_active) {
        if (remainingVmObservation(m_vmObservedAt) == 0)
            setVmObservations({}, 0);
        refresh();
        m_inventoryRefresh.start();
    } else if (!m_terminalInventoryActive) {
        m_inventoryRefresh.stop();
        m_client.cancel();
    }
    emit virtualMachineControlsActivityChanged(virtualMachineControlsActive());
}

void EnvironmentsPage::setTerminalInventoryActive(bool active)
{
    if (m_terminalInventoryActive == active)
        return;

    m_terminalInventoryActive = active;
    if (active) {
        if (!m_client.isLoading())
            m_client.load(m_workdir, m_files);
        m_inventoryRefresh.start();
    } else if (!m_active) {
        m_inventoryRefresh.stop();
        m_client.cancel();
    }
}

QMap<QString, QString> EnvironmentsPage::runningContainerTargets() const
{
    QMap<QString, QString> targets;
    for (const auto &definition : m_definitions) {
        if (!definition.runtimeContainerId.isEmpty() && runtimeState(definition) == "running")
            targets.insert(definition.runtimeContainerId, definitionName(definition));
    }
    return targets;
}

void EnvironmentsPage::showInventory()
{
    showControls(m_inventory);
}

void EnvironmentsPage::setVirtualMachineControls(QWidget *controls)
{
    if (m_virtualMachineControls == controls)
        return;

    if (m_virtualMachineControls) {
        if (m_sections->currentWidget() == m_virtualMachineControls)
            showControls(m_inventory);
        m_sections->removeWidget(m_virtualMachineControls);
    }
    m_virtualMachineControls = controls;
    if (controls)
        m_sections->addWidget(controls);
    selectDefinition(m_selectedPath);
}

void EnvironmentsPage::showControls(QWidget *controls)
{
    if (!controls || m_sections->currentWidget() == controls)
        return;

    m_sections->setCurrentWidget(controls);
    m_back->setVisible(controls != m_inventory);
    emit virtualMachineControlsActivityChanged(virtualMachineControlsActive());
}

void EnvironmentsPage::showVirtualMachineControls()
{
    showControls(m_virtualMachineControls);
}

bool EnvironmentsPage::virtualMachineControlsActive() const
{
    return m_active && m_virtualMachineControls && m_sections->currentWidget() == m_virtualMachineControls;
}

void EnvironmentsPage::setMachineInventory(const QVector<ResourceNode> &nodes, const QVector<ResourceEdge> &edges)
{
    m_machineNodes = nodes;
    m_machineEdges = edges;
    renderDefinitions();
}

void EnvironmentsPage::setMachineCapabilities(const QJsonArray &catalog)
{
    m_machineCapabilities = catalog;
    selectDefinition(m_selectedPath);
}

void EnvironmentsPage::setVirtualMachineInventory(const QJsonArray &resources)
{
    m_virtualMachineInventory = resources;
    renderDefinitions();
}

void EnvironmentsPage::setVmObservations(const QHash<QString, QString> &states, qint64 observedAt)
{
    const auto remaining = remainingVmObservation(observedAt);
    const auto observations = remaining > 0 ? states : QHash<QString, QString>{};
    const bool renewed = !observations.isEmpty() && remainingVmObservation(m_vmObservedAt) == 0;
    m_vmObservedAt = remaining > 0 ? observedAt : 0;
    if (remaining > 0 && !observations.isEmpty())
        m_vmExpiry.start(int(remaining));
    else
        m_vmExpiry.stop();

    if (!renewed && m_vmStates == observations)
        return;
    m_vmStates = observations;
    renderDefinitions();
}

void EnvironmentsPage::refresh()
{
    m_status->setText(tr("Reading environment definitions…"));
    m_status->show();
    m_client.load(m_workdir, m_files);
    emit runtimeInventoryRequested();
    emit machineRefreshRequested();
}

void EnvironmentsPage::addEnvironment()
{
    QDialog dialog(this);
    dialog.setWindowTitle(tr("Add Environment"));
    dialog.setMinimumWidth(520);
    auto *layout = new QVBoxLayout(&dialog);
    auto *tiles = new QGridLayout;
    tiles->setSpacing(10);
    tiles->setColumnStretch(0, 1);
    tiles->setColumnStretch(1, 1);
    layout->addLayout(tiles);

    struct Choice {
        QString kind;
        QString title;
        QString description;
        CanvasIcons::Symbol icon;
    };
    const Choice choices[]{
        {"machine", tr("Remote machine"), tr("Pair another computer with Dockpipe."), CanvasIcons::Symbol::Globe},
        {"vm", tr("Virtual machine"), tr("Choose an operating system and configure a VM."), CanvasIcons::Symbol::Monitor},
        {"container", tr("Container"), tr("Choose an image and configure a Docker container."), CanvasIcons::Symbol::Apps},
        {"file", tr("Import YAML"), tr("Open existing environment definitions."), CanvasIcons::Symbol::Code},
    };
    QString selected;
    int index = 0;
    for (const auto &choice : choices) {
        auto *tile = new QPushButton;
        tile->setObjectName(QStringLiteral("environmentTypeTile"));
        tile->setProperty("environmentKind", choice.kind);
        tile->setAccessibleName(choice.title);
        tile->setAccessibleDescription(choice.description);
        tile->setAutoDefault(false);
        tile->setMinimumHeight(120);
        tile->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Expanding);

        auto *content = new QVBoxLayout(tile);
        content->setContentsMargins(16, 14, 16, 14);
        content->setSpacing(5);
        auto *icon = new QLabel;
        icon->setPixmap(CanvasIcons::icon(choice.icon).pixmap(QSize(22, 22), devicePixelRatioF()));
        auto *title = new QLabel(choice.title);
        title->setObjectName(QStringLiteral("environmentTileTitle"));
        title->setWordWrap(true);
        auto *description = new QLabel(choice.description);
        description->setObjectName(QStringLiteral("hintText"));
        description->setWordWrap(true);
        for (auto *label : {icon, title, description}) {
            label->setAttribute(Qt::WA_TransparentForMouseEvents);
            content->addWidget(label);
        }
        content->addStretch();
        tiles->addWidget(tile, index / 2, index % 2);
        ++index;

        connect(tile, &QPushButton::clicked, &dialog, [&dialog, &selected, kind = choice.kind] {
            selected = kind;
            dialog.accept();
        });
    }

    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Cancel);
    layout->addWidget(buttons);
    connect(buttons, &QDialogButtonBox::rejected, &dialog, &QDialog::reject);
    if (dialog.exec() != QDialog::Accepted)
        return;

    if (selected == "machine")
        emit addRemoteMachineRequested();
    else if (selected == "file")
        includeFiles();
    else if (selected == "container")
        createContainerDefinition();
    else if (selected == "vm")
        createVmDefinition();
}

void EnvironmentsPage::createVmDefinition()
{
    if (m_generation)
        return;

    QDialog dialog(this);
    dialog.setWindowTitle(tr("Add Virtual Machine"));
    dialog.setObjectName(QStringLiteral("environmentVmForm"));
    dialog.setMinimumWidth(560);
    auto *layout = new QVBoxLayout(&dialog);
    auto *host = new QLabel(tr("Runs on this computer’s VM provider."));
    host->setObjectName(QStringLiteral("hintText"));
    layout->addWidget(host);
    auto *form = new QFormLayout;
    auto *name = new QLineEdit;
    name->setPlaceholderText(tr("Development VM"));
    auto *os = new QComboBox;
    os->addItem(tr("Ubuntu"), QStringLiteral("ubuntu"));
    os->addItem(tr("Debian"), QStringLiteral("debian"));
    os->addItem(tr("Fedora"), QStringLiteral("fedora"));
    os->addItem(tr("Other Linux"), QStringLiteral("other-linux"));
    os->addItem(tr("Windows 11"), QStringLiteral("windows11"));
    auto *cpus = new QSpinBox;
    cpus->setMaximum(256);
    auto *memory = new QSpinBox;
    memory->setMaximum(1024);
    memory->setSuffix(tr(" GiB"));
    auto *disk = new QSpinBox;
    disk->setMaximum(16384);
    disk->setSuffix(tr(" GiB"));

    auto *media = new QLineEdit;
    media->setPlaceholderText(tr("Installer ISO file"));
    auto *browse = new QPushButton(tr("Browse…"));
    auto *mediaLayout = new QHBoxLayout;
    mediaLayout->addWidget(media, 1);
    mediaLayout->addWidget(browse);
    connect(browse, &QPushButton::clicked, &dialog, [&] {
        const auto file = QFileDialog::getOpenFileName(&dialog, tr("Select Installer ISO"), m_workdir, tr("ISO images (*.iso *.ISO)"));
        if (!file.isEmpty())
            media->setText(file);
    });

    auto *scope = new QComboBox;
    scope->addItem(tr("Personal"), false);
    scope->addItem(tr("YAML file"), true);
    form->addRow(tr("Name"), name);
    form->addRow(tr("Operating system"), os);
    form->addRow(tr("CPUs"), cpus);
    form->addRow(tr("RAM"), memory);
    form->addRow(tr("Disk size"), disk);
    form->addRow(tr("Installer ISO"), mediaLayout);
    form->addRow(tr("Save to"), scope);
    layout->addLayout(form);

    auto *hint = new QLabel(tr("The ISO is optional when saving a definition. Disk size applies when the virtual disk is first created."));
    hint->setObjectName(QStringLiteral("hintText"));
    hint->setWordWrap(true);
    layout->addWidget(hint);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Cancel);
    auto *save = buttons->addButton(tr("Save Definition"), QDialogButtonBox::ActionRole);
    auto *create = buttons->addButton(tr("Create VM"), QDialogButtonBox::AcceptRole);
    create->setObjectName(QStringLiteral("primaryButton"));
    create->setDefault(true);
    constexpr int saveOnly = 2;
    connect(save, &QPushButton::clicked, &dialog, [&dialog] { dialog.done(saveOnly); });

    const QRegularExpression whitespace(QStringLiteral("\\s+"));
    const QRegularExpression validName(QStringLiteral("^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$"));
    const auto environmentName = [&] { return name->text().trimmed().replace(whitespace, "-"); };
    const auto update = [&] {
        const bool valid = validName.match(environmentName()).hasMatch();
        const auto iso = media->text().trimmed();
        const bool isIso = QFileInfo(iso).suffix().compare(QStringLiteral("iso"), Qt::CaseInsensitive) == 0;
        save->setEnabled(valid && (iso.isEmpty() || isIso));
        create->setEnabled(valid && isIso);
    };
    struct Hardware {
        int cpus;
        int memory;
        int disk;
    };
    const QHash<QString, Hardware> defaults{
        {"ubuntu", {2, 4, 64}}, {"debian", {2, 2, 32}}, {"fedora", {4, 8, 64}},
        {"other-linux", {2, 4, 32}}, {"windows11", {4, 8, 80}},
    };
    const auto selectPreset = [&] {
        const auto selected = os->currentData().toString();
        const auto hardware = defaults.value(selected);
        const bool windows = selected == "windows11";
        cpus->setMinimum(windows ? 2 : 1);
        memory->setMinimum(windows ? 4 : 1);
        disk->setMinimum(windows ? 64 : 1);
        cpus->setValue(hardware.cpus);
        memory->setValue(hardware.memory);
        disk->setValue(hardware.disk);
    };
    connect(os, &QComboBox::currentIndexChanged, &dialog, selectPreset);
    connect(name, &QLineEdit::textChanged, &dialog, update);
    connect(media, &QLineEdit::textChanged, &dialog, update);
    connect(buttons, &QDialogButtonBox::accepted, &dialog, &QDialog::accept);
    connect(buttons, &QDialogButtonBox::rejected, &dialog, &QDialog::reject);
    layout->addWidget(buttons);
    selectPreset();
    update();
    name->setFocus();
    const int result = dialog.exec();
    if (result != QDialog::Accepted && result != saveOnly)
        return;

    EnvironmentDefinition definition;
    definition.name = environmentName();
    definition.kind = QStringLiteral("vm");
    definition.scope = scope->currentData().toBool() ? QStringLiteral("explicit") : QStringLiteral("personal");
    QStringList arguments{"--os", os->currentData().toString(), "--cpus", QString::number(cpus->value()),
        "--memory-gib", QString::number(memory->value()), "--disk-gib", QString::number(disk->value())};
    if (!media->text().trimmed().isEmpty())
        arguments << "--installer-iso" << media->text().trimmed();
    saveDefinition(definition, arguments, result == QDialog::Accepted);
}

void EnvironmentsPage::createContainerDefinition()
{
    if (m_generation)
        return;

    QDialog dialog(this);
    dialog.setWindowTitle(tr("Add Container"));
    dialog.setMinimumWidth(560);
    auto *layout = new QVBoxLayout(&dialog);
    auto *host = new QLabel(tr("Runs on this computer’s Docker engine."));
    host->setObjectName(QStringLiteral("hintText"));
    layout->addWidget(host);

    auto *tabs = new QTabWidget;
    layout->addWidget(tabs);
    auto *general = new QWidget;
    auto *form = new QFormLayout(general);
    tabs->addTab(general, tr("General"));

    auto *name = new QLineEdit;
    name->setPlaceholderText(tr("Test Docker"));
    name->setToolTip(tr("Use letters, numbers, dots, underscores, or hyphens. Spaces become hyphens."));
    auto *base = new QComboBox;
    base->addItem(tr("Ubuntu 24.04"), QStringLiteral("ubuntu"));
    base->addItem(tr("Debian 13 Slim"), QStringLiteral("debian"));
    base->addItem(tr("Alpine 3.22"), QStringLiteral("alpine"));
    base->addItem(tr("NGINX on Alpine"), QStringLiteral("nginx"));
    base->addItem(tr("Custom image"), QString());
    auto *image = new QLineEdit;
    const auto exampleImageReference = QStringLiteral("registry.example.com/team/image:tag");
    image->setPlaceholderText(exampleImageReference);
    auto *startup = new QComboBox;
    startup->addItem(QString(), QString());
    startup->addItem(tr("Keep running for Terminal"), QStringLiteral("keep-running"));
    startup->addItem(tr("Run the image’s command"), QStringLiteral("image"));
    auto *network = new QComboBox;
    network->addItem(tr("Bridge"), QStringLiteral("bridge"));
    network->addItem(tr("No network"), QStringLiteral("none"));
    auto *security = new QComboBox;
    security->addItem(tr("Standard"), QStringLiteral("standard"));
    security->addItem(tr("Restricted capabilities"), QStringLiteral("restricted"));
    security->setToolTip(tr("Both profiles block privilege escalation. Restricted also drops all Linux capabilities."));
    auto *scope = new QComboBox;
    scope->addItem(tr("Personal"), false);
    scope->addItem(tr("YAML file"), true);

    form->addRow(tr("Name"), name);
    form->addRow(tr("Base OS or image"), base);
    form->addRow(tr("Docker image"), image);
    form->addRow(tr("Startup"), startup);
    form->addRow(tr("Network"), network);
    form->addRow(tr("Security"), security);
    form->addRow(tr("Save to"), scope);

    auto *advanced = new QWidget;
    auto *options = new QFormLayout(advanced);
    tabs->addTab(advanced, tr("Advanced"));
    auto *user = new QComboBox;
    user->addItem(tr("Use the image’s user"), QStringLiteral("image"));
    user->addItem(tr("Non-root user"), QStringLiteral("non-root"));
    auto *filesystem = new QComboBox;
    filesystem->addItem(tr("Writable"), false);
    filesystem->addItem(tr("Read-only"), true);
    auto *restart = new QComboBox;
    restart->addItem(tr("Do not restart"), QStringLiteral("no"));
    restart->addItem(tr("Restart unless stopped"), QStringLiteral("unless-stopped"));
    auto *storage = new QComboBox;
    storage->addItem(tr("No separate volume"), false);
    storage->addItem(tr("Persistent Docker volume"), true);
    auto *dataPath = new QLineEdit;
    const auto defaultContainerDataPath = QStringLiteral("/data");
    dataPath->setText(defaultContainerDataPath);
    auto *publish = new QCheckBox(tr("Publish a port on this computer"));
    auto *hostPort = new QSpinBox;
    auto *containerPort = new QSpinBox;
    for (auto *port : {hostPort, containerPort})
        port->setRange(1, 65535);
    hostPort->setValue(8080);
    containerPort->setValue(80);

    options->addRow(tr("Run as"), user);
    options->addRow(tr("Root filesystem"), filesystem);
    options->addRow(tr("Restart policy"), restart);
    options->addRow(tr("Storage"), storage);
    options->addRow(tr("Folder inside container"), dataPath);
    options->addRow(publish);
    options->addRow(tr("Local port"), hostPort);
    options->addRow(tr("Container port"), containerPort);

    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Cancel);
    auto *save = buttons->addButton(tr("Save Definition"), QDialogButtonBox::ActionRole);
    auto *create = buttons->addButton(tr("Create Container"), QDialogButtonBox::AcceptRole);
    create->setObjectName(QStringLiteral("primaryButton"));
    create->setDefault(true);
    layout->addWidget(buttons);
    constexpr int saveOnly = 2;
    connect(save, &QPushButton::clicked, &dialog, [&dialog] { dialog.done(saveOnly); });
    connect(buttons, &QDialogButtonBox::accepted, &dialog, &QDialog::accept);
    connect(buttons, &QDialogButtonBox::rejected, &dialog, &QDialog::reject);

    const QRegularExpression whitespace(QStringLiteral("\\s+"));
    const QRegularExpression validName(QStringLiteral("^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$"));
    const auto environmentName = [&] { return name->text().trimmed().replace(whitespace, "-"); };
    const auto update = [&] {
        const auto preset = base->currentData().toString();
        const bool customImage = preset.isEmpty();
        form->setRowVisible(image, customImage);
        startup->setItemText(0, customImage || preset == "nginx"
            ? tr("Default (run the image’s command)") : tr("Default (keep running for Terminal)"));
        const bool separateVolume = storage->currentData().toBool();
        options->setRowVisible(dataPath, separateVolume);
        const bool connected = network->currentData().toString() == "bridge";
        publish->setEnabled(connected);
        options->setRowVisible(hostPort, connected && publish->isChecked());
        options->setRowVisible(containerPort, connected && publish->isChecked());
        const bool ready = validName.match(environmentName()).hasMatch()
            && (!customImage || !image->text().trimmed().isEmpty())
            && (!separateVolume || !dataPath->text().trimmed().isEmpty());
        create->setEnabled(ready);
        save->setEnabled(ready);
    };
    for (auto *combo : {base, network, storage})
        connect(combo, &QComboBox::currentIndexChanged, &dialog, update);
    for (auto *field : {name, image, dataPath})
        connect(field, &QLineEdit::textChanged, &dialog, update);
    connect(publish, &QCheckBox::toggled, &dialog, update);
    update();
    name->setFocus();

    const int result = dialog.exec();
    if (result != QDialog::Accepted && result != saveOnly)
        return;

    EnvironmentDefinition definition;
    definition.name = environmentName();
    definition.kind = QStringLiteral("container");
    definition.scope = scope->currentData().toBool() ? QStringLiteral("explicit") : QStringLiteral("personal");
    const auto preset = base->currentData().toString();
    QStringList arguments{"--network", network->currentData().toString(), "--security", security->currentData().toString(),
        "--user", user->currentData().toString(), "--restart", restart->currentData().toString()};
    if (preset.isEmpty())
        arguments << "--image" << image->text().trimmed();
    else
        arguments << "--preset" << preset;
    const auto command = startup->currentData().toString();
    if (!command.isEmpty())
        arguments << "--startup" << command;
    if (filesystem->currentData().toBool())
        arguments << "--read-only";
    if (storage->currentData().toBool())
        arguments << "--data-path" << dataPath->text().trimmed();
    if (publish->isEnabled() && publish->isChecked())
        arguments << "--host-port" << QString::number(hostPort->value()) << "--container-port" << QString::number(containerPort->value());
    saveDefinition(definition, arguments, result == QDialog::Accepted);
}

void EnvironmentsPage::saveDefinition(EnvironmentDefinition definition, QStringList arguments, bool start)
{
    if (m_generation)
        return;

    const QStringList command{"environment", "generate", "--kind", definition.kind, "--name", definition.name};
    const auto workdir = m_workdir;
    if (definition.scope == "explicit") {
        definition.path = QFileDialog::getSaveFileName(this, tr("Save environment definition"),
            QDir(workdir).filePath(definition.name + ".yaml"), tr("Environment definitions (*.yaml *.yml)"));
        if (definition.path.isEmpty())
            return;
        if (QFileInfo(definition.path).suffix().isEmpty())
            definition.path += QStringLiteral(".yaml");
        if (!yamlDefinition(definition.path) || (m_files.size() >= maxExplicitFiles && !m_files.contains(definition.path))) {
            m_status->setText(tr("Choose a YAML file and include at most 2,048 definitions."));
            m_status->show();
            return;
        }
        arguments << "--file" << definition.path;
    } else {
        arguments << "--personal";
        if (start)
            arguments << "--print-path";
    }

    auto *process = new QProcess(this);
    m_generation = process;
    process->setProgram(DockpipeChoices::preferredDockpipeBinary(workdir));
    process->setWorkingDirectory(workdir.isEmpty() ? QDir::homePath() : workdir);
    process->setArguments(command + arguments);
    process->setStandardInputFile(QProcess::nullDevice());
    if (!start || !definition.path.isEmpty())
        process->setStandardOutputFile(QProcess::nullDevice());
    const auto finish = [this, process, definition, workdir, start](bool ok) {
        if (m_generation != process)
            return;
        m_generation = nullptr;
        const auto diagnostic = QString::fromLocal8Bit(process->readAllStandardError().right(2048)).trimmed();
        auto savedDefinition = definition;
        if (ok && start && savedDefinition.path.isEmpty())
            savedDefinition.path = QString::fromUtf8(process->readAllStandardOutput().left(8192)).trimmed();
        process->deleteLater();
        if (ok) {
            if (!definition.path.isEmpty() && workdir == m_workdir && !m_files.contains(definition.path))
                m_files.append(definition.path);
            if (start && workdir == m_workdir && QDir::isAbsolutePath(savedDefinition.path)) {
                emit applyRequested(savedDefinition);
            }
            refresh();
        } else {
            m_status->setText(diagnostic.isEmpty() ? tr("Dockpipe could not save the environment definition.") : diagnostic);
            m_status->show();
        }
    };
    connect(process, &QProcess::errorOccurred, this, [finish](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart)
            finish(false);
    });
    connect(process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
        [finish](int result, QProcess::ExitStatus status) { finish(result == 0 && status == QProcess::NormalExit); });
    QTimer::singleShot(15000, process, [process] {
        if (process->state() != QProcess::NotRunning)
            process->kill();
    });
    process->start();
}

QWidget *EnvironmentsPage::buildInspector(bool canvas)
{
    auto *panel = new QFrame;
    if (!canvas)
        panel->setObjectName(QStringLiteral("environmentInspector"));
    auto *layout = new QVBoxLayout(panel);
    layout->setContentsMargins(canvas ? 0 : 16, canvas ? 0 : 14, canvas ? 0 : 16, canvas ? 0 : 14);
    layout->setAlignment(Qt::AlignTop);
    auto *title = new QLabel;
    title->setObjectName(QStringLiteral("sectionTitle"));
    title->setWordWrap(true);
    title->setTextFormat(Qt::PlainText);
    m_inspectorTitles.append(title);
    layout->addWidget(title);

    auto *details = definitionDetails();
    if (canvas) {
        m_canvasDetails = details;
        details->setObjectName(QStringLiteral("environmentCanvasDefinition"));
    } else {
        m_listDetails = details;
        details->setObjectName(QStringLiteral("environmentListDefinition"));
    }
    layout->addWidget(details);

    auto *controls = new QPushButton;
    controls->setObjectName(QStringLiteral("environmentResourceControls"));
    m_resourceControls.append(controls);
    layout->addWidget(controls, 0, Qt::AlignLeft);
    connect(controls, &QPushButton::clicked, this, [this] {
        const auto definition = std::find_if(m_definitions.cbegin(), m_definitions.cend(),
            [this](const auto &entry) { return entry.path == m_selectedPath; });
        if (definition != m_definitions.cend() && !definition->runtimeContainerId.isEmpty()
            && runtimeState(*definition) == "running") {
            emit resourceControlsRequested(QStringLiteral("container"), definition->runtimeContainerId, {});
            return;
        }
        if (definition != m_definitions.cend() && !definition->runtimeRunId.isEmpty()) {
            emit resourceControlsRequested(QStringLiteral("vm"), definition->runtimeRunId, definition->runtimeWorkdir);
            return;
        }
        const auto resource = std::find_if(m_externalNodes.cbegin(), m_externalNodes.cend(),
            [this](const auto &node) { return node.id == m_selectedPath; });
        if (resource != m_externalNodes.cend())
            emit resourceControlsRequested(resource->kind,
                resource->kind == "vm" ? resource->id.mid(virtualMachinePrefix.size()) : resource->id, {});
    });

    auto *actions = new QHBoxLayout;
    auto *edit = new QPushButton(tr("Edit YAML"));
    edit->setObjectName(QStringLiteral("editEnvironmentDefinition"));
    m_edit.append(edit);
    actions->addWidget(edit);
    connect(edit, &QPushButton::clicked, this, &EnvironmentsPage::editDefinition);
    auto *apply = new QPushButton(tr("Apply"));
    apply->setObjectName(QStringLiteral("applyEnvironmentDefinition"));
    m_apply.append(apply);
    actions->addWidget(apply);
    connect(apply, &QPushButton::clicked, this, [this, apply] {
        const auto definition = std::find_if(m_definitions.cbegin(), m_definitions.cend(),
            [this](const auto &entry) { return entry.path == m_selectedPath; });
        if (definition != m_definitions.cend() && apply->isEnabled())
            emit applyRequested(*definition);
    });
    actions->addStretch();
    layout->addLayout(actions);
    return panel;
}

void EnvironmentsPage::setApplying(const QString &path, bool applying)
{
    if (applying)
        m_applying.insert(path);
    else
        m_applying.remove(path);

    selectDefinition(m_selectedPath);
}

void EnvironmentsPage::includeFiles()
{
    const auto selected = QFileDialog::getOpenFileNames(this, tr("Include environment definition files"),
        m_workdir, tr("Environment definitions (*.yaml *.yml)"));
    if (selected.isEmpty())
        return;

    auto files = m_files + selected;
    files.removeDuplicates();
    if (files.size() > maxExplicitFiles) {
        m_status->setText(tr("Choose at most 2,048 definition files."));
        m_status->show();
        return;
    }

    m_files = files;
    refresh();
}

void EnvironmentsPage::editDefinition()
{
    const QFileInfo file(m_selectedPath);
    if (!yamlDefinition(m_selectedPath) || !file.isFile() || file.isSymLink()
        || file.canonicalFilePath() != QDir::cleanPath(file.absoluteFilePath())) {
        m_status->setText(tr("This definition file is unavailable. Refresh the definitions and try again."));
        m_status->show();
        return;
    }

    if (!QDesktopServices::openUrl(QUrl::fromLocalFile(m_selectedPath))) {
        m_status->setText(tr("Could not open the selected environment definition."));
        m_status->show();
    }
}

void EnvironmentsPage::showDefinitions(const QVector<EnvironmentDefinition> &definitions)
{
    m_definitions = definitions;
    m_status->hide();

    m_ambiguousDefinitions.clear();
    QSet<QString> seen;
    for (const auto &definition : m_definitions) {
        if (definition.instanceId.isEmpty())
            continue;
        if (seen.contains(definition.instanceId))
            m_ambiguousDefinitions.insert(definition.instanceId);
        seen.insert(definition.instanceId);
    }
    renderDefinitions();
    emit containerTargetsChanged();
}

void EnvironmentsPage::renderDefinitions()
{
    const QSignalBlocker block(m_list);
    m_list->clear();
    QVector<ResourceNode> nodes;
    QVector<ResourceEdge> edges = m_machineEdges;
    nodes.reserve(m_definitions.size());
    QSet<QString> definitionInstances;

    for (const auto &definition : m_definitions) {
        if (!definition.instanceId.isEmpty())
            definitionInstances.insert(definition.instanceId);
        const QString name = definitionName(definition);
        const QString kind = definitionKind(definition);
        const QString source = definitionSource(definition);
        const QString status = definitionStatus(definition);
        const QString runtime = runtimeState(definition);
        const QString runtimeText = runtimeLabel(definition, runtime);
        auto *item = new QTreeWidgetItem(m_list, {name, kind, source, runtimeText.isEmpty() ? status : runtimeText});
        item->setData(0, Qt::UserRole, definition.path);
        item->setSizeHint(0, QSize(0, 46));

        ResourceNode node;
        node.id = definition.path;
        node.kind = definition.kind;
        node.title = name;
        node.detail = kind.isEmpty() ? source : tr("%1 · %2").arg(kind, source);
        node.state = runtimeText.isEmpty() ? status : runtimeText;
        node.column = definition.error.isEmpty() ? (definition.kind == "vm" ? 0 : 1) : 2;
        node.status = definition.error.isEmpty() ? ResourceStatus::Neutral : ResourceStatus::Warning;
        if (runtime == "running")
            node.status = ResourceStatus::Running;
        else if (runtime == "stopped")
            node.status = ResourceStatus::Inactive;
        if (!definition.runtimeState.isEmpty() && !runtime.isEmpty()) {
            node.column = 1;
            edges.append({localHostId, node.id, {}});
        }
        node.motionEligible = false;
        nodes.append(node);
    }
    m_externalNodes = m_machineNodes;
    for (const auto &entry : m_virtualMachineInventory) {
        const auto resource = entry.toObject();
        if (definitionInstances.contains(resource.value("instance_id").toString()))
            continue;

        ResourceNode node;
        node.id = virtualMachinePrefix + resource.value("id").toString();
        node.kind = QStringLiteral("vm");
        node.title = resource.value("title").toString();
        node.state = resource.value("state").toString();
        node.column = 1;
        node.motionEligible = false;
        if (node.state == "running")
            node.status = ResourceStatus::Running;
        else if (node.state == "stopped")
            node.status = ResourceStatus::Inactive;
        edges.append({localHostId, node.id, {}});
        m_externalNodes.append(node);
    }
    const bool hasHost = std::any_of(m_externalNodes.cbegin(), m_externalNodes.cend(),
        [](const auto &node) { return node.id == localHostId; });
    if (!hasHost && !edges.isEmpty()) {
        ResourceNode host;
        host.id = localHostId;
        host.kind = QStringLiteral("computer");
        host.title = tr("This machine");
        host.motionEligible = false;
        m_externalNodes.append(host);
    }
    for (const auto &node : m_externalNodes) {
        const auto location = node.kind == "machine" || node.kind == "broker" ? tr("Remote") : tr("Local");
        auto *item = new QTreeWidgetItem(m_list, {node.title, resourceKind(node), location, node.state});
        item->setData(0, Qt::UserRole, node.id);
        item->setSizeHint(0, QSize(0, 46));
        nodes.append(node);
    }
    const QString empty = tr("No environments yet. Choose Add Environment to get started.");
    m_canvas->setGraph(nodes, edges, nodes.isEmpty() ? empty : QString());
    selectDefinition(m_selectedPath);
}

void EnvironmentsPage::selectDefinition(const QString &path)
{
    const auto found = std::find_if(m_definitions.cbegin(), m_definitions.cend(),
        [&path](const auto &definition) { return definition.path == path; });
    const auto resource = std::find_if(m_externalNodes.cbegin(), m_externalNodes.cend(),
        [&path](const auto &node) { return node.id == path; });
    m_selectedPath = found == m_definitions.cend() && resource == m_externalNodes.cend() ? QString() : path;
    const bool hasDefinition = found != m_definitions.cend();
    const bool editable = hasDefinition && found->error != "missing" && yamlDefinition(m_selectedPath);
    const bool applicable = hasDefinition && found->error.isEmpty() && yamlDefinition(m_selectedPath)
        && !m_applying.contains(m_selectedPath) && runtimeState(*found) != "running";
    for (auto *button : m_edit) {
        button->setVisible(hasDefinition);
        button->setEnabled(editable);
    }
    for (auto *button : m_apply) {
        button->setVisible(hasDefinition);
        button->setEnabled(applicable);
    }
    const bool definitionConsole = m_virtualMachineControls && found != m_definitions.cend()
        && !found->runtimeRunId.isEmpty() && runtimeState(*found) == "running";
    const bool containerTerminal = hasDefinition && !found->runtimeContainerId.isEmpty()
        && runtimeState(*found) == "running";
    const bool resourceControls = resource != m_externalNodes.cend() && resource->kind != "broker"
        && (resource->kind != "vm" || m_virtualMachineControls);
    const bool resourceConsole = resourceControls && resource->kind == "vm" && resource->state == "running";
    for (auto *button : m_resourceControls) {
        button->setText(containerTerminal ? tr("Open terminal")
            : definitionConsole || resourceConsole ? tr("Open VM console")
            : resourceControls && resource->kind == "vm" ? tr("Manage VM")
            : resourceControls && resource->kind == "computer" ? tr("Connection settings")
            : tr("Machine settings"));
        button->setVisible(containerTerminal || definitionConsole || resourceControls);
        button->setEnabled(containerTerminal || definitionConsole || resourceControls);
    }

    const QSignalBlocker block(m_list);
    if (found != m_definitions.cend())
        m_list->setCurrentItem(m_list->topLevelItem(int(found - m_definitions.cbegin())));
    else if (resource != m_externalNodes.cend())
        m_list->setCurrentItem(m_list->topLevelItem(int(m_definitions.size() + (resource - m_externalNodes.cbegin()))));
    else
        m_list->setCurrentItem(nullptr);

    QString detail = m_definitions.isEmpty() && m_externalNodes.isEmpty()
        ? tr("Choose Add Environment to get started.")
        : tr("Select an environment to see its status and actions.");
    QString title = tr("Environment details");
    if (found != m_definitions.cend()) {
        title = definitionName(*found);
        QStringList lines{definitionStatus(*found), QDir::toNativeSeparators(found->path)};
        if (found->error.isEmpty()) {
            lines << tr("Type  %1").arg(definitionKind(*found));
            if (!found->container.isEmpty()) {
                lines << tr("Image  %1").arg(found->container.value("image").toString());
                lines << (found->container.value("network") == "none" ? tr("No network") : tr("Bridge network"));
                if (found->container.value("security") == "restricted")
                    lines << tr("Restricted capabilities");
                if (found->container.value("read_only").toBool())
                    lines << tr("Read-only root filesystem");
                const auto port = found->container.value("publish").toObject();
                if (!port.isEmpty())
                    lines << tr("Local port  %1 → %2").arg(port.value("host").toInt()).arg(port.value("container").toInt());
                const auto folder = found->container.value("data_path").toString();
                if (!folder.isEmpty())
                    lines << tr("Persistent volume  %1").arg(folder);
            } else if (!found->vm.isEmpty()) {
                const QHash<QString, QString> systems{
                    {"ubuntu", tr("Ubuntu")}, {"debian", tr("Debian")}, {"fedora", tr("Fedora")},
                    {"other-linux", tr("Other Linux")}, {"windows11", tr("Windows 11")},
                };
                const auto os = found->vm.value("os").toString();
                lines << tr("Operating system  %1").arg(systems.value(os, os))
                      << tr("%1 CPU(s) · %2 GiB RAM · %3 GiB disk").arg(found->vm.value("cpus").toInt())
                          .arg(found->vm.value("memory_gib").toInt()).arg(found->vm.value("disk_gib").toInt());
                const auto iso = found->vm.value("installer_iso").toString();
                if (!iso.isEmpty())
                    lines << tr("Installer ISO  %1").arg(QDir::toNativeSeparators(iso));
                else if (found->instanceId.isEmpty())
                    lines << tr("Choose an installer ISO before the first boot.");
            } else {
                lines << tr("Provisioning workflow  %1").arg(found->workflow);
            }
            const auto observed = runtimeLabel(*found, runtimeState(*found));
            if (!observed.isEmpty())
                lines << observed;
            if (!found->package.isEmpty())
                lines << tr("Package  %1").arg(found->package);
            if (!found->storageProfile.isEmpty())
                lines << tr("Storage profile  %1").arg(found->storageProfile);
        }
        detail = lines.join('\n');
    } else if (resource != m_externalNodes.cend()) {
        title = resource->title;
        detail = resourceDetails(*resource);
    }
    for (auto *label : m_inspectorTitles)
        label->setText(title);
    m_canvasDetails->setText(detail);
    m_listDetails->setText(detail);
}

QString EnvironmentsPage::resourceKind(const ResourceNode &node) const
{
    if (node.kind == "vm")
        return tr("Virtual machine");
    if (node.kind == "broker")
        return tr("Managing broker");
    if (node.kind == "machine")
        return tr("Remote machine");
    return tr("Local computer");
}

QString EnvironmentsPage::resourceDetails(const ResourceNode &node) const
{
    QStringList lines{resourceKind(node)};
    if (node.kind != "machine" || node.detail.isEmpty())
        lines << node.state;
    lines << node.detail;
    if (node.kind == "vm") {
        lines << tr("Host  This computer");
    } else if (node.kind == "machine") {
        bool hasReport = false;
        for (const auto &entry : m_machineCapabilities) {
            const auto catalog = entry.toObject();
            if (catalog.value("target").toObject().value("node").toString() != node.id)
                continue;

            hasReport = true;
            QStringList features;
            QStringList roots;
            const auto add = [&features](const QString &feature, bool granted) {
                if (granted && !features.contains(feature))
                    features.append(feature);
            };
            add(tr("Workflows"), catalog.value("workflow_only").toBool());
            add(tr("Containers"), catalog.value("docker_read").toBool());
            for (const auto &entry : catalog.value("roots").toArray()) {
                const auto root = entry.toObject();
                add(tr("Files"), root.value("files_read").toBool());
                add(tr("Git"), root.value("git_read").toBool());
                add(tr("Terminal"), root.value("terminal").toBool());
                const auto name = root.value("name").toString();
                if (!name.isEmpty())
                    roots.append(name);
            }
            if (!features.isEmpty())
                lines << tr("Reported access  %1").arg(features.join(tr(", ")));
            if (!roots.isEmpty())
                lines << tr("Workspace roots  %1").arg(roots.join(tr(", ")));
            const auto reported = QDateTime::fromString(catalog.value("received_at").toString(), Qt::ISODateWithMs);
            if (reported.isValid())
                lines << tr("Last report  %1").arg(QLocale().toString(reported.toLocalTime(), QLocale::ShortFormat));
            break;
        }
        if (!hasReport)
            lines << tr("No workspace feature report is available.");
    }
    lines.removeAll(QString());
    return lines.join('\n');
}

QString EnvironmentsPage::definitionStatus(const EnvironmentDefinition &definition) const
{
    if (definition.error == "missing")
        return tr("Definition file missing");
    if (definition.error == "name-mismatch")
        return tr("Name differs from filename");
    if (!definition.error.isEmpty())
        return tr("Invalid definition");
    return tr("Saved definition");
}

QString EnvironmentsPage::definitionKind(const EnvironmentDefinition &definition) const
{
    if (definition.kind == "vm")
        return tr("Virtual machine");
    if (definition.kind == "container")
        return tr("Container");
    return {};
}

QString EnvironmentsPage::definitionSource(const EnvironmentDefinition &definition) const
{
    return definition.scope == "personal" ? tr("Personal") : tr("Selected file");
}

QString EnvironmentsPage::runtimeState(const EnvironmentDefinition &definition) const
{
    if (definition.instanceId.isEmpty() || m_ambiguousDefinitions.contains(definition.instanceId)) {
        return {};
    }
    if (!definition.runtimeState.isEmpty())
        return definition.runtimeState == "unverified" ? QString() : definition.runtimeState;
    if (definition.kind != "vm" || remainingVmObservation(m_vmObservedAt) == 0) {
        return {};
    }

    const auto state = m_vmStates.value(definition.instanceId);
    return state == "running" || state == "stopped" ? state : QString();
}

QString EnvironmentsPage::runtimeLabel(const EnvironmentDefinition &definition, const QString &state) const
{
    if (state == "running")
        return tr("Observed running");
    if (state == "stopped")
        return tr("Observed stopped");
    return definition.kind == "vm" || !definition.instanceId.isEmpty() ? tr("Runtime unavailable") : QString();
}
