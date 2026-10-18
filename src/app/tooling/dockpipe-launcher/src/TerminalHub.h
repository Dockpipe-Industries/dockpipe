#pragma once

#include <QByteArray>
#include <QHash>
#include <QObject>
#include <QStringList>

class TerminalSession;

// Process lifetime is independent of the page or dashboard displaying it.
class TerminalHub final : public QObject {
    Q_OBJECT
public:
    explicit TerminalHub(QObject *parent = nullptr);
    QString createSession(const QString &workdir);
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
};
