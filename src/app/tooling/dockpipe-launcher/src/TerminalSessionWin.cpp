#include "TerminalSessionWin.h"

#ifdef Q_OS_WIN
#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <windows.h>
#include <QProcessEnvironment>
#include <QScopeGuard>
#include <algorithm>
#include <atomic>
#include <condition_variable>
#include <exception>
#include <mutex>
#include <thread>
#include <vector>

namespace {
constexpr qsizetype QueueLimit = 64 * 1024;
constexpr DWORD PipeChunk = 8192;
// TerminalHub admits eight visible sessions. Another eight transports may finish
// asynchronous teardown, without rapid restart creating an unlimited backlog.
constexpr unsigned MaximumTransports = 16;
std::atomic<unsigned> activeTransports{0};

struct ConsoleApi {
    using Create = HRESULT (WINAPI *)(COORD, HANDLE, HANDLE, DWORD, HANDLE *);
    using Resize = HRESULT (WINAPI *)(HANDLE, COORD);
    using Close = void (WINAPI *)(HANDLE);
    Create create = nullptr;
    Resize resize = nullptr;
    Close close = nullptr;

    ConsoleApi()
    {
        const HMODULE kernel = GetModuleHandleW(L"kernel32.dll");
        if (!kernel)
            return;
        create = reinterpret_cast<Create>(GetProcAddress(kernel, "CreatePseudoConsole"));
        resize = reinterpret_cast<Resize>(GetProcAddress(kernel, "ResizePseudoConsole"));
        close = reinterpret_cast<Close>(GetProcAddress(kernel, "ClosePseudoConsole"));
    }
};

const ConsoleApi &consoleApi()
{
    static const ConsoleApi api;
    return api;
}

class NativeHandle final {
public:
    HANDLE value = nullptr;
    ~NativeHandle() { reset(); }
    NativeHandle() = default;
    NativeHandle(const NativeHandle &) = delete;
    NativeHandle &operator=(const NativeHandle &) = delete;
    void reset()
    {
        if (value && value != INVALID_HANDLE_VALUE)
            CloseHandle(value);
        value = nullptr;
    }
};

QString windowsFailure(const QString &operation, DWORD code)
{
    return QStringLiteral("%1 (Windows error %2).").arg(operation).arg(code);
}

// CreateProcess receives one mutable command line. Quote each structured argv
// element using the CRT backslash/quote convention; never pass through cmd.exe.
QString quoteArgument(const QString &argument)
{
    QString quoted = QStringLiteral("\"");
    qsizetype backslashes = 0;
    for (const QChar character : argument) {
        if (character == QLatin1Char('\\')) {
            ++backslashes;
        } else {
            quoted += QString(backslashes * (character == QLatin1Char('"') ? 2 : 1), QLatin1Char('\\'));
            if (character == QLatin1Char('"'))
                quoted += QLatin1Char('\\');
            quoted += character;
            backslashes = 0;
        }
    }
    quoted += QString(backslashes * 2, QLatin1Char('\\'));
    quoted += QLatin1Char('"');
    return quoted;
}
}

struct TerminalSessionWin::State {
    std::mutex mutex;
    std::condition_variable changed;
    QByteArray input;
    QByteArray output;
    QString error;
    std::atomic<bool> stopping{false};
    std::atomic<bool> discardOutput{false};
    bool completed = false;
    int exitCode = -1;
    int columns = 80;
    int rows = 24;
    NativeHandle stopEvent;
    NativeHandle wakeEvent;

    void fail(const QString &message)
    {
        std::lock_guard<std::mutex> lock(mutex);
        if (error.isEmpty())
            error = message;
    }

    void requestStop(bool discard = true)
    {
        if (discard)
            discardOutput.store(true);
        stopping.store(true);
        SetEvent(stopEvent.value);
        changed.notify_all();
    }

    void readPipe(HANDLE pipe)
    {
        char buffer[PipeChunk];
        while (true) {
            DWORD count = 0;
            const BOOL read = ReadFile(pipe, buffer, PipeChunk, &count, nullptr);
            if (!read || count == 0) {
                const DWORD failure = read ? ERROR_SUCCESS : GetLastError();
                if (failure != ERROR_SUCCESS && failure != ERROR_BROKEN_PIPE && failure != ERROR_OPERATION_ABORTED && !stopping.load()) {
                    fail(windowsFailure(QStringLiteral("Terminal output could not be read"), failure));
                    requestStop();
                }
                return;
            }
            qsizetype consumed = 0;
            while (consumed < qsizetype(count)) {
                std::unique_lock<std::mutex> lock(mutex);
                changed.wait(lock, [&] { return discardOutput.load() || output.size() < QueueLimit; });
                if (discardOutput.load())
                    break; // Teardown must keep draining ConPTY's final frame.
                const qsizetype length = qMin(qsizetype(count) - consumed, QueueLimit - output.size());
                output.append(buffer + consumed, length);
                consumed += length;
            }
        }
    }

