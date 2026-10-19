#pragma once

#include "LauncherUiExport.h"

#include <QModelIndex>
#include <QWidget>

class QAbstractItemModel;
class QItemSelectionModel;
class QHBoxLayout;
class QListView;
class QToolButton;
class QTreeView;

// Presents an owner's scoped model without owning filesystem policy or navigation.
class DOCKPIPE_LAUNCHER_UI_EXPORT FileExplorerView final : public QWidget {
    Q_OBJECT
public:
    enum class Mode { Tree, Folders };
    Q_ENUM(Mode)

    explicit FileExplorerView(QWidget *parent = nullptr);

    void setModel(QAbstractItemModel *model);
    void setRootIndex(const QModelIndex &root);
    void setMode(Mode mode);
    Mode mode() const { return m_mode; }
    void setAvailable(bool available);
    QTreeView *treeView() const { return m_tree; }
    QListView *iconView() const { return m_icons; }
    QModelIndex currentIndex() const;
    QHBoxLayout *toolbarLayout() const { return m_toolbar; }

protected:
    bool eventFilter(QObject *watched, QEvent *event) override;

signals:
    void activated(const QModelIndex &index);
    void clicked(const QModelIndex &index);

private:
    void updateVisibility();

    QHBoxLayout *m_toolbar = nullptr;
    QTreeView *m_tree = nullptr;
    QListView *m_icons = nullptr;
    QToolButton *m_treeMode = nullptr;
    QToolButton *m_foldersMode = nullptr;
    QItemSelectionModel *m_selection = nullptr;
    Mode m_mode = Mode::Tree;
    bool m_available = true;
};
