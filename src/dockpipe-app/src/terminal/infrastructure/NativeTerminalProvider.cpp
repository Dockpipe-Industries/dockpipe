#include "NativeTerminalProvider.h"
#include "NativeTerminalLaunch.h"
#include "TerminalSession.h"

#include <QCoreApplication>
#include <QElapsedTimer>
#include <QJsonDocument>
#include <QJsonArray>
#include <QJsonObject>
#include <QJsonParseError>
#include <QQueue>
#include <QSet>
#include <QTimer>
#include <cmath>
#include <condition_variable>
#include <mutex>
#include <limits>
#include <thread>

#ifdef Q_OS_WIN
#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <windows.h>
#else
#include <cerrno>
#include <signal.h>
#include <pthread.h>
#include <unistd.h>
#endif

namespace {
constexpr qsizetype LaunchLineLimit = 2 * 1024 * 1024;
constexpr qsizetype CommandLineLimit = 96 * 1024;
constexpr qsizetype InputLimit = 64 * 1024;
constexpr qsizetype OutputLimit = 256 * 1024;
constexpr qsizetype ControlReserve = 4096;
constexpr quint64 MaximumId = 9007199254740991ULL;
const QString Schema = QStringLiteral("dockpipe.local-terminal/v1");

// QJsonDocument validates syntax and trailing input but collapses duplicate
// members. A second bounded token walk rejects those before DTO decoding. The
// maximum protocol nesting is four; eight permits the parser's string wrapper.
class UniqueMembers final {
public:
    explicit UniqueMembers(const QByteArray &bytes) : m_bytes(bytes) {}
    bool valid() { return value(0); }
private:
    const QByteArray &m_bytes;
    qsizetype m_position = 0;
    int m_nodes = 0;

    void whitespace()
    {
        while (m_position < m_bytes.size() && (m_bytes.at(m_position) == ' '
            || m_bytes.at(m_position) == '\t' || m_bytes.at(m_position) == '\r' || m_bytes.at(m_position) == '\n'))
            ++m_position;
    }

    QByteArray string()
    {
        const qsizetype beginning = m_position++;
        while (m_position < m_bytes.size()) {
            const char character = m_bytes.at(m_position++);
            if (character == '\\') {
                ++m_position;
            } else if (character == '"') {
                return m_bytes.mid(beginning, m_position - beginning);
            }
        }
        return {};
    }

    bool value(int depth)
    {
        whitespace();
        if (depth > 8 || ++m_nodes > 16384 || m_position >= m_bytes.size())
            return false;
        const char character = m_bytes.at(m_position);
        if (character == '"')
            return !string().isEmpty();
        if (character == '{' || character == '[') {
            ++m_position;
            QSet<QString> keys;
            const char end = character == '{' ? '}' : ']';
            whitespace();
            if (m_position < m_bytes.size() && m_bytes.at(m_position) == end) {
                ++m_position;
                return true;
            }
            while (m_position < m_bytes.size()) {
                whitespace();
                if (character == '{') {
                    if (m_bytes.at(m_position) != '"')
                        return false;
                    const auto encoded = string();
                    const QJsonDocument decoded = QJsonDocument::fromJson('[' + encoded + ']');
                    const QString key = decoded.array().at(0).toString();
                    if (keys.contains(key))
                        return false;
                    keys.insert(key);
                    whitespace();
                    if (m_position >= m_bytes.size() || m_bytes.at(m_position++) != ':')
                        return false;
                }
                if (!value(depth + 1))
                    return false;
                whitespace();
                if (m_position >= m_bytes.size())
                    return false;
                const char separator = m_bytes.at(m_position++);
                if (separator == end)
                    return true;
                if (separator != ',')
                    return false;
            }
            return false;
        }
        const qsizetype beginning = m_position;
        while (m_position < m_bytes.size() && m_bytes.at(m_position) != ','
            && m_bytes.at(m_position) != '}' && m_bytes.at(m_position) != ']')
            ++m_position;
        return m_position > beginning;
    }
};

bool fields(const QJsonObject &object, const QSet<QString> &required)
{
    if (object.size() != required.size())
        return false;
    for (auto entry = object.begin(); entry != object.end(); ++entry) {
        if (!required.contains(entry.key()))
            return false;
    }
    return true;
}

bool integer(const QJsonValue &value, quint64 minimum, quint64 maximum, quint64 &result)
{
    if (!value.isDouble())
        return false;
    const double number = value.toDouble();
    if (!std::isfinite(number) || number < double(minimum) || number > double(maximum) || std::floor(number) != number)
        return false;
    result = quint64(number);
    return true;
}

struct StdioState final {
    std::mutex mutex;
    std::condition_variable changed;
    QByteArray incoming;
    QQueue<QByteArray> outgoing;
    qsizetype outputBytes = 0; // Includes the writer's in-flight bytes.
    bool inputEnded = false;
    bool outputFailed = false;
    bool stopped = false;

