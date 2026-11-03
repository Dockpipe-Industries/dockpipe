#pragma once

#include "TerminalLaunchContext.h"
#include "SshMachineConnection.h"
#include "TerminalShellCatalog.h"

#include <QByteArray>
#include <QHash>
#include <QObject>
#include <QStringList>

class TerminalSession;

// Process lifetime is independent of the page or dashboard displaying it.
class TerminalHub final : public QObject {
    Q_OBJECT
public:
    struct SessionLaunch {
        QString program;
        QStringList arguments;
        QString workingDirectory;
        QMap<QString, QString> environment;
        QString label;
        bool remote = false;
        TerminalLaunchContext context = TerminalLaunchContext::Automatic;
        QString error;

        bool valid() const { return error.isEmpty(); }
    };

    explicit TerminalHub(QObject *parent = nullptr);
    explicit TerminalHub(TerminalLaunchContext context, QObject *parent = nullptr);
    QString createSession(const QString &workdir, const QString &shellProfileId = {});
    TerminalShellCatalog &shellCatalog() const;
    QVector<TerminalShellProfile> shellProfiles() const;
    void setSshMachineConnection(const SshMachineConnection &connection, const QString &stateDirectory);
    void clearSshMachineConnection();
    bool hasSshMachineConnection() const;
    SshMachineConnection sshMachineConnection() const;
    SessionLaunch prepareSessionLaunch(const QString &workdir, const QString &shellProfileId = {}) const;
    QString creationError() const;
    QStringList sessionIds() const;
    QString title(const QString &id) const;
    bool running(const QString &id) const;
    QString error(const QString &id) const;
    bool send(const QString &id, const QString &text, QString *error = nullptr);
    bool sendBytes(const QString &id, const QByteArray &data);
    void resize(const QString &id, int columns, int rows);
    void closeSession(const QString &id);
    QByteArray readOutput(const QString &id, quint64 &cursor, bool &truncated) const;

signals:
    void sessionsChanged();
    void outputAvailable(const QString &id);
    void shellProfilesChanged();

private:
    struct Entry {
        TerminalSession *session = nullptr;
        QString title;
        QByteArray output;
        quint64 end = 0;
    };
    QHash<QString, Entry> m_sessions;
    QStringList m_order;
    int m_nextTitle = 1;
    TerminalShellCatalog *m_shellCatalog = nullptr;
    TerminalLaunchContext m_launchContext = TerminalLaunchContext::Automatic;
    QString m_creationError;
    bool m_hasSshMachineConnection = false;
    SshMachineConnection m_sshMachineConnection;
    QString m_sshStateDirectory;
};
