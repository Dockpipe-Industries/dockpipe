#pragma once

#include "NativeHostConnection.h"

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
        m_nodes = nodes;
        rebuild(currentData().toString());
    }

    void setNativeHost(const QString &label, bool select)
    {
        m_nativeLabel = label;
        rebuild(select ? NativeHostConnection::selectionKey() : currentData().toString());
    }

private:
    void rebuild(const QString &selected)
    {
        const QSignalBlocker blocker(this);
        clear();
        addItem(localLabel(), QString());
        if (!m_nativeLabel.isEmpty())
            addItem(m_nativeLabel, NativeHostConnection::selectionKey());
        for (const auto &node : m_nodes) {
            if (!node.isEmpty() && node != NativeHostConnection::selectionKey() && findData(node) < 0)
                addItem(node, node);
        }
        int index = findData(selected);
        if (index < 0) {
            addItem(selected + tr(" — unavailable"), selected);
            index = count() - 1;
        }
        setCurrentIndex(index);
    }

    QString localLabel() const
    {
        if (!m_nativeLabel.isEmpty())
            return tr("Application container");
        const QString label = qEnvironmentVariable("DOCKPIPE_LOCAL_MACHINE_LABEL").trimmed().left(160);
        return label.isEmpty() ? tr("This computer") : label;
    }

    QStringList m_nodes;
    QString m_nativeLabel;
};
