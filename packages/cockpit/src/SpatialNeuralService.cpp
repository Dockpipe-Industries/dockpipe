#include "SpatialNeuralService.h"
#include "SpatialAnimalModel.h"
#include <QBuffer>
#include <QCryptographicHash>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QImageReader>
#include <QJsonArray>
#include <QJsonDocument>
#include <QStandardPaths>
#include <cmath>
#ifdef Q_OS_UNIX
#include <signal.h>
#include <unistd.h>
#endif

namespace {
constexpr int maximumResponse = 1024 * 1024;
bool features(const QJsonValue &value, QVector<float> &output)
{
    if (!value.isArray() || value.toArray().size() != 8)
        return false;
    output.clear();
    for (const auto &entry : value.toArray()) {
        const double number = entry.toDouble(-1);
        if (!entry.isDouble() || !std::isfinite(number) || number < 0 || number > 1)
            return false;
        output.append(float(number));
    }
    return true;
}
}

SpatialNeuralService::SpatialNeuralService(QObject *parent) : QObject(parent)
{
    m_deadline.setSingleShot(true);
    connect(&m_deadline, &QTimer::timeout, this, [this]() {
        fail(tr("Local enhancement timed out. The current world is still available."));
    });
    m_idle.setSingleShot(true);
    m_idle.setInterval(30000);
    connect(&m_idle, &QTimer::timeout, this, &SpatialNeuralService::stopProcess);
}

SpatialNeuralService::~SpatialNeuralService()
{
    cancel();
}

QString SpatialNeuralService::root() const
{
    const QString override = qEnvironmentVariable("DOCKPIPE_COCKPIT_NEURAL_ROOT");
    if (!override.isEmpty())
        return override;
    return QStandardPaths::writableLocation(QStandardPaths::AppLocalDataLocation)
        + (m_preferGpu ? "/cockpit/neural-v1-cuda" : "/cockpit/neural-v1");
}

bool SpatialNeuralService::available() const
{
    const QDir directory(root());
    return QFileInfo(directory.filePath("runtime/bin/python")).isExecutable()
        && QFile::exists(directory.filePath("ready.json"))
        && QFile::exists(directory.filePath("models/semantic.onnx"))
        && QFile::exists(directory.filePath("models/tokenizer.json"))
        && QFile::exists(directory.filePath("models/cutout.onnx"));
}

bool SpatialNeuralService::prepareScripts()
{
    if (!m_scripts.isValid())
        return false;
    for (const QString &name : {"worker.py", "install.py", "models.json"}) {
        const QString path = m_scripts.filePath(name);
        if (!QFile::exists(path) && !QFile::copy(":/cockpit/neural/" + name, path))
            return false;
    }
    return true;
}

void SpatialNeuralService::setPreferGpu(bool enabled)
{
    if (enabled == m_preferGpu)
        return;
    cancel();
    m_preferGpu = enabled;
    m_cache.clear();
}

void SpatialNeuralService::stopProcess()
{
    m_idle.stop();
    if (!m_process)
        return;
    auto *process = m_process;
    m_process = nullptr;
    disconnect(process, nullptr, this, nullptr);
    if (process->state() != QProcess::NotRunning) {
#ifdef Q_OS_UNIX
        // This child starts a private session. Kill only that session's group,
        // including pip children during an interrupted installation.
        const auto pid = process->processId();
        if (pid > 0)
            ::kill(-pid, SIGKILL);
#endif
        process->kill();
        connect(process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), process, &QObject::deleteLater);
        // The helper has no UI ownership and survives only until the OS reaps it.
        process->setParent(nullptr);
    } else {
        process->deleteLater();
    }
}

void SpatialNeuralService::cancel()
{
    ++m_generation;
    m_deadline.stop();
    m_queue.clear();
    m_busy = false;
    m_installing = false;
    m_output.clear();
    stopProcess();
}

void SpatialNeuralService::fail(const QString &message)
{
    const bool setup = m_installing;
    cancel();
    emit statusChanged(message);
    if (setup)
        emit installationFinished(false);
}

