#include "DashboardWidgetFactory.h"
#include "DashboardFilesWidget.h"
#include "ProjectFilesWidget.h"
#include "RemoteGitContext.h"
#include "WorkspaceMachineMenu.h"
#include "TerminalWidget.h"
#include "SendInputWidget.h"

#include <QCoreApplication>
#include <QJsonDocument>
#include <QLabel>
#include <QLineEdit>
#include <QProcess>
#include <QPushButton>
#include <QTimer>

namespace {
DashboardWidgetFactory *owningFactory(QWidget *widget)
{
    for (QObject *current = widget; current; current = current->parent()) {
        auto *factory = qobject_cast<DashboardWidgetFactory *>(current->property("dashboardMachineFactory").value<QObject *>());
        if (factory)
            return factory;
    }
    return widget->findChild<DashboardWidgetFactory *>();
}

QString guardedChangeMessage()
{
    return QCoreApplication::translate("DashboardWidgetFactory", "Save open drafts and wait for active operations before changing machines.");
}
}

QJsonObject DashboardWidgetFactory::machineTarget() const
{
    auto target = m_sharedMachine.value("target").toObject();
    if (!m_sharedMachine.isEmpty() && target.isEmpty())
        target.insert("node", "unavailable");
    return target;
}

bool DashboardWidgetFactory::canChangeWidgetMachineContext(QWidget *widget, QString *error)
{
    if (error)
        error->clear();
    for (const auto *files : widget->findChildren<ProjectFilesWidget *>()) {
        if (files->hasUnsavedChanges() || files->filesOperationPending()) {
            if (error)
                *error = guardedChangeMessage();
            return false;
        }
    }
    if (const auto *input = widget->findChild<SendInputWidget *>(); input && input->hasPendingInput()) {
        if (error)
            *error = guardedChangeMessage();
        return false;
    }
    if (widget->property("dashboardKind").toString().isEmpty()) {
        for (const auto *client : widget->findChildren<ProjectGitClient *>()) {
            if (client->busy()) {
                if (error)
                    *error = guardedChangeMessage();
                return false;
            }
        }
        for (const auto *process : widget->findChildren<QProcess *>()) {
            if (process->state() != QProcess::NotRunning) {
                if (error)
                    *error = guardedChangeMessage();
                return false;
            }
        }
    }
    const auto *message = widget->findChild<QLineEdit *>("gitCommitMessage");
    if (message && !message->text().trimmed().isEmpty()) {
        if (error)
            *error = guardedChangeMessage();
        return false;
    }
    return true;
}

bool DashboardWidgetFactory::canChangeSharedMachineContext(QString *error) const
{
    if (error)
        error->clear();
    for (const auto &factory : m_scopedFactories) {
        if (factory && factory->m_followMain && !factory->canChangeSharedMachineContext(error))
            return false;
    }
    for (const auto &widget : m_widgets) {
        if (widget && !canChangeWidgetMachineContext(widget, error))
            return false;
    }
    return true;
}

bool DashboardWidgetFactory::setSharedMachineContext(const QJsonObject &catalog, const QString &rootId, QString *error)
{
    if (error)
        error->clear();
    if (catalog == m_sharedMachine && rootId == m_sharedRoot)
        return true;
    if ((!catalog.isEmpty() && !RemoteGitContext::validTarget(catalog.value("target").toObject(),
            rootId.isEmpty() ? QStringLiteral("root") : rootId)) || !canChangeSharedMachineContext(error)) {
        if (error && error->isEmpty())
            *error = QCoreApplication::translate("DashboardFilesWidget", "This directory is missing or cannot be read.");
        return false;
    }
    ++m_generation;
    m_snapshotCommand.cancel();
    m_containerCommand.cancel();
    m_gitCommand.cancel();
    m_sharedMachine = catalog;
    m_sharedRoot = rootId;
    RemoteGitContext::bind(&m_gitCommand, !catalog.isEmpty(), machineTarget(), rootId);
    if (auto *call = findChild<QObject *>(QStringLiteral("dashboardRemoteContainerCall")))
        delete call;
    for (const auto &factory : m_scopedFactories) {
        if (factory && factory->m_followMain)
            factory->setSharedMachineContext(catalog, rootId, nullptr);
    }
    for (const auto &widget : m_widgets) {
        if (widget)
            applyMachineContext(widget);
    }
    emit gitChanged({}, tr("Waiting for this project's Git status…"));
    emit containersChanged({}, tr("Waiting for Docker…"));
    updateMachineMenus();
    QTimer::singleShot(0, this, &DashboardWidgetFactory::refresh);
    return true;
}

