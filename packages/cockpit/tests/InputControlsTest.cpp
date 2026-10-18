#include "CockpitWidget.h"
#include "CockpitController.h"
#include "CockpitInputMenu.h"
#include "FurnishingController.h"
#include "SpatialBikeTrack.h"
#include "FurnishingCatalog.h"
#include <QApplication>
#include <QElapsedTimer>
#include <QKeyEvent>
#include <QLabel>
#include <QListWidget>
#include <QTemporaryDir>
#include <QScreen>
#include <QThread>
#include <cstdio>

#define VERIFY_INPUT(value) do { if (!(value)) { std::fprintf(stderr, "Input assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

namespace {
void idle(int milliseconds)
{
    QElapsedTimer clock;
    clock.start();
    while (clock.elapsed() < milliseconds) {
        QApplication::processEvents();
        QThread::msleep(2);
    }
}
void key(QWidget *target, int code)
{
    QKeyEvent event(QEvent::KeyPress, code, Qt::NoModifier);
    QApplication::sendEvent(target, &event);
}
bool selectCommand(CockpitInputMenu *menu, CockpitController *controller, const QString &id)
{
    auto *list = menu->findChild<QListWidget *>("cockpitMenuEntries");
    for (int step = 0; step < list->count(); ++step) {
        if (list->currentItem()->data(Qt::UserRole).toString() == id) {
            controller->buttonChanged(0, true);
            controller->buttonChanged(0, false);
            return true;
        }
        const int next = list->viewMode() == QListView::IconMode ? 14 : 12;
        controller->buttonChanged(next, true);
        controller->buttonChanged(next, false);
    }
    return false;
}
}

