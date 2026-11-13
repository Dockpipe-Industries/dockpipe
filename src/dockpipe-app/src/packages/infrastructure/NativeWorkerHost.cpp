#include "NativeWorkerHost.h"
#include "DockpipeNativeWorkerPackage.h"

#include <QCryptographicHash>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QJsonDocument>
#include <QJsonArray>
#include <QSet>
#include <vector>
#include <QJsonObject>
#include <QRegularExpression>
#include <cstdio>
#include <cstring>

#if defined(Q_OS_UNIX)
#include <fcntl.h>
#include <sys/stat.h>
#include <unistd.h>
#endif

namespace {
constexpr qint64 MaximumDescriptorBytes = 32768;
constexpr qint64 MaximumImageBytes = 128 * 1024 * 1024;

bool exactFields(const QJsonObject &object, const QStringList &fields)
{
    if (object.size() != fields.size())
        return false;
    for (const auto &field : fields) {
        if (!object.contains(field))
            return false;
    }
    return true;
}

bool digest(const QByteArray &value)
{
    static const QRegularExpression pattern("^[0-9a-f]{64}$");
    return pattern.match(QString::fromLatin1(value)).hasMatch();
}

bool canonicalPath(const QString &path)
{
    return !path.isEmpty() && !path.contains(QChar(0)) && path.toUtf8().size() <= 4096 && QFileInfo(path).isAbsolute()
        && QDir::cleanPath(path) == path && QFileInfo(path).canonicalFilePath() == path;
}

// Qt validates JSON first. This bounded lexical pass only rejects duplicate
// object keys (including escaped aliases), which Qt otherwise silently replaces.
// Each token is visited once; key decoding stays with Qt's JSON implementation.
bool uniqueObjectKeys(const QByteArray &json)
{
    struct Scope {
        bool object = false;
        QSet<QString> keys;
    };
    std::vector<Scope> scopes;
    for (qsizetype index = 0; index < json.size(); ++index) {
        const char value = json[index];
        if (value == '{' || value == '[') {
            scopes.push_back({value == '{', {}});
        } else if (value == '}' || value == ']') {
            if (scopes.empty())
                return false;
            scopes.pop_back();
        } else if (value == '"') {
            const auto start = index;
            for (++index; index < json.size(); ++index) {
                if (json[index] == '\\')
                    ++index;
                else if (json[index] == '"')
                    break;
            }
            auto next = index + 1;
            while (next < json.size() && (json[next] == ' ' || json[next] == '\t' || json[next] == '\r' || json[next] == '\n'))
                ++next;
            if (next < json.size() && json[next] == ':') {
                if (scopes.empty() || !scopes.back().object)
                    return false;
                const auto token = json.mid(start, index - start + 1);
                const auto key = QJsonDocument::fromJson(QByteArrayLiteral("[") + token + ']').array()[0].toString();
                if (scopes.back().keys.contains(key))
                    return false;
                scopes.back().keys.insert(key);
            }
        }
    }
    return scopes.empty();
}

#if defined(Q_OS_UNIX)
std::optional<QByteArray> readOwnedFile(const QString &path, qint64 maximum, bool privateFile, bool executable)
{
    if (!canonicalPath(path))
        return std::nullopt;
    const auto encoded = QFile::encodeName(path);
    const int descriptor = ::open(encoded.constData(), O_RDONLY | O_NOFOLLOW | O_CLOEXEC);
    if (descriptor < 0)
        return std::nullopt;
    struct stat info {};
    const bool admitted = ::fstat(descriptor, &info) == 0 && S_ISREG(info.st_mode)
        && (info.st_uid == ::geteuid() || (!privateFile && info.st_uid == 0))
        && (privateFile ? (info.st_mode & 0777) == 0600 : (info.st_mode & 0022) == 0)
        && (!executable || (info.st_mode & 0111) != 0) && info.st_size > 0 && info.st_size <= maximum;
    QFile file;
    if (!admitted || !file.open(descriptor, QIODevice::ReadOnly, QFileDevice::AutoCloseHandle)) {
        ::close(descriptor);
        return std::nullopt;
    }
    const auto data = file.read(maximum + 1);
    struct stat after {};
    if (file.error() != QFileDevice::NoError || data.size() != info.st_size || data.size() > maximum
        || ::fstat(descriptor, &after) != 0 || after.st_size != info.st_size || after.st_mtime != info.st_mtime)
        return std::nullopt;
    return data;
}

bool privateStateDirectory(const QString &path)
{
    if (!canonicalPath(path))
        return false;
    struct stat info {};
    const auto encoded = QFile::encodeName(path);
    return ::lstat(encoded.constData(), &info) == 0 && S_ISDIR(info.st_mode)
        && info.st_uid == ::geteuid() && (info.st_mode & 0777) == 0700;
}
#endif
} // namespace

NativeWorkerHostArguments parseNativeWorkerHostArguments(int argc, char *argv[])
{
    NativeWorkerHostArguments result;
    for (int index = 1; index < argc; ++index) {
        if (std::strcmp(argv[index], "--native-worker-host") == 0)
            result.requested = true;
    }
    if (!result.requested)
        return result;
    if (argc != 4 || std::strcmp(argv[1], "--native-worker-host") != 0 || std::strcmp(argv[2], "--request") != 0)
        return result;
    result.descriptorPath = QString::fromUtf8(argv[3]);
    result.valid = QFileInfo(result.descriptorPath).isAbsolute();
    return result;
}

