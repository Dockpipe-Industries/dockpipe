#include "SpatialDisplayDialog.h"
#include "DesktopAppCatalog.h"
#include "DesktopAppWindow.h"
#include <QComboBox>
#include <QDialog>
#include <QDialogButtonBox>
#include <QDoubleSpinBox>
#include <QFormLayout>
#include <QLabel>
#include <QPushButton>
#include <cmath>

bool SpatialDisplayDialog::edit(QWidget *parent, SpatialDisplay &display, const SpatialInterior &room)
{
    QDialog dialog(parent);
    dialog.setWindowTitle(QObject::tr("Application display · Layout edit"));
    dialog.resize(600, 420);
    auto *form = new QFormLayout(&dialog);
    auto *applications = new QComboBox;
    applications->setObjectName("spatialApplication");
    applications->setSizeAdjustPolicy(QComboBox::AdjustToMinimumContentsLengthWithIcon);
    applications->setMinimumContentsLength(30);
    QVector<SpatialApplicationReference> references;
    const auto catalog = DesktopAppCatalog::applications();
    QString error;
    const auto windows = DesktopAppWindow::availableWindows(error);
    for (const auto &window : windows) {
        if (window.applicationClass.isEmpty())
            continue;
        const auto reference = DesktopAppCatalog::reference(window, catalog);
        references.append(reference);
        applications->addItem(QObject::tr("Running · %1").arg(window.title));
    }
    for (const auto &app : catalog) {
        references.append({app.desktopId, app.windowClass, app.name});
        applications->addItem(QObject::tr("Installed · %1").arg(app.name));
    }
    if (!display.application.name.isEmpty()) {
        references.prepend(display.application);
        applications->insertItem(0, QObject::tr("Current · %1").arg(display.application.name));
        applications->setCurrentIndex(0);
    }
    auto *wall = new QComboBox;
    wall->setObjectName("spatialWall");
    wall->addItems({"north", "east", "south", "west"});
    wall->setCurrentText(display.wall);
    form->addRow(QObject::tr("Application"), applications);
    form->addRow(QObject::tr("Wall"), wall);
    const auto coordinate = [form](const QString &label, const QString &name, qreal value, qreal minimum, qreal maximum) {
        auto *spin = new QDoubleSpinBox;
        spin->setObjectName(name);
        spin->setDecimals(0);
        spin->setRange(minimum, maximum);
        spin->setSingleStep(20);
        spin->setValue(value);
        form->addRow(label, spin);
        return spin;
    };
    auto *offset = coordinate(QObject::tr("Horizontal offset"), "spatialOffset", display.position.x(), -1200, 1200);
    auto *height = coordinate(QObject::tr("Centre height"), "spatialHeight", display.position.y(), 40, room.height - 40);
    auto *width = coordinate(QObject::tr("Width"), "spatialWidth", display.size.width(), 80, 2360);
    auto *size = coordinate(QObject::tr("Height"), "spatialSize", display.size.height(), 60, room.height - 40);
    auto *hint = new QLabel(QObject::tr("Positions snap to 20-unit increments. Save validates wall bounds. Placing a reference never launches or captures an app. Enable App control and select a display to connect; ambiguous windows require a fresh choice. Window titles and contents are never saved."));
    hint->setWordWrap(true);
    form->addRow(hint);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Ok | QDialogButtonBox::Cancel);
    buttons->button(QDialogButtonBox::Ok)->setEnabled(!references.isEmpty());
    form->addRow(buttons);
    QObject::connect(buttons, &QDialogButtonBox::accepted, &dialog, &QDialog::accept);
    QObject::connect(buttons, &QDialogButtonBox::rejected, &dialog, &QDialog::reject);
    if (dialog.exec() != QDialog::Accepted || applications->currentIndex() < 0)
        return false;
    display.application = references[applications->currentIndex()];
    display.wall = wall->currentText();
    display.position = {std::round(offset->value() / 20) * 20, std::round(height->value() / 20) * 20};
    display.size = {width->value(), size->value()};
    return true;
}
