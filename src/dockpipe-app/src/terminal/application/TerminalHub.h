#pragma once

#include "TerminalLaunchContext.h"
#include "NativeTerminalLaunch.h"
#include "TerminalShellCatalog.h"

#include <QByteArray>
#include <QHash>
#include <QSet>
#include <QJsonObject>
#include <QObject>
#include <QPointer>
#include <QStringList>
#include <functional>

class ManagedTerminalSession;
class NativeHostConnection;
class RemoteTerminalSession;

// Process lifetime is independent of the page or dashboard displaying it.
class TerminalHub final : public QObject {
    Q_OBJECT
public:
    explicit TerminalHub(QObject *parent = nullptr);
    explicit TerminalHub(TerminalLaunchContext context, QObject *parent = nullptr);
    QString createSession(const QString &workdir, const QString &shellProfileId = {});
    QString createSession(const QString &workdir, const QString &shellProfileId,
                          const QJsonObject &catalog, const QString &root);
    QString createContainerSession(const QJsonObject &catalog, const QString &container, const QString &shell);
    QString createLocalContainerSession(const QString &container, const QString &name, const QString &shell);
    TerminalShellCatalog &shellCatalog() const;
    QVector<TerminalShellProfile> shellProfiles() const;
    void setNativeHostConnection(NativeHostConnection *connection);
    NativeHostConnection *nativeHostConnection() const;
    bool nativeHostConfigured() const;
    QString creationError() const;
    QStringList sessionIds() const;
    QStringList sessionIds(const QJsonObject &catalog, const QString &root) const;
    bool connecting(const QString &id) const;
    QString title(const QString &id) const;
    bool running(const QString &id) const;
    QString error(const QString &id) const;
    bool send(const QString &id, const QString &text, QString *error = nullptr);
    bool sendBytes(const QString &id, const QByteArray &data);
    void sendAsync(const QString &id, const QString &text, std::function<void(bool, const QString &)> completion);
    void resize(const QString &id, int columns, int rows);
    void closeSession(const QString &id);
    void releaseRecoveredSession(const QJsonObject &reference);
    void releaseRecoveredContainerSession(const QJsonObject &reference);
    QByteArray readOutput(const QString &id, quint64 &cursor, bool &truncated) const;

signals:
    void sessionsChanged();
    void outputAvailable(const QString &id);
    void shellProfilesChanged();

private:
    QString createLocalSession(const NativeTerminalLaunch::Snapshot &profile, const QString &title, const QString &root,
                               bool nativeHost = false);
    QString createRemoteSession(const QJsonObject &catalog, const QString &root, const QString &container, const QString &shell);
    void nativeHostChanged();
    struct Entry {
        ManagedTerminalSession *session = nullptr;
        RemoteTerminalSession *remote = nullptr;
        QJsonObject catalog;
        QString root;
        QString title;
        QByteArray output;
        quint64 end = 0;
        bool native = false;
    };
    QHash<QString, Entry> m_sessions;
    QSet<QObject *> m_closingSessions;
    QStringList m_order;
    int m_nextTitle = 1;
    TerminalShellCatalog *m_shellCatalog = nullptr;
    TerminalLaunchContext m_launchContext = TerminalLaunchContext::Automatic;
    QString m_creationError;
    QPointer<NativeHostConnection> m_nativeHost;
    bool m_nativeHostConfigured = false;
};
