#pragma once

#include "DesktopPermissionAccess.h"
#include <QGroupBox>
#include <array>

class QLabel;
class QPushButton;
class QTimer;

class DesktopPermissionsWidget final : public QGroupBox {
    Q_OBJECT
public:
    explicit DesktopPermissionsWidget(QWidget *parent = nullptr,
        std::unique_ptr<DesktopPermissionAccess> access = createDesktopPermissionAccess());

protected:
    void showEvent(QShowEvent *event) override;
    void hideEvent(QHideEvent *event) override;

private:
    struct PermissionRow {
        DesktopPermission permission;
        QLabel *status;
        QPushButton *request;
        QPushButton *settings;
    };

    PermissionRow addPermission(DesktopPermission permission, const QString &name,
        const QString &title, const QString &description);
    void refresh();

    std::unique_ptr<DesktopPermissionAccess> access_;
    std::array<PermissionRow, 2> rows_;
    QTimer *timer_;
    QLabel *error_;
};
