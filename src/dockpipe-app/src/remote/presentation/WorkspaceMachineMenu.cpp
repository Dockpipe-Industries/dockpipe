#include "WorkspaceMachineMenu.h"

#include <QAction>
#include <QMenu>
#include <QSet>

WorkspaceMachineMenu::WorkspaceMachineMenu(QWidget *parent) : QToolButton(parent)
{
    setObjectName(QStringLiteral("workspaceMachineMenu"));
    setPopupMode(QToolButton::InstantPopup);
    setToolButtonStyle(Qt::ToolButtonTextOnly);
    auto *choices = new QMenu(this);
    setMenu(choices);
    connect(choices, &QMenu::aboutToShow, this, &WorkspaceMachineMenu::rebuildMenu);
    updateLabel();
}

void WorkspaceMachineMenu::setNodeOnlyMode(bool enabled)
{
    m_nodeOnly = enabled;
}

void WorkspaceMachineMenu::setCatalog(const QJsonArray &catalog)
{
    // Core validates catalogs. Bound the presentation independently so a broken
    // local client cannot construct an unbounded menu.
    m_catalog = {};
    for (qsizetype index = 0; index < catalog.size() && index < 64; ++index)
        m_catalog.append(catalog.at(index));
    updateLabel();
}

void WorkspaceMachineMenu::setPairedNodes(const QStringList &nodes)
{
    m_pairedNodes = nodes.mid(0, 64);
    m_pairedNodes.removeDuplicates();
}

void WorkspaceMachineMenu::setCurrent(const QJsonObject &target, const QString &root)
{
    m_target = target;
    m_root = root;
    updateLabel();
}

void WorkspaceMachineMenu::setAccessUnlocked(bool unlocked)
{
    m_unlocked = unlocked;
    updateLabel();
}

void WorkspaceMachineMenu::setWidgetMode(bool enabled, bool followsMain)
{
    m_widgetMode = enabled;
    m_followsMain = enabled && followsMain;
    updateLabel();
}

void WorkspaceMachineMenu::updateLabel()
{
    if (m_widgetMode && m_followsMain) {
        setText(tr("Follow main machine"));
        setToolTip(tr("Shared machine selection. Dashboard widgets can choose their own machine."));
        return;
    }
    if (m_target.isEmpty()) {
        setText(tr("This computer"));
        setToolTip(tr("Choose the machine for Containers, Files, Git, Terminal and the dashboard."));
        return;
    }
    bool available = false;
    QString rootName = m_root;
    for (const auto &value : m_catalog) {
        const auto catalog = value.toObject();
        if (catalog.value(QStringLiteral("target")).toObject() != m_target)
            continue;
        const auto roots = catalog.value(QStringLiteral("roots")).toArray();
        if (m_root.isEmpty())
            available = !m_nodeOnly || catalog.value("enabled").toBool();
        for (const auto &rootValue : roots) {
            const auto root = rootValue.toObject();
            if (root.value(QStringLiteral("id")).toString() == m_root) {
                available = true;
                rootName = root.value(QStringLiteral("name")).toString();
                break;
            }
        }
    }
    QString label = m_target.value(QStringLiteral("node")).toString();
    if (!rootName.isEmpty())
        label += QStringLiteral(" · ") + rootName;
    if (!m_unlocked)
        label += tr(" · Locked");
    else if (!available)
        label += tr(" · Unavailable");
    setText(label);
    setToolTip(tr("Shared machine selection. Dashboard widgets can choose their own machine."));
}

void WorkspaceMachineMenu::rebuildMenu()
{
    menu()->clear();
    if (m_widgetMode) {
        auto *follow = menu()->addAction(tr("Follow main machine"));
        follow->setCheckable(true);
        follow->setChecked(m_followsMain);
        connect(follow, &QAction::triggered, this, &WorkspaceMachineMenu::followMainRequested);
    }
    auto *local = menu()->addAction(tr("This computer"));
    local->setCheckable(true);
    local->setChecked(m_target.isEmpty() && !m_followsMain);
    connect(local, &QAction::triggered, this, [this] { emit contextRequested({}, {}); });
    menu()->addSeparator();
    QSet<QString> shown;
    if (!m_unlocked) {
        auto *unlock = menu()->addAction(tr("Unlock remote access…"));
        connect(unlock, &QAction::triggered, this, &WorkspaceMachineMenu::unlockRequested);
    } else if (m_catalog.isEmpty() && m_pairedNodes.isEmpty()) {
        menu()->addAction(tr("No remote workspaces available"))->setEnabled(false);
    } else {
        for (const auto &value : m_catalog) {
            const auto catalog = value.toObject();
            if (catalog.value("workflow_only").toBool())
                continue;
            const auto target = catalog.value(QStringLiteral("target")).toObject();
            QString node = target.value(QStringLiteral("node")).toString();
            if (node.isEmpty())
                continue;
            if (m_nodeOnly) {
                shown.insert(node);
                auto *action = menu()->addAction(node.replace('&', "&&"));
                action->setCheckable(true);
                action->setChecked(target == m_target);
                if (!catalog.value("enabled").toBool())
                    action->setText(action->text() + tr(" · Unavailable"));
                connect(action, &QAction::triggered, this, [this, target] { emit contextRequested(target, {}); });
                continue;
            }
            shown.insert(node);
            auto *machine = menu()->addMenu(node);
            const auto roots = catalog.value(QStringLiteral("roots")).toArray();
            for (qsizetype index = 0; index < roots.size() && index < 32; ++index) {
                const auto root = roots.at(index).toObject();
                const QString id = root.value(QStringLiteral("id")).toString();
                if (id.isEmpty())
                    continue;
                auto *action = machine->addAction(root.value(QStringLiteral("name")).toString().replace('&', "&&"));
                action->setCheckable(true);
                action->setChecked(target == m_target && id == m_root);
                connect(action, &QAction::triggered, this, [this, target, id] { emit contextRequested(target, id); });
            }
            if (roots.isEmpty() && catalog.value(QStringLiteral("docker_read")).toBool()) {
                auto *action = machine->addAction(tr("Containers"));
                connect(action, &QAction::triggered, this, [this, target] { emit contextRequested(target, {}); });
            }
            if (machine->actions().isEmpty()) {
                machine->setTitle(machine->title() + tr(" · Unavailable"));
                machine->setEnabled(false);
            }
        }
    }
    for (const auto &node : m_pairedNodes) {
        if (node.isEmpty() || shown.contains(node))
            continue;
        const auto label = m_unlocked ? tr("%1 · Unavailable") : tr("%1 · Locked");
        menu()->addAction(label.arg(QString(node).replace('&', "&&")))->setEnabled(false);
    }
    menu()->addSeparator();
    auto *refresh = menu()->addAction(tr("Refresh machines"));
    connect(refresh, &QAction::triggered, this, &WorkspaceMachineMenu::refreshRequested);
}