    void writePipe(HANDLE pipe)
    {
        while (true) {
            QByteArray bytes;
            {
                std::unique_lock<std::mutex> lock(mutex);
                changed.wait(lock, [&] { return stopping.load() || !input.isEmpty(); });
                if (stopping.load())
                    return;
                bytes = input.left(PipeChunk);
            }
            DWORD written = 0;
            if (!WriteFile(pipe, bytes.constData(), DWORD(bytes.size()), &written, nullptr) || written == 0) {
                if (!stopping.load()) {
                    fail(windowsFailure(QStringLiteral("Terminal input could not be written"), GetLastError()));
                    requestStop();
                }
                return;
            }
            std::lock_guard<std::mutex> lock(mutex);
            // Keep in-flight input in the queue until acknowledged. Appends
            // cannot circumvent the aggregate pending-input limit.
            input.remove(0, qsizetype(written));
        }
    }

    void run(const QString &directory, const QString &program, const QString &command,
        const QMap<QString, QString> &environmentOverrides)
    {
        NativeHandle inputRead;
        NativeHandle inputWrite;
        NativeHandle outputRead;
        NativeHandle outputWrite;
        NativeHandle process;
        NativeHandle processThread;
        NativeHandle job;
        HANDLE console = nullptr;
        std::thread reader;
        std::thread writer;
        bool childResumed = false;
        const auto cleanup = [&] {
            // Kill the complete job even when the shell exited normally, so
            // detached descendants cannot retain terminal pipes indefinitely.
            if (job.value && !TerminateJobObject(job.value, 1))
                fail(windowsFailure(QStringLiteral("Terminal process cleanup failed"), GetLastError()));
            if (process.value && !childResumed)
                TerminateProcess(process.value, 1);
            requestStop(false);
            if (writer.joinable())
                CancelSynchronousIo(reinterpret_cast<HANDLE>(writer.native_handle()));
            // ConPTY duplicates these endpoints; it never takes ownership.
            // Release them on every launch failure as well as successful starts.
            inputRead.reset();
            outputWrite.reset();
            if (writer.joinable())
                writer.join();
            // Input EOF must precede the output-reader join. Otherwise modern
            // ClosePseudoConsole may return while its host still waits for input,
            // leaving the reader waiting forever for ConPTY's output EOF.
            inputWrite.reset();
            // Older Windows can wait here for the final frame to be drained.
            // Keep the reader alive until the pseudoconsole closes its output.
            if (console)
                consoleApi().close(console);
            console = nullptr;
            if (reader.joinable())
                reader.join();
            outputRead.reset();
            std::lock_guard<std::mutex> lock(mutex);
            completed = true;
        };
        const auto launch = [&]() -> bool {
            if (stopping.load())
                return false;
            if (!CreatePipe(&inputRead.value, &inputWrite.value, nullptr, PipeChunk)
                || !CreatePipe(&outputRead.value, &outputWrite.value, nullptr, PipeChunk)) {
                fail(windowsFailure(QStringLiteral("Terminal pipes could not be created"), GetLastError()));
                return false;
            }
            // Start draining before ConPTY creation, including setup failures.
            reader = std::thread([&] { readPipe(outputRead.value); });
            COORD dimensions;
            {
                std::lock_guard<std::mutex> lock(mutex);
                dimensions = {SHORT(columns), SHORT(rows)};
            }
            const HRESULT created = consoleApi().create(dimensions, inputRead.value, outputWrite.value, 0, &console);
            if (FAILED(created)) {
                fail(windowsFailure(QStringLiteral("The Windows pseudoconsole could not be created"), DWORD(created)));
                return false;
            }
            SIZE_T attributeBytes = 0;
            InitializeProcThreadAttributeList(nullptr, 1, 0, &attributeBytes);
            // A single OS-defined startup attribute never needs a large list.
            if (attributeBytes == 0 || attributeBytes > 64 * 1024) {
                fail(QStringLiteral("The Windows terminal startup attributes exceeded 64 KiB."));
                return false;
            }
            std::vector<unsigned char> attributes(attributeBytes);
            auto *list = reinterpret_cast<LPPROC_THREAD_ATTRIBUTE_LIST>(attributes.data());
            if (!InitializeProcThreadAttributeList(list, 1, 0, &attributeBytes)) {
                fail(windowsFailure(QStringLiteral("Terminal startup attributes could not be initialized"), GetLastError()));
                return false;
            }
            const auto releaseAttributes = qScopeGuard([list] { DeleteProcThreadAttributeList(list); });
            STARTUPINFOEXW startup{};
            startup.StartupInfo.cb = sizeof(startup);
            // Suppress Windows' implicit duplication of redirected parent stdio,
            // even with bInheritHandles=false. Null standard handles are then
            // initialized from the pseudoconsole instead of the launcher's pipes.
            // https://github.com/microsoft/terminal/discussions/15814
            startup.StartupInfo.dwFlags = STARTF_USESTDHANDLES;
            startup.lpAttributeList = list;
            // PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE is the console handle value,
            // rather than the address of that value (Microsoft ConPTY contract).
            constexpr DWORD_PTR pseudoConsoleAttribute = 0x00020016;
            const BOOL configured = UpdateProcThreadAttribute(list, 0, pseudoConsoleAttribute, console, sizeof(console), nullptr, nullptr);
            if (!configured) {
                const DWORD failure = GetLastError();
                fail(windowsFailure(QStringLiteral("The pseudoconsole startup attribute could not be set"), failure));
                return false;
            }
            job.value = CreateJobObjectW(nullptr, nullptr);
            JOBOBJECT_EXTENDED_LIMIT_INFORMATION limits{};
            limits.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
            if (!job.value || !SetInformationJobObject(job.value, JobObjectExtendedLimitInformation, &limits, sizeof(limits))) {
                const DWORD failure = GetLastError();
                fail(windowsFailure(QStringLiteral("Terminal process containment could not be created"), failure));
                return false;
            }
            auto environment = QProcessEnvironment::systemEnvironment();
            environment.insert(QStringLiteral("TERM"), QStringLiteral("xterm-256color"));
            environment.insert(QStringLiteral("COLORTERM"), QStringLiteral("truecolor"));
            for (auto entry = environmentOverrides.cbegin(); entry != environmentOverrides.cend(); ++entry)
                environment.insert(entry.key(), entry.value());
            QStringList entries = environment.toStringList();
            std::sort(entries.begin(), entries.end(), [](const QString &left, const QString &right) {
                return QString::compare(left, right, Qt::CaseInsensitive) < 0;
            });
            // Bound the inherited+profile environment to 1 MiB of UTF-16. One
            // variable additionally respects the Win32 32767-character limit.
            constexpr qsizetype maximumEnvironmentCharacters = 512 * 1024;
            qsizetype environmentCharacters = 1;
            for (const QString &entry : entries) {
                if (entry.size() > 32767 || entry.size() + 1 > maximumEnvironmentCharacters - environmentCharacters) {
                    fail(QStringLiteral("The terminal environment exceeds its 1 MiB limit or a variable exceeds 32767 characters."));
                    return false;
                }
                environmentCharacters += entry.size() + 1;
            }
            std::wstring environmentBlock;
            environmentBlock.reserve(size_t(environmentCharacters));
            for (const QString &entry : entries) {
                environmentBlock.append(reinterpret_cast<const wchar_t *>(entry.utf16()), size_t(entry.size()));
                environmentBlock.push_back(L'\0');
            }
            environmentBlock.push_back(L'\0');
            std::wstring mutableCommand = command.toStdWString();
            PROCESS_INFORMATION child{};
            const BOOL launched = CreateProcessW(reinterpret_cast<LPCWSTR>(program.utf16()), mutableCommand.data(), nullptr, nullptr,
                FALSE, EXTENDED_STARTUPINFO_PRESENT | CREATE_SUSPENDED | CREATE_UNICODE_ENVIRONMENT, environmentBlock.data(),
                reinterpret_cast<LPCWSTR>(directory.utf16()), &startup.StartupInfo, &child);
            const DWORD failure = GetLastError();
            if (!launched) {
                fail(windowsFailure(QStringLiteral("The terminal shell could not be started"), failure));
                return false;
            }
            process.value = child.hProcess;
            processThread.value = child.hThread;
            // Nothing executes before job assignment; failure never leaves an
            // uncontained shell or any descendants behind.
            if (!AssignProcessToJobObject(job.value, process.value)) {
                fail(windowsFailure(QStringLiteral("The terminal shell could not be contained"), GetLastError()));
                return false;
            }
            inputRead.reset();
            outputWrite.reset();
            writer = std::thread([&] { writePipe(inputWrite.value); });
            if (stopping.load())
                return false;
            if (ResumeThread(processThread.value) == DWORD(-1)) {
                fail(windowsFailure(QStringLiteral("The terminal shell could not be resumed"), GetLastError()));
                return false;
            }
            childResumed = true;
            processThread.reset();
            return true;
        };
        try {
            if (launch()) {
                const HANDLE events[] = {stopEvent.value, process.value, wakeEvent.value};
                while (true) {
                    const DWORD event = WaitForMultipleObjects(3, events, FALSE, INFINITE);
                    if (event == WAIT_OBJECT_0 || event == WAIT_OBJECT_0 + 1) {
                        DWORD code = DWORD(-1);
                        if (event == WAIT_OBJECT_0 + 1 && GetExitCodeProcess(process.value, &code)) {
                            std::lock_guard<std::mutex> lock(mutex);
                            exitCode = int(code);
                        }
                        break;
                    }
                    if (event != WAIT_OBJECT_0 + 2) {
                        fail(windowsFailure(QStringLiteral("Terminal process monitoring failed"), GetLastError()));
                        break;
                    }
                    COORD dimensions;
                    {
                        std::lock_guard<std::mutex> lock(mutex);
                        dimensions = {SHORT(columns), SHORT(rows)};
                    }
                    const HRESULT resized = consoleApi().resize(console, dimensions);
                    if (FAILED(resized))
                        fail(windowsFailure(QStringLiteral("The terminal could not be resized"), DWORD(resized)));
                }
            }
        } catch (const std::exception &) {
            fail(QStringLiteral("The Windows terminal worker could not allocate its resources."));
        }
        cleanup();
    }
};

