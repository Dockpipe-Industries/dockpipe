#pragma once

#include "DesktopAppWindow.h"
#include <QObject>
#include <QTimer>
#include <QElapsedTimer>
#include <QRectF>
#include <QSet>
#include <vector>

class WorkspaceFlightView;

struct FlightAppFrame {
    QString title;
    QImage image;
    QString status;
    QString controlError;
};

// A flight owns at most three selected streams and one native interaction pane.
class FlightAppBillboards final : public QObject {
public:
    static QVector<DesktopAppChoice> chooseWindows(QWidget *parent);
    FlightAppBillboards(WorkspaceFlightView *flight, const QVector<DesktopAppChoice> &choices);
    ~FlightAppBillboards() override;
    const QVector<FlightAppFrame> &frames() const { return m_frames; }
    DesktopAppChoice choice(int index) const { return m_windows.at(index)->choice(); }
    bool windowAlive(int index) const { return m_windows.at(index)->alive(); }
    void control(int index);
    void setVisibleStreams(const QSet<int> &indices);
    qint64 lastCaptureNanoseconds() const { return m_lastCaptureNanoseconds; }
    quint64 captureCount() const { return m_captureCount; }
    void releaseControl(bool resumeRenderer = true);
    void stopSharing(bool resumeRenderer = true);
    bool controlling() const { return m_active >= 0; }
    void setResumeHardware(bool enabled) { m_restoreHardware = enabled; }
protected:
    bool eventFilter(QObject *object, QEvent *event) override;
private:
    void refresh();
    void recoverWindow(int index);
    bool m_visibilityFiltered = false;
    QSet<int> m_visibleStreams;
    qint64 m_lastCaptureNanoseconds = 0;
    quint64 m_captureCount = 0;
    void positionPanel();
    void finishControl();
    void failControl(const QString &message);
    WorkspaceFlightView *m_flight;
    std::vector<std::unique_ptr<DesktopAppWindow>> m_windows;
    QVector<FlightAppFrame> m_frames;
    QVector<int> m_failures;
    QVector<QElapsedTimer> m_recoveryClocks;
    QTimer m_timer;
    QTimer m_connectTimer;
    QTimer m_geometryTimer;
    QElapsedTimer m_connectElapsed;
    QWidget *m_panel = nullptr;
    QWidget *m_cabinetExit = nullptr;
    int m_active = -1;
    int m_next = 0;
    bool m_restoreHardware = false;
    bool m_controlCreated = false;
    QRectF m_panelPlacement{0.12, 0.12, 0.76, 0.76};
};
