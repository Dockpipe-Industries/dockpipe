#include "FileExplorerView.h"
#include "CanvasIcons.h"

#include <QAbstractItemModel>
#include <QButtonGroup>
#include <QHelpEvent>
#include <QHBoxLayout>
#include <QItemSelectionModel>
#include <QListView>
#include <QKeyEvent>
#include <QPainter>
#include <QStyledItemDelegate>
#include <QTextLayout>
#include <QToolButton>
#include <QToolTip>
#include <QTreeView>
#include <QVBoxLayout>

namespace {
class ExplorerTree final : public QTreeView {
public:
    using QTreeView::QTreeView;

protected:
    void drawBranches(QPainter *painter, const QRect &rect, const QModelIndex &index) const override
    {
        if (!model() || !model()->hasChildren(index))
            return;

        const auto symbol = isExpanded(index) ? CanvasIcons::Symbol::ChevronDown
                                              : CanvasIcons::Symbol::ChevronRight;
        CanvasIcons::icon(symbol).paint(painter, QRect(rect.right() - 18, rect.center().y() - 7, 14, 14));
    }
};

class FolderCardDelegate final : public QStyledItemDelegate {
public:
    using QStyledItemDelegate::QStyledItemDelegate;

    QSize sizeHint(const QStyleOptionViewItem &option, const QModelIndex &) const override
    {
        return QSize(116, 72 + option.fontMetrics.lineSpacing() * 2);
    }

    void paint(QPainter *painter, const QStyleOptionViewItem &original,
               const QModelIndex &index) const override
    {
        QStyleOptionViewItem option(original);
        initStyleOption(&option, index);
        const QRect card = option.rect.adjusted(3, 3, -3, -3);
        const bool selected = option.state.testFlag(QStyle::State_Selected);
        const bool hovered = option.state.testFlag(QStyle::State_MouseOver);
        const bool enabled = option.state.testFlag(QStyle::State_Enabled);
        painter->save();
        painter->setRenderHint(QPainter::Antialiasing);

        if (selected || hovered) {
            QColor fill = option.palette.color(QPalette::Highlight);
            fill.setAlpha(selected ? 255 : 60);
            painter->setPen(Qt::NoPen);
            painter->setBrush(fill);
            painter->drawRoundedRect(card, 9, 9);
        }
        if (option.state.testFlag(QStyle::State_HasFocus)) {
            painter->setPen(QPen(option.palette.color(QPalette::Highlight), 1));
            painter->setBrush(Qt::NoBrush);
            painter->drawRoundedRect(card.adjusted(1, 1, -1, -1), 8, 8);
        }

        const QRect iconRect(card.center().x() - 22, card.top() + 10, 44, 44);
        option.icon.paint(painter, iconRect, Qt::AlignCenter, enabled ? QIcon::Normal : QIcon::Disabled);
        painter->setFont(option.font);
        painter->setPen(option.palette.color(enabled ? QPalette::Active : QPalette::Disabled,
                                             QPalette::Text));
        const QRect textRect = card.adjusted(7, 60, -7, -5);
        QTextLayout text(option.text, option.font);
        QTextOption textOption;
        textOption.setWrapMode(QTextOption::WrapAtWordBoundaryOrAnywhere);
        text.setTextOption(textOption);
        text.beginLayout();
        // At most two lines per visible card; very long names retain their full tooltip.
        for (int lineNumber = 0; lineNumber < 2; ++lineNumber) {
            QTextLine line = text.createLine();
            if (!line.isValid())
                break;
            line.setLineWidth(textRect.width());
            const qreal top = textRect.top() + lineNumber * option.fontMetrics.lineSpacing();
            if (lineNumber == 1) {
                const QString remainder = option.text.mid(line.textStart());
                const QString label = option.fontMetrics.elidedText(remainder, Qt::ElideRight,
                                                                    textRect.width());
                painter->drawText(QRectF(textRect.left(), top, textRect.width(), line.height()),
                                  Qt::AlignHCenter | Qt::AlignTop, label);
            } else {
                line.draw(painter, QPointF(textRect.left() + (textRect.width() - line.naturalTextWidth()) / 2,
                                          top));
            }
        }
        text.endLayout();
        painter->restore();
    }

    bool helpEvent(QHelpEvent *event, QAbstractItemView *view,
                   const QStyleOptionViewItem &option, const QModelIndex &index) override
    {
        if (event && event->type() == QEvent::ToolTip && index.isValid()) {
            QToolTip::showText(event->globalPos(), index.data(Qt::DisplayRole).toString(), view);
            return true;
        }
        return QStyledItemDelegate::helpEvent(event, view, option, index);
    }
};
}