TerminalSessionWin::TerminalSessionWin() = default;
TerminalSessionWin::~TerminalSessionWin() { stop(); }

bool TerminalSessionWin::start(const QString &directory, const QString &program, const QStringList &arguments,
    int columns, int rows, const QMap<QString, QString> &environment, QString &error)
{
    const auto &api = consoleApi();
    if (!api.create || !api.resize || !api.close) {
        error = QStringLiteral("Interactive terminals require Windows 10 version 1809 or newer (ConPTY).");
        return false;
    }
    QString command = quoteArgument(program);
    for (const QString &argument : arguments) {
        // CreateProcessW limits the complete command line to 32767 characters,
        // including its terminator. Bound before expanding quoted arguments.
        if (argument.contains(QChar::Null) || argument.size() > 32766 || command.size() + 1 + argument.size() > 32766) {
            error = QStringLiteral("The terminal shell command exceeds the Windows command-line limit or contains NUL.");
            return false;
        }
        const QString quoted = quoteArgument(argument);
        if (command.size() + 1 + quoted.size() > 32766) {
            error = QStringLiteral("The terminal shell command exceeds the Windows command-line limit.");
            return false;
        }
        command += QLatin1Char(' ') + quoted;
    }
    if (command.size() > 32766) {
        error = QStringLiteral("The terminal shell command exceeds the Windows command-line limit.");
        return false;
    }
    unsigned count = activeTransports.load();
    do {
        if (count >= MaximumTransports) {
            error = QStringLiteral("Windows terminals are still closing. Wait before opening another terminal.");
            return false;
        }
    } while (!activeTransports.compare_exchange_weak(count, count + 1));
    try {
        auto state = std::make_shared<State>();
        state->columns = columns;
        state->rows = rows;
        state->stopEvent.value = CreateEventW(nullptr, TRUE, FALSE, nullptr);
        state->wakeEvent.value = CreateEventW(nullptr, FALSE, FALSE, nullptr);
        if (!state->stopEvent.value || !state->wakeEvent.value) {
            --activeTransports;
            error = windowsFailure(QStringLiteral("Terminal worker events could not be created"), GetLastError());
            return false;
        }
        std::thread worker([state, directory, program, command, environment] {
            state->run(directory, program, command, environment);
            --activeTransports;
        });
        worker.detach();
        m_state = std::move(state);
        return true;
    } catch (const std::exception &) {
        --activeTransports;
        error = QStringLiteral("The Windows terminal worker could not be started.");
        return false;
    }
}