void SpatialNeuralService::install(bool gpu)
{
    cancel();
    m_preferGpu = gpu;
#ifndef Q_OS_LINUX
    emit statusChanged(tr("Local model installation currently supports native Linux."));
    emit installationFinished(false);
    return;
#endif
    const QString python = QStandardPaths::findExecutable("python3");
    if (python.isEmpty() || !prepareScripts()) {
        emit statusChanged(tr("Local enhancement needs Python 3 with venv support."));
        emit installationFinished(false);
        return;
    }
    m_installing = true;
    auto *process = new QProcess(this);
    m_process = process;
#ifdef Q_OS_UNIX
    process->setChildProcessModifier([] { ::setsid(); });
#endif
    m_diagnosticBytes = 0;
    process->setProcessChannelMode(QProcess::MergedChannels);
    connect(process, &QProcess::readyReadStandardOutput, this, [this, process]() {
        const auto bytes = process->readAllStandardOutput();
        m_diagnosticBytes += bytes.size();
        if (m_diagnosticBytes > 128 * 1024) {
            fail(tr("Local installation exceeded its diagnostic budget."));
            return;
        }
        const QString line = QString::fromUtf8(bytes).trimmed().section('\n', -1).left(240);
        if (line.startsWith("Preparing") || line.startsWith("Downloading") || line.startsWith("Local world"))
            emit statusChanged(line);
    });
    connect(process, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart)
            fail(tr("The local enhancement installer could not start."));
    });
    connect(process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
        [this](int code, QProcess::ExitStatus status) {
            m_deadline.stop();
            m_installing = false;
            const bool success = code == 0 && status == QProcess::NormalExit && available();
            stopProcess();
            emit statusChanged(success ? tr("Local world intelligence ready.")
                : tr("Local installation failed. Check Python venv support and connectivity, then retry."));
            emit installationFinished(success);
        });
    QStringList arguments{"-I", m_scripts.filePath("install.py"), "--destination", root()};
    if (gpu)
        arguments.append("--gpu");
    process->start(python, arguments);
    m_deadline.start(600000);
    emit statusChanged(tr("Downloading optional local models and runtime. Click Enhance again to cancel."));
}

void SpatialNeuralService::plan(const SpatialWebDocument &document)
{
    if (document.items.isEmpty() || document.items.size() > SpatialWebDocument::maximumItems)
        return;
    QJsonArray items;
    for (const auto &item : document.items) {
        items.append(QJsonObject{{"id", item.id}, {"title", item.title},
            {"category", item.category}, {"description", item.description},
            {"animal", item.kind == "animal" && item.animalForm.isEmpty()}});
    }
    enqueue({{{"operation", "plan"}, {"title", document.title},
        {"evidence", document.evidence}, {"items", items}}, {}, {}});
}

void SpatialNeuralService::cutout(const QString &id, const QImage &image)
{
    if (image.isNull())
        return;
    const auto bounded = image.scaled(512, 320, Qt::KeepAspectRatio, Qt::SmoothTransformation);
    QByteArray bytes;
    QBuffer buffer(&bytes);
    buffer.open(QIODevice::WriteOnly);
    bounded.save(&buffer, "PNG");
    enqueue({{{"operation", "cutout"}, {"image", QString::fromLatin1(bytes.toBase64())}}, {}, id});
}

void SpatialNeuralService::enqueue(Request request)
{
    if (!available() || m_installing || m_queue.size() >= 13)
        return;
    const auto payload = QJsonDocument(request.payload).toJson(QJsonDocument::Compact);
    if (payload.size() >= maximumResponse)
        return;
    request.key = QCryptographicHash::hash(payload, QCryptographicHash::Sha256);
    if (const auto *cached = m_cache.object(request.key)) {
        auto result = *cached;
        result["cached"] = true;
        const auto generation = m_generation;
        QTimer::singleShot(0, this, [this, generation, request, result]() {
            if (generation == m_generation)
                deliver(request, result);
        });
        return;
    }
    m_queue.enqueue(request);
    pump();
}