    void read()
    {
        char buffer[8192];
        while (true) {
#ifdef Q_OS_WIN
            DWORD count = 0;
            const bool success = ReadFile(GetStdHandle(STD_INPUT_HANDLE), buffer, DWORD(sizeof(buffer)), &count, nullptr);
            const qint64 received = success ? qint64(count) : -1;
#else
            const qint64 received = ::read(STDIN_FILENO, buffer, sizeof(buffer));
            if (received < 0 && errno == EINTR)
                continue;
#endif
            std::unique_lock<std::mutex> lock(mutex);
            if (received <= 0) {
                inputEnded = true;
                return;
            }
            changed.wait(lock, [&] { return stopped || incoming.size() + received <= LaunchLineLimit + 8192; });
            if (stopped)
                return;
            incoming.append(buffer, received);
        }
    }

    void write()
    {
#ifndef Q_OS_WIN
        // Block SIGPIPE only on this private writer. Terminal children retain
        // their ordinary signal disposition and never inherit an ignored one.
        sigset_t blocked;
        ::sigemptyset(&blocked);
        ::sigaddset(&blocked, SIGPIPE);
        ::pthread_sigmask(SIG_BLOCK, &blocked, nullptr);
#endif
        while (true) {
            QByteArray bytes;
            {
                std::unique_lock<std::mutex> lock(mutex);
                changed.wait(lock, [&] { return stopped || !outgoing.isEmpty(); });
                if (outgoing.isEmpty())
                    return;
                bytes = outgoing.dequeue();
            }
            qsizetype offset = 0;
            while (offset < bytes.size()) {
#ifdef Q_OS_WIN
                DWORD count = 0;
                const bool success = WriteFile(GetStdHandle(STD_OUTPUT_HANDLE), bytes.constData() + offset,
                    DWORD(bytes.size() - offset), &count, nullptr);
                const qint64 written = success ? qint64(count) : -1;
#else
                const qint64 written = ::write(STDOUT_FILENO, bytes.constData() + offset, size_t(bytes.size() - offset));
                if (written < 0 && errno == EINTR)
                    continue;
#endif
                std::lock_guard<std::mutex> lock(mutex);
                if (written <= 0) {
                    outputFailed = true;
                    return;
                }
                offset += written;
                outputBytes -= written;
            }
        }
    }
};
}

struct NativeTerminalProvider::State {
    std::shared_ptr<StdioState> stdio = std::make_shared<StdioState>();
    TerminalSession terminal{TerminalLaunchContext::Local};
    QTimer poll;
    QByteArray input;
    bool started = false;
    bool ready = false;
    bool closing = false;
    bool completed = false;
    bool failed = false;
    int exitCode = -1;
    quint64 lastId = 0;
    quint64 pendingInputId = 0;
    quint64 pendingInputEnd = 0;
    qsizetype pendingInputBytes = 0;
    QElapsedTimer cleanupDeadline;
    QElapsedTimer outputDeadline;
};

