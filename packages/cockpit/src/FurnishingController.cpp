#include "FurnishingController.h"
#include "FurnishingPreview.h"
#include "SpatialAssetLibrary.h"
#include "WorkspaceFlightView.h"
#include "SpatialInteriorGeometry.h"
#include "ScreenProviders.h"
#include "DesktopAppCatalog.h"
#include <QFileDialog>
#include <QComboBox>
#include <QFrame>
#include <QHBoxLayout>
#include <QKeyEvent>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QSettings>
#include <QSpinBox>
#include <QScrollArea>
#include <algorithm>
#include <QSignalBlocker>
#include <QVBoxLayout>
#include <QtMath>
#include <cmath>

FurnishingController::FurnishingController(WorkspaceFlightView *view) : QObject(view), m_view(view)
{
    m_toggle = new QPushButton(tr("Furnish · F2 / menu"), view);
    m_toggle->setObjectName("furnishingToggle");
    m_toggle->setProperty("flightOverlay", true);
    connect(m_toggle, &QPushButton::clicked, this, &FurnishingController::toggle);
    m_hud = new QFrame(view);
    m_hud->setObjectName("furnishingHud");
    m_hud->setProperty("flightOverlay", true);
    m_hud->setStyleSheet(
        "QFrame#furnishingHud { background:#102330; border:1px solid #4b8d99; border-radius:12px; }"
        "QWidget#furnishingContents, QScrollArea { background:#102330; border:0; }"
        "QLabel { color:#cfe5ec; background:transparent; }"
        "QPushButton, QLineEdit, QComboBox, QPlainTextEdit, QSpinBox { background:#1b3546; color:#e2eff3; border:1px solid #3e5e72; border-radius:6px; padding:7px; }"
        "QPushButton:hover { background:#285367; }"
        "QComboBox QAbstractItemView { background:#1b3546; color:#e2eff3; selection-background-color:#285367; }");
    auto *frameLayout = new QVBoxLayout(m_hud);
    frameLayout->setContentsMargins(5, 5, 5, 5);
    auto *scroll = new QScrollArea;
    scroll->setWidgetResizable(true);
    scroll->setFrameShape(QFrame::NoFrame);
    auto *content = new QWidget;
    content->setObjectName("furnishingContents");
    auto *layout = new QVBoxLayout(content);
    scroll->setWidget(content);
    frameLayout->addWidget(scroll);
    auto *title = new QLabel(tr("BUILD & FURNISH"));
    layout->addWidget(title);
    m_search = new QLineEdit;
    m_search->setObjectName("furnishingSearch");
    m_search->setPlaceholderText(tr("Search furniture, models, building pieces…"));
    m_search->setMaxLength(120);
    layout->addWidget(m_search);
    m_categories = new QComboBox;
    m_categories->setObjectName("furnishingCategory");
    m_categories->addItem(tr("All types"));
    for (const auto &item : FurnishingCatalog::items()) {
        if (m_categories->findText(item.category) < 0)
            m_categories->addItem(item.category);
    }
    layout->addWidget(m_categories);
    m_items = new QComboBox(m_hud);
    m_items->setObjectName("furnishingItem");
    m_items->hide(); // Selection model shared with keyboard/controller browsing.
    m_grid = new QListWidget;
    m_grid->setObjectName("furnishingGrid");
    m_grid->setViewMode(QListView::IconMode);
    m_grid->setResizeMode(QListView::Adjust);
    m_grid->setMovement(QListView::Static);
    m_grid->setIconSize({128, 100});
    m_grid->setGridSize({146, 146});
    m_grid->setWordWrap(true);
    m_grid->setSpacing(2);
    m_grid->setMinimumHeight(190);
    m_grid->setMaximumHeight(310);
    m_grid->setStyleSheet("QListWidget { background:#10212e; color:#d8e8ef; border:0; } QListWidget::item { padding:4px; border:1px solid #294354; border-radius:6px; } QListWidget::item:selected { background:#244e5b; color:#ffffff; border:1px solid #66ddc3; } QListWidget::item:hover { background:#1e394b; }");
    layout->addWidget(m_grid);
    m_buildHeight = new QSpinBox;
    m_buildHeight->setObjectName("furnishingBuildHeight");
    m_buildHeight->setRange(-1, 780);
    m_buildHeight->setValue(-1);
    m_buildHeight->setSingleStep(10);
    m_buildHeight->setSpecialValueText(tr("Height: automatic surface"));
    m_buildHeight->setPrefix(tr("Bottom height: "));
    m_buildHeight->setToolTip(tr("Aim at the ground to position fixed-height pieces. H / left stick click cycles floor, foundation, roof and upper roof heights."));
    m_buildHeight->hide();
    layout->addWidget(m_buildHeight);
    connect(m_buildHeight, &QSpinBox::valueChanged, this, [this]() { updatePreview(); });
    connect(m_categories, &QComboBox::currentIndexChanged, this, [this]() { browse(0, 0); });
    connect(m_search, &QLineEdit::textChanged, this, [this](const QString &text) {
        const QSignalBlocker categoryBlock(m_categories);
        if (!text.trimmed().isEmpty())
            m_categories->setCurrentIndex(0);
        browse(0, 0);
    });
    connect(m_grid, &QListWidget::currentRowChanged, m_items, &QComboBox::setCurrentIndex);
    connect(m_grid, &QListWidget::itemClicked, this, [this](QListWidgetItem *item) { selectType(item->data(Qt::UserRole).toString()); });
    connect(m_grid, &QListWidget::itemActivated, this, [this](QListWidgetItem *item) { selectType(item->data(Qt::UserRole).toString()); });
    auto row = new QHBoxLayout;
    auto add = [&](QBoxLayout *box, const QString &text, auto action) {
        auto *control = new QPushButton(text);
        box->addWidget(control);
        connect(control, &QPushButton::clicked, this, action);
    };
    add(row, tr("Spawn"), [this]() { selectType(m_items->currentData().toString()); });
    add(row, tr("Place / select"), &FurnishingController::confirm);
    add(row, tr("Cancel"), &FurnishingController::cancel);
    layout->addLayout(row);
    row = new QHBoxLayout;
    add(row, tr("Move"), &FurnishingController::moveSelected);
    add(row, tr("Rotate ↻"), [this]() { rotate(15); });
    add(row, tr("Duplicate"), &FurnishingController::duplicateSelected);
    add(row, tr("Delete"), &FurnishingController::removeSelected);
    layout->addLayout(row);
    add(layout, tr("Configure screen / artwork"), &FurnishingController::configure);
    m_configuration = new QWidget;
    auto *inputs = new QVBoxLayout(m_configuration);
    inputs->setContentsMargins(0, 0, 0, 0);
    m_surface = new QComboBox;
    m_surface->setObjectName("furnishingCabinetSurface");
    m_surface->addItems({tr("Main screen"), tr("Marquee artwork")});
    inputs->addWidget(m_surface);
    connect(m_surface, &QComboBox::currentIndexChanged, this, &FurnishingController::loadSurfaceBinding);
    m_provider = new QComboBox;
    m_provider->setObjectName("furnishingProvider");
    for (const auto &pair : QList<QPair<QString, QString>>{{"Text / notice", "text"}, {"Application (running / installed)", "application"}, {"Scene surface (live / interactive)", "surface"},
        {"Local file / image (static)", "file"}, {"Web URL (provider unavailable)", "web"}, {"Terminal (provider unavailable)", "terminal"}})
        m_provider->addItem(pair.first, pair.second);
    inputs->addWidget(m_provider);
    m_source = new QComboBox;
    m_source->setObjectName("furnishingSource");
    inputs->addWidget(m_source);
    m_application = new QComboBox;
    m_application->setObjectName("furnishingApplication");
    m_application->setSizeAdjustPolicy(QComboBox::AdjustToMinimumContentsLengthWithIcon);
    m_application->setMinimumContentsLength(20);
    inputs->addWidget(m_application);
    m_target = new QLineEdit;
    m_target->setMaxLength(4096);
    m_target->setPlaceholderText(tr("File path or future provider target"));
    inputs->addWidget(m_target);
    m_target->setObjectName("furnishingFile");
    m_browseFile = new QPushButton(tr("Choose image or file…"));
    m_browseFile->setObjectName("furnishingBrowseFile");
    m_browseFile->setToolTip(tr("Static images up to 2048 × 2048 pixels and files up to 4 MiB."));
    inputs->addWidget(m_browseFile);
    connect(m_browseFile, &QPushButton::clicked, this, [this]() {
        const auto path = QFileDialog::getOpenFileName(m_hud, tr("Choose screen or artwork image"), m_target->text(),
            tr("Images (*.png *.jpg *.jpeg *.webp *.bmp *.gif);;All files (*)"));
        if (!path.isEmpty())
            m_target->setText(path);
    });
    m_text = new QPlainTextEdit;
    m_text->setMaximumHeight(65);
    m_text->setPlaceholderText(tr("Screen text (up to 4096 characters)"));
    inputs->addWidget(m_text);
    add(inputs, tr("Apply input"), [this]() {
        bindSelected(configuredBinding());
    });
    connect(m_provider, &QComboBox::currentIndexChanged, this, [this]() {
        const QString provider = m_provider->currentData().toString();
        m_source->setVisible(provider == "surface");
        m_application->setVisible(provider == "application");
        m_browseFile->setVisible(provider == "file");
        m_target->setVisible(provider == "file" || provider == "web" || provider == "terminal");
        m_text->setVisible(provider == "text");
        resizeHud();
    });
    layout->addWidget(m_configuration);
    m_configuration->hide();
    m_source->hide();
    m_application->hide();
    m_browseFile->hide();
    m_target->hide();
    m_info = new QLabel;
    m_info->setObjectName("furnishingStatus");
    m_info->setTextFormat(Qt::PlainText);
    m_info->setWordWrap(true);
    layout->insertWidget(layout->indexOf(m_configuration), m_info);
    m_hints = new QLabel;
    m_hints->setObjectName("furnishingInputHints");
    m_hints->setWordWrap(true);
    layout->addWidget(m_hints);
    m_hud->hide();
    m_tick.setInterval(33);
    connect(&m_tick, &QTimer::timeout, this, &FurnishingController::updatePreview);
    m_categories->setCurrentIndex(1);
}
void FurnishingController::attach(const SpatialInterior &base)
{
    m_base = base;
    m_ghost.reset();
    m_selected.clear();
    m_configuring = false;
    m_configuration->hide();
    m_ready = m_layout.load(base.environmentKey, m_error);
    m_objects.clear();
    // Existing generated/application displays become selectable screen objects.
    // Their providers retain ownership of content and interaction identities.
    for (const auto &display : base.displays) {
        const auto corners = SpatialInteriorGeometry::surface(base, display);
        FurnishingObject object;
        object.id = "base:" + display.id;
        object.type = "wall-screen";
        object.baseDisplay = display.id;
        object.screenSize = display.size;
        object.position = (corners[0] + corners[2]) / 2;
        const auto right = corners[1] - corners[0];
        object.yaw = qRadiansToDegrees(std::atan2(right.z(), right.x()));
        object.binding = {"surface", display.id, {}};
        m_objects.append(object);
    }
    if (m_ready) {
        for (const auto &saved : m_layout.objects) {
            auto found = std::find_if(m_objects.begin(), m_objects.end(), [&](const auto &item) { return item.id == saved.id; });
            if (found != m_objects.end())
                *found = saved;
            else if (saved.baseDisplay.isEmpty())
                m_objects.append(saved);
        }
    }
    m_toggle->show();
    m_toggle->setEnabled(m_ready);
    m_toggle->setToolTip(m_error);
    rebuild(true);
    resizeHud();
    const QString environment = m_base.environmentKey;
    QTimer::singleShot(0, this, [this, environment]() {
        if (!m_view->isVisible() || !m_view->applicationDisplayRequested || m_base.environmentKey != environment)
            return;
        for (const auto &object : m_objects) {
            if (!object.removed && object.binding.provider == "application")
                m_view->applicationDisplayRequested(FurnishingCatalog::display(object).id, false);
        }
    });
}
void FurnishingController::detach()
{
    if (m_active)
        toggle();
    m_tick.stop();
    m_ready = false;
    m_toggle->hide();
    m_objects.clear();
    m_suppressed.clear();
    m_sources.clear();
    m_view->m_furnishingGeometry.clear();
    m_view->m_furnishingStaticFaces = 0;
    m_view->m_interiorPaintDirty = true;
}
void FurnishingController::toggle()
{
    if (!m_active && (!m_ready || !m_view->walking() || m_view->editorActive() || !m_view->activeVehicleId().isEmpty()))
        return;
    m_active = !m_active;
    m_ghost.reset();
    m_configuring = false;
    m_configuration->hide();
    m_hud->setVisible(m_active);
    m_view->viewport()->setMouseTracking(m_active);
    m_view->suspendPilot();
    m_mouseTarget = false;
    if (m_active) {
        m_view->m_pitch = -0.3;
        m_tick.start();
    } else {
        m_tick.stop();
    }
    rebuild();
    updateHud();
    resizeHud();
    m_view->setFocus();
}
void FurnishingController::resizeHud()
{
    m_toggle->adjustSize();
    m_toggle->move(16, 58);
    m_hud->setGeometry(16, 105, qMin(510, qMax(280, m_view->width() - 32)), qMax(100, qMin(m_view->height() - 120, 760)));
    m_grid->setVisible(!m_configuring);
    m_search->setVisible(!m_configuring);
    m_categories->setVisible(!m_configuring);
    m_buildHeight->setVisible(!m_configuring && m_ghost && FurnishingCatalog::modular(*m_ghost));
    m_hud->raise();
    m_toggle->raise();
}
void FurnishingController::browse(int categoryDelta, int itemDelta)
{
    if (categoryDelta) {
        m_categories->setCurrentIndex((m_categories->currentIndex() + categoryDelta + m_categories->count()) % m_categories->count());
        return;
    }
    if (!itemDelta) {
        const QSignalBlocker block(m_items);
        const QSignalBlocker gridBlock(m_grid);
        m_items->clear();
        m_grid->clear();
        const QString query = m_search->text().trimmed();
        QVector<FurnishingDefinition> choices = FurnishingCatalog::items();
        std::stable_sort(choices.begin(), choices.end(), [](const auto &first, const auto &second) {
            if (first.asset.isEmpty() != second.asset.isEmpty())
                return !first.asset.isEmpty();
            return first.name.localeAwareCompare(second.name) < 0;
        });
        for (const auto &item : choices) {
            if (m_categories->currentIndex() != 0 && item.category != m_categories->currentText())
                continue;
            if (!query.isEmpty() && !(item.name + " " + item.category).contains(query, Qt::CaseInsensitive))
                continue;
            m_items->addItem(item.name, item.id);
            auto *tile = new QListWidgetItem(QIcon(FurnishingPreview::thumbnail(item.id)), item.name, m_grid);
            tile->setData(Qt::UserRole, item.id);
            tile->setToolTip(tr("%1 · %2\n%3 × %4 × %5 units\n%6").arg(item.name, item.category)
                .arg(qRound(item.size.x())).arg(qRound(item.size.y())).arg(qRound(item.size.z()))
                .arg(item.asset.isEmpty() ? tr("DockPipe procedural piece") : tr("Kenney Furniture Kit · CC0")));
        }
        const int last = m_items->findData(QSettings().value("cockpit/furnishingLastItem"));
        if (last >= 0)
            m_items->setCurrentIndex(last);
        m_grid->setCurrentRow(m_items->currentIndex());
    } else {
        if (!m_items->count())
            return;
        m_items->setCurrentIndex((m_items->currentIndex() + itemDelta + m_items->count()) % m_items->count());
        selectType(m_items->currentData().toString());
    }
}
void FurnishingController::selectType(const QString &type)
{
    if (!m_active || !FurnishingCatalog::find(type))
        return;
    const auto *item = FurnishingCatalog::find(type);
    if (m_items->findData(type) < 0) {
        m_search->clear();
        m_categories->setCurrentIndex(m_categories->findText(item->category));
    }
    m_items->setCurrentIndex(m_items->findData(type));
    m_grid->setCurrentRow(m_items->currentIndex());
    m_grid->scrollToItem(m_grid->currentItem());
    m_selected.clear();
    m_configuring = false;
    m_configuration->hide();
    m_ghost = FurnishingObject{};
    m_ghost->id = SpatialWorkspace::newId();
    m_ghost->type = type;
    {
        const QSignalBlocker block(m_buildHeight);
        m_buildHeight->setValue(item->family == "roof" ? 310 : -1);
    }
    if (item->category == "Wall art" && item->screen)
        m_ghost->binding = {"file", {}, {}};
    m_rotation = 0;
    QSettings().setValue("cockpit/furnishingLastItem", type);
    updatePreview();
    resizeHud();
    m_view->setFocus();
}
FurnishingObject *FurnishingController::selected()
{
    for (auto &object : m_objects) {
        if (object.id == m_selected && !object.removed)
            return &object;
    }
    return nullptr;
}
void FurnishingController::selectObject(const QString &id)
{
    if (!m_active)
        return;
    m_selected = id;
    m_surface->setCurrentIndex(0);
    if (const auto *object = selected()) {
        const auto *item = FurnishingCatalog::find(object->type);
        m_categories->setCurrentIndex(m_categories->findText(item->category));
        m_items->setCurrentIndex(m_items->findData(object->type));
        m_grid->setCurrentRow(m_items->currentIndex());
    }
    m_ghost.reset();
    m_configuring = false;
    m_configuration->hide();
    rebuild();
    updateHud();
}
void FurnishingController::point(const QPointF &position)
{
    m_pointer = position;
    m_mouseTarget = true;
}
void FurnishingController::selectAt(const QPointF &position)
{
    if (m_ghost) {
        point(position);
        updatePreview();
        confirm();
        return;
    }
    const auto ray = m_view->selectionRay(position);
    const auto occlusion = FurnishingLayout::target(m_base, m_view->m_camera, ray);
    float closest = occlusion.distance + 10;
    QString id;
    for (const auto &object : m_objects) {
        if (object.removed)
            continue;
        const float distance = FurnishingLayout::rayObject(object, m_view->m_camera, ray);
        if (distance < closest) {
            closest = distance;
            id = object.id;
        }
    }
    selectObject(id);
}
void FurnishingController::updatePreview()
{
    if (!m_active || !m_view->isVisible() || !m_ghost || m_configuring)
        return;
    auto room = m_base;
    for (const auto &object : m_objects) {
        if (object.id != m_ghost->id && !object.removed && object.baseDisplay.isEmpty())
            room.solids += FurnishingCatalog::parts(object);
    }
    const QPointF point = m_mouseTarget && !m_view->m_looking ? m_pointer : QPointF(m_view->viewport()->rect().center());
    const auto ray = m_view->selectionRay(point);
    m_hit = FurnishingLayout::target(room, m_view->m_camera, ray);
    const bool fixedHeight = FurnishingCatalog::modular(*m_ghost) && m_buildHeight->value() >= 0;
    if (fixedHeight) {
        auto ground = m_base;
        ground.outdoors = true;
        ground.solids.clear();
        m_hit = FurnishingLayout::target(ground, m_view->m_camera, ray);
    }
    const auto previous = *m_ghost;
    const auto *definition = FurnishingCatalog::find(m_ghost->type);
    const auto size = FurnishingCatalog::size(*m_ghost);
    m_ghost->yaw = m_rotation;
    m_ghost->position = m_hit.hit ? m_hit.point : m_view->m_camera + ray * 450;
    if (definition->mount == FurnishingMount::Wall && std::abs(m_hit.normal.y()) < 0.1f) {
        m_ghost->yaw = std::remainder(qRadiansToDegrees(std::atan2(m_hit.normal.x(), -m_hit.normal.z())) + m_rotation, 360.0f);
        m_ghost->position += m_hit.normal * (size.z() / 2 + 3);
        m_ghost->position.setY(std::round(m_ghost->position.y() / 10) * 10);
    } else {
        m_ghost->position.setY(m_ghost->position.y() + (definition->mount == FurnishingMount::Ceiling ? -1 : 1) * (size.y() / 2 + 1));
        const float grid = FurnishingCatalog::modular(*m_ghost) ? 100 : 10;
        m_ghost->position.setX(std::round(m_ghost->position.x() / grid) * grid);
        m_ghost->position.setZ(std::round(m_ghost->position.z() / grid) * grid);
    }
    if (fixedHeight)
        m_ghost->position.setY(m_buildHeight->value() + size.y() / 2 + 1);
    const QString validity = FurnishingLayout::placementError(*m_ghost, m_base, m_objects, m_hit);
    if (previous.position != m_ghost->position || previous.yaw != m_ghost->yaw || validity != m_validity) {
        m_validity = validity;
        rebuild();
        updateHud();
    }
}
bool FurnishingController::publish(const FurnishingObject &object)
{
    const auto previous = m_layout.objects;
    auto found = std::find_if(m_layout.objects.begin(), m_layout.objects.end(), [&](const auto &item) { return item.id == object.id; });
    if (found == m_layout.objects.end())
        m_layout.objects.append(object);
    else
        *found = object;
    if (object.removed && object.baseDisplay.isEmpty())
        m_layout.objects.removeIf([&](const auto &item) { return item.id == object.id; });
    if (!m_layout.save(m_error)) {
        m_layout.objects = previous;
        updateHud();
        return false;
    }
    auto current = std::find_if(m_objects.begin(), m_objects.end(), [&](const auto &item) { return item.id == object.id; });
    if (current == m_objects.end())
        m_objects.append(object);
    else
        *current = object;
    if (object.removed && object.baseDisplay.isEmpty())
        m_objects.removeIf([&](const auto &item) { return item.id == object.id; });
    m_error.clear();
    rebuild(true);
    return true;
}
void FurnishingController::confirm()
{
    if (!m_active)
        return;
    if (m_configuring) {
        bindSelected(configuredBinding());
        return;
    }
    if (!m_ghost) {
        selectAt(m_view->viewport()->rect().center());
        return;
    }
    updatePreview();
    if (!m_validity.isEmpty())
        return;
    const auto object = *m_ghost;
    if (publish(object)) {
        m_selected = object.id;
        if (FurnishingCatalog::modular(object)) {
            m_ghost->id = SpatialWorkspace::newId();
            m_selected.clear();
            updatePreview();
        } else {
            m_ghost.reset();
        }
        rebuild();
        updateHud();
    }
}
void FurnishingController::cancel()
{
    if (m_configuring) {
        m_configuring = false;
        m_configuration->hide();
    } else if (m_ghost) {
        m_ghost.reset();
    } else if (!m_selected.isEmpty()) {
        m_selected.clear();
    } else {
        toggle();
        return;
    }
    rebuild();
    updateHud();
    resizeHud();
}
void FurnishingController::rotate(float degrees)
{
    if (!m_ghost)
        moveSelected();
    if (m_ghost) {
        if (FurnishingCatalog::modular(*m_ghost))
            degrees = degrees < 0 ? -90 : 90;
        m_rotation = std::remainder(m_rotation + degrees, 360.0f);
        updatePreview();
    }
}
void FurnishingController::moveSelected()
{
    if (const auto *object = selected()) {
        m_ghost = *object;
        m_buildHeight->setValue(FurnishingCatalog::modular(*object)
            ? qRound(object->position.y() - FurnishingCatalog::size(*object).y() / 2 - 1) : -1);
        m_rotation = FurnishingCatalog::find(object->type)->mount == FurnishingMount::Wall ? 0 : object->yaw;
        m_mouseTarget = false;
        updatePreview();
        resizeHud();
        m_view->setFocus();
    }
}
void FurnishingController::duplicateSelected()
{
    if (const auto *object = selected()) {
        m_ghost = *object;
        m_buildHeight->setValue(FurnishingCatalog::modular(*object)
            ? qRound(object->position.y() - FurnishingCatalog::size(*object).y() / 2 - 1) : -1);
        m_ghost->id = SpatialWorkspace::newId();
        m_ghost->baseDisplay.clear();
        if (FurnishingCatalog::find(object->type)->screen && !object->baseDisplay.isEmpty())
            m_ghost->screenSize = object->screenSize;
        m_rotation = FurnishingCatalog::find(object->type)->mount == FurnishingMount::Wall ? 0 : object->yaw;
        m_selected.clear();
        updatePreview();
        resizeHud();
    }
}
void FurnishingController::removeSelected()
{
    if (const auto *object = selected()) {
        auto removed = *object;
        removed.removed = true;
        if (publish(removed)) {
            m_view->m_interiorBindings.remove(FurnishingCatalog::display(removed).id);
            m_selected.clear();
            m_ghost.reset();
            rebuild();
            updateHud();
        }
    }
}
void FurnishingController::configure()
{
    const auto *object = selected();
    if (!object || !FurnishingCatalog::find(object->type)->screen)
        return;
    m_ghost.reset();
    m_configuring = !m_configuring;
    m_configuration->setVisible(m_configuring);
    m_surface->setVisible(object->type == "arcade-cabinet");
    loadSurfaceBinding();
    m_view->suspendPilot();
    rebuild();
    updateHud();
    resizeHud();
}

