#include "DashboardPage.h"
#include "DashboardWidgetFactory.h"
#include "DashboardRepositoryPanel.h"
#include "DashboardFilePreview.h"
#include "DashboardGitPreview.h"
#include "DashboardWorkspace.h"
#include "WorkspaceMachineMenu.h"

#include <QHBoxLayout>
#include <QLayout>
#include <QPushButton>
#include <QVBoxLayout>

QJsonObject DashboardPage::resolveMachineCatalog(const QJsonObject &machine) const
{
    const QString mode = machine.value("mode").toString("follow");
    if (mode == "follow")
        return m_factory->sharedMachineCatalog();
    if (mode == "local")
        return {};
    auto target = machine.value("target").toObject();
    if (target.isEmpty())
        target.insert("node", "unavailable");
    for (const auto &entry : m_factory->machineCatalog()) {
        if (entry.toObject().value("target").toObject() == target)
            return entry.toObject();
    }
    // Preserve an unavailable worker generation. Discovery never upgrades a
    // persisted target to a different worker or makes it a local selection.
    return {{"target", target}};
}

QString DashboardPage::resolveMachineRoot(const QJsonObject &machine) const
{
    return machine.value("mode").toString("follow") == "follow"
        ? m_factory->sharedMachineRootId() : machine.value("rootId").toString();
}

bool DashboardPage::canChangeSharedMachineContext(QString *error) const
{
    if (!m_factory->canChangeSharedMachineContext(error))
        return false;
    for (const auto &panel : m_definition.panels) {
        auto *frame = workspaceFor(panel.environment)->findChild<QWidget *>("dashboardPanel_" + panel.id);
        if (!frame)
            continue;
        auto *view = frame->findChild<DashboardRepositoryPanel *>();
        if (!view || view->property("dashboardMachineContext").toJsonObject().value("mode").toString("follow") != "follow")
            continue;
        if (!view->canChangeMachineContext(error))
            return false;
    }
    return true;
}

bool DashboardPage::setSharedMachineContext(const QJsonObject &catalog, const QString &rootId, QString *error)
{
    if (property("dashboardSharedMachineConfigured").toBool()
        && property("dashboardSharedMachineCatalog").toJsonObject() == catalog
        && property("dashboardSharedMachineRootId").toString() == rootId)
        return true;
    if (!canChangeSharedMachineContext(error) || !m_factory->setSharedMachineContext(catalog, rootId, error))
        return false;
    for (const auto &panel : m_definition.panels) {
        auto *frame = workspaceFor(panel.environment)->findChild<QWidget *>("dashboardPanel_" + panel.id);
        auto *view = frame ? frame->findChild<DashboardRepositoryPanel *>() : nullptr;
        if (!view)
            continue;
        const auto machine = view->property("dashboardMachineContext").toJsonObject();
        if (machine.value("mode").toString("follow") == "follow"
            && !applyPanelMachineContext(view, machine, error))
            return false;
    }
    setProperty("dashboardSharedMachineConfigured", true);
    setProperty("dashboardSharedMachineCatalog", catalog);
    setProperty("dashboardSharedMachineRootId", rootId);
    scheduleSave();
    return true;
}

void DashboardPage::setMachineCatalog(const QJsonArray &catalog)
{
    m_factory->setMachineCatalog(catalog);
    for (auto *menu : findChildren<WorkspaceMachineMenu *>())
        menu->setCatalog(catalog);
}

bool DashboardPage::applyPanelMachineContext(QWidget *content, const QJsonObject &machine, QString *error)
{
    auto *view = qobject_cast<DashboardRepositoryPanel *>(content);
    if (!view || !view->canChangeMachineContext(error))
        return false;
    const auto catalog = resolveMachineCatalog(machine);
    const auto root = resolveMachineRoot(machine);
    if (!view->setMachineContext(catalog, root, error))
        return false;
    auto stored = machine;
    stored.insert("mode", machine.value("mode").toString("follow"));
    if (!catalog.isEmpty()) {
        stored.insert("target", catalog.value("target"));
        stored.insert("rootId", root);
    } else {
        stored.remove("target");
        stored.remove("rootId");
    }
    content->setProperty("dashboardMachineContext", stored);
    if (auto *menu = content->findChild<WorkspaceMachineMenu *>("dashboardPanelMachine")) {
        menu->setCurrent(catalog.value("target").toObject(), root);
        menu->setWidgetMode(true, stored.value("mode") == QStringLiteral("follow"));
    }
    return true;
}

void DashboardPage::installMachineMenu(QWidget *content, const QString &kind, const QJsonObject &machine)
{
    const auto selected = resolveMachineCatalog(machine);
    const auto root = resolveMachineRoot(machine);
    const bool pinned = kind == "file-preview" || kind == "git-preview";
    QString mode = machine.value("mode").toString("follow");
    if (pinned)
        mode = selected.isEmpty() ? QStringLiteral("local") : QStringLiteral("remote");
    auto stored = machine;
    stored.insert("mode", mode);
    if (!selected.isEmpty()) {
        stored.insert("target", selected.value("target"));
        stored.insert("rootId", root);
    } else {
        stored.remove("target");
        stored.remove("rootId");
    }
    content->setProperty("dashboardMachineContext", stored);
    auto *menu = new WorkspaceMachineMenu(content);
    menu->setObjectName(QStringLiteral("dashboardPanelMachine"));
    menu->setCatalog(m_factory->machineCatalog());
    menu->setAccessUnlocked(true);
    menu->setCurrent(selected.value("target").toObject(), root);
    menu->setWidgetMode(!pinned, mode == "follow");
    menu->setEnabled(!pinned);
    if (auto *column = qobject_cast<QVBoxLayout *>(content->layout())) {
        auto *first = column->itemAt(0);
        if (auto *toolbar = first ? qobject_cast<QHBoxLayout *>(first->layout()) : nullptr)
            toolbar->insertWidget(0, menu);
        else
            column->insertWidget(0, menu, 0, Qt::AlignLeft);
    }
    if (pinned)
        return;
    connect(menu, &WorkspaceMachineMenu::contextRequested, content,
        [this, content](const QJsonObject &target, const QString &rootId) {
        QString error;
        const QJsonObject requested{{"mode", target.isEmpty() ? "local" : "remote"},
            {"target", target}, {"rootId", rootId}};
        if (applyPanelMachineContext(content, requested, &error))
            scheduleSave();
        else
            showSaveError(error);
    });
    connect(menu, &WorkspaceMachineMenu::followMainRequested, content, [this, content] {
        QString error;
        if (applyPanelMachineContext(content, {{"mode", "follow"}}, &error))
            scheduleSave();
        else
            showSaveError(error);
    });
    connect(menu, &WorkspaceMachineMenu::refreshRequested, m_factory, &DashboardWidgetFactory::machineCatalogRefreshRequested);
}