bool TerminalSessionWin::send(const QByteArray &bytes)
{
    if (!m_state)
        return false;
    std::lock_guard<std::mutex> lock(m_state->mutex);
    if (m_state->stopping.load() || m_state->completed || bytes.size() > QueueLimit - m_state->input.size())
        return false;
    m_state->input.append(bytes);
    m_state->changed.notify_all();
    return true;
}

void TerminalSessionWin::resize(int columns, int rows)
{
    if (!m_state || m_state->stopping.load())
        return;
    std::lock_guard<std::mutex> lock(m_state->mutex);
    m_state->columns = columns;
    m_state->rows = rows;
    SetEvent(m_state->wakeEvent.value);
}

void TerminalSessionWin::stop()
{
    if (m_state)
        m_state->requestStop();
}

TerminalSessionWin::Update TerminalSessionWin::takeUpdate()
{
    Update update;
    if (!m_state)
        return update;
    {
        std::lock_guard<std::mutex> lock(m_state->mutex);
        update.output.swap(m_state->output);
        update.error = m_state->error;
        update.completed = m_state->completed;
        update.exitCode = m_state->exitCode;
    }
    m_state->changed.notify_all();
    return update;
}

bool TerminalSessionWin::running() const
{
    if (!m_state || m_state->stopping.load())
        return false;
    std::lock_guard<std::mutex> lock(m_state->mutex);
    return !m_state->completed;
}
#endif