NativeTerminalProvider::NativeTerminalProvider(QObject *parent)
    : QObject(parent), m_state(std::make_unique<State>())
{
    connect(&m_state->poll, &QTimer::timeout, this, [this] { poll(); });
    connect(&m_state->terminal, &TerminalSession::started, this, [this] {
        if (m_state->closing)
            return;
        m_state->ready = true;
        send({{QStringLiteral("type"), QStringLiteral("ready")}, {QStringLiteral("id"), 1}});
    });
    connect(&m_state->terminal, &TerminalSession::stateChanged, this, [this] {
        // A startup error may be emitted inside native descriptor setup. Let
        // that stack unwind before cancellation takes ownership of teardown.
        QTimer::singleShot(0, this, [this] {
            if (!m_state->closing && !m_state->terminal.error().isEmpty())
                fail(m_state->ready ? QStringLiteral("provider_failed") : QStringLiteral("start_failed"), m_state->ready ? 0 : 1);
        });
    });
    connect(&m_state->terminal, &TerminalSession::inputWritten, this, [this](quint64) { acknowledgeInput(); });
    connect(&m_state->terminal, &TerminalSession::output, this, [this](const QByteArray &bytes) {
        if (m_state->completed)
            return;
        for (qsizetype offset = 0; offset < bytes.size(); offset += 32 * 1024) {
            if (!send({{QStringLiteral("type"), QStringLiteral("output")},
                    {QStringLiteral("id"), 0},
                    {QStringLiteral("data"), QString::fromLatin1(bytes.mid(offset, 32 * 1024).toBase64())}}, false)) {
                fail(QStringLiteral("output_overflow"));
                return;
            }
        }
    });
    connect(&m_state->terminal, &TerminalSession::finished, this, [this](int exitCode) {
        acknowledgeInput();
        m_state->exitCode = exitCode;
        if (!m_state->ready) {
            fail(QStringLiteral("start_failed"), 1);
        } else if (!m_state->terminal.error().isEmpty()) {
            fail(QStringLiteral("provider_failed"));
        } else {
            close();
        }
    });
    connect(&m_state->terminal, &TerminalSession::cleanupFinished, this, [this](bool verified) { complete(verified); });
}

NativeTerminalProvider::~NativeTerminalProvider()
{
    const auto io = m_state->stdio;
    std::lock_guard<std::mutex> lock(io->mutex);
    io->stopped = true;
    io->changed.notify_all();
}

void NativeTerminalProvider::start()
{
    const auto io = m_state->stdio;
    std::thread([io] { io->read(); }).detach();
    std::thread([io] { io->write(); }).detach();
    m_state->poll.setInterval(4);
    m_state->poll.setTimerType(Qt::PreciseTimer);
    m_state->poll.start();
}

bool NativeTerminalProvider::send(const QJsonObject &payload, bool control)
{
    QJsonObject frame = payload;
    frame.insert(QStringLiteral("schema"), Schema);
    const QByteArray bytes = QJsonDocument(frame).toJson(QJsonDocument::Compact) + '\n';
    const auto io = m_state->stdio;
    std::lock_guard<std::mutex> lock(io->mutex);
    const qsizetype limit = control ? OutputLimit : OutputLimit - ControlReserve;
    if (io->outputFailed || bytes.size() > limit - io->outputBytes)
        return false;
    io->outgoing.enqueue(bytes);
    io->outputBytes += bytes.size();
    io->changed.notify_all();
    return true;
}