void FurnishingController::loadSurfaceBinding()
{
    const auto *object = selected();
    if (!object)
        return;
    const bool marquee = object->type == "arcade-cabinet" && m_surface->currentIndex() == 1;
    const auto &binding = marquee ? object->marqueeBinding : object->binding;
    m_provider->clear();
    m_provider->addItem(tr("Text / notice"), "text");
    m_provider->addItem(tr("Local file / image (static)"), "file");
    if (!marquee) {
        m_provider->addItem(tr("Application (running / installed)"), "application");
        m_provider->addItem(tr("Scene surface (live / interactive)"), "surface");
        m_provider->addItem(tr("Web URL (provider unavailable)"), "web");
        m_provider->addItem(tr("Terminal (provider unavailable)"), "terminal");
    }
    refreshApplications();
    m_provider->setCurrentIndex(qMax(0, m_provider->findData(binding.provider)));
    m_source->clear();
    for (const auto &display : m_base.displays)
        m_source->addItem(display.application.name.isEmpty() ? display.id : display.application.name, display.id);
    m_source->setCurrentIndex(qMax(0, m_source->findData(binding.target)));
    for (int index = 0; index < m_applications.size(); ++index) {
        const auto &application = m_applications[index];
        if (application.desktopId == object->binding.application.desktopId
            && application.windowClass == object->binding.application.windowClass)
            m_application->setCurrentIndex(index);
    }
    m_target->setText(binding.target);
    m_text->setPlainText(binding.text);
    updateHud();
    resizeHud();
}
bool FurnishingController::bindSelected(const ScreenBinding &binding)
{
    const auto *object = selected();
    if (!object || !FurnishingCatalog::find(object->type)->screen)
        return false;
    if (binding.provider == "application" && !ScreenProviders::capabilities(binding, {}).available) {
        m_error = tr("Choose an application before applying this input.");
        updateHud();
        return false;
    }
    auto updated = *object;
    const bool marquee = object->type == "arcade-cabinet" && m_surface->currentIndex() == 1;
    if (marquee) {
        if (binding.provider != "file" && binding.provider != "text")
            return false;
        updated.marqueeBinding = binding;
    } else {
        updated.binding = binding;
    }
    if (!publish(updated))
        return false;
    if (!marquee)
        m_view->m_interiorBindings.remove(FurnishingCatalog::display(updated).id);
    m_configuring = false;
    m_configuration->hide();
    updateHud();
    resizeHud();
    m_view->setFocus();
    if (binding.provider == "application" && m_view->applicationDisplayRequested)
        m_view->applicationDisplayRequested(FurnishingCatalog::display(updated).id, false);
    return true;
}

