#include "PipeLangLanguageClient.h"

#include <QDir>
#include <QElapsedTimer>
#include <QProcessEnvironment>
#include <QFileInfo>
#include <QJsonDocument>
#include <QJsonParseError>
#include <QUrl>
#include <limits>

namespace {
// Match the shared server's documented document/aggregate contracts. Frames allow
// one full aggregate response; headers and queued writes have independent bounds.
constexpr qint64 MaximumDocumentBytes = 1024 * 1024;
constexpr qint64 MaximumOpenBytes = 16 * MaximumDocumentBytes;
constexpr qint64 MaximumFrameBytes = MaximumOpenBytes + 1024 * 1024;
constexpr qint64 MaximumHeaderBytes = 8192;
constexpr int RequestDeadlineMs = 15000;

bool selectedFile(const QString &path, bool executable)
{
    const QFileInfo info(path);
    return QDir::isAbsolutePath(path) && info.isFile()
        && info.isReadable() && (!executable || info.isExecutable());
}
}

PipeLangLanguageClient::PipeLangLanguageClient(QObject *parent) : QObject(parent)
{
    m_process.setProcessChannelMode(QProcess::SeparateChannels);
    m_deadline.setSingleShot(true);
    m_changes.setSingleShot(true);
    m_changes.setInterval(150);
    m_hoverDeadline.setSingleShot(true);
    connect(&m_changes, &QTimer::timeout, this, &PipeLangLanguageClient::flushDocuments);
    connect(&m_hoverDeadline, &QTimer::timeout, this, [this] {
        cancelHover();
        emit statusChanged(tr("PipeLang hover timed out."));
    });
    connect(&m_deadline, &QTimer::timeout, this, [this] {
        if (m_state == State::Failed) {
            m_process.kill();
            emit statusChanged(tr("PipeLang server required forced termination after stdin closure."));
        } else {
            fail(tr("PipeLang server lifecycle deadline exceeded."));
        }
    });
    connect(&m_process, &QProcess::started, this, [this] {
        if (m_state != State::Starting)
            return;

        m_state = State::Initializing;
        const QJsonObject options{{"executablePath", m_configuration.compilerPath},
            {"sourceRoots", m_configuration.sourceRoot.isEmpty()
                ? QJsonArray{} : QJsonArray{m_configuration.sourceRoot}}, {"timeoutMs", 10000}};
        m_initializeId = request(QStringLiteral("initialize"), {
            {"processId", QJsonValue::Null},
            {"rootUri", m_configuration.sourceRoot.isEmpty()
                ? QJsonValue(QJsonValue::Null) : QJsonValue(uri(m_configuration.sourceRoot))},
            {"capabilities", QJsonObject{{"general", QJsonObject{
                {"positionEncodings", QJsonArray{"utf-16"}}}}}},
            {"initializationOptions", options}});
    });
    connect(&m_process, &QProcess::readyReadStandardOutput, this, &PipeLangLanguageClient::receive);
    connect(&m_process, &QProcess::readyReadStandardError, this, [this] {
        // Drain in bounded chunks. Child stderr is never retained as an unbounded
        // log or rendered as markup supplied by the selected executable.
        const auto channel = m_process.readChannel();
        m_process.setReadChannel(QProcess::StandardError);
        while (m_process.bytesAvailable() > 0)
            m_process.read(4096);
        m_process.setReadChannel(channel);
    });
    connect(&m_process, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart)
            fail(tr("PipeLang server could not start: %1").arg(m_process.errorString()));
    });
    connect(&m_process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
        [this](int code, QProcess::ExitStatus exitStatus) {
            const bool expected = m_state == State::Stopping;
            m_deadline.stop();
            m_state = State::Stopped;
            clearResults();
            emit statusChanged(expected && code == 0 && exitStatus == QProcess::NormalExit
                ? tr("PipeLang server stopped.") : tr("PipeLang server exited (%1). Reconnect explicitly.").arg(code));
        });
}

PipeLangLanguageClient::~PipeLangLanguageClient()
{
    stop();
    disconnect(&m_process, nullptr, this, nullptr);
    QElapsedTimer shutdown;
    shutdown.start();
    while (m_process.state() != QProcess::NotRunning && shutdown.elapsed() < 1000) {
        m_process.waitForReadyRead(25);
        receive();
        m_process.waitForFinished(0);
    }
    if (m_process.state() != QProcess::NotRunning) {
        m_process.kill();
        m_process.waitForFinished(1000);
    }
}

