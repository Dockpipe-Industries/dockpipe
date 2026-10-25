#include "DashboardSettingsDialog.h"
#include "DesktopWindowWidget.h"

#include <QCheckBox>
#include <QComboBox>
#include <QDialogButtonBox>
#include <QFileDialog>
#include <QFormLayout>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QVBoxLayout>

DashboardSettingsDialog::DashboardSettingsDialog(const DashboardOptions &options, QWidget *parent)
    : QDialog(parent)
{
    setObjectName("dashboardSettingsDialog");
    setWindowTitle(tr("Dashboard settings"));
    resize(560, 430);
    auto *layout = new QVBoxLayout(this);
    auto *description = new QLabel(tr("Canvas is your independent workspace. Desktop applications stay attached in Tiling while you use Canvas. When app management is enabled, closing a panel asks that OS window to close. Quitting DockPipe releases the apps to the desktop."));
    description->setWordWrap(true);
    layout->addWidget(description);
    m_compact = new QCheckBox(tr("Compact tiling — no gaps, borders, or extra widget footers"));
    m_compact->setObjectName("dashboardSettingsCompact");
    m_compact->setChecked(options.compactTiling);
    layout->addWidget(m_compact);
    m_autoHideDock = new QCheckBox(tr("Automatically hide the dock"));
    m_autoHideDock->setObjectName("dashboardSettingsAutoHideDock");
    m_autoHideDock->setToolTip(tr("Reveal it at the bottom edge or press Ctrl+Shift+D. Turn off to keep it visible below the workspace."));
    m_autoHideDock->setChecked(options.autoHideDock);
    layout->addWidget(m_autoHideDock);
    m_headers = new QCheckBox(tr("Show title bars on canvas and floating panels"));
    m_headers->setObjectName("dashboardSettingsHeaders");
    m_headers->setChecked(options.showPanelHeaders);
    layout->addWidget(m_headers);
    m_acquire = new QCheckBox(tr("Bring open desktop apps into Tiling when I enter it"));
    m_acquire->setObjectName("dashboardSettingsAutoAcquire");
    m_acquire->setChecked(options.autoAcquireApps);
    layout->addWidget(m_acquire);
    const QString unavailable = DesktopWindowWidget::automaticAcquisitionUnavailableReason();
    if (!unavailable.isEmpty()) {
        m_acquire->setEnabled(false);
        auto *note = new QLabel(unavailable);
        note->setWordWrap(true);
        layout->addWidget(note);
    }
    auto *form = new QFormLayout;
    m_background = new QComboBox;
    m_background->setObjectName("dashboardSettingsBackground");
    m_background->addItem(tr("Grid"), "grid");
    m_background->addItem(tr("Desktop background"), "desktop");
    m_background->addItem(tr("Plain"), "plain");
    m_background->addItem(tr("Dusk"), "dusk");
    m_background->addItem(tr("Ocean"), "ocean");
    m_background->addItem(tr("Background image"), "image");
    m_background->setCurrentIndex(m_background->findData(options.canvasBackground));
    form->addRow(tr("Dashboard background"), m_background);
    auto *imageRow = new QWidget;
    auto *imageLayout = new QHBoxLayout(imageRow);
    imageLayout->setContentsMargins(0, 0, 0, 0);
    m_image = new QLineEdit(options.canvasImagePath);
    m_image->setObjectName("dashboardSettingsImage");
    m_image->setMaxLength(4096);
    m_image->setPlaceholderText(tr("Choose a wallpaper or image"));
    imageLayout->addWidget(m_image, 1);
    auto *browse = new QPushButton(tr("Browse…"));
    imageLayout->addWidget(browse);
    connect(browse, &QPushButton::clicked, this, [this]() {
        const QString path = QFileDialog::getOpenFileName(this, tr("Dashboard background"), m_image->text(),
            tr("Images (*.png *.jpg *.jpeg *.webp *.bmp)"));
        if (!path.isEmpty())
            m_image->setText(path);
    });
    const auto updateImageRow = [this, imageRow]() {
        imageRow->setEnabled(m_background->currentData() == "image");
    };
    connect(m_background, &QComboBox::currentIndexChanged, imageRow, updateImageRow);
    updateImageRow();
    form->addRow(tr("Image"), imageRow);
    m_canvasFiles = new QComboBox;
    m_canvasFiles->setObjectName("dashboardSettingsCanvasFiles");
    m_tilingFiles = new QComboBox;
    m_tilingFiles->setObjectName("dashboardSettingsTilingFiles");
    for (auto *scope : {m_canvasFiles, m_tilingFiles}) {
        scope->addItem(tr("Selected project"), "project");
        scope->addItem(tr("Full system — start in Home"), "system");
    }
    m_canvasFiles->setCurrentIndex(m_canvasFiles->findData(options.canvasFileScope));
    m_tilingFiles->setCurrentIndex(m_tilingFiles->findData(options.tilingFileScope));
    form->addRow(tr("Canvas file manager"), m_canvasFiles);
    form->addRow(tr("Tiling file manager"), m_tilingFiles);
    layout->addLayout(form);
    auto *backgroundNote = new QLabel(tr("Desktop background uses your wallpaper image when available and refreshes when you return to either workspace. Your previous background stays if it cannot be loaded."));
    backgroundNote->setWordWrap(true);
    layout->addWidget(backgroundNote);
    auto *tour = new QPushButton(tr("Replay dashboard tour"));
    tour->setObjectName("dashboardReplayTour");
    connect(tour, &QPushButton::clicked, this, [this]() {
        reject();
        emit tourRequested();
    });
    layout->addWidget(tour);
    layout->addStretch();
    auto *actions = new QHBoxLayout;
    auto *rename = new QPushButton(tr("Rename dashboard…"));
    auto *remove = new QPushButton(tr("Remove dashboard…"));
    rename->setObjectName("dashboardRenameButton");
    remove->setObjectName("dashboardRemoveButton");
    connect(rename, &QPushButton::clicked, this, [this]() {
        reject();
        emit renameRequested();
    });
    connect(remove, &QPushButton::clicked, this, [this]() {
        reject();
        emit removeRequested();
    });
    actions->addWidget(rename);
    actions->addWidget(remove);
    actions->addStretch();
    layout->addLayout(actions);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Save | QDialogButtonBox::Cancel);
    connect(buttons, &QDialogButtonBox::accepted, this, &QDialog::accept);
    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
    layout->addWidget(buttons);
}

DashboardOptions DashboardSettingsDialog::selectedOptions() const
{
    DashboardOptions options;
    options.autoHideDock = m_autoHideDock->isChecked();
    options.compactTiling = m_compact->isChecked();
    options.showPanelHeaders = m_headers->isChecked();
    options.autoAcquireApps = m_acquire->isChecked();
    options.canvasBackground = m_background->currentData().toString();
    options.canvasImagePath = m_image->text();
    options.canvasFileScope = m_canvasFiles->currentData().toString();
    options.tilingFileScope = m_tilingFiles->currentData().toString();
    return options;
}
