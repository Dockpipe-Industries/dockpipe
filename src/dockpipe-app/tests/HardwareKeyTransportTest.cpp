#include "RemoteCommand.h"

#include <QCoreApplication>
#include <QEventLoop>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QThread>
#include <QTimer>
#include <cstdio>
#ifdef Q_OS_WIN
#include <fcntl.h>
#include <io.h>
#endif

namespace {
const QString secret = "private-test-pin";

QJsonObject prompt(const QString &id)
{
    return {{"event", "hardware-key-prompt"}, {"id", id},
        {"title", "Approve machine pairing"}, {"details", "Code ABCD; fingerprint verified"},
        {"devices", QJsonArray{QJsonObject{{"id", "0"}, {"name", "Physical security key"}}}},
        {"pin_required", true}};
}

void writeFrame(QFile &stream, const QJsonObject &frame, bool newline = true)
{
    const auto bytes = QJsonDocument(frame).toJson(QJsonDocument::Compact);
    stream.write(bytes.left(31));
    stream.flush();
    QThread::msleep(10);
    stream.write(bytes.mid(31));
    if (newline)
        stream.write("\n");
    stream.flush();
}

int fixture(const QString &mode)
{
#ifdef Q_OS_WIN
    // Match Go's byte-preserving private pipes rather than CRT text-mode I/O.
    if (_setmode(_fileno(stdout), _O_BINARY) == -1 || _setmode(_fileno(stdin), _O_BINARY) == -1)
        return 13;
#endif
    QFile output;
    QFile input;
    if (!output.open(stdout, QIODevice::WriteOnly) || !input.open(stdin, QIODevice::ReadOnly))
        return 10;
    if (QCoreApplication::arguments().join(' ').contains(secret))
        return 11;
    auto frame = prompt(QString(32, 'a'));
    if (mode == "invalid-id")
        frame["id"] = QString(32, 'A');
    if (mode == "unknown")
        frame["event"] = "hardware-key-unknown";
    if (mode == "too-many-devices") {
        QJsonArray devices;
        for (int index = 0; index < 9; ++index)
            devices.append(QJsonObject{{"id", QString::number(index)}, {"name", "Key"}});
        frame["devices"] = devices;
    }
    if (mode == "oversized-frame")
        frame["details"] = QString(9000, 'x');
    if (mode == "malformed") {
        output.write("!");
        output.flush();
    }
    writeFrame(output, frame, mode != "unterminated");
    if (mode == "unterminated")
        return 0;
    if (mode == "duplicate")
        writeFrame(output, frame);
    if (mode == "output-limit") {
        output.write(QByteArray(20 * 1024 * 1024 + 1, 'x'));
        output.flush();
    }
    const int phases = mode == "two-phase" || mode == "reused-id" ? 2 : 1;
    for (int phase = 0; phase < phases; ++phase) {
        const auto response = QJsonDocument::fromJson(input.readLine(8193)).object();
        const QString expectedId(32, phase == 0 ? 'a' : 'b');
        if (response.value("id").toString() != expectedId
            || response.value("device").toString() != "0"
            || response.value("pin").toString() != secret
            || response.value("cancel").toBool(true))
            return 12;
        writeFrame(output, {{"event", "hardware-key-status"}, {"message", "Touch your physical security key."}});
        if (phase == 0 && phases > 1)
            writeFrame(output, prompt(QString(32, mode == "reused-id" ? 'a' : 'b')));
    }
    output.write(mode == "crlf-result" ? "{\"status\":\"complete\"}\r\n" : "{\"status\":\"complete\"}\n");
    output.flush();
    return 0;
}

bool transport(const QString &mode, bool gui, bool expectedSuccess, int expectedPrompts)
{
    RemoteCommand command;
    QString displayed;
    QByteArray streamed;
    QByteArray result;
    int prompts = 0;
    int statuses = 0;
    bool success = false;
    bool completed = false;
    QEventLoop loop;
    QObject::connect(&command, &RemoteCommand::output, &loop,
                     [&](const QString &text) { displayed += text; });
    QObject::connect(&command, &RemoteCommand::standardOutput, &loop,
                     [&](const QByteArray &data) { streamed += data; });
    QObject::connect(&command, &RemoteCommand::hardwareKeyStatus, &loop,
                     [&](const QString &) { ++statuses; });
    QObject::connect(&command, &RemoteCommand::hardwareKeyPrompt, &loop,
                     [&](const QJsonObject &frame) {
        ++prompts;
        const auto id = frame.value("id").toString();
        if (mode == "cancel") {
            command.cancel();
        } else if (mode != "duplicate" && mode != "output-limit" && mode != "unterminated") {
            // Neither stale consent nor an unknown device may write to stdin.
            command.submitHardwareKeyResponse("wrong-id", "0", "wrong-pin");
            command.submitHardwareKeyResponse(id, "unknown-device", "wrong-pin");
            command.submitHardwareKeyResponse(id, "0", secret);
            command.submitHardwareKeyResponse(id, "0", "duplicate-pin");
        }
    });
    QStringList arguments{"--fixture", mode};
    if (gui)
        arguments << "--interaction" << "gui";
    command.run(QCoreApplication::applicationFilePath(), arguments,
                [&](bool ok, const QByteArray &data) {
        success = ok;
        completed = true;
        result = data;
        loop.quit();
    }, 3000);
    QTimer::singleShot(6000, &loop, &QEventLoop::quit);
    loop.exec();
    const QByteArray expectedResult = mode == "crlf-result"
        ? "{\"status\":\"complete\"}\r\n" : "{\"status\":\"complete\"}\n";
    const bool passed = completed && success == expectedSuccess && prompts == expectedPrompts
        && !displayed.contains(secret) && !streamed.contains(secret.toUtf8())
        && !result.contains(secret.toUtf8()) && !displayed.contains("hardware-key-")
        && !streamed.contains("hardware-key-") && !result.contains("hardware-key-")
        && (!expectedSuccess || mode == "unterminated"
            || (streamed == expectedResult && statuses == expectedPrompts));
    if (!passed)
        std::fprintf(stderr,
            "Hardware-key transport case failed: %s; completed=%d success=%d prompts=%d/%d statuses=%d output-bytes=%lld crlf=%d\n",
            qPrintable(mode), completed, success, prompts, expectedPrompts, statuses,
            static_cast<long long>(streamed.size()), streamed.endsWith("\r\n"));
    return passed;
}
}

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    const auto arguments = application.arguments();
    if (arguments.value(1) == "--fixture")
        return fixture(arguments.value(2));
    const bool passed = transport("valid", true, true, 1)
        && transport("crlf-result", true, true, 1)
        && transport("two-phase", true, true, 2)
        && transport("unterminated", true, true, 1)
        && transport("invalid-id", true, false, 0)
        && transport("unknown", true, false, 0)
        && transport("malformed", true, false, 0)
        && transport("too-many-devices", true, false, 0)
        && transport("oversized-frame", true, false, 0)
        && transport("cancel", true, false, 1)
        && transport("duplicate", true, false, 1)
        && transport("reused-id", true, false, 1)
        && transport("output-limit", true, false, 1)
        && transport("unsolicited", false, false, 0);
    return passed ? 0 : 1;
}
