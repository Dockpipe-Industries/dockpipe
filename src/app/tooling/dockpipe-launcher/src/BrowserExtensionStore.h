#pragma once

#include <QObject>
#include <QJsonObject>
#include <QPointer>
#include <memory>

class QLockFile;
class QWebEngineProfile;
class QWebEngineExtensionInfo;

// Extension code and explicit enable decisions outlive the disposable browsing profile.
class BrowserExtensionStore final : public QObject {
    Q_OBJECT
public:
    explicit BrowserExtensionStore(QWebEngineProfile *profile, QObject *parent = nullptr,
        const QString &root = QString());
    static BrowserExtensionStore *forProfile(QWebEngineProfile *profile);

    bool persist(const QWebEngineExtensionInfo &extension, QString *error = nullptr);
    bool activate(const QString &id, QString *error = nullptr);
    bool setEnabled(const QString &id, bool enabled, QString *error = nullptr);
    bool remove(const QString &id, QString *error = nullptr);
    bool enabled(const QString &id) const;
    bool owns(const QString &id) const;
    QString lastError() const { return m_lastError; }

signals:
    void restoreFailed(const QString &message);

private:
    bool loadRegistry(QString *error);
    bool beginMutation(std::unique_ptr<QLockFile> &lock, QString *error);
    QString storedId(const QString &id) const;
    bool saveRegistry(const QJsonObject &entries, QString *error);
    void restore();

    QPointer<QWebEngineProfile> m_profile;
    QString m_root;
    QString m_lastError;
    QJsonObject m_entries;
};
