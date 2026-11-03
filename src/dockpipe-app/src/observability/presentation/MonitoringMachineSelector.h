#pragma once

#include <QComboBox>
#include <QSignalBlocker>

// Node discovery is owned by the existing Machines connection. Retain a missing
// selection explicitly; never silently route a remote inspection to localhost.
class MonitoringMachineSelector : public QComboBox {
public:
    explicit MonitoringMachineSelector(QWidget *parent = nullptr) : QComboBox(parent)
    {
        setObjectName("monitoringMachine");
        addItem(localLabel(), QString());
        setMinimumWidth(180);
    }

    void setNodes(const QStringList &nodes)
    {
        const QSignalBlocker blocker(this);
        const QString selected = currentData().toString();
        clear();
        addItem(localLabel(), QString());
        for (const auto &node : nodes)
            addItem(node, node);
        int index = findData(selected);
        if (index < 0) {
            addItem(selected + tr(" — unavailable"), selected);
            index = count() - 1;
        }
        setCurrentIndex(index);
    }

private:
    QString localLabel() const
    {
        const QString label = qEnvironmentVariable("DOCKPIPE_LOCAL_MACHINE_LABEL").trimmed().left(160);
        return label.isEmpty() ? tr("This computer") : label;
    }
};