int inputControlsTest()
{
    QTemporaryDir temporary;
    VERIFY_INPUT(temporary.isValid());
    CockpitWidget cockpit;
    auto settings = cockpit.settings();
    settings.enabled = true;
    settings.hardwareAcceleration = QGuiApplication::platformName() != "offscreen";
    settings.controller = false;
    settings.voiceCommands = false;
    settings.appControl = false;
    cockpit.applySettings(settings);
    cockpit.setWorkspace(temporary.path());
    cockpit.resize(1200, 850);
    cockpit.show();
    cockpit.activateWindow();
    auto *flight = cockpit.flight();
    flight->setFocus();
    auto *controller = cockpit.findChild<CockpitController *>();
    auto *menu = cockpit.findChild<CockpitInputMenu *>();
    VERIFY_INPUT(controller && menu);
    idle(100);
    key(flight, Qt::Key_F1);
    VERIFY_INPUT(menu->isVisible() && !flight->moving());
    auto *list = menu->findChild<QListWidget *>("cockpitMenuEntries");
    VERIFY_INPUT(list && !list->item(1)->data(Qt::UserRole + 2).toBool());
    key(list, Qt::Key_Return);
    VERIFY_INPUT(list->currentItem()->data(Qt::UserRole).toString() == "visit:orbit");
    key(list, Qt::Key_Escape);
    VERIFY_INPUT(list->currentItem()->data(Qt::UserRole).toString() == "menu:navigation");
    key(list, Qt::Key_Escape);
    VERIFY_INPUT(menu->isHidden());

    SpatialInterior room;
    room.environmentKey = "input-fixture:" + temporary.path();
    room.size = {2200, 2200};
    room.height = 700;
    room.spawn = {0, -700};
    room.exit = {0, -1000};
    flight->setInterior(room, "Controller workshop");
    flight->setFocus();
    controller->pilotInput({}, {}, false);
    controller->pilotInput({0, 0, 1}, {}, false);
    VERIFY_INPUT(flight->moving());
    controller->buttonChanged(6, true);
    controller->buttonChanged(6, false);
    VERIFY_INPUT(menu->isVisible() && !flight->moving() && flight->controllerHints());
    VERIFY_INPUT(menu->findChild<QLabel *>("cockpitMenuHints")->text().contains("D-pad"));
    const QString capture = qEnvironmentVariable("DOCKPIPE_INPUT_CAPTURE");
    if (!capture.isEmpty()) {
        idle(120);
        VERIFY_INPUT(cockpit.screen()->grabWindow(cockpit.winId()).save(capture + "-menu.png"));
    }
    const auto position = flight->cameraPosition();
    controller->flightControls(1, true);
    controller->pilotInput({0, 0, -1}, {1, 0}, true);
    const int row = list->currentRow();
    controller->pilotInput({0, 0, -1}, {}, false);
    VERIFY_INPUT(list->currentRow() == row);
    key(list, Qt::Key_W);
    idle(80);
    VERIFY_INPUT(flight->cameraPosition() == position && !flight->moving());
    controller->pilotInput({}, {}, false);
    controller->flightControls(0, false);
    VERIFY_INPUT(selectCommand(menu, controller, "menu:build"));
    idle(120);
    VERIFY_INPUT(list->viewMode() == QListView::IconMode && menu->width() > 900);
    VERIFY_INPUT((menu->geometry().center() - flight->rect().center()).manhattanLength() <= 2);
    VERIFY_INPUT(list->count() == FurnishingCatalog::items().size());
    VERIFY_INPUT(!list->item(0)->icon().isNull());
    for (int index = 1; index < 12; ++index)
        VERIFY_INPUT(!list->visualItemRect(list->item(index - 1)).intersects(list->visualItemRect(list->item(index))));
    controller->buttonChanged(14, true);
    controller->buttonChanged(14, false);
    VERIFY_INPUT(list->currentRow() == 1 && !flight->furnishing()->ghost());
    const int columns = list->viewport()->width() / list->gridSize().width();
    controller->buttonChanged(12, true);
    controller->buttonChanged(12, false);
    VERIFY_INPUT(list->currentRow() == 1 + columns);
    controller->pilotInput({1, 0, 0}, {}, false);
    VERIFY_INPUT(list->currentRow() == 2 + columns);
    controller->pilotInput({}, {}, false);
    controller->buttonChanged(10, true);
    controller->buttonChanged(10, false);
    VERIFY_INPUT(menu->findChild<QLabel *>("cockpitModelCategory")->text().contains("Seating"));
    for (int index = 0; index < list->count(); ++index) {
        const auto *definition = FurnishingCatalog::find(list->item(index)->data(Qt::UserRole + 3).toString());
        VERIFY_INPUT(definition && definition->category == "Seating");
    }
    controller->buttonChanged(9, true);
    controller->buttonChanged(9, false);
    VERIFY_INPUT(list->count() == FurnishingCatalog::items().size());
    list->setCurrentRow(list->count() - 1);
    list->scrollToItem(list->currentItem());
    idle(120);
    VERIFY_INPUT(!list->currentItem()->icon().isNull());
    controller->buttonChanged(12, true);
    controller->buttonChanged(12, false);
    VERIFY_INPUT(list->currentRow() == list->count() - 1);
    list->setCurrentRow(0);
    key(list, Qt::Key_Right);
    VERIFY_INPUT(list->currentRow() == 1 && menu->isVisible());
    key(list, Qt::Key_Left);
    VERIFY_INPUT(list->currentRow() == 0 && menu->isVisible());
    cockpit.resize(850, 650);
    idle(80);
    VERIFY_INPUT(flight->rect().contains(menu->geometry()));
    cockpit.resize(1200, 850);
    idle(80);
    controller->buttonChanged(1, true);
    controller->buttonChanged(1, false);
    VERIFY_INPUT(menu->isVisible() && list->viewMode() == QListView::ListMode);
    VERIFY_INPUT(selectCommand(menu, controller, "menu:build"));
    if (!capture.isEmpty()) {
        idle(120);
        VERIFY_INPUT(cockpit.screen()->grabWindow(cockpit.winId()).save(capture + "-catalogue.png"));
    }
    VERIFY_INPUT(selectCommand(menu, controller, "spawn:chair"));
    auto *furnishing = flight->furnishing();
    VERIFY_INPUT(menu->isHidden() && furnishing->active() && furnishing->ghost());
    VERIFY_INPUT(furnishing->ghost()->type == "chair");
    controller->pilotInput({0, 0, 1}, {}, false);
    VERIFY_INPUT(!flight->moving()); // A held stick must return to neutral after menus.
    controller->pilotInput({}, {}, false);
    const float rotation = furnishing->ghost()->yaw;
    controller->buttonChanged(14, true);
    controller->buttonChanged(14, false);
    VERIFY_INPUT(furnishing->ghost()->yaw != rotation);
    furnishing->updatePreview();
    VERIFY_INPUT(furnishing->validity().isEmpty());
    controller->buttonChanged(0, true);
    controller->buttonChanged(0, false);
    VERIFY_INPUT(!furnishing->ghost() && !furnishing->selectedId().isEmpty());
    controller->buttonChanged(6, true);
    controller->buttonChanged(6, false);
    VERIFY_INPUT(selectCommand(menu, controller, "object:duplicate"));
    VERIFY_INPUT(furnishing->ghost());
    controller->buttonChanged(1, true);
    controller->buttonChanged(1, false);
    VERIFY_INPUT(!furnishing->ghost());
    key(flight, Qt::Key_F2);
    VERIFY_INPUT(!furnishing->active());

    key(flight, Qt::Key_P);
    VERIFY_INPUT(!flight->previewAutoScroll() && !flight->controllerHints());
    controller->buttonChanged(4, true);
    controller->buttonChanged(4, false);
    VERIFY_INPUT(flight->previewAutoScroll() && flight->controllerHints());
    controller->buttonChanged(6, true);
    controller->buttonChanged(6, false);
    VERIFY_INPUT(menu->isVisible());
    controller->inputSuspended();
    VERIFY_INPUT(menu->isHidden() && !flight->moving() && !flight->controllerHints());

    flight->setInterior(SpatialBikeTrack::create(), "Controller bike test");
    flight->setFocus();
    VERIFY_INPUT(flight->enterVehicle("mesa-bike"));
    controller->vehicleTriggers(0, 0);
    controller->pilotInput({}, {}, false);
    VERIFY_INPUT(flight->hasTexturedTerrain() && flight->terrainSharp());
    key(flight, Qt::Key_F1);
    VERIFY_INPUT(selectCommand(menu, controller, "terrain-detail"));
    VERIFY_INPUT(!flight->terrainSharp());
    controller->pilotInput({}, {}, false);
    key(flight, Qt::Key_F1);
    VERIFY_INPUT(selectCommand(menu, controller, "terrain-detail"));
    VERIFY_INPUT(flight->terrainSharp());
    controller->pilotInput({}, {}, false);
    controller->buttonChanged(3, true);
    controller->buttonChanged(3, false);
    VERIFY_INPUT(!flight->bikeChaseCamera());
    key(flight, Qt::Key_F1);
    VERIFY_INPUT(menu->isVisible());
    controller->vehicleTriggers(1, 1);
    controller->pilotInput({}, {}, false);
    key(list, Qt::Key_Escape);
    VERIFY_INPUT(menu->isHidden());
    const auto parked = flight->vehicles().first().position;
    controller->vehicleTriggers(1, 1);
    controller->pilotInput({}, {}, false);
    controller->vehicleTriggers(1, 0);
    controller->pilotInput({0, 1, 0}, {}, false);
    idle(80);
    VERIFY_INPUT(flight->vehicles().first().position == parked);
    controller->vehicleTriggers(0, 0);
    controller->pilotInput({}, {}, false);
    controller->vehicleTriggers(1, 0);
    controller->pilotInput({0, 1, 0}, {}, false);
    idle(400);
    VERIFY_INPUT(flight->vehicles().first().position.z() > parked.z() + 10);
    controller->inputSuspended();
    VERIFY_INPUT(!flight->moving());
    key(flight, Qt::Key_F1);
    VERIFY_INPUT(menu->isVisible());
    cockpit.hide();
    VERIFY_INPUT(menu->isHidden());
    std::puts("Unified input: keyboard/controller menus, contextual hints, spawning, placement, duplication, preview toggle and focus cleanup passed.");
    return 0;
}