ScreenBinding FurnishingController::configuredBinding() const
{
    ScreenBinding binding{m_provider->currentData().toString(), m_target->text(), m_text->toPlainText()};
    if (binding.provider == "surface")
        binding.target = m_source->currentData().toString();
    if (binding.provider == "application") {
        const int index = m_application->currentIndex();
        if (index >= 0 && index < m_applications.size())
            binding.application = m_applications[index];
        binding.target.clear();
        binding.text.clear();
    }
    return binding;
}

void FurnishingController::refreshApplications()
{
    m_application->clear();
    m_applications.clear();
    auto append = [&](const SpatialApplicationReference &reference, const QString &label) {
        for (const auto &existing : m_applications) {
            if (existing.desktopId == reference.desktopId && existing.windowClass == reference.windowClass)
                return;
        }
        m_applications.append(reference);
        m_application->addItem(label);
    };
    if (const auto *object = selected(); object && object->binding.provider == "application")
        append(object->binding.application, tr("Current · %1").arg(object->binding.application.name));
    const auto catalog = DesktopAppCatalog::applications();
    QString error;
    for (const auto &window : DesktopAppWindow::availableWindows(error)) {
        if (!window.applicationClass.isEmpty()) {
            const auto reference = DesktopAppCatalog::reference(window, catalog);
            append(reference, tr("Running · %1").arg(reference.name));
        }
    }
    for (const auto &application : catalog)
        append({application.desktopId, application.windowClass, application.name}, tr("Installed · %1").arg(application.name));
}
QString FurnishingController::sourceId(const QString &id) const
{
    return m_sources.value(id, id);
}
bool FurnishingController::baseSource(const QString &id) const
{
    return baseDisplay(id) != nullptr;
}
bool FurnishingController::applicationSource(const QString &id) const
{
    for (const auto &object : m_objects) {
        if (!object.removed && object.binding.provider == "application" && FurnishingCatalog::display(object).id == id)
            return true;
    }
    return false;
}
const SpatialDisplay *FurnishingController::baseDisplay(const QString &id) const
{
    for (const auto &display : m_base.displays) {
        if (display.id == id)
            return &display;
    }
    return nullptr;
}
bool FurnishingController::activateScreen(const QString &id)
{
    for (const auto &object : m_objects) {
        if (!object.removed && object.type == "arcade-cabinet" && FurnishingCatalog::marquee(object).id == id) {
            if (!m_active)
                toggle();
            selectObject(object.id);
            configure();
            m_surface->setCurrentIndex(1);
            return true;
        }
        if (object.removed || !FurnishingCatalog::find(object.type)->screen || FurnishingCatalog::display(object).id != id)
            continue;
        if (object.binding.provider == "application" && m_view->applicationDisplayRequested) {
            m_view->applicationDisplayRequested(id, true);
        } else if (object.binding.provider == "surface" && baseSource(object.binding.target)) {
            const auto *source = baseDisplay(object.binding.target);
            if (object.type == "arcade-cabinet" && source && !source->application.windowClass.isEmpty()
                && m_view->applicationDisplayRequested) {
                m_view->applicationDisplayRequested(id, true);
                return true;
            }
            if (m_view->destinationActivated)
                m_view->destinationActivated("display:" + object.binding.target, false);
        } else {
            if (!m_active)
                toggle();
            selectObject(object.id);
            configure();
        }
        return true;
    }
    return false;
}
void FurnishingController::updateHud()
{
    if (m_view->controllerHints()) {
        m_hints->setText(m_configuring
            ? tr("LB/RB: cabinet surface · D-pad ↑/↓: provider · ←/→: source · A: apply · B: cancel · Start: menu")
            : tr("Left stick: move · Right stick: aim · A: place/select · B: cancel\nLB/RB: categories · D-pad ↑/↓: items · ←/→: rotate\nX: move · Y: configure · R3: delete · View: finish · Start: menu"));
    } else {
        m_hints->setText(m_configuring
            ? tr("Page Up/Down: cabinet surface · Tab: next input · Enter: apply · Esc: cancel")
            : tr("WASD: move · Right-drag: aim · Enter: place/select · Esc: cancel\n↑/↓: items · ←/→: rotate · Page Up/Down: categories\nM: move · I: configure · Delete: remove · Ctrl+D: duplicate · F2: finish · F1: menu"));
    }
    const auto *object = m_ghost ? &*m_ghost : selected();
    QString label = object ? FurnishingCatalog::find(object->type)->name : tr("Choose an item or select something in the room");
    QString mode = m_configuring ? tr("CONFIGURE INPUT") : (m_ghost ? tr("PLACING") : tr("SELECT / EDIT"));
    QString status = tr("Layout saved automatically · %1 / %2 objects").arg(m_layout.objects.size()).arg(FurnishingLayout::maximumObjects);
    if (m_ghost) {
        status = m_validity;
        if (status.isEmpty()) {
            status = FurnishingCatalog::modular(*m_ghost)
                ? tr("Ready · 100-unit grid · 90° rotation · repeat placement")
                : tr("Ready to place · 10-unit snap");
        }
    }
    if (!m_error.isEmpty())
        status = m_error;
    QString details;
    if (object && FurnishingCatalog::find(object->type)->screen) {
        QStringList sources;
        for (const auto &display : m_base.displays)
            sources.append(display.id);
        const auto &binding = m_configuring && object->type == "arcade-cabinet" && m_surface->currentIndex() == 1
            ? object->marqueeBinding : object->binding;
        const auto capability = ScreenProviders::capabilities(binding, sources);
        details = "\n" + capability.label + " · " + capability.notice;
    }
    m_info->setText(mode + " · " + label + "\n" + status + details);
    const bool building = m_ghost && FurnishingCatalog::modular(*m_ghost);
    m_buildHeight->setVisible(building && !m_configuring);
    if (building)
        m_hints->setText(m_hints->text() + tr("\nH / left stick click: cycle build height"));
    m_info->setStyleSheet(m_ghost && !m_validity.isEmpty() ? "color: #ff9b93;" : "color: #a2ddcf;");
}
void FurnishingController::refreshInputHints()
{
    if (m_active)
        updateHud();
}
void FurnishingController::cycleBuildHeight()
{
    if (!m_ghost || !FurnishingCatalog::modular(*m_ghost))
        return;
    const QVector<int> heights{-1, 0, 9, 310, 610};
    const int next = (heights.indexOf(m_buildHeight->value()) + 1) % heights.size();
    m_buildHeight->setValue(heights[next]);
}
bool FurnishingController::key(QKeyEvent *event)
{
    if (event->key() == Qt::Key_F2) {
        if (!event->isAutoRepeat())
            toggle();
        return true;
    }
    if (!m_active)
        return false;
    if (m_configuring && m_surface->isVisible() && (event->key() == Qt::Key_PageUp || event->key() == Qt::Key_PageDown)) {
        m_surface->setCurrentIndex(1 - m_surface->currentIndex());
        return true;
    }
    switch (event->key()) {
    case Qt::Key_Escape: cancel(); break;
    case Qt::Key_Return: case Qt::Key_Enter: case Qt::Key_E: confirm(); break;
    case Qt::Key_R: rotate(event->modifiers() & Qt::ShiftModifier ? -15 : 15); break;
    case Qt::Key_M: moveSelected(); break;
    case Qt::Key_I: configure(); break;
    case Qt::Key_H: cycleBuildHeight(); break;
    case Qt::Key_Delete: removeSelected(); break;
    case Qt::Key_Left: rotate(-15); break;
    case Qt::Key_Right: rotate(15); break;
    case Qt::Key_Up: browse(0, -1); break;
    case Qt::Key_Down: browse(0, 1); break;
    case Qt::Key_PageUp: browse(-1, 0); break;
    case Qt::Key_PageDown: browse(1, 0); break;
    case Qt::Key_D:
        if (!(event->modifiers() & Qt::ControlModifier))
            return false;
        duplicateSelected();
        break;
    default: return false;
    }
    return true;
}
bool FurnishingController::button(int button, bool pressed)
{
    if (button == 4) {
        if (pressed)
            toggle();
        return true;
    }
    if (!m_active)
        return false;
    if (!pressed)
        return true;
    m_mouseTarget = false;
    if (m_configuring && m_surface->isVisible() && (button == 9 || button == 10)) {
        m_surface->setCurrentIndex(1 - m_surface->currentIndex());
    } else if (m_configuring && (button >= 11 && button <= 14)) {
        auto *combo = button < 13 ? m_provider : m_source;
        if (button >= 13 && m_provider->currentData() == "application")
            combo = m_application;
        const int delta = button == 11 || button == 13 ? -1 : 1;
        if (combo->count())
            combo->setCurrentIndex((combo->currentIndex() + delta + combo->count()) % combo->count());
    } else {
        switch (button) {
        case 0: confirm(); break;
        case 1: cancel(); break;
        case 2: moveSelected(); break;
        case 3: configure(); break;
        case 7: cycleBuildHeight(); break;
        case 8: removeSelected(); break;
        case 9: browse(-1, 0); break;
        case 10: browse(1, 0); break;
        case 11: browse(0, -1); break;
        case 12: browse(0, 1); break;
        case 13: rotate(-15); break;
        case 14: rotate(15); break;
        default: break;
        }
    }
    return true;
}

