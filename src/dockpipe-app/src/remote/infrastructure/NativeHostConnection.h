#pragma once

#include "NativeHostStatus.h"
#include "RemoteCommand.h"
#include "LauncherUiExport.h"

#include <QObject>
#include <QTimer>

// A reference selects an authenticated CLI connection. It is never a username,
// credential or grant, and the GUI does not read its connection/key contents.
class DOCKPIPE_LAUNCHER_UI_EXPORT NativeHostConnection final : public QObject {
    Q_OBJECT
public:
    enum class Operation { Status, Snapshot, Terminal };
    struct Plan {
        QString program;
        QStringList arguments;
        QString workingDirectory;
        QString error;

        bool valid() const { return error.isEmpty() && !program.isEmpty(); }
    };

    explicit NativeHostConnection(const QString &reference, QObject *parent = nullptr);
    static QString selectionKey() { return QStringLiteral("native-host:"); }
    QString reference() const;
    bool available() const;
    QString label() const;
    QString error() const;
    quint64 revision() const;
    NativeHostStatus status() const;
    Plan plan(Operation operation) const;
    Plan dockerPlan(const QString &action, const QString &id) const;
    Plan dockerQueryPlan(const QString &kind, const QString &id) const;
    void refresh();

signals:
    void statusChanged();

private:
    QString referenceError() const;
    void update(const NativeHostStatus &status, const QString &error);

    QString m_reference;
    NativeHostStatus m_status;
    QString m_error;
    quint64 m_revision = 0;
    RemoteCommand m_command;
    QTimer m_timer;
};