QString PipeLangLanguageClient::uri(const QString &path)
{
    return QUrl::fromLocalFile(path).toString(QUrl::FullyEncoded);
}

bool PipeLangLanguageClient::start(const Configuration &configuration)
{
    if (m_state != State::Stopped || m_process.state() != QProcess::NotRunning) {
        emit statusChanged(tr("Stop the current PipeLang server before reconnecting."));
        return false;
    }
    QString invalidSelection;
    if (!selectedFile(configuration.nodePath, true))
        invalidSelection = tr("Node executable: %1").arg(configuration.nodePath);
    else if (!selectedFile(configuration.serverPath, false))
        invalidSelection = tr("server entrypoint: %1").arg(configuration.serverPath);
    else if (!selectedFile(configuration.compilerPath, true))
        invalidSelection = tr("compiler executable: %1").arg(configuration.compilerPath);
    else if (!configuration.sourceRoot.isEmpty()
        && (!QDir::isAbsolutePath(configuration.sourceRoot) || !QFileInfo(configuration.sourceRoot).isDir()))
        invalidSelection = tr("source directory: %1").arg(configuration.sourceRoot);

    if (!invalidSelection.isEmpty()) {
        emit statusChanged(tr("Invalid absolute installed PipeLang selection (%1).").arg(invalidSelection));
        return false;
    }

    m_configuration = configuration;
    m_input.clear();
    m_bodyLength = -1;
    clearResults();
    m_state = State::Starting;
    // The installed server's directory is the process cwd, never the project.
    m_process.setWorkingDirectory(QFileInfo(configuration.serverPath).absolutePath());
    auto environment = QProcessEnvironment::systemEnvironment();
    environment.remove(QStringLiteral("NODE_OPTIONS"));
    environment.remove(QStringLiteral("NODE_PATH"));
    m_process.setProcessEnvironment(environment);
    m_process.setProgram(configuration.nodePath);
    m_process.setArguments({configuration.serverPath});
    m_deadline.start(RequestDeadlineMs);
    m_process.start();
    emit statusChanged(tr("Starting selected PipeLang server…"));
    return true;
}

bool PipeLangLanguageClient::isReady() const { return m_state == State::Ready; }

void PipeLangLanguageClient::clearResults()
{
    m_hoverDeadline.stop();
    m_hoverId = 0;
    m_changes.stop();
    for (auto it = m_documents.begin(); it != m_documents.end(); ++it) {
        it->opened = false;
        it->dirty = true;
        emit diagnostics(it.key(), it->version, {});
    }
}

void PipeLangLanguageClient::stop()
{
    if (m_state == State::Stopped || m_state == State::Stopping || m_state == State::Failed)
        return;

    cancelHover();
    const bool initialized = m_state == State::Ready;
    m_state = State::Stopping;
    clearResults();
    m_deadline.start(2000);
    if (initialized) {
        m_shutdownId = request(QStringLiteral("shutdown"));
    } else {
        m_process.kill();
    }
}

void PipeLangLanguageClient::fail(const QString &message)
{
    m_deadline.stop();
    m_state = m_process.state() == QProcess::NotRunning ? State::Stopped : State::Failed;
    clearResults();
    if (m_state == State::Failed) {
        // The canonical server aborts compiler work when stdin ends. Give that
        // cleanup a bounded grace before terminating the directly owned Node.
        m_process.closeWriteChannel();
        m_deadline.start(500);
    }
    emit statusChanged(message);
}

void PipeLangLanguageClient::send(const QJsonObject &message)
{
    const QByteArray body = QJsonDocument(message).toJson(QJsonDocument::Compact);
    const QByteArray header = "Content-Length: " + QByteArray::number(body.size()) + "\r\n\r\n";
    if (body.size() > MaximumFrameBytes
        || m_process.bytesToWrite() + header.size() + body.size() > 2 * MaximumFrameBytes) {
        fail(tr("PipeLang protocol output limit exceeded."));
        return;
    }

    const QByteArray frame = header + body;
    if (m_process.write(frame) != frame.size())
        fail(tr("PipeLang protocol write failed."));
}

void PipeLangLanguageClient::notify(const QString &method, const QJsonObject &parameters)
{
    send({{"jsonrpc", "2.0"}, {"method", method}, {"params", parameters}});
}

