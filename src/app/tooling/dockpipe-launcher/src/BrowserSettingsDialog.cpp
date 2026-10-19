#include "BrowserSettingsDialog.h"
#include "BrowserPreferences.h"
#include "CanvasIcons.h"

#include <QApplication>
#include <QCheckBox>
#include <QComboBox>
#include <QDialogButtonBox>
#include <QDir>
#include <QEvent>
#include <QFileDialog>
#include <QFormLayout>
#include <QFrame>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QPointer>
#include <QPainter>
#include <QPushButton>
#include <QScrollArea>
#include <QTreeWidget>
#include <QUrl>
#include <QVBoxLayout>

namespace {

constexpr int MaximumSiteOrigins = 200;

bool darkTheme()
{
    return QApplication::palette().color(QPalette::Window).lightness() < 128;
}

class SettingsComboBox final : public QComboBox {
protected:
    void paintEvent(QPaintEvent *event) override
    {
        QComboBox::paintEvent(event);
        QPainter painter(this);
        CanvasIcons::icon(CanvasIcons::Symbol::ChevronDown)
            .paint(&painter, QRect(width() - 27, (height() - 16) / 2, 16, 16));
    }
};

class SettingsCheckBox final : public QCheckBox {
public:
    SettingsCheckBox()
    {
        setFixedSize(36, 30);
    }

protected:
    bool hitButton(const QPoint &position) const override
    {
        return rect().contains(position);
    }

