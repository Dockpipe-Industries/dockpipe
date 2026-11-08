#pragma once

#include "TerminalLaunchContext.h"

#include <QByteArray>
#include <QHash>
#include <QSet>
#include <QJsonObject>
#include <QObject>
#include <QStringList>
#include <functional>

class ManagedTerminalSession;
class RemoteTerminalSession;
class TerminalShellCatalog;

// Process lifetime is independent of the page or dashboard displaying it.
class TerminalHub final : public QObject {
    Q_OBJECT
public:
    explicit TerminalHub(QObject *parent = nullptr);
    explicit TerminalHub(TerminalLaunchContext context, QObject *parent = nullptr);
    QString createSession(const QString &workdir, const QString &shellProfileId = {});
    QString createSession(const QString &workdir, const QString &shellProfileId,
                          const QJsonObject &catalog, const QString &root);
    TerminalShellCatalog &shellCatalog() const;
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
    QByteArray readOutput(const QString &id, quint64 &cursor, bool &truncated) const;

signals:
    void sessionsChanged();
    void outputAvailable(const QString &id);

private:
    struct Entry {
        ManagedTerminalSession *session = nullptr;
        RemoteTerminalSession *remote = nullptr;
        QJsonObject catalog;
        QString root;
        QString title;
        QByteArray output;
        quint64 end = 0;
    };
    QHash<QString, Entry> m_sessions;
    QSet<QObject *> m_closingSessions;
    QStringList m_order;
    int m_nextTitle = 1;
    TerminalShellCatalog *m_shellCatalog = nullptr;
    TerminalLaunchContext m_launchContext = TerminalLaunchContext::Automatic;
    QString m_creationError;
};
