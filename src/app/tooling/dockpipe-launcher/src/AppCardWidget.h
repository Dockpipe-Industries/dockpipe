#pragma once

#include "WorkflowCatalog.h"
#include <QFrame>

class QIcon;

class AppCardWidget : public QFrame {
    Q_OBJECT
public:
    AppCardWidget(const WorkflowMeta &app, const QIcon &icon, bool compact,
                  bool running, bool launching, QWidget *parent = nullptr);
signals:
    void launchRequested();
    void configureRequested();
};
