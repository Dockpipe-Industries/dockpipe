#pragma once

#include <QImage>
#include <QRect>
#include <QString>
#include <QVector>
#include <memory>

class QWidget;

struct DesktopAppChoice {
    quint64 id = 0;
    QString title;
    QString applicationClass;
};

// Local, explicitly selected X11 windows only. The owner must release control
// before its native container is destroyed. No input injection or persistence.
class DesktopAppWindow final {
public:
    static QString unavailableReason();
    static QVector<DesktopAppChoice> availableWindows(QString &error);
    explicit DesktopAppWindow(const DesktopAppChoice &choice);
    ~DesktopAppWindow();
    DesktopAppWindow(const DesktopAppWindow &) = delete;
    DesktopAppWindow &operator=(const DesktopAppWindow &) = delete;
    QString title() const { return m_choice.title; }
    QImage capture(QString &error);
    bool prepareControl(QString &error);
    bool controlReady() const;
    QWidget *takeControl(QWidget *parent, QString &error);
    bool controlAttached() const;
    bool syncControlGeometry();
    bool focusControl();
    void releaseControl();
    bool alive() const;
private:
    struct Native;
    std::unique_ptr<Native> m_native;
    DesktopAppChoice m_choice;
};