void SpatialNeuralService::pump()
{
    if (m_busy || m_queue.isEmpty() || m_installing)
        return;
    m_idle.stop();
    if (!m_process) {
        if (!prepareScripts()) {
            fail(tr("Local enhancement scripts are unavailable."));
            return;
        }
        auto *process = new QProcess(this);
        m_process = process;
        m_requests = 0;
        m_diagnosticBytes = 0;
#ifdef Q_OS_UNIX
        process->setChildProcessModifier([] { ::setsid(); });
#endif
        connect(process, &QProcess::readyReadStandardOutput, this, [this, process]() {
            m_output += process->readAllStandardOutput();
            if (m_output.size() > maximumResponse) {
                fail(tr("Local enhancement exceeded its response budget."));
                return;
            }
            const int end = m_output.indexOf('\n');
            if (end < 0)
                return;
            const auto reply = QJsonDocument::fromJson(m_output.left(end)).object();
            if (!m_busy || m_queue.isEmpty() || end + 1 != m_output.size() || !reply.value("success").toBool()) {
                fail(tr("Local enhancement failed. Original content is preserved."));
                return;
            }
            m_output.clear();
            m_deadline.stop();
            m_busy = false;
            const auto request = m_queue.dequeue();
            const auto result = reply.value("result").toObject();
            m_cache.insert(request.key, new QJsonObject(result));
            const auto generation = m_generation;
            deliver(request, result);
            if (generation != m_generation)
                return;
            if (++m_requests >= 256)
                stopProcess();
            if (m_queue.isEmpty())
                m_idle.start();
            else
                QTimer::singleShot(0, this, &SpatialNeuralService::pump);
        });
        connect(process, &QProcess::readyReadStandardError, this, [this, process]() {
            m_diagnosticBytes += process->readAllStandardError().size();
            if (m_diagnosticBytes > 64 * 1024)
                fail(tr("Local inference could not initialize its hardware backend."));
        });
        connect(process, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
            if (error == QProcess::FailedToStart)
                fail(tr("Local inference runtime could not start."));
        });
        connect(process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
            [this](int, QProcess::ExitStatus) {
                if (m_busy)
                    fail(tr("Local inference stopped. Original content is preserved."));
                else
                    stopProcess();
            });
        QStringList arguments{"-I", m_scripts.filePath("worker.py"), "--models", QDir(root()).filePath("models")};
        if (m_preferGpu)
            arguments.append("--gpu");
        process->start(QDir(root()).filePath("runtime/bin/python"), arguments);
    }
    m_busy = true;
    m_output.clear();
    m_process->write(QJsonDocument(m_queue.head().payload).toJson(QJsonDocument::Compact) + '\n');
    m_deadline.start(30000);
}

void SpatialNeuralService::deliver(const Request &request, const QJsonObject &result)
{
    if (request.payload.value("operation") == "plan") {
        emit planReady(result);
    } else if (result.value("accepted").toBool()) {
        auto bytes = QByteArray::fromBase64(result.value("image").toString().toLatin1());
        QBuffer buffer(&bytes);
        buffer.open(QIODevice::ReadOnly);
        QImageReader reader(&buffer, "PNG");
        const QSize size = reader.size();
        if (size.width() < 8 || size.height() < 8 || size.width() > 512 || size.height() > 320)
            return;
        const auto image = reader.read();
        if (!image.isNull())
            emit imageReady(request.id, image);
    }
}

bool SpatialNeuralService::apply(const QJsonObject &proposal, SpatialWebDocument &document)
{
    auto candidate = document;
    if (!features(proposal.value("features"), candidate.worldFeatures))
        return false;
    const auto items = proposal.value("items").toArray();
    if (!proposal.value("items").isArray() || items.size() != candidate.items.size())
        return false;
    for (int index = 0; index < items.size(); ++index) {
        const auto item = items[index].toObject();
        if (item.value("id").toString() != candidate.items[index].id
            || !features(item.value("features"), candidate.items[index].worldFeatures))
            return false;
        const QString animal = item.value("animalForm").toString();
        if (!animal.isEmpty()) {
            if (!SpatialAnimalModel::validForm(animal) || candidate.items[index].kind != "animal")
                return false;
            if (candidate.items[index].animalForm.isEmpty())
                candidate.items[index].animalForm = animal;
        }
    }
    document = candidate;
    return true;
}