    void paintEvent(QPaintEvent *) override
    {
        QPainter painter(this);
        painter.setRenderHint(QPainter::Antialiasing);
        const bool dark = darkTheme();
        const QColor border(dark ? "#7f8a99" : "#8993a1");
        painter.setPen(QPen(isChecked() ? QColor(dark ? "#75b5ec" : "#307bbd") : border, 1.5));
        painter.setBrush(isChecked() ? QColor(dark ? "#263c50" : "#e9f2fc")
                                    : QColor(dark ? "#191d24" : "#ffffff"));
        const QRect indicator(8, 5, 20, 20);
        painter.drawRoundedRect(indicator, 4, 4);
        if (isChecked())
            CanvasIcons::icon(CanvasIcons::Symbol::Check).paint(&painter, indicator.adjusted(2, 2, -2, -2));
        if (hasFocus()) {
            painter.setPen(QPen(QColor(dark ? "#75b5ec" : "#307bbd"), 1, Qt::DashLine));
            painter.setBrush(Qt::NoBrush);
            painter.drawRoundedRect(rect().adjusted(1, 1, -2, -2), 5, 5);
        }
    }
};

void applySettingsTheme(QDialog *dialog)
{
    const bool dark = darkTheme();
    // Match the existing pipeon theme surfaces instead of Fusion's fallback palette.
    const QString background = dark ? "#191d24" : "#f7f8fb";
    const QString surface = dark ? "#1d232d" : "#ffffff";
    const QString border = dark ? "#363f4c" : "#cdd4df";
    const QString muted = dark ? "#a8b9ce" : "#53657c";
    dialog->setStyleSheet(QStringLiteral(R"CSS(
        QLabel[browserSettingsHeading="true"] { font-size: 16px; font-weight: 600; padding-top: 10px; }
        QLabel[browserSettingsDescription="true"] { color: %4; font-size: 12px; }
        QWidget#browserSettingsBody, QScrollArea#browserSettingsScroll,
        QScrollArea#browserSettingsScroll > QWidget { background: %1; border: none; }
        QFrame[browserSettingsCard="true"] { background: %2; border: 1px solid %3; border-radius: 12px; }
        QFrame[browserSettingsDivider="true"] { border: none; border-top: 1px solid %3; max-height: 1px; }
        QTreeWidget#browserSettingsSiteRules { background: %2; border: none; }
        QComboBox { padding-right: 28px; }
        QComboBox::down-arrow { image: none; width: 0; height: 0; }
        QLabel#browserSettingsError { color: #e26c75; padding: 8px; }
    )CSS").arg(background, surface, border, muted));
}

class SettingsThemeWatcher final : public QObject {
public:
    explicit SettingsThemeWatcher(QDialog *dialog) : QObject(dialog), m_dialog(dialog)
    {
        dialog->installEventFilter(this);
    }

protected:
    bool eventFilter(QObject *, QEvent *event) override
    {
        if (event->type() == QEvent::ApplicationPaletteChange)
            applySettingsTheme(m_dialog);
        return false;
    }

private:
    QDialog *m_dialog;
};

QLabel *description(const QString &text)
{
    auto *label = new QLabel(text);
    label->setProperty("browserSettingsDescription", true);
    label->setTextFormat(Qt::PlainText);
    label->setWordWrap(true);
    return label;
}

QVBoxLayout *section(QVBoxLayout *layout, const QString &title)
{
    auto *heading = new QLabel(title);
    heading->setProperty("browserSettingsHeading", true);
    layout->addWidget(heading);
    auto *card = new QFrame;
    card->setProperty("browserSettingsCard", true);
    auto *content = new QVBoxLayout(card);
    content->setContentsMargins(18, 8, 18, 8);
    content->setSpacing(0);
    layout->addWidget(card);
    return content;
}

void addRow(QVBoxLayout *card, const QString &title, const QString &detail, QWidget *control)
{
    if (card->count() > 0) {
        auto *line = new QFrame;
        line->setFrameShape(QFrame::HLine);
        line->setProperty("browserSettingsDivider", true);
        card->addWidget(line);
    }
    auto *row = new QHBoxLayout;
    row->setContentsMargins(0, 12, 0, 12);
    row->setSpacing(20);
    auto *labels = new QVBoxLayout;
    labels->setSpacing(4);
    auto *label = new QLabel(title);
    label->setTextFormat(Qt::PlainText);
    label->setWordWrap(true);
    labels->addWidget(label);
    if (!detail.isEmpty())
        labels->addWidget(description(detail));
    row->addLayout(labels, 1);
    control->setAccessibleName(title);
    row->addWidget(control, 0, Qt::AlignVCenter);
    card->addLayout(row);
}

QComboBox *linkDestination(const QString &name)
{
    auto *combo = new SettingsComboBox;
    combo->setObjectName(name);
    combo->addItem(BrowserSettingsDialog::tr("Default browser"), QStringLiteral("system"));
    combo->addItem(BrowserSettingsDialog::tr("DockPipe browser"), QStringLiteral("dockpipe"));
    combo->setMinimumWidth(165);
    return combo;
}

QComboBox *permissionPolicy(const QString &name)
{
    auto *combo = new SettingsComboBox;
    combo->setObjectName(name);
    combo->addItem(BrowserSettingsDialog::tr("Ask"), QStringLiteral("ask"));
    combo->addItem(BrowserSettingsDialog::tr("Allow"), QStringLiteral("allow"));
    combo->addItem(BrowserSettingsDialog::tr("Block"), QStringLiteral("block"));
    combo->setMinimumWidth(165);
    return combo;
}

QString featureLabel(const QString &feature)
{
    if (feature == QStringLiteral("camera"))
        return BrowserSettingsDialog::tr("Camera");
    if (feature == QStringLiteral("microphone"))
        return BrowserSettingsDialog::tr("Microphone");
    if (feature == QStringLiteral("location"))
        return BrowserSettingsDialog::tr("Location");
    return BrowserSettingsDialog::tr("Notifications");
}

QString policyLabel(const QString &policy)
{
    if (policy == QStringLiteral("allow"))
        return BrowserSettingsDialog::tr("Allow");
    if (policy == QStringLiteral("block"))
        return BrowserSettingsDialog::tr("Block");
    return BrowserSettingsDialog::tr("Ask");
}

const QStringList PermissionFeatures = {
    QStringLiteral("camera"), QStringLiteral("microphone"),
    QStringLiteral("location"), QStringLiteral("notifications")
};

} // namespace

BrowserSettingsDialog::BrowserSettingsDialog(QWidget *parent) : QDialog(parent)
{
    setObjectName(QStringLiteral("browserSettingsDialog"));
    setWindowTitle(tr("Browser settings"));
    resize(780, 820);
    setMinimumSize(580, 440);
    applySettingsTheme(this);
    new SettingsThemeWatcher(this);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(22, 18, 22, 14);
    layout->addWidget(description(tr("Preferences save automatically. Browsing data and extensions stay in this app session.")));
    auto *scroll = new QScrollArea;
    scroll->setObjectName(QStringLiteral("browserSettingsScroll"));
    scroll->setWidgetResizable(true);
    scroll->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    auto *body = new QWidget;
    body->setObjectName(QStringLiteral("browserSettingsBody"));
    auto *sections = new QVBoxLayout(body);
    sections->setContentsMargins(0, 0, 10, 12);
    sections->setSpacing(10);

    auto *general = section(sections, tr("General"));
    m_webLinks = linkDestination(QStringLiteral("browserSettingsWebLinks"));
    addRow(general, tr("Web links"), tr("Choose where web links opened from DockPipe go."), m_webLinks);
    m_localLinks = linkDestination(QStringLiteral("browserSettingsLocalLinks"));
    addRow(general, tr("Local development sites"), tr("Where links to local development sites open."), m_localLinks);
    m_fullUrl = new SettingsCheckBox;
    m_fullUrl->setObjectName(QStringLiteral("browserSettingsFullUrl"));
    addRow(general, tr("Always show the full address"), tr("Include the protocol, path, query and fragment."), m_fullUrl);
    auto *history = new QPushButton(tr("View history"));
    history->setObjectName(QStringLiteral("browserSettingsHistory"));
    addRow(general, tr("History"), tr("Review pages visited in this browser session."), history);
    auto *clearData = new QPushButton(tr("Clear data…"));
    clearData->setObjectName(QStringLiteral("browserSettingsClearData"));
    addRow(general, tr("Browsing data"), tr("Manage cookies, website storage, cache and history."), clearData);

    auto *passwords = section(sections, tr("Passwords and autofill"));
    auto *passwordExtensions = new QPushButton(tr("Manage extensions"));
    addRow(passwords, tr("Your password manager"),
           tr("Use an installed password-manager extension for passwords and autofill."), passwordExtensions);
    auto *extensions = section(sections, tr("Extensions"));
    auto *manageExtensions = new QPushButton(tr("Manage extensions"));
    manageExtensions->setObjectName(QStringLiteral("browserSettingsExtensions"));
    addRow(extensions, tr("Browser extensions"), tr("View, load, enable or remove browser extensions."), manageExtensions);

    auto *downloads = section(sections, tr("Downloads"));
    auto *directoryRow = new QWidget;
    auto *directoryLayout = new QVBoxLayout(directoryRow);
    directoryLayout->setContentsMargins(0, 0, 0, 0);
    m_downloadDirectory = description({});
    m_downloadDirectory->setObjectName(QStringLiteral("browserSettingsDownloadDirectory"));
    m_downloadDirectory->setTextInteractionFlags(Qt::TextSelectableByMouse);
    directoryLayout->addWidget(m_downloadDirectory);
    auto *chooseDirectory = new QPushButton(tr("Change folder…"));
    chooseDirectory->setObjectName(QStringLiteral("browserSettingsChangeDownloadDirectory"));
    directoryLayout->addWidget(chooseDirectory, 0, Qt::AlignRight);
    directoryRow->setMaximumWidth(300);
    directoryRow->setMinimumWidth(260);
    addRow(downloads, tr("Save downloads to"), {}, directoryRow);
    m_askDownload = new SettingsCheckBox;
    m_askDownload->setObjectName(QStringLiteral("browserSettingsAskDownload"));
    addRow(downloads, tr("Ask where to save each file"), tr("Choose a destination before a download starts."), m_askDownload);
    auto *downloadHistory = new QPushButton(tr("View downloads"));
    downloadHistory->setObjectName(QStringLiteral("browserSettingsDownloads"));
    addRow(downloads, tr("Download history"), tr("View progress and files downloaded in this session."), downloadHistory);

    auto *permissions = section(sections, tr("Site permissions"));
    for (const QString &feature : PermissionFeatures) {
        auto *policy = permissionPolicy(QStringLiteral("browserSettingsPermission_") + feature);
        m_permissionDefaults.insert(feature, policy);
        addRow(permissions, featureLabel(feature), tr("Default for websites without a site rule."), policy);
        connect(policy, &QComboBox::currentIndexChanged, this, [this, feature, policy] {
            persist([feature, policy](BrowserSettings &settings) {
                settings.permissionDefaults[feature] = policy->currentData().toString();
            });
        });
    }
    auto *rules = section(sections, tr("Site rules"));
    rules->addWidget(description(tr("Rules apply only to the exact website origin, including its protocol and port.")));
    m_siteRules = new QTreeWidget;
    m_siteRules->setObjectName(QStringLiteral("browserSettingsSiteRules"));
    m_siteRules->setHeaderLabels({tr("Website"), tr("Permission"), tr("Behavior")});
    m_siteRules->setRootIsDecorated(false);
    m_siteRules->setSelectionMode(QAbstractItemView::SingleSelection);
    m_siteRules->setMinimumHeight(150);
    m_siteRules->setMaximumHeight(220);
    m_siteRules->setColumnWidth(0, 290);
    rules->addWidget(m_siteRules);
    auto *ruleActions = new QHBoxLayout;
    ruleActions->setContentsMargins(0, 10, 0, 6);
    auto *addRule = new QPushButton(tr("Add site rule…"));
    addRule->setObjectName(QStringLiteral("browserSettingsAddSiteRule"));
    m_editRule = new QPushButton(tr("Edit…"));
    m_editRule->setObjectName(QStringLiteral("browserSettingsEditSiteRule"));
    m_removeRule = new QPushButton(tr("Remove"));
    m_removeRule->setObjectName(QStringLiteral("browserSettingsRemoveSiteRule"));
    ruleActions->addWidget(addRule);
    ruleActions->addStretch();
    ruleActions->addWidget(m_editRule);
    ruleActions->addWidget(m_removeRule);
    rules->addLayout(ruleActions);

    auto *content = section(sections, tr("Content"));
    m_javascript = new SettingsCheckBox;
    m_javascript->setObjectName(QStringLiteral("browserSettingsJavascript"));
    addRow(content, tr("JavaScript"), tr("Allow websites to run JavaScript. Reload open pages after changing this."), m_javascript);
    m_images = new SettingsCheckBox;
    m_images->setObjectName(QStringLiteral("browserSettingsImages"));
    addRow(content, tr("Images"), tr("Load images on websites. Reload open pages after changing this."), m_images);
    sections->addStretch();
    scroll->setWidget(body);
    layout->addWidget(scroll, 1);
    m_error = new QLabel;
    m_error->setObjectName(QStringLiteral("browserSettingsError"));
    m_error->setTextFormat(Qt::PlainText);
    m_error->setWordWrap(true);
    m_error->hide();
    layout->addWidget(m_error);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Close);
    layout->addWidget(buttons);
    for (auto *button : findChildren<QPushButton *>()) {
        button->setAutoDefault(false);
        button->setDefault(false);
    }

    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::close);
    connect(history, &QPushButton::clicked, this, &BrowserSettingsDialog::historyRequested);
    connect(clearData, &QPushButton::clicked, this, &BrowserSettingsDialog::clearDataRequested);
    connect(passwordExtensions, &QPushButton::clicked, this, &BrowserSettingsDialog::extensionsRequested);
    connect(manageExtensions, &QPushButton::clicked, this, &BrowserSettingsDialog::extensionsRequested);
    connect(downloadHistory, &QPushButton::clicked, this, &BrowserSettingsDialog::downloadsRequested);
    connect(chooseDirectory, &QPushButton::clicked, this, &BrowserSettingsDialog::chooseDownloadDirectory);
    connect(m_webLinks, &QComboBox::currentIndexChanged, this, [this] {
        persist([this](BrowserSettings &settings) { settings.webLinkDestination = m_webLinks->currentData().toString(); });
    });
    connect(m_localLinks, &QComboBox::currentIndexChanged, this, [this] {
        persist([this](BrowserSettings &settings) { settings.localLinkDestination = m_localLinks->currentData().toString(); });
    });
    const auto bindToggle = [this](QCheckBox *toggle, bool BrowserSettings::*field) {
        connect(toggle, &QCheckBox::toggled, this, [this, field](bool value) {
            persist([field, value](BrowserSettings &settings) { settings.*field = value; });
        });
    };
    bindToggle(m_fullUrl, &BrowserSettings::showFullUrl);
    bindToggle(m_askDownload, &BrowserSettings::askWhereToSave);
    bindToggle(m_javascript, &BrowserSettings::javascript);
    bindToggle(m_images, &BrowserSettings::images);
    connect(addRule, &QPushButton::clicked, this, [this] { editSiteRule(false); });
    connect(m_editRule, &QPushButton::clicked, this, [this] { editSiteRule(true); });
    connect(m_removeRule, &QPushButton::clicked, this, &BrowserSettingsDialog::removeSiteRule);
    connect(m_siteRules, &QTreeWidget::itemDoubleClicked, this, [this] { editSiteRule(true); });
    connect(m_siteRules, &QTreeWidget::itemSelectionChanged, this, [this] {
        const bool selected = m_siteRules->currentItem() != nullptr;
        m_editRule->setEnabled(selected);
        m_removeRule->setEnabled(selected);
    });
    connect(BrowserPreferences::instance(), &BrowserPreferences::changed, this, &BrowserSettingsDialog::synchronize);
    synchronize();
}

