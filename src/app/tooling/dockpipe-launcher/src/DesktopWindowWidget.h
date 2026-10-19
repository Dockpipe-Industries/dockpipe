#pragma once

#include <QWidget>
#include "DesktopAppWindow.h"
#include <memory>
struct DesktopApplicationEntry;

// A session-only, explicitly selected desktop surface. Native window identities
// and portal grants are never written into a dashboard layout.
class DesktopWindowWidget final : public QWidget {
    Q_OBJECT
public:
    explicit DesktopWindowWidget(QWidget *parent = nullptr);
    ~DesktopWindowWidget() override;
    bool hasInteractiveWindow() const;
    bool requestClose(QString &error);
    void launchApplication(const DesktopApplicationEntry &application);
    void setApplicationHint(const QString &name);
    static QVector<DesktopAppChoice> availableWindows(QString &error);
    static QString automaticAcquisitionUnavailableReason();
    bool acquireWindow(quint64 nativeId, QString &error);
    quint64 acquiredWindowId() const;
    QString acquiredWindowTitle() const;
    QString acquiredApplicationClass() const;
    QIcon applicationIcon() const;
    void setApplicationIcon(const QIcon &icon);

signals:
    void windowChanged();
    void windowUnavailable(const QString &reason);
    void applicationIconChanged(const QIcon &icon);

public slots:
    void chooseWindow();
    void releaseWindow();

protected:
    void hideEvent(QHideEvent *event) override;
    void showEvent(QShowEvent *event) override;

private:
    struct State;
    std::unique_ptr<State> m_state;
    void chooseNativeWindow();
    void startPortalCapture();
    void startMacCapture();
    void advanceNativeWindow();
    void findLaunchedWindow();
    void fail(const QString &message);
    void updateCaptureVisibility();
};