int PipeLangLanguageClient::request(const QString &method, const QJsonObject &parameters)
{
    if (m_nextId == std::numeric_limits<int>::max()) {
        fail(tr("PipeLang request identity space exhausted; restart the editor session."));
        return 0;
    }
    const int id = ++m_nextId;
    send({{"jsonrpc", "2.0"}, {"id", id}, {"method", method}, {"params", parameters}});
    return id;
}

void PipeLangLanguageClient::setDocument(const QString &path, const QString &text)
{
    if (!QDir::isAbsolutePath(path) || QFileInfo(path).suffix() != QStringLiteral("pipe"))
        return;
    auto found = m_documents.find(path);
    if (found != m_documents.end() && found->text == text)
        return;

    qint64 bytes = text.toUtf8().size();
    qint64 aggregate = bytes;
    for (auto it = m_documents.cbegin(); it != m_documents.cend(); ++it) {
        if (it.key() != path)
            aggregate += it->text.toUtf8().size();
    }
    if (bytes > MaximumDocumentBytes || aggregate > MaximumOpenBytes
        || (found == m_documents.end() && m_documents.size() >= 32)
        || m_nextVersion == std::numeric_limits<int>::max()) {
        closeDocument(path);
        emit statusChanged(tr("PipeLang document limit exceeded (1 MiB per file, 16 MiB total, 32 files)."));
        return;
    }

    if (m_hoverPath == path)
        cancelHover();
    if (found == m_documents.end()) {
        found = m_documents.insert(path, Document{text});
    } else {
        found->text = text;
        found->dirty = true;
    }
    found->version = ++m_nextVersion;
    emit documentVersionChanged(path, found->version);
    emit diagnostics(path, found->version, {});
    if (isReady())
        m_changes.start();
}

void PipeLangLanguageClient::closeDocument(const QString &path)
{
    const auto found = m_documents.find(path);
    if (found == m_documents.end())
        return;
    if (m_hoverPath == path)
        cancelHover();
    if (isReady() && found->opened)
        notify(QStringLiteral("textDocument/didClose"), {{"textDocument", QJsonObject{{"uri", uri(path)}}}});
    emit diagnostics(path, found->version, {});
    m_documents.erase(found);
}

void PipeLangLanguageClient::flushDocuments()
{
    if (!isReady())
        return;
    for (auto it = m_documents.begin(); it != m_documents.end(); ++it) {
        if (!it->dirty)
            continue;
        QJsonObject document{{"uri", uri(it.key())}, {"version", it->version}};
        if (!it->opened) {
            document.insert("languageId", "pipelang");
            document.insert("text", it->text);
            notify(QStringLiteral("textDocument/didOpen"), {{"textDocument", document}});
        } else {
            notify(QStringLiteral("textDocument/didChange"), {{"textDocument", document},
                {"contentChanges", QJsonArray{QJsonObject{{"text", it->text}}}}});
        }
        it->opened = true;
        it->dirty = false;
    }
}

void PipeLangLanguageClient::cancelHover()
{
    m_hoverDeadline.stop();
    if (m_hoverId && isReady())
        notify(QStringLiteral("$/cancelRequest"), {{"id", m_hoverId}});
    m_hoverId = 0;
}

void PipeLangLanguageClient::hover(const QString &path, int line, int character)
{
    cancelHover();
    if (!isReady() || !m_documents.contains(path) || line < 0 || character < 0)
        return;
    flushDocuments();
    m_hoverPath = path;
    m_hoverVersion = m_documents.value(path).version;
    m_hoverId = request(QStringLiteral("textDocument/hover"), {
        {"textDocument", QJsonObject{{"uri", uri(path)}}},
        {"position", QJsonObject{{"line", line}, {"character", character}}}});
    m_hoverDeadline.start(RequestDeadlineMs);
}

