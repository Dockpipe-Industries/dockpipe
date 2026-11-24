#pragma once

#include "EnvironmentDefinitionsClient.h"
#include "ResourceCanvas.h"

#include <QHash>
#include <QJsonArray>
#include <QMap>
#include <QSet>
#include <QTimer>
#include <QWidget>

class QLabel;
class QPushButton;
class QStackedWidget;
class QTreeWidget;
class QProcess;
class EnvironmentsPage final : public QWidget {
    Q_OBJECT

public:
    explicit EnvironmentsPage(QWidget *parent = nullptr);

    void setWorkspace(const QString &workdir);
    void setActive(bool active);
    void setTerminalInventoryActive(bool active);
    QMap<QString, QString> runningContainerTargets() const;
    void setVmObservations(const QHash<QString, QString> &states, qint64 observedAt);
    void setApplying(const QString &path, bool applying);
    void showInventory();
    void setMachineCapabilities(const QJsonArray &catalog);
    void setVirtualMachineControls(QWidget *controls);
    void showVirtualMachineControls();
    bool virtualMachineControlsActive() const;
    void setMachineInventory(const QVector<ResourceNode> &nodes, const QVector<ResourceEdge> &edges);
    void setVirtualMachineInventory(const QJsonArray &resources);

signals:
    void applyRequested(const EnvironmentDefinition &definition);
    void virtualMachineControlsActivityChanged(bool active);
    void resourceControlsRequested(const QString &kind, const QString &id, const QString &workdir);
    void runtimeInventoryRequested();
    void machineRefreshRequested();
    void addRemoteMachineRequested();
    void containerTargetsChanged();

private:
    void refresh();
    void addEnvironment();
    void createVmDefinition();
    void createContainerDefinition();
    void saveDefinition(EnvironmentDefinition definition, QStringList arguments, bool start);
    QWidget *buildInspector(bool canvas);
    void includeFiles();
    void editDefinition();
    void showDefinitions(const QVector<EnvironmentDefinition> &definitions);
    void renderDefinitions();
    void selectDefinition(const QString &path);
    QString definitionStatus(const EnvironmentDefinition &definition) const;
    QString definitionKind(const EnvironmentDefinition &definition) const;
    QString definitionSource(const EnvironmentDefinition &definition) const;
    QString runtimeState(const EnvironmentDefinition &definition) const;
    QString runtimeLabel(const EnvironmentDefinition &definition, const QString &state) const;
    QString resourceKind(const ResourceNode &node) const;
    QString resourceDetails(const ResourceNode &node) const;
    void showControls(QWidget *controls);

    EnvironmentDefinitionsClient m_client;
    QVector<EnvironmentDefinition> m_definitions;
    QSet<QString> m_ambiguousDefinitions;
    QVector<ResourceNode> m_machineNodes;
    QVector<ResourceEdge> m_machineEdges;
    QVector<ResourceNode> m_externalNodes;
    QJsonArray m_virtualMachineInventory;
    QJsonArray m_machineCapabilities;
    QSet<QString> m_applying;
    QHash<QString, QString> m_vmStates;
    QTimer m_vmExpiry;
    QTimer m_inventoryRefresh;
    qint64 m_vmObservedAt = 0;
    QString m_selectedPath;
    QString m_workdir;
    QStringList m_files;
    bool m_active = false;
    bool m_terminalInventoryActive = false;
    QStackedWidget *m_sections = nullptr;
    QWidget *m_inventory = nullptr;
    QWidget *m_virtualMachineControls = nullptr;
    QStackedWidget *m_views = nullptr;
    ResourceCanvas *m_canvas = nullptr;
    QTreeWidget *m_list = nullptr;
    QLabel *m_canvasDetails = nullptr;
    QLabel *m_listDetails = nullptr;
    QLabel *m_status = nullptr;
    QVector<QLabel *> m_inspectorTitles;
    QProcess *m_generation = nullptr;
    QVector<QPushButton *> m_edit;
    QVector<QPushButton *> m_apply;
    QPushButton *m_back = nullptr;
    QVector<QPushButton *> m_resourceControls;
};