void NativeTerminalProvider::poll()
{
    bool ended = false;
    bool inputOverflow = false;
    bool outputFailed = false;
    qsizetype outputBytes = 0;
    {
        const auto io = m_state->stdio;
        std::lock_guard<std::mutex> lock(io->mutex);
        inputOverflow = io->incoming.size() > LaunchLineLimit + 8192 - m_state->input.size();
        if (!inputOverflow)
            m_state->input.append(io->incoming);
        io->incoming.clear();
        ended = io->inputEnded;
        outputFailed = io->outputFailed;
        outputBytes = io->outputBytes;
        io->changed.notify_all();
    }
    if (m_state->completed) {
        if (outputBytes == 0 || outputFailed || m_state->outputDeadline.elapsed() >= 2000)
            QCoreApplication::exit(m_state->failed ? 1 : 0);
        return;
    }
    if (outputFailed)
        close();
    if (inputOverflow)
        fail(QStringLiteral("invalid_frame"));
    if (m_state->closing) {
        m_state->input.clear();
        if (m_state->cleanupDeadline.elapsed() >= 3000)
            complete(false);
        return;
    }
    const qsizetype limit = m_state->started ? CommandLineLimit : LaunchLineLimit;
    const qsizetype newline = m_state->input.indexOf('\n');
    if ((newline < 0 && m_state->input.size() > limit) || newline > limit) {
        fail(QStringLiteral("invalid_frame"));
        return;
    }
    if (newline >= 0) {
        const QByteArray line = m_state->input.left(newline);
        m_state->input.remove(0, newline + 1);
        receive(line);
        return;
    }
    if (ended) {
        if (!m_state->input.isEmpty())
            fail(QStringLiteral("invalid_frame"));
        else
            close();
    }
}

void NativeTerminalProvider::receive(const QByteArray &line)
{
    QJsonParseError parseError;
    const QJsonDocument document = QJsonDocument::fromJson(line, &parseError);
    if (parseError.error != QJsonParseError::NoError || !document.isObject() || !UniqueMembers(line).valid()) {
        fail(QStringLiteral("invalid_frame"));
        return;
    }
    const QJsonObject object = document.object();
    quint64 id = 0;
    if (object.value(QStringLiteral("schema")) != Schema || !object.value(QStringLiteral("type")).isString()
        || !integer(object.value(QStringLiteral("id")), 1, MaximumId, id) || id <= m_state->lastId) {
        fail(QStringLiteral("invalid_frame"));
        return;
    }
    const QString type = object.value(QStringLiteral("type")).toString();
    m_state->lastId = id;
    const QSet<QString> common = {QStringLiteral("schema"), QStringLiteral("type"), QStringLiteral("id")};
    if (!m_state->started) {
        auto expected = common;
        expected.unite({QStringLiteral("profile"), QStringLiteral("columns"), QStringLiteral("rows")});
        quint64 columns = 0;
        quint64 rows = 0;
        NativeTerminalLaunch::Snapshot launch;
        if (type != QStringLiteral("start") || id != 1 || !fields(object, expected)
            || !object.value(QStringLiteral("profile")).isObject()
            || !integer(object.value(QStringLiteral("columns")), 1, 1000, columns)
            || !integer(object.value(QStringLiteral("rows")), 1, 1000, rows)
            || !NativeTerminalLaunch::fromJson(object.value(QStringLiteral("profile")).toObject(), launch)) {
            fail(QStringLiteral("invalid_profile"), id);
            return;
        }
        m_state->terminal.resize(int(columns), int(rows));
        m_state->started = true;
        if (!m_state->terminal.startPrepared(launch)) {
            fail(QStringLiteral("start_failed"), id);
            return;
        }
    } else if (!m_state->ready) {
        fail(QStringLiteral("invalid_frame"), id);
    } else if (type == QStringLiteral("input")) {
        auto expected = common;
        expected.insert(QStringLiteral("data"));
        if (!fields(object, expected) || !object.value(QStringLiteral("data")).isString()
            || m_state->pendingInputId != 0) {
            fail(QStringLiteral("invalid_frame"), id);
            return;
        }
        const QByteArray encoded = object.value(QStringLiteral("data")).toString().toLatin1();
        const auto decoded = QByteArray::fromBase64Encoding(encoded, QByteArray::AbortOnBase64DecodingErrors);
        if (!decoded || decoded.decoded.isEmpty() || decoded.decoded.size() > InputLimit
            || decoded.decoded.toBase64() != encoded) {
            fail(QStringLiteral("invalid_frame"), id);
            return;
        }
        if (m_state->terminal.inputWrittenCursor() > std::numeric_limits<quint64>::max() - quint64(decoded.decoded.size())) {
            fail(QStringLiteral("input_failed"), id);
            return;
        }
        m_state->pendingInputId = id;
        m_state->pendingInputBytes = decoded.decoded.size();
        m_state->pendingInputEnd = m_state->terminal.inputWrittenCursor() + quint64(decoded.decoded.size());
        if (!m_state->terminal.send(decoded.decoded)) {
            fail(QStringLiteral("input_failed"), id);
            return;
        }
        acknowledgeInput();
    } else if (type == QStringLiteral("resize")) {
        auto expected = common;
        expected.unite({QStringLiteral("columns"), QStringLiteral("rows")});
        quint64 columns = 0;
        quint64 rows = 0;
        if (!fields(object, expected) || !integer(object.value(QStringLiteral("columns")), 1, 1000, columns)
            || !integer(object.value(QStringLiteral("rows")), 1, 1000, rows)) {
            fail(QStringLiteral("invalid_frame"), id);
            return;
        }
        m_state->terminal.resize(int(columns), int(rows));
        send({{QStringLiteral("type"), QStringLiteral("ack")}, {QStringLiteral("id"), double(id)}});
    } else if (type == QStringLiteral("close") && fields(object, common)) {
        close();
    } else {
        fail(QStringLiteral("invalid_frame"), id);
    }
}

