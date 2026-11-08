#include "TerminalSettingsDialog.h"

#include <QCheckBox>
#include <QComboBox>
#include <QDialogButtonBox>
#include <QFontComboBox>
#include <QFormLayout>
#include <QFrame>
#include <QTabWidget>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QPalette>
#include <QPlainTextEdit>
#include <QScrollArea>
#include <QSpinBox>
#include <QSlider>
#include <QVBoxLayout>

namespace {
QSpinBox *numberControl(const QString &name, int minimum, int maximum, int value)
{
    auto *control = new QSpinBox;
    control->setObjectName(name);
    control->setButtonSymbols(QAbstractSpinBox::NoButtons);
    control->setRange(minimum, maximum);
    control->setValue(value);
    return control;
}

QLabel *description(const QString &text)
{
    auto *label = new QLabel(text);
    label->setTextFormat(Qt::PlainText);
    label->setWordWrap(true);
    label->setObjectName("settingsDescription");
    return label;
}

QVBoxLayout *settingsPage(QTabWidget *tabs, const QString &title)
{
    auto *scroll = new QScrollArea;
    scroll->setObjectName("settingsScroll");
    scroll->setWidgetResizable(true);
    scroll->setFrameShape(QFrame::NoFrame);
    scroll->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    auto *body = new QWidget;
    body->setObjectName("settingsBody");
    auto *layout = new QVBoxLayout(body);
    layout->setContentsMargins(0, 16, 4, 0);
    layout->setSpacing(12);
    scroll->setWidget(body);
    tabs->addTab(scroll, title);
    return layout;
}

QFormLayout *settingsSection(QVBoxLayout *page, const QString &title)
{
    auto *card = new QFrame;
    card->setObjectName("settingsCard");
    auto *layout = new QVBoxLayout(card);
    layout->setContentsMargins(16, 12, 16, 12);
    layout->setSpacing(12);
    auto *heading = new QLabel(title);
    heading->setObjectName("settingsSectionTitle");
    layout->addWidget(heading);
    auto *form = new QFormLayout;
    form->setFieldGrowthPolicy(QFormLayout::AllNonFixedFieldsGrow);
    form->setLabelAlignment(Qt::AlignLeft | Qt::AlignVCenter);
    form->setHorizontalSpacing(24);
    form->setVerticalSpacing(8);
    layout->addLayout(form);
    page->addWidget(card);
    return form;
}

QWidget *sliderControl(QSlider *&slider, const QString &name, const QString &title,
                       int maximum, int value, const QString &suffix)
{
    auto *row = new QWidget;
    auto *layout = new QHBoxLayout(row);
    layout->setContentsMargins(0, 0, 0, 0);
    slider = new QSlider(Qt::Horizontal);
    slider->setObjectName(name);
    slider->setAccessibleName(title);
    slider->setRange(0, maximum);
    slider->setPageStep(qMax(1, maximum / 10));
    slider->setValue(value);
    layout->addWidget(slider, 1);
    auto *readout = new QLabel(QString::number(value) + suffix);
    readout->setObjectName(name + "Value");
    readout->setMinimumWidth(readout->fontMetrics().horizontalAdvance(QString::number(maximum) + suffix));
    readout->setAlignment(Qt::AlignRight | Qt::AlignVCenter);
    layout->addWidget(readout);
    QObject::connect(slider, &QSlider::valueChanged, readout, [readout, suffix](int current) {
        readout->setText(QString::number(current) + suffix);
    });
    return row;
}
}

