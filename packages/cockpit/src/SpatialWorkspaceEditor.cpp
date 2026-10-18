#include "SpatialWorkspaceEditor.h"
#include "SpatialDisplayDialog.h"
#include <QComboBox>
#include <QDialog>
#include <QDialogButtonBox>
#include <QFormLayout>
#include <QGridLayout>
#include <QDoubleSpinBox>
#include <QLabel>
#include <QLineEdit>
#include <QMessageBox>
#include <QPushButton>
#include <QTreeWidget>
#include <QVBoxLayout>

namespace {
bool planetForm(QWidget *parent, SpatialPlanet &planet)
{
    QDialog dialog(parent);
    dialog.setWindowTitle(QObject::tr("Planet settings"));
    auto *form = new QFormLayout(&dialog);
    auto *name = new QLineEdit(planet.name);
    name->setObjectName("spatialName");
    name->setMaxLength(180);
    auto *theme = new QComboBox;
    theme->addItems({"ocean", "desert", "forest"});
    theme->setCurrentText(planet.theme);
    form->addRow(QObject::tr("Name"), name);
    form->addRow(QObject::tr("Theme"), theme);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Ok | QDialogButtonBox::Cancel);
    form->addRow(buttons);
    QObject::connect(buttons, &QDialogButtonBox::accepted, &dialog, &QDialog::accept);
    QObject::connect(buttons, &QDialogButtonBox::rejected, &dialog, &QDialog::reject);
    if (dialog.exec() != QDialog::Accepted)
        return false;
    planet.name = name->text().trimmed();
    planet.theme = theme->currentText();
    return true;
}
bool buildingForm(QWidget *parent, SpatialBuilding &building)
{
    QDialog dialog(parent);
    dialog.setWindowTitle(QObject::tr("Building settings"));
    auto *form = new QFormLayout(&dialog);
    auto *name = new QLineEdit(building.name);
    name->setObjectName("spatialName");
    name->setMaxLength(180);
    auto *style = new QComboBox;
    style->addItems({"hangar", "tower", "pavilion", "plot"});
    style->setCurrentText(building.style);
    auto *x = new QDoubleSpinBox;
    x->setRange(-8000, 8000);
    x->setSingleStep(20);
    x->setValue(building.position.x());
    auto *z = new QDoubleSpinBox;
    z->setRange(-800, 8000);
    z->setSingleStep(20);
    z->setValue(building.position.y());
    form->addRow(QObject::tr("Name"), name);
    form->addRow(QObject::tr("Style"), style);
    form->addRow(QObject::tr("East / west"), x);
    form->addRow(QObject::tr("North / south"), z);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Ok | QDialogButtonBox::Cancel);
    form->addRow(buttons);
    QObject::connect(buttons, &QDialogButtonBox::accepted, &dialog, &QDialog::accept);
    QObject::connect(buttons, &QDialogButtonBox::rejected, &dialog, &QDialog::reject);
    if (dialog.exec() != QDialog::Accepted)
        return false;
    building.name = name->text().trimmed();
    building.style = style->currentText();
    building.position = {x->value(), z->value()};
    return true;
}

}

