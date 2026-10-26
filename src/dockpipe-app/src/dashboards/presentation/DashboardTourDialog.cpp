#include "DashboardTourDialog.h"
#include "LauncherSettings.h"
#include "CanvasIcons.h"
#include "DesktopWindowWidget.h"

#include <QCheckBox>
#include <QComboBox>
#include <QFormLayout>
#include <QHBoxLayout>
#include <QLabel>
#include <QPushButton>
#include <QRadioButton>
#include <QStackedWidget>
#include <QVBoxLayout>

namespace {
QLabel *description(const QString &text)
{
    auto *label = new QLabel(text);
    label->setTextFormat(Qt::PlainText);
    label->setWordWrap(true);
    return label;
}

QWidget *tourPage(QStackedWidget *pages, CanvasIcons::Symbol symbol, const QString &title,
                  const QString &text, QVBoxLayout *&layout)
{
    auto *page = new QWidget;
    layout = new QVBoxLayout(page);
    layout->setContentsMargins(0, 8, 0, 8);
    layout->setSpacing(16);
    auto *heading = new QHBoxLayout;
    auto *icon = new QLabel;
    icon->setPixmap(CanvasIcons::icon(symbol).pixmap(28, 28));
    heading->addWidget(icon);
    auto *label = description(title);
    QFont font = label->font();
    font.setPointSize(font.pointSize() + 3);
    font.setBold(true);
    label->setFont(font);
    heading->addWidget(label, 1);
    layout->addLayout(heading);
    layout->addWidget(description(text));
    pages->addWidget(page);
    return page;
}
}