TerminalSettingsDialog::TerminalSettingsDialog(QWidget *parent) : QDialog(parent)
{
    setObjectName("terminalSettingsDialog");
    setWindowTitle(tr("Terminal settings"));
    resize(640, 750);
    const TerminalSettings settings = TerminalPreferences::instance()->settings();
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(24, 16, 24, 24);
    layout->setSpacing(14);
    layout->addWidget(description(tr("Make every terminal feel like yours. Saved changes apply to all terminal views.")));
    auto *tabs = new QTabWidget;
    tabs->setObjectName("settingsTabs");
    auto *appearance = settingsPage(tabs, tr("Appearance"));
    auto *effects = settingsPage(tabs, tr("Background && effects"));
    auto *typography = settingsSection(appearance, tr("Theme & typography"));
    auto *behavior = settingsSection(appearance, tr("Cursor & history"));
    auto *background = settingsSection(effects, tr("Background"));
    auto *decoration = settingsSection(effects, tr("Text effects"));
    m_theme = new QComboBox;
    m_theme->setObjectName("terminalSettingsTheme");
    const QStringList names = {tr("Dockpipe"), tr("Amber"), tr("Green"), tr("Paper"), tr("Midnight")};
    const QStringList ids = TerminalPreferences::themeIds();
    for (int index = 0; index < ids.size(); ++index)
        m_theme->addItem(names[index], ids[index]);
    m_theme->setCurrentIndex(m_theme->findData(settings.theme));
    typography->addRow(tr("Theme"), m_theme);
    m_font = new QFontComboBox;
    m_font->setObjectName("terminalSettingsFont");
    m_font->setFontFilters(QFontComboBox::MonospacedFonts);
    m_font->setEditable(true);
    m_font->lineEdit()->setMaxLength(128);
    m_font->setCurrentText(settings.fontFamily);
    typography->addRow(tr("Monospace font"), m_font);
    m_fontSize = numberControl("terminalSettingsFontSize", 9, 28, settings.fontSize);
    m_fontSize->setSuffix(tr(" px"));
    typography->addRow(tr("Font size"), m_fontSize);
    m_cursorStyle = new QComboBox;
    m_cursorStyle->setObjectName("terminalSettingsCursorStyle");
    m_cursorStyle->addItem(tr("Block"), "block");
    m_cursorStyle->addItem(tr("Underline"), "underline");
    m_cursorStyle->addItem(tr("Bar"), "bar");
    m_cursorStyle->setCurrentIndex(m_cursorStyle->findData(settings.cursorStyle));
    behavior->addRow(tr("Cursor style"), m_cursorStyle);
    m_cursorBlink = new QCheckBox(tr("Blink cursor"));
    m_cursorBlink->setObjectName("terminalSettingsCursorBlink");
    m_cursorBlink->setChecked(settings.cursorBlink);
    behavior->addRow(QString(), m_cursorBlink);
    m_scrollback = numberControl("terminalSettingsScrollback", 100, 10000, settings.scrollback);
    m_scrollback->setSuffix(tr(" lines"));
    behavior->addRow(tr("Scrollback"), m_scrollback);
    m_opacity = numberControl("terminalSettingsOpacity", 30, 100, settings.opacity);
    m_opacity->setSuffix("%");
    background->addRow(tr("Opacity"), m_opacity);
    m_backgroundTarget = new QComboBox;
    m_backgroundTarget->setObjectName("terminalSettingsBackgroundTarget");
    m_backgroundTarget->setAccessibleName(tr("Transparency reveals"));
    m_backgroundTarget->addItem(tr("Panel background"), "panel");
    m_backgroundTarget->addItem(tr("Dockpipe canvas"), "canvas");
    m_backgroundTarget->addItem(tr("Desktop behind Dockpipe"), "desktop");
    m_backgroundTarget->setCurrentIndex(m_backgroundTarget->findData(settings.backgroundTarget));
    background->addRow(tr("Transparency reveals"), m_backgroundTarget);
    m_scanlines = new QCheckBox(tr("Static scanlines"));
    m_scanlines->setObjectName("terminalSettingsScanlines");
    m_scanlines->setChecked(settings.scanlines);
    decoration->addRow(QString(), m_scanlines);
    m_glow = new QCheckBox(tr("Text glow"));
    m_glow->setObjectName("terminalSettingsGlow");
    m_glow->setChecked(settings.glow);
    decoration->addRow(QString(), m_glow);
    auto *strengthRow = sliderControl(m_glowStrength, "terminalSettingsGlowStrength", tr("Glow strength"),
        100, settings.glowStrength, tr("%"));
    auto *spreadRow = sliderControl(m_glowSpread, "terminalSettingsGlowSpread", tr("Glow spread"),
        8, settings.glowSpread, tr(" px"));
    decoration->addRow(tr("Glow strength"), strengthRow);
    decoration->addRow(tr("Glow spread"), spreadRow);
    strengthRow->setEnabled(settings.glow);
    spreadRow->setEnabled(settings.glow);
    connect(m_glow, &QCheckBox::toggled, strengthRow, &QWidget::setEnabled);
    connect(m_glow, &QCheckBox::toggled, spreadRow, &QWidget::setEnabled);
    for (auto *form : {typography, behavior, background, decoration}) {
        for (int row = 0; row < form->rowCount(); ++row) {
            auto *item = form->itemAt(row, QFormLayout::LabelRole);
            if (item && item->widget())
                item->widget()->setMinimumWidth(144);
        }
    }
    auto *transparencyNote = description(tr("Transparency requires a restart and uses software rendering for terminals and browser views. WebGL sites may not work. Return to 100% opacity and restart to restore acceleration."));
    transparencyNote->setObjectName("settingsNotice");
    effects->addWidget(transparencyNote);
    auto updateTransparency = [this, transparencyNote]() {
        const bool transparent = m_opacity->value() < 100;
        m_backgroundTarget->setEnabled(transparent);
        transparencyNote->setVisible(transparent);
    };
    connect(m_opacity, &QSpinBox::valueChanged, this, updateTransparency);
    updateTransparency();
    appearance->addStretch();
    effects->addStretch();
    layout->addWidget(tabs, 1);
    auto *previewHeading = new QLabel(tr("PREVIEW"));
    previewHeading->setObjectName("settingsEyebrow");
    layout->addWidget(previewHeading);
    m_preview = new QPlainTextEdit;
    m_preview->setObjectName("terminalSettingsPreview");
    m_preview->setAccessibleName(tr("Terminal font and color preview"));
    m_preview->setReadOnly(true);
    m_preview->setFixedHeight(96);
    m_preview->setPlainText(tr("$ dockpipe --version\nDockpipe terminal\n\n$ Ready for your next command"));
    layout->addWidget(m_preview);
    m_preview->setToolTip(tr("Font, colors and approximate opacity. Cursor and text effects apply after saving."));
    m_error = description(QString());
    m_error->setObjectName("terminalSettingsError");
    m_error->hide();
    layout->addWidget(m_error);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Save | QDialogButtonBox::Cancel);
    buttons->setObjectName("terminalSettingsButtons");
    connect(buttons, &QDialogButtonBox::accepted, this, &TerminalSettingsDialog::save);
    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
    layout->addWidget(buttons);
    connect(m_theme, &QComboBox::currentIndexChanged, this, &TerminalSettingsDialog::updatePreview);
    connect(m_font, &QComboBox::currentTextChanged, this, &TerminalSettingsDialog::updatePreview);
    connect(m_fontSize, &QSpinBox::valueChanged, this, &TerminalSettingsDialog::updatePreview);
    connect(m_opacity, &QSpinBox::valueChanged, this, &TerminalSettingsDialog::updatePreview);
    updatePreview();
}