SpatialWorkspaceEditor::SpatialWorkspaceEditor(const SpatialWorkspace &workspace, QWidget *parent)
    : QWidget(parent), m_draft(workspace)
{
    setObjectName("spatialWorkspaceEditor");
    auto *layout = new QVBoxLayout(this);
    auto *hint = new QLabel(tr("WORKSPACE EDIT MODE · Save applies your layout. Closing discards unsaved edits. Applications remain independent of their displays."));
    hint->setWordWrap(true);
    layout->addWidget(hint);
    m_tree = new QTreeWidget;
    m_tree->setObjectName("spatialTree");
    m_tree->setHeaderLabels({tr("Workspace"), tr("Appearance")});
    layout->addWidget(m_tree, 1);
    auto *actions = new QGridLayout;
    layout->addLayout(actions);
    auto button = [this, actions](const QString &name, const QString &label, auto action) {
        auto *control = new QPushButton(label);
        control->setObjectName(name);
        const int index = actions->count();
        actions->addWidget(control, index / 4, index % 4);
        connect(control, &QPushButton::clicked, this, action);
    };
    button("spatialAddPlanet", tr("Add planet"), [this]() { addPlanet(); });
    button("spatialAddBuilding", tr("Add building"), [this]() { addBuilding(); });
    button("spatialAddPlot", tr("Add empty plot"), [this]() { addBuilding(true); });
    button("spatialFurnishRoom", tr("Furnish room"), [this]() { furnishRoom(); });
    button("spatialEdit", tr("Edit"), [this]() { editSelected(); });
    button("spatialRemove", tr("Remove"), [this]() { removeSelected(); });
    button("spatialSave", tr("Save layout"), [this]() { saveDraft(); });
    button("spatialReload", tr("Restore saved"), [this]() {
        QString error;
        SpatialWorkspace restored;
        if (reload && reload(restored, error)) {
            m_draft = restored;
            rebuild();
            m_status->setText(tr("Saved layout restored."));
        } else {
            m_status->setText(error);
        }
    });
    button("spatialTravel", tr("Travel now"), [this]() {
        if (auto *planet = selectedPlanet()) {
            const auto *building = selectedBuilding();
            const QString path = building ? "building:" + building->id : "planet:" + planet->id;
            if (saveDraft())
                emit travelRequested(path);
        }
    });
    button("spatialApproach", tr("Fly to selection"), [this]() {
        if (auto *planet = selectedPlanet()) {
            const auto *building = selectedBuilding();
            const QString path = building ? "building:" + building->id : "planet:" + planet->id;
            if (saveDraft())
                emit approachRequested(path);
        }
    });
    m_status = new QLabel;
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    layout->addWidget(m_status);
    rebuild();
}

SpatialPlanet *SpatialWorkspaceEditor::selectedPlanet()
{
    const auto *item = m_tree->currentItem();
    if (!item)
        return nullptr;
    while (item->parent())
        item = item->parent();
    for (auto &planet : m_draft.planets) {
        if (planet.id == item->data(0, Qt::UserRole).toString())
            return &planet;
    }
    return nullptr;
}

void SpatialWorkspaceEditor::rebuild(const QString &selected)
{
    m_tree->clear();
    for (const auto &planet : m_draft.planets) {
        auto *item = new QTreeWidgetItem(m_tree, {planet.name, planet.theme});
        item->setData(0, Qt::UserRole, planet.id);
        if (planet.id == selected)
            m_tree->setCurrentItem(item);
        for (const auto &building : planet.buildings) {
            auto *child = new QTreeWidgetItem(item, {building.name, building.style});
            child->setData(0, Qt::UserRole, building.id);
            if (building.id == selected)
                m_tree->setCurrentItem(child);
            for (const auto &display : building.interior.displays) {
                auto *surface = new QTreeWidgetItem(child, {display.application.name, display.wall});
                surface->setData(0, Qt::UserRole, display.id);
                if (display.id == selected)
                    m_tree->setCurrentItem(surface);
            }
            child->setExpanded(true);
        }
        item->setExpanded(true);
    }
    m_tree->resizeColumnToContents(0);
}

void SpatialWorkspaceEditor::addPlanet()
{
    if (m_draft.planets.size() >= SpatialWorkspace::maximumPlanets) {
        m_status->setText(tr("The universe supports 24 custom planets."));
        return;
    }
    SpatialPlanet planet;
    planet.id = SpatialWorkspace::newId();
    planet.name = tr("New workspace");
    if (planetForm(this, planet)) {
        m_draft.planets.append(planet);
        rebuild(planet.id);
    }
}

void SpatialWorkspaceEditor::editSelected()
{
    if (auto *display = selectedDisplay()) {
        SpatialDisplay edited = *display;
        if (SpatialDisplayDialog::edit(this, edited, selectedBuilding()->interior)) {
            *display = edited;
            rebuild(edited.id);
        }
        return;
    }
    if (auto *building = selectedBuilding()) {
        SpatialBuilding edited = *building;
        if (buildingForm(this, edited)) {
            *building = edited;
            rebuild(edited.id);
        }
        return;
    }
    if (auto *planet = selectedPlanet()) {
        SpatialPlanet edited = *planet;
        if (planetForm(this, edited)) {
            *planet = edited;
            rebuild(edited.id);
        }
    }
}

