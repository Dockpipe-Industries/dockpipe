#pragma once

#include "DashboardStore.h"
#include <QDialog>

class QCheckBox;
class QComboBox;
class QLineEdit;

class DashboardSettingsDialog final : public QDialog {
    Q_OBJECT
public:
    explicit DashboardSettingsDialog(const DashboardOptions &options, QWidget *parent = nullptr);
    DashboardOptions selectedOptions() const;

signals:
    void renameRequested();
    void removeRequested();
    void tourRequested();

private:
    QCheckBox *m_compact;
    QCheckBox *m_autoHideDock;
    QCheckBox *m_headers;
    QCheckBox *m_acquire;
    QComboBox *m_background;
    QComboBox *m_canvasFiles;
    QComboBox *m_tilingFiles;
    QLineEdit *m_image;
};
