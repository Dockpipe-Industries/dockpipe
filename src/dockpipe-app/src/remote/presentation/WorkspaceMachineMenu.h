#pragma once

#include <QJsonArray>
#include <QJsonObject>
#include <QToolButton>

// The shell commits a selection only after its views accept leaving their drafts.
// A missing catalog entry remains selected and visibly unavailable.
class WorkspaceMachineMenu final : public QToolButton {
    Q_OBJECT
public:
    explicit WorkspaceMachineMenu(QWidget *parent = nullptr);
    void setCatalog(const QJsonArray &catalog);
    void setCurrent(const QJsonObject &target, const QString &root);
    void setAccessUnlocked(bool unlocked);
    void setWidgetMode(bool enabled, bool followsMain = true);
    QJsonObject currentTarget() const { return m_target; }
    QString currentRoot() const { return m_root; }
    QJsonArray catalog() const { return m_catalog; }

signals:
    void contextRequested(const QJsonObject &target, const QString &root);
    void refreshRequested();
    void unlockRequested();
    void followMainRequested();

private:
    void rebuildMenu();
    void updateLabel();
    QJsonArray m_catalog;
    QJsonObject m_target;
    QString m_root;
    bool m_unlocked = false;
    bool m_widgetMode = false;
    bool m_followsMain = false;
};
