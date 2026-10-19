#pragma once

#include <QObject>
#include <QPointer>
#include <QTemporaryDir>
#include <QUrl>

class QNetworkAccessManager;
class QNetworkReply;
class QTimer;

class BrowserExtensionPackage final : public QObject {
    Q_OBJECT
public:
    explicit BrowserExtensionPackage(QObject *parent = nullptr);
    ~BrowserExtensionPackage() override;
    static QString extensionId(const QString &input);
    static QUrl downloadUrl(const QString &id);
    static bool prepareFile(const QString &input, const QString &outputZip, QString *error, QString *publicKey = nullptr);
    QString publicKey() const { return m_publicKey; }
    void fetch(const QString &id);
    void cancel();
signals:
    void ready(const QString &zipPath);
    void failed(const QString &error);
private:
    void request(const QUrl &url);
    void fail(const QString &error);
    QNetworkAccessManager *m_network;
    QPointer<QNetworkReply> m_reply;
    QTimer *m_timeout;
    QTemporaryDir m_directory;
    QByteArray m_bytes;
    int m_redirects = 0;
    QString m_publicKey;
    QString m_expectedId;
};