void PipeLangLanguageClient::receive()
{
    while (m_process.bytesAvailable() > 0) {
        if (m_state == State::Failed || m_state == State::Stopped) {
            m_process.read(65536);
            continue;
        }
        if (m_input.size() >= MaximumFrameBytes + MaximumHeaderBytes) {
            fail(tr("PipeLang protocol input limit exceeded."));
            return;
        }
        m_input += m_process.read(qMin<qint64>(65536,
            MaximumFrameBytes + MaximumHeaderBytes - m_input.size()));
        while (true) {
            if (m_bodyLength < 0) {
                const auto end = m_input.indexOf("\r\n\r\n");
                if (end < 0) {
                    if (m_input.size() > MaximumHeaderBytes)
                        fail(tr("PipeLang protocol header limit exceeded."));
                    break;
                }
                if (end > MaximumHeaderBytes) {
                    fail(tr("PipeLang protocol header limit exceeded."));
                    return;
                }
                int lengths = 0;
                for (const auto &line : m_input.left(end).split('\n')) {
                    if (!line.trimmed().toLower().startsWith("content-length:"))
                        continue;
                    bool valid = false;
                    const auto length = line.mid(line.indexOf(':') + 1).trimmed().toLongLong(&valid);
                    if (!valid || length <= 0 || length > MaximumFrameBytes) {
                        fail(tr("Invalid PipeLang protocol Content-Length."));
                        return;
                    }
                    m_bodyLength = length;
                    ++lengths;
                }
                if (lengths != 1) {
                    fail(tr("PipeLang protocol requires one Content-Length header."));
                    return;
                }
                m_input.remove(0, end + 4);
            }
            if (m_input.size() < m_bodyLength)
                break;

            QJsonParseError error;
            const auto message = QJsonDocument::fromJson(m_input.left(m_bodyLength), &error);
            m_input.remove(0, m_bodyLength);
            m_bodyLength = -1;
            if (error.error != QJsonParseError::NoError || !message.isObject()) {
                fail(tr("Invalid PipeLang JSON-RPC message."));
                return;
            }
            handle(message.object());
            if (m_state == State::Stopped)
                return;
        }
        if (m_state == State::Stopped)
            return;
    }
}

void PipeLangLanguageClient::handle(const QJsonObject &message)
{
    if (message.value("jsonrpc") != QJsonValue("2.0")) {
        fail(tr("Invalid PipeLang JSON-RPC version."));
        return;
    }
    if (message.contains("method")) {
        if (message.contains("id")) {
            send({{"jsonrpc", "2.0"}, {"id", message.value("id")},
                {"error", QJsonObject{{"code", -32601}, {"message", "Client method not supported"}}}});
            return;
        }
        if (message.value("method") == QJsonValue("textDocument/publishDiagnostics") && isReady()) {
            const auto parameters = message.value("params").toObject();
            const auto documentUri = QUrl(parameters.value("uri").toString());
            const QString path = documentUri.isLocalFile() ? documentUri.toLocalFile() : QString();
            const auto found = m_documents.constFind(path);
            const auto version = parameters.value("version");
            if (found != m_documents.cend() && !found->dirty && version.isDouble()
                && version.toDouble() == found->version && parameters.value("diagnostics").isArray())
                emit diagnostics(path, found->version, parameters.value("diagnostics").toArray());
        }
        return;
    }
    const auto responseId = message.value("id");
    if (!responseId.isDouble())
        return;
    const double id = responseId.toDouble();
    if (m_state == State::Initializing && id == m_initializeId) {
        const auto capabilities = message.value("result").toObject().value("capabilities").toObject();
        if (message.contains("error") || capabilities.value("hoverProvider") != QJsonValue(true)
            || (capabilities.contains("positionEncoding") && capabilities.value("positionEncoding") != QJsonValue("utf-16"))) {
            fail(tr("Selected PipeLang server does not provide the required UTF-16 hover contract."));
            return;
        }
        m_deadline.stop();
        m_state = State::Ready;
        notify(QStringLiteral("initialized"));
        flushDocuments();
        emit statusChanged(tr("PipeLang compiler diagnostics and hover connected."));
        emit ready();
    } else if (m_state == State::Stopping && id == m_shutdownId) {
        if (message.contains("error")) {
            fail(tr("PipeLang server rejected shutdown."));
            return;
        }
        notify(QStringLiteral("exit"));
        m_process.closeWriteChannel();
    } else if (isReady() && m_hoverId && id == m_hoverId) {
        m_hoverDeadline.stop();
        m_hoverId = 0;
        const auto found = m_documents.constFind(m_hoverPath);
        if (found != m_documents.cend() && found->version == m_hoverVersion && !message.contains("error"))
            emit hoverResult(m_hoverPath, m_hoverVersion, message.value("result").toObject().value("contents"));
    }
}