FileExplorerView::FileExplorerView(QWidget *parent) : QWidget(parent)
{
    setObjectName("fileExplorer");
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->setSpacing(6);
    auto *modes = new QHBoxLayout;
    m_toolbar = modes;
    modes->setContentsMargins(0, 0, 0, 0);
    modes->setSpacing(4);
    auto *group = new QButtonGroup(this);
    group->setExclusive(true);
    auto addMode = [this, modes, group](const QString &name, const QString &label,
                                       CanvasIcons::Symbol icon, Mode mode) {
        auto *button = new QToolButton(this);
        button->setObjectName(name);
        button->setText(label);
        button->setAccessibleName(label);
        button->setToolTip(label);
        button->setIcon(CanvasIcons::icon(icon));
        button->setIconSize(QSize(16, 16));
        button->setToolButtonStyle(Qt::ToolButtonIconOnly);
        button->setFixedSize(32, 32);
        button->setCheckable(true);
        group->addButton(button);
        modes->addWidget(button);
        connect(button, &QToolButton::clicked, this, [this, mode] { setMode(mode); });
        return button;
    };
    m_treeMode = addMode("explorerTreeMode", tr("Tree"), CanvasIcons::Symbol::List, Mode::Tree);
    m_foldersMode = addMode("explorerFoldersMode", tr("Folders"), CanvasIcons::Symbol::Apps, Mode::Folders);
    modes->addStretch();
    layout->addLayout(modes);

    m_tree = new ExplorerTree(this);
    m_tree->setObjectName("explorerTree");
    m_tree->setProperty("dockpipeFileView", true);
    m_tree->setHeaderHidden(true);
    m_tree->setUniformRowHeights(true);
    m_tree->setIndentation(18);
    m_tree->setIconSize(QSize(18, 18));
    m_tree->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_tree->setSelectionMode(QAbstractItemView::SingleSelection);
    m_tree->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_tree->setMouseTracking(true);
    m_tree->installEventFilter(this);
    layout->addWidget(m_tree, 1);

    m_icons = new QListView(this);
    m_icons->setObjectName("explorerFolders");
    m_icons->setProperty("dockpipeFileView", true);
    m_icons->setViewMode(QListView::IconMode);
    m_icons->setMovement(QListView::Static);
    m_icons->setResizeMode(QListView::Adjust);
    m_icons->setWrapping(true);
    m_icons->setWordWrap(true);
    m_icons->setSpacing(4);
    m_icons->setIconSize(QSize(44, 44));
    m_icons->setUniformItemSizes(true);
    m_icons->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_icons->setSelectionMode(QAbstractItemView::SingleSelection);
    m_icons->setMouseTracking(true);
    m_icons->installEventFilter(this);
    m_icons->setItemDelegate(new FolderCardDelegate(m_icons));
    layout->addWidget(m_icons, 1);

    connect(m_tree, &QTreeView::doubleClicked, this, &FileExplorerView::activated);
    connect(m_tree, &QTreeView::clicked, this, &FileExplorerView::clicked);
    connect(m_icons, &QListView::activated, this, &FileExplorerView::activated);
    updateVisibility();
}

void FileExplorerView::setModel(QAbstractItemModel *model)
{
    if (m_tree->model() == model)
        return;

    QItemSelectionModel *oldSelection = m_selection;
    m_tree->setModel(model);
    m_icons->setModel(model);
    m_selection = nullptr;
    if (model) {
        QItemSelectionModel *treeSelection = m_tree->selectionModel();
        QItemSelectionModel *iconSelection = m_icons->selectionModel();
        m_selection = new QItemSelectionModel(model, this);
        m_tree->setSelectionModel(m_selection);
        m_icons->setSelectionModel(m_selection);
        delete treeSelection;
        delete iconSelection;
        for (int column = 1; column < model->columnCount(); ++column)
            m_tree->hideColumn(column);
    }
    delete oldSelection;
}

void FileExplorerView::setRootIndex(const QModelIndex &root)
{
    if (m_tree->rootIndex() == root && m_icons->rootIndex() == root)
        return;

    m_tree->setRootIndex(root);
    m_icons->setRootIndex(root);
    if (m_selection)
        m_selection->clear();
}

void FileExplorerView::setMode(Mode mode)
{
    m_mode = mode;
    updateVisibility();
    if (m_selection && m_selection->currentIndex().isValid()) {
        if (m_mode == Mode::Tree)
            m_tree->scrollTo(m_selection->currentIndex());
        else
            m_icons->scrollTo(m_selection->currentIndex());
    }
}

void FileExplorerView::setAvailable(bool available)
{
    m_available = available;
    updateVisibility();
}

QModelIndex FileExplorerView::currentIndex() const
{
    return m_selection ? m_selection->currentIndex() : QModelIndex();
}

void FileExplorerView::updateVisibility()
{
    m_treeMode->setEnabled(m_available);
    m_foldersMode->setEnabled(m_available);
    m_treeMode->setChecked(m_mode == Mode::Tree);
    m_foldersMode->setChecked(m_mode == Mode::Folders);
    m_tree->setVisible(m_available && m_mode == Mode::Tree);
    m_icons->setVisible(m_available && m_mode == Mode::Folders);
}

bool FileExplorerView::eventFilter(QObject *watched, QEvent *event)
{
    if ((watched == m_tree || watched == m_icons) && event->type() == QEvent::KeyPress) {
        auto *key = static_cast<QKeyEvent *>(event);
        if ((key->key() == Qt::Key_Return || key->key() == Qt::Key_Enter)
            && currentIndex().isValid()) {
            emit activated(currentIndex());
            return true;
        }
    }
    return QWidget::eventFilter(watched, event);
}
