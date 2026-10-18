#pragma once

#include "SpatialWebDocument.h"
#include <QCache>
#include <QJsonObject>
#include <QObject>
#include <QProcess>
#include <QQueue>
#include <QTemporaryDir>
#include <QTimer>

// Package-owned optional inference. It never runs on the painting thread, and
// has one active request, twelve pending images, and bounded private RAM caches.
class SpatialNeuralService final : public QObject {
    Q_OBJECT
public:
    explicit SpatialNeuralService(QObject *parent = nullptr);
    ~SpatialNeuralService() override;
    bool available() const;
    bool installing() const { return m_installing; }
    void install(bool gpu = false);
    void plan(const SpatialWebDocument &document);
    void cutout(const QString &id, const QImage &image);
    void cancel();
    void setPreferGpu(bool enabled);
    static bool apply(const QJsonObject &proposal, SpatialWebDocument &document);
signals:
    void planReady(const QJsonObject &proposal);
    void imageReady(const QString &id, const QImage &image);
    void statusChanged(const QString &message);
    void installationFinished(bool success);
private:
    struct Request {
        QJsonObject payload;
        QByteArray key;
        QString id;
    };
    QString root() const;
    bool prepareScripts();
    void enqueue(Request request);
    void pump();
    void deliver(const Request &request, const QJsonObject &result);
    void fail(const QString &message);
    void stopProcess();
    QProcess *m_process = nullptr;
    QTemporaryDir m_scripts;
    QTimer m_deadline;
    QTimer m_idle;
    QQueue<Request> m_queue;
    QCache<QByteArray, QJsonObject> m_cache{32};
    QByteArray m_output;
    qint64 m_diagnosticBytes = 0;
    quint64 m_generation = 0;
    int m_requests = 0;
    bool m_busy = false;
    bool m_installing = false;
    bool m_preferGpu = false;
};
