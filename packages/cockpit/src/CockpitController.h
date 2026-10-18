#pragma once
#include <QObject>
#include <QPointF>
#include <QVector3D>
#include <QTimer>
#include <functional>
#include <memory>

class CockpitController final : public QObject {
    Q_OBJECT
public:
    explicit CockpitController(QObject *parent = nullptr);
    ~CockpitController() override;
    bool enable();
    void disable();
    void setInputActive(bool active);
    static qreal normalizedAxis(int value);
    static float normalizedTrigger(int value);
    // Query the current view at polling time, including mode changes from Y.
    std::function<bool()> pilotFlightActive;
signals:
    void flightControls(float roll, bool brake);
    void vehicleTriggers(float throttle, float brake);
    void inputSuspended();
    void pilotInput(const QVector3D &movement, const QPointF &look, bool boost);
    void buttonChanged(int button, bool pressed);
    void statusChanged(const QString &status);
private:
    struct Native;
    std::unique_ptr<Native> m_native;
    QTimer m_timer;
    bool m_inputActive = false;
    void poll();
};
