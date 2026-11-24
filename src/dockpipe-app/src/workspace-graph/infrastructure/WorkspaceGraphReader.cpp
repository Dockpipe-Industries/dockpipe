#include "WorkspaceGraphReader.h"
#include "WorkspaceGraphPage.h"
#include "WorkspaceFilesClient.h"

#include <QDir>
#include <QJsonArray>
#include <QJsonDocument>
#include <QProcess>
#include <QStringDecoder>

namespace {
QPointer<QProcess> activeHelper;
QPointer<WorkspaceGraphReader> activeReader;

// Child-owned implementation state preserves the exported reader's object layout.
class ReaderState final : public QObject {
public:
    explicit ReaderState(QObject *parent) : QObject(parent), client(new WorkspaceFilesClient(this))
    {
        setObjectName(QStringLiteral("workspaceGraphAdmissionState"));
    }
    WorkspaceFilesClient *client;
    bool active = false;
};

ReaderState *state(WorkspaceGraphReader *reader)
{
    return static_cast<ReaderState *>(reader->findChild<QObject *>(
        QStringLiteral("workspaceGraphAdmissionState"), Qt::FindDirectChildrenOnly));
}

QString relativePath(const QString &root, const QString &path)
{
    // Pure lexical conversion only. Core admission owns filesystem inspection,
    // root confinement, symlink rejection and cursor validation.
    return QDir::fromNativeSeparators(QDir(root).relativeFilePath(path));
}
}

WorkspaceGraphReader::WorkspaceGraphReader(QObject *parent, int deadlineMilliseconds)
    : QObject(parent), m_deadlineMilliseconds(qBound(1, deadlineMilliseconds, 25000))
{
    new ReaderState(this);
    m_timeout.setSingleShot(true);
    connect(&m_timeout, &QTimer::timeout, this, [this]() {
        m_failure = tr("Directory read timed out. Retry or choose a smaller folder.");
        m_timedOut = true;
        m_abandoned = true;
        const quint64 token = m_pending.isEmpty() ? m_token : m_pendingToken;
        m_pending = {};
        const QPointer<WorkspaceGraphReader> guard(this);
        emit completed(token, {{"error", m_failure}});
        if (guard)
            state(this)->client->cancel();
    });
}

WorkspaceGraphReader::~WorkspaceGraphReader()
{
    // The client reaps its child asynchronously. Keep activeHelper until that
    // process really exits so a blocked filesystem cannot accumulate readers.
    delete state(this);
    if (activeReader == this)
        activeReader = nullptr;
}

bool WorkspaceGraphReader::busy() const
{
    auto *self = const_cast<WorkspaceGraphReader *>(this);
    return state(self)->active || !m_pending.isEmpty();
}

bool WorkspaceGraphReader::helperActive()
{
    return activeReader || (activeHelper && activeHelper->state() != QProcess::NotRunning);
}

void WorkspaceGraphReader::request(quint64 token, const QJsonObject &request)
{
    auto *current = state(this);
    if (current->active && m_timedOut) {
        emit completed(token, {{"error", tr("The timed-out filesystem read is still stopping. No additional reader will be started.")}});
        return;
    }
    m_pending = request;
    m_pendingToken = token;
    if (current->active) {
        m_abandoned = true;
        current->client->cancel();
        m_timeout.start(m_deadlineMilliseconds);
    } else {
        startPending();
    }
}

void WorkspaceGraphReader::cancel()
{
    m_pending = {};
    m_abandoned = true;
    m_timeout.stop();
    state(this)->client->cancel();
}

void WorkspaceGraphReader::startPending()
{
    if (m_pending.isEmpty())
        return;
    const auto request = m_pending;
    m_token = m_pendingToken;
    m_pending = {};
    if (helperActive()) {
        emit completed(m_token, {{"error", tr("The previous directory read is still stopping. Retry in a moment.")}});
        return;
    }
    auto *current = state(this);
    const QString root = request.value("root").toString();
    const bool preview = request.value("operation").toString() == QStringLiteral("preview");
    const QString path = relativePath(root, request.value(preview ? "path" : "directory").toString());
    // This graph belongs to the local Files root. Remote graph support requires
    // an explicit captured remote context; it must never become a local request.
    if (request.contains("target") || request.contains("rootId") || !QDir::isAbsolutePath(root)
        || QJsonDocument(request).toJson(QJsonDocument::Compact).size() > 256 * 1024) {
        emit completed(m_token, {{"error", tr("Invalid local workspace graph request.")}});
        return;
    }
    QJsonObject operation{{"schema", "dockpipe.files/v1"},
        {"action", preview ? "read" : "list"}, {"path", path}};
    if (!preview) {
        operation.insert("limit", request.value("limit"));
        operation.insert("hidden", request.value("hidden").toBool(true));
        operation.insert("after", request.value("after").toString());
        operation.insert("revision", request.value("stamp").toString());
    }
    WorkspaceFilesClient::Context context;
    context.localRoot = root;
    current->client->setContext(context);
    current->active = true;
    activeReader = this;
    m_abandoned = false;
    m_timedOut = false;
    m_failure.clear();
    m_timeout.start(m_deadlineMilliseconds);
    current->client->run(operation, this, [this, preview, path, limit = request.value("limit").toInt()](const WorkspaceFilesClient::Result &result) {
        auto *current = state(this);
        current->active = false;
        activeReader = nullptr;
        m_timeout.stop();
        if (!m_abandoned) {
            QJsonObject page;
            if (!result.success()) {
                page.insert("error", result.error);
            } else if (result.value.value("path").toString() != path) {
                page.insert("error", tr("Directory reader returned a different workspace path."));
            } else if (preview) {
                const QByteArray bytes = QByteArray::fromBase64(result.value.value("content").toString().toLatin1());
                const QByteArray excerpt = bytes.left(8192);
                if (excerpt.contains('\0')) {
                    page.insert("text", tr("Binary file · Open in the file viewer for details."));
                } else {
                    QStringDecoder decoder(QStringDecoder::Utf8);
                    const QString text = decoder(excerpt);
                    if (decoder.hasError()) {
                        page.insert("text", tr("Non-UTF-8 file · Text preview unavailable."));
                    } else {
                        const auto lines = text.split('\n');
                        page.insert("text", lines.mid(0, 128).join('\n'));
                        page.insert("truncated", bytes.size() > excerpt.size() || lines.size() > 128);
                    }
                }
            } else if (!result.value.value("entries").isArray()
                && result.value.contains("entries")) {
                page.insert("error", tr("Invalid directory page response."));
            } else if (result.value.value("entries").toArray().size() > qMin(limit, WorkspaceGraphPage::maximumPageNodes)) {
                page.insert("error", tr("Directory page exceeds the graph node limit."));
            } else {
                page = result.value;
                page.insert("stamp", page.take("revision"));
            }
            const QPointer<WorkspaceGraphReader> guard(this);
            emit completed(m_token, page);
            if (!guard)
                return;
        }
        startPending();
    }, m_deadlineMilliseconds);
    activeHelper = current->client->findChild<QProcess *>();
}

// Retained private symbols for the exported class's existing binary layout.
void WorkspaceGraphReader::drain() {}
void WorkspaceGraphReader::finish(bool) {}
