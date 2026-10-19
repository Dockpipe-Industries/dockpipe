#include "TerminalSettingsDialog.h"

#include <QCheckBox>
#include <QComboBox>
#include <QDialogButtonBox>
#include <QFontComboBox>
#include <QFormLayout>
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
    control->setRange(minimum, maximum);
    control->setValue(value);
    return control;
}

QLabel *description(const QString &text)
{
    auto *label = new QLabel(text);
    label->setTextFormat(Qt::PlainText);
    label->setWordWrap(true);
    return label;
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
    resize(560, 640);
    const TerminalSettings settings = TerminalPreferences::instance()->settings();
    auto *layout = new QVBoxLayout(this);
    auto *scroll = new QScrollArea;
    scroll->setWidgetResizable(true);
    scroll->setFrameShape(QFrame::NoFrame);
    auto *body = new QWidget;
    auto *bodyLayout = new QVBoxLayout(body);
    bodyLayout->addWidget(description(tr("Appearance applies to all DockPipe terminal views. Changes are applied when you save.")));
    auto *form = new QFormLayout;
    form->setFieldGrowthPolicy(QFormLayout::AllNonFixedFieldsGrow);
    m_theme = new QComboBox;
    m_theme->setObjectName("terminalSettingsTheme");
    const QStringList names = {tr("DockPipe"), tr("Amber"), tr("Green"), tr("Paper"), tr("Midnight")};
    const QStringList ids = TerminalPreferences::themeIds();
    for (int index = 0; index < ids.size(); ++index)
        m_theme->addItem(names[index], ids[index]);
    m_theme->setCurrentIndex(m_theme->findData(settings.theme));
    form->addRow(tr("Theme"), m_theme);
    m_font = new QFontComboBox;
    m_font->setObjectName("terminalSettingsFont");
    m_font->setFontFilters(QFontComboBox::MonospacedFonts);
    m_font->setEditable(true);
    m_font->lineEdit()->setMaxLength(128);
    m_font->setCurrentText(settings.fontFamily);
    form->addRow(tr("Monospace font"), m_font);
    m_fontSize = numberControl("terminalSettingsFontSize", 9, 28, settings.fontSize);
    m_fontSize->setSuffix(tr(" px"));
    form->addRow(tr("Font size"), m_fontSize);
    m_cursorStyle = new QComboBox;
    m_cursorStyle->setObjectName("terminalSettingsCursorStyle");
    m_cursorStyle->addItem(tr("Block"), "block");
    m_cursorStyle->addItem(tr("Underline"), "underline");
    m_cursorStyle->addItem(tr("Bar"), "bar");
    m_cursorStyle->setCurrentIndex(m_cursorStyle->findData(settings.cursorStyle));
    form->addRow(tr("Cursor style"), m_cursorStyle);
    m_cursorBlink = new QCheckBox(tr("Blink cursor"));
    m_cursorBlink->setObjectName("terminalSettingsCursorBlink");
    m_cursorBlink->setChecked(settings.cursorBlink);
    form->addRow(QString(), m_cursorBlink);
    m_scrollback = numberControl("terminalSettingsScrollback", 100, 10000, settings.scrollback);
    m_scrollback->setSuffix(tr(" lines"));
    form->addRow(tr("Scrollback"), m_scrollback);
    m_opacity = numberControl("terminalSettingsOpacity", 30, 100, settings.opacity);
    m_opacity->setSuffix("%");
    form->addRow(tr("Terminal background opacity"), m_opacity);
    m_backgroundTarget = new QComboBox;
    m_backgroundTarget->setObjectName("terminalSettingsBackgroundTarget");
    m_backgroundTarget->setAccessibleName(tr("Transparency reveals"));
    m_backgroundTarget->addItem(tr("Panel background"), "panel");
    m_backgroundTarget->addItem(tr("DockPipe canvas"), "canvas");
    m_backgroundTarget->addItem(tr("Desktop behind DockPipe"), "desktop");
    m_backgroundTarget->setCurrentIndex(m_backgroundTarget->findData(settings.backgroundTarget));
    form->addRow(tr("Transparency reveals"), m_backgroundTarget);
    m_scanlines = new QCheckBox(tr("Static scanlines"));
    m_scanlines->setObjectName("terminalSettingsScanlines");
    m_scanlines->setChecked(settings.scanlines);
    form->addRow(QString(), m_scanlines);
    m_glow = new QCheckBox(tr("Text glow"));
    m_glow->setObjectName("terminalSettingsGlow");
    m_glow->setChecked(settings.glow);
    form->addRow(QString(), m_glow);
    auto *strengthRow = sliderControl(m_glowStrength, "terminalSettingsGlowStrength", tr("Glow strength"),
        100, settings.glowStrength, tr("%"));
    auto *spreadRow = sliderControl(m_glowSpread, "terminalSettingsGlowSpread", tr("Glow spread"),
        8, settings.glowSpread, tr(" px"));
    form->addRow(tr("Glow strength"), strengthRow);
    form->addRow(tr("Glow spread"), spreadRow);
    strengthRow->setEnabled(settings.glow);
    spreadRow->setEnabled(settings.glow);
    connect(m_glow, &QCheckBox::toggled, strengthRow, &QWidget::setEnabled);
    connect(m_glow, &QCheckBox::toggled, spreadRow, &QWidget::setEnabled);
    bodyLayout->addLayout(form);
    bodyLayout->addWidget(description(tr("Opacity is 100% by default (transparency off). Lower opacity reveals the selected background; text stays opaque. Transparency uses software rendering for terminal and browser views so overlapping panels stack correctly. This disables WebGL: websites that require it may appear blank or fail to start. Set opacity to 100% and restart DockPipe to restore accelerated browsing. First enabling transparency also requires a restart. Desktop mode requires window compositing. No screen capture is used.")));
    m_preview = new QPlainTextEdit;
    m_preview->setObjectName("terminalSettingsPreview");
    m_preview->setAccessibleName(tr("Terminal font and color preview"));
    m_preview->setReadOnly(true);
    m_preview->setFixedHeight(130);
    m_preview->setPlainText(tr("$ dockpipe --version\nDockPipe terminal\n\n$ Ready for your next command"));
    bodyLayout->addWidget(m_preview);
    bodyLayout->addWidget(description(tr("Preview shows font, colors and approximate opacity. Cursor and static effects apply in terminal views after saving; no animated CRT effect is used.")));
    scroll->setWidget(body);
    layout->addWidget(scroll, 1);
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
    QFont font(settings.fontFamily);
    font.setStyleHint(QFont::Monospace);
    font.setPixelSize(settings.fontSize);
    m_preview->setFont(font);
}

void TerminalSettingsDialog::save()
{
    if (!TerminalPreferences::instance()->save(selectedSettings())) {
        m_error->setText(tr("Could not save terminal settings. Check the font and values, and whether launcher settings are locked, unreadable or malformed."));
        m_error->show();
        return;
    }
    accept();
}
