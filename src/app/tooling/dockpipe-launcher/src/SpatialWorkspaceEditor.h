#pragma once
#include "SpatialWorkspace.h"
#include <QWidget>
#include <functional>

class QTreeWidget;
class QTreeWidgetItem;
class QLabel;

// Explicit edit mode: edits are drafts until Save; navigation saves successfully first.
class SpatialWorkspaceEditor final : public QWidget {
    Q_OBJECT
public:
    explicit SpatialWorkspaceEditor(const SpatialWorkspace &workspace, QWidget *parent = nullptr);
    void showStatus(const QString &status);
    void preferApplication(const SpatialApplicationReference &reference) { m_preferredApplication = reference; }
    std::function<bool(const SpatialWorkspace &, QString &)> save;
    std::function<bool(SpatialWorkspace &, QString &)> reload;
signals:
    void travelRequested(const QString &path);
    void approachRequested(const QString &path);
private:
    SpatialWorkspace m_draft;
    SpatialApplicationReference m_preferredApplication;
    QTreeWidget *m_tree;
    QLabel *m_status;
    void rebuild(const QString &selected = {});
    void addPlanet();
    void addBuilding();
    void addDisplay();
    SpatialDisplay *selectedDisplay();
    SpatialBuilding *selectedBuilding();
    void editSelected();
    void removeSelected();
    bool saveDraft();
    SpatialPlanet *selectedPlanet();
};