void FurnishingController::rebuild(bool refreshSources)
{
    if (!m_view->m_interior)
        return;
    m_suppressed.clear();
    m_sources.clear();
    const auto previousGeometry = m_view->m_furnishingGeometry;
    const int previousStaticFaces = m_view->m_furnishingStaticFaces;
    m_view->m_furnishingGeometry.clear();
    auto &room = *m_view->m_interior;
    room.solids = m_base.solids;
    room.displays = m_base.displays;
    room.actions = m_base.actions;
    const auto appendFace = [&](const QVector<QVector3D> &vertices, const QColor &color, const QString &display = QString()) {
        WorkspaceFlightView::MeshFace face;
        face.vertices = vertices;
        for (const auto &vertex : vertices)
            face.center += vertex;
        face.center /= vertices.size();
        face.color = color;
        face.edge = color;
        face.displayId = display;
        m_view->m_furnishingGeometry.append(face);
    };
    auto drawParts = [&](const FurnishingObject &object, bool ghost, bool selected) {
        const auto mesh = FurnishingCatalog::mesh(object);
        if (!mesh.faces.isEmpty()) {
            const auto sunlight = QVector3D(-0.4f, 0.85f, -0.3f).normalized();
            for (int index = 0; index < mesh.faces.size(); ++index) {
                QVector<QVector3D> polygon;
                for (int vertex : mesh.faces[index])
                    polygon.append(mesh.vertices[vertex]);
                auto color = mesh.faceColors.value(index, mesh.color);
                if (ghost) {
                    color = m_validity.isEmpty() ? QColor(79, 225, 178, 130) : QColor(248, 102, 107, 130);
                } else {
                    const auto normal = QVector3D::crossProduct(polygon[1] - polygon[0], polygon[2] - polygon[0]).normalized();
                    const float light = 0.65f + 0.35f * qMax(0.0f, QVector3D::dotProduct(normal, sunlight));
                    color = QColor::fromRgbF(color.redF() * light, color.greenF() * light, color.blueF() * light);
                    if (selected)
                        color = color.lighter(140);
                }
                appendFace(polygon, color);
            }
            return;
        }
        for (auto part : FurnishingCatalog::parts(object)) {
            const auto half = part.size / 2;
            auto color = part.color;
            if (ghost)
                color = m_validity.isEmpty() ? QColor(79, 225, 178, 130) : QColor(248, 102, 107, 130);
            else if (selected)
                color = color.lighter(140);
            QVector<QVector3D> vertices;
            for (int y : {-1, 1}) {
                for (int z : {-1, 1}) {
                    for (int x : {-1, 1})
                        vertices.append(part.center + FurnishingCatalog::rotate({x * half.x(), y * half.y(), z * half.z()}, part.yaw));
                }
            }
            if (part.shape == SpatialSolidShape::Octahedron) {
                const QVector3D top = part.center + QVector3D(0, half.y(), 0);
                const QVector3D bottom = part.center - QVector3D(0, half.y(), 0);
                const QVector<QVector3D> ring{part.center + FurnishingCatalog::rotate({-half.x(), 0, 0}, part.yaw),
                    part.center + FurnishingCatalog::rotate({0, 0, half.z()}, part.yaw),
                    part.center + FurnishingCatalog::rotate({half.x(), 0, 0}, part.yaw),
                    part.center + FurnishingCatalog::rotate({0, 0, -half.z()}, part.yaw)};
                for (int side = 0; side < 4; ++side) {
                    appendFace({ring[side], ring[(side + 1) % 4], top}, color.lighter(100 + side * 8));
                    appendFace({ring[(side + 1) % 4], ring[side], bottom}, color.darker(120));
                }
            } else {
                for (const auto &face : QVector<QVector<int>>{{0, 1, 5, 4}, {3, 2, 6, 7}, {2, 0, 4, 6}, {1, 3, 7, 5}, {4, 5, 7, 6}, {2, 3, 1, 0}}) {
                    QVector<QVector3D> polygon;
                    for (int index : face)
                        polygon.append(vertices[index]);
                    appendFace(polygon, color);
                }
            }
        }
    };
    QStringList sceneSources;
    for (const auto &display : m_base.displays)
        sceneSources.append(display.id);
    int partCount = 0;
    int faceCount = 0;
    for (const auto &object : m_objects) {
        const bool editedBase = !object.baseDisplay.isEmpty() && std::any_of(m_layout.objects.cbegin(), m_layout.objects.cend(),
            [&](const auto &saved) { return saved.id == object.id; });
        if (editedBase) {
            m_suppressed.insert(object.baseDisplay);
            room.displays.removeIf([&](const auto &display) { return display.id == object.baseDisplay; });
        }
        if (object.removed)
            continue;
        if (m_ghost && m_ghost->id == object.id) {
            if (!object.baseDisplay.isEmpty()) {
                m_suppressed.insert(object.baseDisplay);
                room.displays.removeIf([&](const auto &display) { return display.id == object.baseDisplay; });
            }
            continue;
        }
        const auto *item = FurnishingCatalog::find(object.type);
        if (!item)
            continue;
        auto display = FurnishingCatalog::display(object);
        if (object.binding.provider == "surface") {
            if (const auto *original = baseDisplay(object.binding.target))
                display.application = original->application;
        }
        if (item->screen && object.binding.provider == "surface" && sceneSources.contains(object.binding.target))
            m_sources[display.id] = object.binding.target;
        if (item->screen && object.binding.provider == "application")
            m_sources[display.id] = display.id;
        if (!object.baseDisplay.isEmpty() && !editedBase)
            continue;
        const auto parts = FurnishingCatalog::parts(object);
        const auto actions = FurnishingCatalog::actions(object);
        const int objectFaces = FurnishingCatalog::faceCount(object);
        if (partCount + parts.size() > FurnishingLayout::maximumParts
            || faceCount + objectFaces > SpatialAssetLibrary::maximumSceneFaces
            || room.displays.size() + FurnishingCatalog::screenCount(object) > SpatialWorkspace::maximumDisplays
            || room.actions.size() + actions.size() > SpatialInterior::maximumActions) {
            m_error = tr("Scene capacity reached; excess saved furnishings are retained but hidden.");
            continue;
        }
        partCount += parts.size();
        faceCount += objectFaces;
        room.solids += parts;
        room.actions += actions;
        drawParts(object, false, m_active && object.id == m_selected);
        if (!item->screen)
            continue;
        room.displays.append(display);
        if (object.type == "arcade-cabinet") {
            const auto marquee = FurnishingCatalog::marquee(object);
            room.displays.append(marquee);
            appendFace(SpatialInteriorGeometry::surface(room, marquee), QColor("#102330"), marquee.id);
            const QString contentId = "furnishing-content:" + object.id + ":marquee";
            m_sources[marquee.id] = contentId;
            if (refreshSources || !m_view->m_interiorImages.contains(contentId))
                m_view->m_interiorImages[contentId] = ScreenProviders::marquee(object.marqueeBinding);
        }
        appendFace(SpatialInteriorGeometry::surface(room, display), QColor("#102330"), display.id);
        if (!m_sources.contains(display.id)) {
            const QString contentId = "furnishing-content:" + object.id;
            m_sources[display.id] = contentId;
            if (refreshSources || !m_view->m_interiorImages.contains(contentId))
                m_view->m_interiorImages[contentId] = ScreenProviders::preview(object.binding, ScreenProviders::capabilities(object.binding, sceneSources));
        }
    }
    QSet<QString> retainedImages;
    for (const auto &object : m_objects) {
        if (!object.removed)
            retainedImages.insert("furnishing-content:" + object.id);
        if (!object.removed && object.type == "arcade-cabinet")
            retainedImages.insert("furnishing-content:" + object.id + ":marquee");
    }
    for (auto image = m_view->m_interiorImages.begin(); image != m_view->m_interiorImages.end();) {
        if (image.key().startsWith("furnishing-content:") && !retainedImages.contains(image.key()))
            image = m_view->m_interiorImages.erase(image);
        else
            ++image;
    }
    m_view->m_furnishingStaticFaces = m_view->m_furnishingGeometry.size();
    if (previousStaticFaces != m_view->m_furnishingStaticFaces) {
        m_view->m_interiorPaintDirty = true;
    } else if (!m_view->m_interiorPaintDirty) {
        for (int index = 0; index < previousStaticFaces; ++index) {
            if (previousGeometry[index].vertices != m_view->m_furnishingGeometry[index].vertices) {
                m_view->m_interiorPaintDirty = true;
                break;
            }
        }
    }
    // The moving placement preview traverses the cached tree like a vehicle;
    // aiming a ghost must not rebuild the whole room's spatial partition.
    if (m_ghost)
        drawParts(*m_ghost, true, false);
    m_view->viewport()->update();
}
