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
        m_nodes = nodes;
        rebuild(currentData().toString());
    }

    void setSshConnection(const QString &id, const QString &label)
    {
        m_sshKey = QStringLiteral("ssh-machine:") + id;
        m_sshLabel = label + tr(" · SSH");
        rebuild(m_sshKey);
    }

    void clearSshConnection()
    {
        const QString selected = currentData().toString();
        m_sshKey.clear();
        m_sshLabel.clear();
        rebuild(selected);
    }

private:
    QString localLabel() const
    {
        const QString label = qEnvironmentVariable("DOCKPIPE_LOCAL_MACHINE_LABEL").trimmed().left(160);
        return label.isEmpty() ? tr("This computer") : label;
    }

    void rebuild(const QString &selected)
    {
        const QSignalBlocker blocker(this);
        clear();
        addItem(localLabel(), QString());
        if (!m_sshKey.isEmpty())
            addItem(m_sshLabel, m_sshKey);
        for (const auto &node : m_nodes) {
            if (!node.isEmpty() && !node.startsWith("ssh-machine:") && findData(node) < 0)
                addItem(node, node);
        }
        int index = findData(selected);
        if (index < 0) {
            addItem(selected + tr(" — unavailable"), selected);
            index = count() - 1;
        }
        setCurrentIndex(index);
    }

    QStringList m_nodes;
    QString m_sshKey;
    QString m_sshLabel;
};
