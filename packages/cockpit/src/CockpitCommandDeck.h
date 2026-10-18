#pragma once
#include <QFrame>
#include <QVector>

class QToolButton;

class CockpitCommandDeck final : public QFrame {
    Q_OBJECT
public:
    explicit CockpitCommandDeck(QWidget *parent);
    void setDestination(const QString &destination);
signals:
    void destinationRequested(const QString &destination);
protected:
    void resizeEvent(QResizeEvent *event) override;
private:
    QVector<QToolButton *> m_buttons;
};