void BrowserSettingsDialog::persist(const std::function<void(BrowserSettings &)> &change)
{
    if (m_synchronizing)
        return;
    BrowserSettings settings = BrowserPreferences::instance()->settings();
    change(settings);
    if (!BrowserPreferences::instance()->save(settings)) {
        m_error->setText(tr("Could not save browser settings. Your previous settings are still in use. Try again."));
        m_error->show();
        synchronize();
        return;
    }
    m_error->hide();
}

void BrowserSettingsDialog::synchronize()
{
    const BrowserSettings settings = BrowserPreferences::instance()->settings();
    m_synchronizing = true;
    m_webLinks->setCurrentIndex(m_webLinks->findData(settings.webLinkDestination));
    m_localLinks->setCurrentIndex(m_localLinks->findData(settings.localLinkDestination));
    m_fullUrl->setChecked(settings.showFullUrl);
    m_askDownload->setChecked(settings.askWhereToSave);
    m_javascript->setChecked(settings.javascript);
    m_images->setChecked(settings.images);
    m_downloadDirectory->setText(QDir::toNativeSeparators(settings.downloadDirectory));
    for (auto feature = m_permissionDefaults.cbegin(); feature != m_permissionDefaults.cend(); ++feature)
        feature.value()->setCurrentIndex(feature.value()->findData(settings.permissionDefaults.value(feature.key(), QStringLiteral("ask"))));
    QString selectedOrigin;
    QString selectedFeature;
    if (auto *selected = m_siteRules->currentItem()) {
        selectedOrigin = selected->data(0, Qt::UserRole).toString();
        selectedFeature = selected->data(1, Qt::UserRole).toString();
    }
    m_siteRules->clear();
    for (auto site = settings.sitePermissions.cbegin(); site != settings.sitePermissions.cend(); ++site) {
        for (auto feature = site.value().cbegin(); feature != site.value().cend(); ++feature) {
            auto *item = new QTreeWidgetItem(m_siteRules, {site.key(), featureLabel(feature.key()), policyLabel(feature.value())});
            item->setData(0, Qt::UserRole, site.key());
            item->setData(1, Qt::UserRole, feature.key());
            if (site.key() == selectedOrigin && feature.key() == selectedFeature)
                m_siteRules->setCurrentItem(item);
        }
    }
    const bool selected = m_siteRules->currentItem() != nullptr;
    m_editRule->setEnabled(selected);
    m_removeRule->setEnabled(selected);
    m_synchronizing = false;
}