void NativeTerminalProvider::acknowledgeInput()
{
    if (m_state->pendingInputId == 0 || m_state->terminal.inputWrittenCursor() < m_state->pendingInputEnd)
        return;
    send({{QStringLiteral("type"), QStringLiteral("ack")},
        {QStringLiteral("id"), double(m_state->pendingInputId)},
        {QStringLiteral("bytes"), double(m_state->pendingInputBytes)}});
    m_state->pendingInputId = 0;
}

void NativeTerminalProvider::fail(const QString &code, quint64 id)
{
    if (m_state->completed)
        return;
    if (!m_state->failed)
        send({{QStringLiteral("type"), QStringLiteral("error")}, {QStringLiteral("id"), double(id)},
            {QStringLiteral("code"), code}});
    m_state->failed = true;
    close();
}

void NativeTerminalProvider::close()
{
    if (m_state->closing || m_state->completed)
        return;
    m_state->closing = true;
    m_state->cleanupDeadline.start();
    if (!m_state->started) {
        complete(true);
        return;
    }
    m_state->terminal.stopAndWait();
}

void NativeTerminalProvider::complete(bool verified)
{
    if (m_state->completed)
        return;
    acknowledgeInput();
    if (m_state->pendingInputId != 0) {
        // A partial native write has uncertain external effects. Report it
        // without retrying the bytes or manufacturing a queue-acceptance ACK.
        if (!m_state->failed)
            send({{QStringLiteral("type"), QStringLiteral("error")},
                {QStringLiteral("id"), double(m_state->pendingInputId)},
                {QStringLiteral("code"), QStringLiteral("input_failed")}});
        m_state->failed = true;
        verified = false;
    }
    m_state->completed = true;
    send({{QStringLiteral("type"), QStringLiteral("closed")}, {QStringLiteral("id"), 0},
        {QStringLiteral("exit_code"), m_state->exitCode},
        {QStringLiteral("cleanup"), verified ? QStringLiteral("complete") : QStringLiteral("unknown")}});
    m_state->outputDeadline.start();
}
