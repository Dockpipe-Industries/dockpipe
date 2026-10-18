#include "CockpitWidget.h"
#include "CockpitHudPanel.h"
#include "SpatialOperationsWorld.h"
#include <QComboBox>
#include <QCryptographicHash>
#include <QFormLayout>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QSettings>

namespace {
QString profileKey(const QString &id)
{
    return "cockpit/androids/" + QString::fromLatin1(QCryptographicHash::hash(id.toUtf8(), QCryptographicHash::Sha256).toHex());
}
const QStringList androidColors{"#65cfcc", "#e4b96b", "#c09be7", "#d98b95", "#a3c4df"};
const QStringList androidBodies{"Explorer", "Heavy utility", "Research"};
}

void CockpitWidget::setResourceSection(const QString &section)
{
    const QString previous = m_resourceSection;
    m_resourceTimer.stop();
    m_resourceSection = section;
    m_resourcePage = 0;
    m_machineId.clear();
    m_insideMachine = false;
    m_resourceSnapshot = {};
    if (!previous.isEmpty())
        emit panelActivityChanged(previous, false);
    if (!section.isEmpty() && isVisible()) {
        emit panelActivityChanged(section, true);
        m_resourceTimer.start();
    }
}

void CockpitWidget::refreshResources(bool force)
{
    if (m_resourceSection.isEmpty() || m_content || window()->isMinimized())
        return;
    const QJsonObject snapshot = resourceSnapshot ? resourceSnapshot(m_resourceSection) : QJsonObject();
    if (!force && (m_flight->moving() || m_flight->travelling()))
        return;
    if (!force && snapshot == m_resourceSnapshot)
        return;
    const bool preservePosition = !force && m_flight->walking();
    m_resourceSnapshot = snapshot;
    if (m_resourceSection == "machines") {
        if (!m_machineId.isEmpty()) {
            showMachine(m_insideMachine);
            return;
        }
        const auto machines = SpatialOperationsWorld::machines(snapshot);
        const int pages = qMax(1, (int(machines.size()) + 11) / 12);
        m_resourcePage = qBound(0, m_resourcePage, pages - 1);
        QVector<WorkspaceFlightBuilding> planets;
        for (int index = m_resourcePage * 12; index < qMin(int(machines.size()), (m_resourcePage + 1) * 12); ++index) {
            const auto &machine = machines[index];
            WorkspaceFlightBuilding planet{machine.value("title").toString(), 100, true, false,
                "machine-planet:" + machine.value("id").toString()};
            planet.theme = index % 2 == 0 ? "ocean" : "forest";
            planets.append(planet);
        }
        if (m_resourcePage > 0)
            planets.append({tr("Previous sector"), 100, true, false, "operations:previous"});
        if (m_resourcePage + 1 < pages)
            planets.append({tr("Next sector"), 100, true, false, "operations:next"});
        planets.append({tr("Machine controls"), 100, true, false, "operations:controls"});
        m_flight->setCockpitWorld(planets, true);
        m_status->setText(tr("Machine system · Sector %1/%2 · Each machine has a laboratory and a customizable android. Pairing status does not prove that a machine is online. %3")
            .arg(m_resourcePage + 1).arg(pages).arg(snapshot.value("notice").toString()));
        return;
    }
    const auto scene = SpatialOperationsWorld::campus(m_resourceSection, snapshot, m_resourcePage);
    m_resourcePage = scene.page;
    m_flight->setInterior(scene.interior, m_resourceSection == "containers" ? tr("CONTAINER INDUSTRIAL DISTRICT") : tr("EXECUTION MISSION CAMPUS"),
        preservePosition, preservePosition, "operations:" + m_root + ":" + m_resourceSection + ":" + QString::number(scene.page));
    for (auto image = scene.signs.cbegin(); image != scene.signs.cend(); ++image)
        m_flight->setInteriorImage(image.key(), image.value());
    m_status->setText(scene.notice);
    m_flight->setFlightStatus(scene.notice);
}