void BrowserSettingsDialog::chooseDownloadDirectory()
{
    auto *dialog = new QFileDialog(this, tr("Choose download folder"), BrowserPreferences::instance()->settings().downloadDirectory);
    dialog->setAttribute(Qt::WA_DeleteOnClose);
    dialog->setFileMode(QFileDialog::Directory);
    dialog->setOption(QFileDialog::ShowDirsOnly);
    connect(dialog, &QFileDialog::fileSelected, this, [this](const QString &path) {
        persist([path](BrowserSettings &settings) { settings.downloadDirectory = path; });
    });
    dialog->open();
}

void BrowserSettingsDialog::editSiteRule(bool editSelected)
{
    QString originalOrigin;
    QString originalFeature;
    if (editSelected) {
        auto *selected = m_siteRules->currentItem();
        if (!selected)
            return;
        originalOrigin = selected->data(0, Qt::UserRole).toString();
        originalFeature = selected->data(1, Qt::UserRole).toString();
    }
    auto *dialog = new QDialog(this);
    dialog->setObjectName(QStringLiteral("browserSettingsSiteRuleEditor"));
    dialog->setAttribute(Qt::WA_DeleteOnClose);
    dialog->setWindowTitle(editSelected ? tr("Edit site rule") : tr("Add site rule"));
    dialog->resize(480, 260);
    auto *layout = new QVBoxLayout(dialog);
    auto *form = new QFormLayout;
    form->setFieldGrowthPolicy(QFormLayout::AllNonFixedFieldsGrow);
    form->setRowWrapPolicy(QFormLayout::WrapLongRows);
    auto *origin = new QLineEdit(originalOrigin);
    origin->setObjectName(QStringLiteral("browserSettingsRuleOrigin"));
    origin->setPlaceholderText(QStringLiteral("https://example.com"));
    origin->setMaxLength(2048);
    auto *feature = new SettingsComboBox;
    feature->setObjectName(QStringLiteral("browserSettingsRuleFeature"));
    for (const QString &key : PermissionFeatures)
        feature->addItem(featureLabel(key), key);
    if (!originalFeature.isEmpty())
        feature->setCurrentIndex(feature->findData(originalFeature));
    auto *policy = permissionPolicy(QStringLiteral("browserSettingsRulePolicy"));
    if (editSelected) {
        const auto settings = BrowserPreferences::instance()->settings();
        policy->setCurrentIndex(policy->findData(settings.sitePermissions.value(originalOrigin).value(originalFeature)));
    }
    form->addRow(tr("Website origin"), origin);
    form->addRow(tr("Permission"), feature);
    form->addRow(tr("Behavior"), policy);
    layout->addLayout(form);
    layout->addWidget(description(tr("Enter a website origin without a page path, query, fragment or sign-in information.")));
    auto *error = new QLabel;
    error->setObjectName(QStringLiteral("browserSettingsRuleError"));
    error->setWordWrap(true);
    error->setTextFormat(Qt::PlainText);
    layout->addWidget(error);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Save | QDialogButtonBox::Cancel);
    layout->addWidget(buttons);
    connect(buttons, &QDialogButtonBox::rejected, dialog, &QDialog::reject);
    connect(buttons, &QDialogButtonBox::accepted, this,
            [this, dialog, origin, feature, policy, error, originalOrigin, originalFeature] {
        const QUrl url(origin->text().trimmed());
        const QString key = BrowserPreferences::originKey(url);
        if (key.isEmpty() || !url.userInfo().isEmpty() || url.hasQuery() || url.hasFragment()
            || (!url.path().isEmpty() && url.path() != QStringLiteral("/"))) {
            error->setText(tr("Enter an HTTP or HTTPS origin, such as https://example.com:8443."));
            return;
        }
        auto settings = BrowserPreferences::instance()->settings();
        if (!originalOrigin.isEmpty()) {
            settings.sitePermissions[originalOrigin].remove(originalFeature);
            if (settings.sitePermissions.value(originalOrigin).isEmpty())
                settings.sitePermissions.remove(originalOrigin);
        }
        if (!settings.sitePermissions.contains(key) && settings.sitePermissions.size() >= MaximumSiteOrigins) {
            error->setText(tr("Remove an existing site before adding another. Up to 200 sites are supported."));
            return;
        }
        settings.sitePermissions[key][feature->currentData().toString()] = policy->currentData().toString();
        if (!BrowserPreferences::instance()->save(settings)) {
            error->setText(tr("Could not save this site rule. Try again."));
            return;
        }
        dialog->accept();
    });
    dialog->open();
}

void BrowserSettingsDialog::removeSiteRule()
{
    auto *selected = m_siteRules->currentItem();
    if (!selected)
        return;
    const QString origin = selected->data(0, Qt::UserRole).toString();
    const QString feature = selected->data(1, Qt::UserRole).toString();
    persist([origin, feature](BrowserSettings &settings) {
        settings.sitePermissions[origin].remove(feature);
        if (settings.sitePermissions.value(origin).isEmpty())
            settings.sitePermissions.remove(origin);
    });
}