DashboardTourDialog::DashboardTourDialog(const DashboardOptions &options, QWidget *parent)
    : QDialog(parent), m_options(options)
{
    const bool tilingEnabled = LauncherSettings::current().tilingEnabled;
    setObjectName("dashboardTourDialog");
    setWindowTitle(tr("Make this dashboard yours"));
    resize(580, 470);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(24, 20, 24, 20);
    layout->setSpacing(16);
    m_step = new QLabel;
    m_step->setObjectName("dashboardTourStep");
    layout->addWidget(m_step);
    m_pages = new QStackedWidget;
    m_pages->setObjectName("dashboardTourPages");
    layout->addWidget(m_pages, 1);

    QVBoxLayout *page = nullptr;
    tourPage(m_pages, CanvasIcons::Symbol::Apps, tr("Two independent workspaces"),
        tr("Canvas lets you arrange and overlap widgets. Tiling keeps its own widgets in splits and tabs. Switching between them preserves each workspace."), page);
    page->addWidget(description(tr("Use Add widget to get started. The dock opens tools and brings existing panels forward.")));
    m_canvas = new QRadioButton(tr("Start in Canvas"));
    m_canvas->setObjectName("dashboardTourCanvas");
    m_canvas->setChecked(true);
    m_tiling = new QRadioButton(tr("Start in Tiling · Experimental"));
    m_tiling->setObjectName("dashboardTourTiling");
    m_tiling->setEnabled(tilingEnabled);
    m_tiling->setToolTip(tr("Enable tiling mode in Settings to try this experimental workspace."));
    page->addWidget(m_canvas);
    page->addWidget(m_tiling);
    page->addStretch();

    tourPage(m_pages, CanvasIcons::Symbol::Settings, tr("Choose your look"),
        tr("Keep your current appearance or choose a starting point. You can change these choices later using the dashboard settings cog."), page);
    m_compact = new QCheckBox(tr("Compact tiling"));
    m_compact->setObjectName("dashboardTourCompact");
    m_compact->setChecked(options.compactTiling);
    m_compact->setEnabled(tilingEnabled);
    page->addWidget(m_compact);
    page->addWidget(description(tr("Compact tiling uses smaller gaps and removes extra widget footers.")));
    auto *form = new QFormLayout;
    m_background = new QComboBox;
    m_background->setObjectName("dashboardTourBackground");
    m_background->setAccessibleName(tr("Dashboard background"));
    m_background->addItem(tr("Keep current"), QString());
    m_background->addItem(tr("Grid"), "grid");
    m_background->addItem(tr("Desktop background"), "desktop");
    m_background->addItem(tr("Dusk"), "dusk");
    m_background->addItem(tr("Ocean"), "ocean");
    form->addRow(tr("Dashboard background"), m_background);
    page->addLayout(form);
    page->addWidget(description(tr("Desktop background uses your system wallpaper where supported. If it is unavailable, your existing background is kept.")));
    page->addStretch();

    tourPage(m_pages, CanvasIcons::Symbol::Monitor, tr("Desktop apps, when you want them"),
        tr("Optionally bring open desktop applications into Tiling. They stay attached and hidden while you use Canvas or another page. When app management is enabled, closing a panel asks that OS window to close. Quitting DockPipe releases the apps to the desktop. Canvas never acquires desktop apps."), page);
    m_acquire = new QCheckBox(tr("Bring open desktop apps into Tiling when I enter it"));
    m_acquire->setObjectName("dashboardTourAutoAcquire");
    m_acquire->setChecked(options.autoAcquireApps);
    page->addWidget(m_acquire);
    const QString unavailable = DesktopWindowWidget::automaticAcquisitionUnavailableReason();
    auto *availability = description(unavailable);
    availability->setObjectName("dashboardTourAvailability");
    availability->setVisible(!unavailable.isEmpty());
    page->addWidget(availability);
    m_acquire->setEnabled(tilingEnabled && unavailable.isEmpty());
    page->addWidget(description(tr("This is optional. Finish applies your choices; Dismiss leaves the dashboard unchanged.")));
    page->addStretch();

    m_doNotShowAgain = new QCheckBox(tr("Do not show this tour again"));
    m_doNotShowAgain->setObjectName("dashboardTourDoNotShowAgain");
    layout->addWidget(m_doNotShowAgain);
    auto *buttons = new QHBoxLayout;
    auto *dismiss = new QPushButton(tr("Dismiss"));
    dismiss->setObjectName("dashboardTourDismiss");
    dismiss->setAutoDefault(false);
    buttons->addWidget(dismiss);
    buttons->addStretch();
    m_back = new QPushButton(tr("Back"));
    m_back->setObjectName("dashboardTourBack");
    m_back->setAutoDefault(false);
    buttons->addWidget(m_back);
    m_next = new QPushButton(tr("Next"));
    m_next->setObjectName("dashboardTourNext");
    buttons->addWidget(m_next);
    m_finish = new QPushButton(tr("Finish"));
    m_finish->setObjectName("dashboardTourFinish");
    buttons->addWidget(m_finish);
    layout->addLayout(buttons);
    connect(dismiss, &QPushButton::clicked, this, &QDialog::reject);
    connect(m_back, &QPushButton::clicked, this, [this]() { showPage(m_pages->currentIndex() - 1); });
    connect(m_next, &QPushButton::clicked, this, [this]() { showPage(m_pages->currentIndex() + 1); });
    connect(m_finish, &QPushButton::clicked, this, &QDialog::accept);
    showPage(0);
}

void DashboardTourDialog::showPage(int index)
{
    if (index < 0 || index >= m_pages->count())
        return;
    m_pages->setCurrentIndex(index);
    m_step->setText(tr("Step %1 of %2").arg(index + 1).arg(m_pages->count()));
    m_back->setEnabled(index > 0);
    const bool last = index == m_pages->count() - 1;
    m_next->setVisible(!last);
    m_next->setDefault(!last);
    m_finish->setVisible(last);
    m_finish->setDefault(last);
    (last ? m_finish : m_next)->setFocus();
}

DashboardOptions DashboardTourDialog::selectedOptions() const
{
    DashboardOptions options = m_options;
    options.compactTiling = m_compact->isChecked();
    options.autoAcquireApps = m_acquire->isChecked();
    const QString background = m_background->currentData().toString();
    if (!background.isEmpty()) {
        options.canvasBackground = background;
        options.canvasImagePath.clear();
    }
    return options;
}

QString DashboardTourDialog::initialEnvironment() const
{
    return m_tiling->isEnabled() && m_tiling->isChecked() ? QStringLiteral("tiling") : QStringLiteral("canvas");
}

bool DashboardTourDialog::doNotShowAgain() const
{
    return m_doNotShowAgain->isChecked();
}

void DashboardTourDialog::setDoNotShowAgain(bool suppressed)
{
    m_doNotShowAgain->setChecked(suppressed);
}
