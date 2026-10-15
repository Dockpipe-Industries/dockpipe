#include "CockpitCommandDeck.h"
#include "CanvasIcons.h"
#include <QHBoxLayout>
#include <QResizeEvent>
#include <QToolButton>

CockpitCommandDeck::CockpitCommandDeck(QWidget *parent) : QFrame(parent)
{
    setObjectName("cockpitCommandDeck");
    setStyleSheet(
        "QFrame#cockpitCommandDeck { background: #080e15; border: 1px solid #3b4a51; border-top: 2px solid #617c80; border-radius: 8px; }"
        "QToolButton { color: #9eb1b8; background: #101922; border: 1px solid #29363f; border-radius: 4px; padding: 5px 8px; font-size: 11px; }"
        "QToolButton:hover { color: #e8f4ee; background: #1a3039; border-color: #648e90; }"
        "QToolButton:checked { color: #c4f5e6; background: #193b3d; border-color: #7dbcb4; }"
        "QToolButton:focus { border: 2px solid #b5e4d7; }");
    auto *layout = new QHBoxLayout(this);
    layout->setContentsMargins(10, 8, 10, 8);
    layout->setSpacing(6);
    struct Destination {
        QString key;
        QString name;
        CanvasIcons::Symbol icon;
    };
    const QVector<Destination> destinations{{"files", tr("Files"), CanvasIcons::Symbol::Folder},
        {"apps", tr("Apps"), CanvasIcons::Symbol::Monitor},
        {"workflows", tr("Workflows"), CanvasIcons::Symbol::Workflow},
        {"containers", tr("Containers"), CanvasIcons::Symbol::Parallel},
        {"settings", tr("Settings"), CanvasIcons::Symbol::Properties},
        {"machines", tr("Machines"), CanvasIcons::Symbol::Terminal},
        {"executions", tr("Executions"), CanvasIcons::Symbol::Play}};
    for (const auto &destination : destinations) {
        auto *button = new QToolButton(this);
        button->setObjectName("cockpitRoute_" + destination.key);
        button->setProperty("destination", destination.key);
        button->setText(destination.name);
        button->setAccessibleName(tr("Navigate to %1").arg(destination.name));
        button->setToolTip(tr("Navigate to %1 · autopilot from orbit").arg(destination.name));
        button->setIcon(CanvasIcons::icon(destination.icon));
        button->setIconSize(QSize(16, 16));
        button->setToolButtonStyle(Qt::ToolButtonTextBesideIcon);
        button->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Preferred);
        button->setCheckable(true);
        connect(button, &QToolButton::clicked, this, [this, key = destination.key]() {
            emit destinationRequested(key);
        });
        m_buttons.append(button);
        layout->addWidget(button);
    }
}

void CockpitCommandDeck::setDestination(const QString &destination)
{
    for (auto *button : m_buttons)
        button->setChecked(button->property("destination").toString() == destination);
}

void CockpitCommandDeck::resizeEvent(QResizeEvent *event)
{
    QFrame::resizeEvent(event);
    for (auto *button : m_buttons)
        button->setToolButtonStyle(width() < 760 ? Qt::ToolButtonIconOnly : Qt::ToolButtonTextBesideIcon);
}