void DashboardWidgetFactory::setMachineCatalog(const QJsonArray &catalog)
{
    m_machineCatalog = {};
    for (qsizetype index = 0; index < catalog.size() && index < 64; ++index)
        m_machineCatalog.append(catalog.at(index));
    if (!m_sharedMachine.isEmpty()) {
        const auto capturedTarget = machineTarget();
        for (const auto &entry : m_machineCatalog) {
            if (entry.toObject().value("target").toObject() == capturedTarget) {
                m_sharedMachine = entry.toObject();
                break;
            }
        }
    }
    for (const auto &factory : m_scopedFactories) {
        if (factory)
            factory->setMachineCatalog(m_machineCatalog);
    }
    for (const auto &widget : m_widgets) {
        if (!widget)
            continue;
        if (auto *terminal = widget->findChild<TerminalWidget *>())
            terminal->updateMachineCatalog(m_sharedMachine);
    }
    updateMachineMenus();
}

void DashboardWidgetFactory::updateMachineMenus()
{
    for (const auto &widget : m_widgets) {
        if (!widget)
            continue;
        for (auto *menu : widget->findChildren<WorkspaceMachineMenu *>()) {
            menu->setCatalog(m_machineCatalog);
            menu->setAccessUnlocked(true);
            menu->setCurrent(machineTarget(), m_sharedRoot);
            menu->setWidgetMode(true, m_followMain);
        }
    }
}

QJsonObject DashboardWidgetFactory::widgetMachineContext(QWidget *widget) const
{
    if (auto *factory = owningFactory(widget)) {
        QJsonObject context{{"mode", factory->m_followMain ? "follow" : (factory->m_sharedMachine.isEmpty() ? "local" : "remote")}};
        if (!factory->m_sharedMachine.isEmpty()) {
            context.insert("target", factory->machineTarget());
            context.insert("rootId", factory->m_sharedRoot);
        }
        return context;
    }
    return widget->property("dashboardMachineContext").toJsonObject();
}

bool DashboardWidgetFactory::setWidgetMachineContext(QWidget *widget, const QJsonObject &context, QString *error)
{
    auto *factory = owningFactory(widget);
    if (!factory || !canChangeWidgetMachineContext(widget, error))
        return false;
    const QString mode = context.value("mode").toString("follow");
    QJsonObject selected;
    QString root;
    if (mode == "follow") {
        auto *upstream = factory->m_upstream.data();
        selected = upstream ? upstream->m_sharedMachine : m_sharedMachine;
        root = upstream ? upstream->m_sharedRoot : m_sharedRoot;
    } else if (mode == "remote") {
        const auto target = context.value("target").toObject();
        selected = {{"target", target}};
        for (const auto &entry : factory->m_machineCatalog) {
            if (entry.toObject().value("target").toObject() == target) {
                selected = entry.toObject();
                break;
            }
        }
        root = context.value("rootId").toString();
    } else if (mode != "local") {
        return false;
    }
    if (factory->m_followMain != (mode == "follow")) {
        ++factory->m_generation;
        factory->m_gitCommand.cancel();
        factory->m_containerCommand.cancel();
        if (auto *call = factory->findChild<QObject *>(QStringLiteral("dashboardRemoteContainerCall")))
            delete call;
    }
    if (!factory->setSharedMachineContext(selected, root, error))
        return false;
    factory->m_followMain = mode == "follow";
    if (factory->m_upstream) {
        auto *upstream = factory->m_upstream.data();
        if (!upstream->hasVisibleKind({"git"}))
            upstream->m_gitCommand.cancel();
        if (!upstream->hasVisibleKind({"containers"})) {
            upstream->m_containerCommand.cancel();
            if (auto *call = upstream->findChild<QObject *>(QStringLiteral("dashboardRemoteContainerCall")))
                delete call;
        }
    }
    factory->updateMachineMenus();
    widget->setProperty("dashboardMachineContext", factory->widgetMachineContext(widget));
    emit factory->widgetMachineContextChanged(widget);
    return true;
}

void DashboardWidgetFactory::applyMachineContext(QWidget *widget)
{
    const auto target = machineTarget();
    const bool remote = !m_sharedMachine.isEmpty();
    widget->setProperty("dashboardMachineTarget", target);
    widget->setProperty("dashboardMachineRootId", remote ? m_sharedRoot : QString());
    widget->setProperty("dashboardMachineEpoch", QVariant::fromValue(m_generation));
    widget->setProperty("dashboardMachineContext", widgetMachineContext(widget));
    if (auto *files = widget->findChild<DashboardFilesWidget *>()) {
        files->setMachineContext(target, m_sharedRoot, m_workdir);
        if (auto *open = widget->findChild<QPushButton *>("dashboardOpenFullView"))
            open->setEnabled(!remote);
    }
    if (auto *terminal = widget->findChild<TerminalWidget *>())
        terminal->setMachineContext(m_sharedMachine, m_sharedRoot);
    if (auto *input = widget->findChild<SendInputWidget *>())
        input->setMachineContext(m_sharedMachine, m_sharedRoot);
}