bool verifyNativeWorkerHostFile(const QString &path, const QByteArray &sha256, bool executable, QString &error)
{
#if defined(Q_OS_UNIX)
    const auto content = readOwnedFile(path, MaximumImageBytes, false, executable);
    if (digest(sha256) && content && QCryptographicHash::hash(*content, QCryptographicHash::Sha256).toHex() == sha256)
        return true;
#else
    Q_UNUSED(path);
    Q_UNUSED(sha256);
    Q_UNUSED(executable);
#endif
    error = QStringLiteral("Native worker image ownership, path or pinned hash is invalid.");
    return false;
}

std::optional<NativeWorkerHostDescriptor> readNativeWorkerHostDescriptor(const QString &path, QString &error)
{
    error = QStringLiteral("Native worker host requires an exact owned private descriptor.");
#if defined(Q_OS_UNIX)
    const auto content = readOwnedFile(path, MaximumDescriptorBytes, true, false);
    if (!content)
        return std::nullopt;
    QJsonParseError parseError {};
    const auto document = QJsonDocument::fromJson(*content, &parseError);
    if (parseError.error != QJsonParseError::NoError || !document.isObject() || !uniqueObjectKeys(*content))
        return std::nullopt;
    const auto object = document.object();
    const auto module = object.value("module").toObject();
    const auto cli = object.value("cli").toObject();
    const auto trust = object.value("trust").toObject();
    if (!exactFields(object, {"schema", "module", "cli", "state_directory", "profile", "trust"})
        || object.value("schema") != "dockpipe.native-worker-host/v1"
        || !exactFields(module, {"path", "sha256"}) || !exactFields(cli, {"path", "sha256"})
        || !object.value("profile").isObject())
        return std::nullopt;
    NativeWorkerHostDescriptor result;
    result.modulePath = module.value("path").toString();
    result.moduleSha256 = module.value("sha256").toString().toLatin1();
    result.cliPath = cli.value("path").toString();
    result.cliSha256 = cli.value("sha256").toString().toLatin1();
    result.stateDirectory = object.value("state_directory").toString();
    result.profileJson = QJsonDocument(object.value("profile").toObject()).toJson(QJsonDocument::Compact);
    if (!privateStateDirectory(result.stateDirectory) || result.profileJson.size() > DOCKPIPE_NATIVE_WORKER_PROFILE_LIMIT_V1)
        return std::nullopt;
    if (trust.value("mode") == "production") {
        static const QRegularExpression team("^[A-Z0-9]{10}$");
        if (!exactFields(trust, {"mode", "team_identifier"}) || !team.match(trust.value("team_identifier").toString()).hasMatch())
            return std::nullopt;
        result.teamIdentifier = trust.value("team_identifier").toString();
    } else if (trust.value("mode") == "review-adhoc") {
        result.hostSha256 = trust.value("host_sha256").toString().toLatin1();
        if (!exactFields(trust, {"mode", "host_sha256"}) || !digest(result.hostSha256))
            return std::nullopt;
        result.trust = NativeWorkerHostDescriptor::Trust::ReviewAdhoc;
    } else {
        return std::nullopt;
    }
    if (!verifyNativeWorkerHostFile(result.modulePath, result.moduleSha256, false, error)
        || !verifyNativeWorkerHostFile(result.cliPath, result.cliSha256, true, error))
        return std::nullopt;
    error.clear();
    return result;
#else
    Q_UNUSED(path);
    return std::nullopt;
#endif
}

#ifdef Q_OS_MACOS
int runNativeWorkerHostMac(const NativeWorkerHostDescriptor &descriptor);
#endif

std::optional<int> dispatchNativeWorkerHost(int argc, char *argv[])
{
    const auto arguments = parseNativeWorkerHostArguments(argc, argv);
    if (!arguments.requested)
        return std::nullopt;
    if (!arguments.valid) {
        std::fputs("Native worker host requires --native-worker-host --request <absolute-private-file>.\n", stderr);
        return 2;
    }
#ifdef Q_OS_MACOS
    QString error;
    const auto descriptor = readNativeWorkerHostDescriptor(arguments.descriptorPath, error);
    if (!descriptor) {
        std::fputs("Native worker host descriptor admission failed.\n", stderr);
        return 2;
    }
    return runNativeWorkerHostMac(*descriptor);
#else
    std::fputs("Native worker host is supported only by the macOS Dockpipe app.\n", stderr);
    return 2;
#endif
}

bool nativeWorkerHostResultIsClean(const DockpipeNativeWorkerResultV1 &result)
{
    return result.struct_size == sizeof(result) && result.outcome <= DOCKPIPE_NATIVE_WORKER_FAILURE_V1
        && result.cleanup_verified == 1;
}

int nativeWorkerHostResultExitCode(const DockpipeNativeWorkerResultV1 &result)
{
    if (!nativeWorkerHostResultIsClean(result))
        return 1;
    if (result.outcome == DOCKPIPE_NATIVE_WORKER_SUCCESS_V1)
        return 0;
    return result.outcome == DOCKPIPE_NATIVE_WORKER_CANCELLED_V1 ? 130 : 1;
}

bool nativeWorkerHostSignatureAllowed(const NativeWorkerHostDescriptor &descriptor,
    const NativeWorkerHostSignature &signature, bool host)
{
    if (!signature.valid || signature.libraryValidationDisabled
        || (host && signature.identifier != QStringLiteral("com.dockpipe.launcher")))
        return false;
    if (descriptor.trust == NativeWorkerHostDescriptor::Trust::ReviewAdhoc)
        return signature.adhoc;
    return !signature.adhoc && signature.developerId && !descriptor.teamIdentifier.isEmpty()
        && signature.teamIdentifier == descriptor.teamIdentifier && (!host || signature.hardenedRuntime);
}
