#pragma once

#include "DashboardStore.h"
#include <QDialog>

class QCheckBox;
class QComboBox;
class QLabel;
class QPushButton;
class QRadioButton;
class QStackedWidget;

// Choices stay local until the owner applies them after Finish. Dismissal can
// separately retain the user's global "Do not show again" preference.
class DashboardTourDialog final : public QDialog {
    Q_OBJECT
public:
    explicit DashboardTourDialog(const DashboardOptions &options, QWidget *parent = nullptr);
    DashboardOptions selectedOptions() const;
    QString initialEnvironment() const;
    bool doNotShowAgain() const;
    void setDoNotShowAgain(bool suppressed);

private:
    void showPage(int index);
    DashboardOptions m_options;
    QStackedWidget *m_pages;
    QLabel *m_step;
    QRadioButton *m_canvas;
    QRadioButton *m_tiling;
    QCheckBox *m_compact;
    QComboBox *m_background;
    QCheckBox *m_acquire;
    QCheckBox *m_doNotShowAgain;
    QPushButton *m_back;
    QPushButton *m_next;
    QPushButton *m_finish;
};
