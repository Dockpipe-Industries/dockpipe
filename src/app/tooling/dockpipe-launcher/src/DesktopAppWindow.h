#pragma once

#include <QImage>
#include <QIcon>
#include <QRect>
#include <QString>
#include <QVector>
#include <memory>
#include <optional>

class QWidget;

struct DesktopAppChoice {
    quint64 id = 0;
    QString title;
    QString applicationClass;
};

// Local, explicitly selected supported desktop windows. The owner must release control
// before its native container is destroyed. No input injection or persistence.
class DesktopAppWindow final {
public:
    static QString unavailableReason();
    // An optional native ID revalidates one selection without inspecting every
    // other application's process again during batch acquisition.
    static QVector<DesktopAppChoice> availableWindows(QString &error, quint64 onlyWindowId = 0);
    explicit DesktopAppWindow(const DesktopAppChoice &choice);
    ~DesktopAppWindow();
    DesktopAppWindow(const DesktopAppWindow &) = delete;
    DesktopAppWindow &operator=(const DesktopAppWindow &) = delete;
    QString title() const { return m_choice.title; }
    DesktopAppChoice choice() const { return m_choice; }
    std::optional<DesktopAppChoice> replacement(QString &error) const;
    QImage capture(QString &error);
    bool prepareControl(QString &error);
    bool controlReady() const;
    QWidget *takeControl(QWidget *parent, QString &error);
    bool controlAttached() const;
    bool syncControlGeometry();
    bool focusControl();
    bool requestClose(QString &error);
    void releaseControl();
    bool alive() const;
    QIcon applicationIcon() const;
private:
    void acknowledgeControlFrame();
    struct Native;
    std::unique_ptr<Native> m_native;
    DesktopAppChoice m_choice;
};
