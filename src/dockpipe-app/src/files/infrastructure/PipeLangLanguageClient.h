#pragma once

#include <QObject>
#include <QHash>
#include <QJsonArray>
#include <QJsonObject>
#include <QProcess>
#include <QTimer>

// Native stdio boundary only. Language semantics belong to the selected server.
class PipeLangLanguageClient : public QObject {
    Q_OBJECT
public:
    struct Configuration {
        QString nodePath;
        QString serverPath;
        QString compilerPath;
        QString sourceRoot;
    };

    explicit PipeLangLanguageClient(QObject *parent = nullptr);
    ~PipeLangLanguageClient() override;
    bool start(const Configuration &configuration);
    void stop();
    void setDocument(const QString &path, const QString &text);
    void closeDocument(const QString &path);
    void hover(const QString &path, int line, int character);
    void cancelHover();
    bool isReady() const;

signals:
    void statusChanged(const QString &status);
    void ready();
    void diagnostics(const QString &path, int version, const QJsonArray &items);
    void hoverResult(const QString &path, int version, const QJsonValue &contents);
    void documentVersionChanged(const QString &path, int version);

private:
    struct Document {
        QString text;
        int version = 1;
        bool opened = false;
        bool dirty = true;
    };
    enum class State { Stopped, Starting, Initializing, Ready, Stopping, Failed };
    void send(const QJsonObject &message);
    void notify(const QString &method, const QJsonObject &parameters = {});
    int request(const QString &method, const QJsonObject &parameters = {});
    void receive();
    void handle(const QJsonObject &message);
    void flushDocuments();
    void fail(const QString &message);
    void clearResults();
    static QString uri(const QString &path);

    QProcess m_process;
    QTimer m_deadline;
    QTimer m_changes;
    Configuration m_configuration;
    QHash<QString, Document> m_documents;
    QByteArray m_input;
    qint64 m_bodyLength = -1;
    State m_state = State::Stopped;
    int m_nextId = 0;
    int m_nextVersion = 0;
    int m_initializeId = 0;
    int m_shutdownId = 0;
    int m_hoverId = 0;
    QString m_hoverPath;
    int m_hoverVersion = 0;
    QTimer m_hoverDeadline;
};