void SpatialWorkspaceEditor::removeSelected()
{
    if (auto *display = selectedDisplay()) {
        auto *building = selectedBuilding();
        const QString id = display->id;
        for (int index = 0; index < building->interior.displays.size(); ++index) {
            if (building->interior.displays[index].id == id) {
                building->interior.displays.removeAt(index);
                break;
            }
        }
        rebuild(building->id);
        return;
    }
    if (auto *building = selectedBuilding()) {
        const QString id = building->id;
        if (QMessageBox::question(this, tr("Remove building?"),
            tr("Remove this building and its layout? Applications will keep running.")) != QMessageBox::Yes)
            return;
        auto *planet = selectedPlanet();
        for (int index = 0; index < planet->buildings.size(); ++index) {
            if (planet->buildings[index].id == id) {
                planet->buildings.removeAt(index);
                break;
            }
        }
        rebuild(planet->id);
        return;
    }
    if (auto *planet = selectedPlanet()) {
        const QString id = planet->id;
        if (QMessageBox::question(this, tr("Remove workspace?"),
            tr("Remove this planet and its layouts? Applications will keep running.")) != QMessageBox::Yes)
            return;
        for (int index = 0; index < m_draft.planets.size(); ++index) {
            if (m_draft.planets[index].id == id) {
                m_draft.planets.removeAt(index);
                break;
            }
        }
        rebuild();
    }
}

bool SpatialWorkspaceEditor::saveDraft()
{
    QString error = m_draft.validate();
    if (error.isEmpty() && save && save(m_draft, error)) {
        m_status->setText(tr("Layout saved."));
        return true;
    }
    m_status->setText(error);
    return false;
}

SpatialBuilding *SpatialWorkspaceEditor::selectedBuilding()
{
    auto *planet = selectedPlanet();
    const auto *item = m_tree->currentItem();
    if (!planet || !item || !item->parent())
        return nullptr;
    while (item->parent()->parent())
        item = item->parent();
    for (auto &building : planet->buildings) {
        if (building.id == item->data(0, Qt::UserRole).toString())
            return &building;
    }
    return nullptr;
}

void SpatialWorkspaceEditor::addBuilding(bool plot)
{
    auto *planet = selectedPlanet();
    if (!planet) {
        m_status->setText(tr("Select a planet first."));
        return;
    }
    if (planet->buildings.size() >= SpatialWorkspace::maximumBuildings) {
        m_status->setText(tr("A planet supports 64 buildings."));
        return;
    }
    SpatialBuilding building;
    if (plot) {
        building = SpatialBuilding::emptyPlot(tr("My base"));
    } else {
        building.id = SpatialWorkspace::newId();
        building.name = tr("New building");
    }
    building.position = {qreal(planet->buildings.size() % 8 * 320), qreal(planet->buildings.size() / 8 * 320)};
    if (buildingForm(this, building)) {
        planet->buildings.append(building);
        rebuild(building.id);
    }
}

SpatialDisplay *SpatialWorkspaceEditor::selectedDisplay()
{
    auto *building = selectedBuilding();
    const auto *item = m_tree->currentItem();
    if (!building || !item)
        return nullptr;
    for (auto &display : building->interior.displays) {
        if (display.id == item->data(0, Qt::UserRole).toString())
            return &display;
    }
    return nullptr;
}

void SpatialWorkspaceEditor::furnishRoom()
{
    auto *building = selectedBuilding();
    if (!building) {
        m_status->setText(tr("Select a building first."));
        return;
    }
    const QString id = building->id;
    if (saveDraft())
        emit furnishRequested("building:" + id);
}

void SpatialWorkspaceEditor::showStatus(const QString &status)
{
    m_status->setText(status);
}