bool CockpitWidget::activateOperation(const QString &path)
{
    if (m_resourceSection.isEmpty())
        return false;
    QString action = path;
    if (action.startsWith("display:"))
        action = action.mid(8);
    if (action.startsWith("machine-planet:")) {
        m_machineId = action.mid(15);
        showMachine(false);
    } else if (action == "machine:enter") {
        showMachine(true);
    } else if (action == "machine:customize") {
        customizeAndroid();
    } else if (action == "operations:next" || action == "operations:previous") {
        m_resourcePage += action == "operations:next" ? 1 : -1;
        refreshResources(true);
    } else if (action == "operations:controls" || action == "operations:machine-controls" || action.startsWith("resource:")) {
        const QString id = action.startsWith("resource:") ? action.mid(9) : m_machineId;
        if (pageProvider)
            openPanel(m_resourceSection, pageProvider(m_resourceSection), false);
        if (!id.isEmpty() && resourceSelected)
            resourceSelected(m_resourceSection, id);
    } else {
        return false;
    }
    return true;
}

void CockpitWidget::showMachine(bool inside)
{
    const auto machines = SpatialOperationsWorld::machines(m_resourceSnapshot);
    QJsonObject machine;
    for (const auto &candidate : machines) {
        if (candidate.value("id").toString() == m_machineId) {
            machine = candidate;
            break;
        }
    }
    if (machine.isEmpty()) {
        m_machineId.clear();
        refreshResources(true);
        return;
    }
    const bool preservePosition = m_insideMachine == inside && m_flight->walking();
    m_insideMachine = inside;
    QSettings settings;
    const QString key = profileKey(m_machineId);
    const int color = qBound(0, settings.value(key + "/color", 0).toInt(), int(androidColors.size()) - 1);
    const int body = qBound(0, settings.value(key + "/body", 0).toInt(), int(androidBodies.size()) - 1);
    const auto scene = SpatialOperationsWorld::laboratory(machine, inside,
        settings.value(key + "/name").toString().left(48), QColor(androidColors[color]), body);
    m_flight->setInterior(scene.interior, machine.value("title").toString(), preservePosition, preservePosition,
        "machine:" + m_root + ":" + m_machineId + (inside ? ":inside" : ":outside"));
    for (auto image = scene.signs.cbegin(); image != scene.signs.cend(); ++image)
        m_flight->setInteriorImage(image.key(), image.value());
    m_status->setText(scene.notice);
    m_flight->setFlightStatus(scene.notice);
    m_flight->setFocus();
}

void CockpitWidget::customizeAndroid()
{
    if (m_machineId.isEmpty())
        return;
    QSettings settings;
    const QString key = profileKey(m_machineId);
    auto *editor = new QWidget;
    auto *layout = new QFormLayout(editor);
    auto *name = new QLineEdit(settings.value(key + "/name").toString().left(48));
    name->setObjectName("androidName");
    name->setMaxLength(48);
    auto *color = new QComboBox;
    color->setObjectName("androidColor");
    color->addItems({tr("Teal"), tr("Gold"), tr("Violet"), tr("Rose"), tr("Ice")});
    color->setCurrentIndex(qBound(0, settings.value(key + "/color", 0).toInt(), 4));
    auto *body = new QComboBox;
    body->setObjectName("androidBody");
    body->addItems(androidBodies);
    body->setCurrentIndex(qBound(0, settings.value(key + "/body", 0).toInt(), 2));
    auto *save = new QPushButton(tr("Save appearance"));
    save->setObjectName("saveAndroid");
    auto *status = new QLabel(tr("Stored on this computer. Does not change the machine or its access."));
    status->setWordWrap(true);
    layout->addRow(tr("Android name"), name);
    layout->addRow(tr("Finish"), color);
    layout->addRow(tr("Chassis"), body);
    layout->addRow(status);
    layout->addRow(save);
    connect(save, &QPushButton::clicked, editor, [this, key, name, color, body, status]() {
        QSettings preferences;
        preferences.beginGroup("cockpit/androids");
        const auto profiles = preferences.childGroups();
        preferences.endGroup();
        if (!preferences.contains(key + "/body") && profiles.size() >= 256) {
            status->setText(tr("The local limit of 256 android appearances has been reached."));
            return;
        }
        preferences.setValue(key + "/name", name->text().trimmed());
        preferences.setValue(key + "/color", color->currentIndex());
        preferences.setValue(key + "/body", body->currentIndex());
        preferences.sync();
        if (preferences.status() != QSettings::NoError) {
            status->setText(tr("Could not save appearance to local settings."));
            return;
        }
        closePanel();
        showMachine(true);
    });
    openPanel("ephemeral", editor, false);
    m_panel->setTitle(tr("MACHINE LAB / ANDROID APPEARANCE"));
}
