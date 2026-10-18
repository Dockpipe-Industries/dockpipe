#pragma once

#include "TerminalPreferences.h"
#include <QDialog>

class QCheckBox;
class QComboBox;
class QFontComboBox;
class QLabel;
class QPlainTextEdit;
class QSpinBox;
class QSlider;

class TerminalSettingsDialog final : public QDialog {
    Q_OBJECT
public:
    explicit TerminalSettingsDialog(QWidget *parent = nullptr);

private:
    TerminalSettings selectedSettings() const;
    void updatePreview();
    void save();
    QComboBox *m_theme;
    QFontComboBox *m_font;
    QSpinBox *m_fontSize;
    QComboBox *m_cursorStyle;
    QCheckBox *m_cursorBlink;
    QSpinBox *m_scrollback;
    QSpinBox *m_opacity;
    QComboBox *m_backgroundTarget;
    QCheckBox *m_scanlines;
    QCheckBox *m_glow;
    QSlider *m_glowStrength;
    QSlider *m_glowSpread;
    QPlainTextEdit *m_preview;
    QLabel *m_error;
};
