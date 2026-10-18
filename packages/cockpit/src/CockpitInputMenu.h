#pragma once
#include <QFrame>
#include <QElapsedTimer>
#include <QTimer>

class QLabel;
class QListWidget;
class QKeyEvent;

struct CockpitMenuEntry {
    QString id;
    QString label;
    QString description;
    bool enabled = true;
    QString previewType;
    QString category;
};

// One in-world command surface. Keyboard, D-pad and stick share selection/actions.
class CockpitInputMenu final : public QFrame {
    Q_OBJECT
public:
    explicit CockpitInputMenu(QWidget *parent);
    void present(QWidget *surface, const QString &title, const QVector<CockpitMenuEntry> &entries, bool controller);
    void setControllerHints(bool controller);
    void reposition();
    bool key(QKeyEvent *event);
    void button(int button);
    void stick(qreal vertical, qreal horizontal = 0);
signals:
    void triggered(const QString &id);
    void backRequested();
private:
    void move(int delta);
    void activate();
    void populate();
    void changeCategory(int delta);
    void refreshPreviews();
    int columns() const;
    QLabel *m_title;
    QLabel *m_description;
    QLabel *m_hints;
    QListWidget *m_entries;
    QWidget *m_categoryBar;
    QLabel *m_categoryLabel;
    QVector<CockpitMenuEntry> m_sourceEntries;
    QStringList m_categories;
    QString m_category;
    QString m_lastModel;
    bool m_catalogue = false;
    QTimer m_previewTimer;
    QElapsedTimer m_repeat;
    int m_stickDirection = 0;
};