TerminalSettings TerminalSettingsDialog::selectedSettings() const
{
    TerminalSettings settings;
    settings.theme = m_theme->currentData().toString();
    settings.fontFamily = m_font->currentText().trimmed();
    settings.fontSize = m_fontSize->value();
    settings.cursorStyle = m_cursorStyle->currentData().toString();
    settings.cursorBlink = m_cursorBlink->isChecked();
    settings.scrollback = m_scrollback->value();
    settings.opacity = m_opacity->value();
    settings.backgroundTarget = m_backgroundTarget->currentData().toString();
    settings.scanlines = m_scanlines->isChecked();
    settings.glow = m_glow->isChecked();
    settings.glowStrength = m_glowStrength->value();
    settings.glowSpread = m_glowSpread->value();
    return settings;
}

void TerminalSettingsDialog::updatePreview()
{
    const TerminalSettings settings = selectedSettings();
    const QJsonObject theme = TerminalPreferences::theme(settings.theme);
    QColor background(theme.value("background").toString());
    const QColor foreground(theme.value("foreground").toString());
    const QColor canvas = palette().color(QPalette::Window);
    const qreal opacity = settings.opacity / 100.0;
    background.setRedF(background.redF() * opacity + canvas.redF() * (1 - opacity));
    background.setGreenF(background.greenF() * opacity + canvas.greenF() * (1 - opacity));
    background.setBlueF(background.blueF() * opacity + canvas.blueF() * (1 - opacity));
    QPalette colors = m_preview->palette();
    colors.setColor(QPalette::Base, background);
    colors.setColor(QPalette::Text, foreground);
    m_preview->setPalette(colors);
    m_preview->setStyleSheet(QStringLiteral("QPlainTextEdit#terminalSettingsPreview { background: %1; color: %2; }")
        .arg(background.name(), foreground.name()));
    QFont font(settings.fontFamily);
    font.setStyleHint(QFont::Monospace);
    font.setPixelSize(settings.fontSize);
    m_preview->setFont(font);
}

void TerminalSettingsDialog::save()
{
    if (!TerminalPreferences::instance()->save(selectedSettings())) {
        m_error->setText(tr("Could not save terminal settings. Check the font and values, and whether Dockpipe settings are locked, unreadable or malformed."));
        m_error->show();
        return;
    }
    accept();
}
