#pragma once

#include "TerminalLaunchContext.h"
#include "TerminalShellCatalog.h"

#include <QByteArray>
#include <QHash>
#include <QObject>
#include <QPointer>
#include <QStringList>

class TerminalSession;
class NativeHostConnection;

// Process lifetime is independent of the page or dashboard displaying it.
class TerminalHub final : public QObject {
    Q_OBJECT
public:
    explicit TerminalHub(QObject *parent = nullptr);
    explicit TerminalHub(TerminalLaunchContext context, QObject *parent = nullptr);
    QString createSession(const QString &workdir, const QString &shellProfileId = {});
    TerminalShellCatalog &shellCatalog() const;
    QVector<TerminalShellProfile> shellProfiles() const;
    void setNativeHostConnection(NativeHostConnection *connection);
    NativeHostConnection *nativeHostConnection() const;
    bool nativeHostConfigured() const;
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
    void nativeHostChanged();
    struct Entry {
        TerminalSession *session = nullptr;
        QString title;
        QByteArray output;
        quint64 end = 0;
        bool native = false;
    };
    QHash<QString, Entry> m_sessions;
    QStringList m_order;
    int m_nextTitle = 1;
    TerminalShellCatalog *m_shellCatalog = nullptr;
    TerminalLaunchContext m_launchContext = TerminalLaunchContext::Automatic;
    QString m_creationError;
    QPointer<NativeHostConnection> m_nativeHost;
    bool m_nativeHostConfigured = false;
};
