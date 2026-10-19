#include "TerminalHostLaunch.h"

#include <QDir>
#include <QFileInfo>

namespace {
// Paths/argv are separate process arguments, never inserted into this script.
// The supervisor remains the session leader, preventing session-ID reuse while
// it verifies and cleans children in the shell's distinct job-control groups.
const char Supervisor[] = R"DOCKPIPE(
for tool in ps head timeout sleep env; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        printf 'Dockpipe: host terminal requires %s.\n' "$tool" >&2
        exit 125
    fi
done
if ! env --default-signal=INT,QUIT /bin/sh -c :; then
    printf 'Dockpipe: host terminal requires env --default-signal support.\n' >&2
    exit 125
fi
session=$$
stopping=0
trap 'stopping=1' HUP INT TERM

# Linux comm is at most 15 bytes and may contain newlines. Read the complete
# stat record: at most 16 valid lines plus one overflow probe, 4 KiB total.
# The caller scopes LC_ALL=C to this function, preserving the shell's locale.
read_member_stat()
{
    member_process=$1
    member_session=
    information=
    lines=0
    if ! {
        while IFS= read -r line; do
            lines=$((lines + 1))
            if [ "$lines" -gt 16 ] || [ "${#line}" -gt 4096 ]; then
                return 1
            fi
            if [ $((${#information} + ${#line} + 1)) -gt 4096 ]; then
                return 1
            fi
            information="$information$line "
        done
    } 2>/dev/null < "/proc/$member_process/stat"; then
        if [ -e "/proc/$member_process/stat" ]; then
            return 1
        fi
        return 2
    fi
    if [ ! -e "/proc/$member_process/stat" ]; then
        return 2
    fi
    case "$information" in
        "$member_process ("*") "*) ;;
        *) return 1 ;;
    esac
    fields=${information##*) }
    IFS=' ' read -r member_state member_parent member_group member_session remaining <<DOCKPIPE_STAT_FIELDS
$fields
DOCKPIPE_STAT_FIELDS
    if [ "${#member_state}" -ne 1 ]; then
        return 1
    fi
    for number in "$member_parent" "$member_group" "$member_session"; do
        case "$number" in ''|*[!0-9]*) return 1 ;; esac
    done
}

signal_members()
{
    signal=$1
    members=$(timeout --signal=KILL 1 /bin/sh -c '
        ps -s "$1" -o pid= || exit 1
        printf "DOCKPIPE_PS_DONE\n"
    ' dockpipe-terminal-cleanup "$session" | head -n 1026)
    count=0
    complete=0
    supervisor_present=0
    for process in $members; do
        if [ "$process" = DOCKPIPE_PS_DONE ]; then
            complete=1
            break
        fi
        case "$process" in ''|*[!0-9]*) return 1 ;; esac
        count=$((count + 1))
        if [ "$count" -gt 1024 ]; then
            printf 'Dockpipe: host terminal cleanup exceeded 1024 session members.\n' >&2
            return 1
        fi
        if [ "$process" = "$session" ]; then
            supervisor_present=1
            continue
        fi
        if [ "$process" -le 1 ]; then
            continue
        fi
        LC_ALL=C read_member_stat "$process"
        result=$?
        if [ "$result" -eq 2 ]; then
            continue
        fi
        if [ "$result" -ne 0 ]; then
            return 1
        fi
        if [ "$member_session" = "$session" ]; then
            kill "-$signal" "$process" 2>/dev/null || :
        fi
    done
    [ "$complete" -eq 1 ] && [ "$supervisor_present" -eq 1 ]
}

# Save stdin before the asynchronous command resets fd0 to /dev/null. A
# self-duplication (<&0) would duplicate that already-reset descriptor instead.
# POSIX asynchronous commands also inherit ignored INT/QUIT. Reset those with
# GNU env before the interactive shell establishes foreground job control.
exec 3<&0
env --default-signal=INT,QUIT "$@" <&3 3<&- >&1 2>&2 &
shell=$!
exec 3<&-
status=0
if [ "$stopping" -eq 0 ]; then
    wait "$shell" || status=$?
fi
trap '' HUP INT TERM
cleanup_failed=0
signal_members HUP || cleanup_failed=1
signal_members CONT || cleanup_failed=1
sleep 0.2
signal_members KILL || cleanup_failed=1
if [ "$cleanup_failed" -ne 0 ]; then
    printf 'Dockpipe: host terminal could not verify complete session cleanup.\n' >&2
    exit 125
fi
exit "$status"
)DOCKPIPE";

bool validArgument(const QString &argument)
{
    return !argument.contains(QChar::Null);
}
}

bool TerminalHostLaunch::required()
{
#ifdef Q_OS_LINUX
    return QFileInfo::exists(QStringLiteral("/.flatpak-info")) || !qEnvironmentVariable("FLATPAK_ID").isEmpty();
#else
    return false;
#endif
}

TerminalHostLaunch::Plan TerminalHostLaunch::prepare(const QString &shell,
    const QStringList &arguments, const QString &workingDirectory, const QMap<QString, QString> &environment)
{
    if (required())
        return forHost(shell, arguments, workingDirectory, QStringLiteral("/usr/bin/flatpak-spawn"), environment);
    return {shell, arguments, workingDirectory, {}, false};
}

TerminalHostLaunch::Plan TerminalHostLaunch::forHost(const QString &shell,
    const QStringList &arguments, const QString &workingDirectory, const QString &bridge,
    const QMap<QString, QString> &environment)
{
    Plan plan;
    plan.host = true;
    // Linux execve accepts at most 128 KiB per argument. Keep the entire launch
    // below that boundary, including this fixed script and the bridge options.
    constexpr qsizetype maximumBytes = 64 * 1024;
    bool valid = QDir::isAbsolutePath(shell) && QDir::isAbsolutePath(workingDirectory)
        && validArgument(shell) && validArgument(workingDirectory) && arguments.size() <= 64 && environment.size() <= 64
        && shell.size() <= maximumBytes && workingDirectory.size() <= maximumBytes;
    qsizetype bytes = sizeof(Supervisor);
    if (valid)
        bytes += shell.toUtf8().size() + workingDirectory.toUtf8().size();
    if (valid && bytes <= maximumBytes) {
        for (const auto &argument : arguments) {
            if (argument.size() > maximumBytes || !validArgument(argument)) {
                valid = false;
                break;
            }
            bytes += argument.toUtf8().size() + 1;
            if (bytes > maximumBytes)
                break;
        }
    }
    if (valid && bytes <= maximumBytes) {
        for (auto entry = environment.cbegin(); entry != environment.cend(); ++entry) {
            if (entry.key().isEmpty() || entry.key().contains('=') || !validArgument(entry.key())
                || !validArgument(entry.value()) || entry.key().size() > maximumBytes || entry.value().size() > maximumBytes) {
                valid = false;
                break;
            }
            bytes += entry.key().toUtf8().size() + entry.value().toUtf8().size() + 8;
            if (bytes > maximumBytes)
                break;
        }
    }
    if (!valid || bytes > maximumBytes) {
        plan.error = QStringLiteral("The host terminal launch requires absolute paths and at most 64 KiB of valid arguments.");
        return plan;
    }
    const QFileInfo executable(bridge);
    if (!executable.isAbsolute() || !executable.isFile() || !executable.isExecutable()) {
        plan.error = QStringLiteral("The Flatpak host terminal bridge is unavailable.");
        return plan;
    }
    plan.program = bridge;
    plan.workingDirectory = QDir::homePath();
    plan.arguments = {QStringLiteral("--host"), QStringLiteral("--watch-bus"),
        QStringLiteral("--directory=") + workingDirectory,
        QStringLiteral("--env=TERM=xterm-256color"), QStringLiteral("--env=COLORTERM=truecolor")};
    for (auto entry = environment.cbegin(); entry != environment.cend(); ++entry)
        plan.arguments.append(QStringLiteral("--env=") + entry.key() + '=' + entry.value());
    plan.arguments.append({QStringLiteral("--"), QStringLiteral("/bin/sh"), QStringLiteral("-c"),
        QString::fromLatin1(Supervisor), QStringLiteral("dockpipe-terminal"), shell});
    plan.arguments.append(arguments);
    return plan;
}
