#include "NativeWorkerHost.h"
#include "DockpipeNativeWorkerPackage.h"

#import <Foundation/Foundation.h>
#import <Security/Security.h>
#include <QCryptographicHash>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QTemporaryDir>
#include <atomic>
#include <cerrno>
#include <sys/xattr.h>
#include <chrono>
#include <condition_variable>
#include <cstdio>
#include <cstdlib>
#include <dlfcn.h>
#include <mutex>
#include <signal.h>
#include <thread>

namespace {
template<typename Type>
class ScopedCF final {
public:
    ScopedCF() = default;
    ScopedCF(const ScopedCF &) = delete;
    ScopedCF &operator=(const ScopedCF &) = delete;
    ~ScopedCF()
    {
        if (m_value)
            CFRelease(m_value);
    }
    Type get() const
    {
        return m_value;
    }
    Type *output()
    {
        return &m_value;
    }
private:
    Type m_value = nullptr;
};

std::atomic_bool stopRequested{false};
static_assert(std::atomic_bool::is_always_lock_free, "Host signal admission requires a lock-free flag");

void requestStop(int)
{
    stopRequested.store(true, std::memory_order_relaxed);
}

int nativeStopRequested(void *)
{
    return stopRequested.load(std::memory_order_relaxed) ? 1 : 0;
}

class HostSignals final {
public:
    HostSignals()
    {
        stopRequested.store(false, std::memory_order_relaxed);
        struct sigaction action {};
        action.sa_handler = requestStop;
        sigemptyset(&action.sa_mask);
        m_termInstalled = sigaction(SIGTERM, &action, &m_term) == 0;
        m_intInstalled = sigaction(SIGINT, &action, &m_int) == 0;
        action.sa_handler = SIG_IGN;
        m_pipeInstalled = sigaction(SIGPIPE, &action, &m_pipe) == 0;
    }
    ~HostSignals()
    {
        if (m_termInstalled)
            sigaction(SIGTERM, &m_term, nullptr);
        if (m_intInstalled)
            sigaction(SIGINT, &m_int, nullptr);
        if (m_pipeInstalled)
            sigaction(SIGPIPE, &m_pipe, nullptr);
    }
    bool valid() const { return m_termInstalled && m_intInstalled && m_pipeInstalled; }
private:
    struct sigaction m_term {}, m_int {}, m_pipe {};
    bool m_termInstalled = false;
    bool m_intInstalled = false;
    bool m_pipeInstalled = false;
};

// A dedicated host process may live indefinitely while its provider is idle.
// After a local stop request, a broken provider gets at most15s to return. A
// forced exit is unknown; it never fabricates a native closure receipt.
class StopDeadline final {
public:
    StopDeadline() : m_thread([this] {
        std::unique_lock lock(m_mutex);
        while (!m_finished && !stopRequested.load(std::memory_order_relaxed))
            m_changed.wait_for(lock, std::chrono::milliseconds(50));
        if (m_finished)
            return;
        if (!m_changed.wait_for(lock, std::chrono::seconds(15), [this] { return m_finished; })) {
            std::fputs("Native worker host stop exceeded its deadline; cleanup is unknown.\n", stderr);
            std::_Exit(1);
        }
    }) {}
    ~StopDeadline()
    {
        {
            std::lock_guard lock(m_mutex);
            m_finished = true;
        }
        m_changed.notify_one();
        m_thread.join();
    }
private:
    std::mutex m_mutex;
    std::condition_variable m_changed;
    bool m_finished = false;
    std::thread m_thread;
};

bool signatureAllowed(const QString &path, const NativeWorkerHostDescriptor &descriptor, bool host)
{
    NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path.toUtf8().constData()]];
    ScopedCF<SecStaticCodeRef> code;
    if (SecStaticCodeCreateWithPath((__bridge CFURLRef)url, kSecCSDefaultFlags, code.output()) != errSecSuccess)
        return false;
    ScopedCF<SecRequirementRef> requirement;
    if (descriptor.trust == NativeWorkerHostDescriptor::Trust::Production) {
        // Match Apple's Developer ID Application chain and the actual host's
        // frozen team. No designated requirement or entitlement is rewritten.
        QString rule = QStringLiteral("anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6] exists "
            "and certificate leaf[field.1.2.840.113635.100.6.1.13] exists and certificate leaf[subject.OU] = \"%1\"")
            .arg(descriptor.teamIdentifier);
        if (host)
            rule += QStringLiteral(" and identifier \"com.dockpipe.launcher\"");
        NSString *text = [NSString stringWithUTF8String:rule.toUtf8().constData()];
        if (SecRequirementCreateWithString((__bridge CFStringRef)text, kSecCSDefaultFlags, requirement.output()) != errSecSuccess)
            return false;
    }
    if (SecStaticCodeCheckValidity(code.get(), kSecCSStrictValidate | kSecCSCheckAllArchitectures, requirement.get()) != errSecSuccess)
        return false;
    ScopedCF<CFDictionaryRef> information;
    if (SecCodeCopySigningInformation(code.get(), kSecCSSigningInformation, information.output()) != errSecSuccess)
        return false;
    NativeWorkerHostSignature signature;
    signature.valid = true;
    signature.developerId = descriptor.trust == NativeWorkerHostDescriptor::Trust::Production;
    const auto entitlements = static_cast<CFDictionaryRef>(CFDictionaryGetValue(information.get(), kSecCodeInfoEntitlementsDict));
    if (entitlements) {
        if (CFGetTypeID(entitlements) != CFDictionaryGetTypeID())
            return false;
        signature.libraryValidationDisabled =
            CFDictionaryGetValue(entitlements, CFSTR("com.apple.security.cs.disable-library-validation")) == kCFBooleanTrue;
    }
    const auto flags = static_cast<CFNumberRef>(CFDictionaryGetValue(information.get(), kSecCodeInfoFlags));
    uint32_t value = 0;
    if (!flags || CFGetTypeID(flags) != CFNumberGetTypeID() || !CFNumberGetValue(flags, kCFNumberSInt32Type, &value))
        return false;
    signature.adhoc = (value & kSecCodeSignatureAdhoc) != 0;
    signature.hardenedRuntime = (value & kSecCodeSignatureRuntime) != 0;
    const auto team = static_cast<CFStringRef>(CFDictionaryGetValue(information.get(), kSecCodeInfoTeamIdentifier));
    if (team && CFGetTypeID(team) == CFStringGetTypeID()) {
        NSString *text = (__bridge NSString *)team;
        signature.teamIdentifier = QString::fromUtf8(text.UTF8String);
    }
    const auto identifier = static_cast<CFStringRef>(CFDictionaryGetValue(information.get(), kSecCodeInfoIdentifier));
    if (identifier && CFGetTypeID(identifier) == CFStringGetTypeID()) {
        NSString *text = (__bridge NSString *)identifier;
        signature.identifier = QString::fromUtf8(text.UTF8String);
    }
    return nativeWorkerHostSignatureAllowed(descriptor, signature, host);
}

bool preserveQuarantine(const QFile &input, const QFile &output)
{
    // Preserve the OS's quarantine decision on the admitted snapshot rather
    // than laundering downloaded code through a newly created private file.
    constexpr auto name = "com.apple.quarantine";
    const auto length = fgetxattr(input.handle(), name, nullptr, 0, 0, 0);
    if (length < 0)
        return errno == ENOATTR;
    if (length > 4096)
        return false;
    QByteArray value(length, Qt::Uninitialized);
    if (fgetxattr(input.handle(), name, value.data(), value.size(), 0, 0) != length)
        return false;
    return fsetxattr(output.handle(), name, value.constData(), value.size(), 0, 0) == 0;
}

QString snapshotModule(const NativeWorkerHostDescriptor &descriptor, QTemporaryDir &directory)
{
    if (!directory.isValid())
        return {};
    // First slice supports self-contained native modules with OS framework
    // dependencies. A private snapshot avoids racing a package update between
    // hash/signature admission and dlopen (which can execute constructors).
    QFile input(descriptor.modulePath);
    const auto path = QDir(directory.path()).filePath(QStringLiteral("native-worker-module"));
    QFile output(path);
    if (!input.open(QIODevice::ReadOnly) || !output.open(QIODevice::WriteOnly | QIODevice::NewOnly)
        || !output.setPermissions(QFile::ReadOwner | QFile::WriteOwner))
        return {};
    QCryptographicHash hash(QCryptographicHash::Sha256);
    qint64 total = 0;
    while (!input.atEnd()) {
        const auto bytes = input.read(65536);
        if (bytes.isEmpty() || total + bytes.size() > 128 * 1024 * 1024 || output.write(bytes) != bytes.size())
            return {};
        total += bytes.size();
        hash.addData(bytes);
    }
    if (input.error() != QFileDevice::NoError || total == 0 || hash.result().toHex() != descriptor.moduleSha256
        || !output.flush() || !preserveQuarantine(input, output))
        return {};
    output.close();
    if (!output.setPermissions(QFile::ReadOwner | QFile::ExeOwner))
        return {};
    return path;
}
} // namespace

int runNativeWorkerHostMac(const NativeWorkerHostDescriptor &descriptor)
{
    @autoreleasepool {
        if (![NSThread isMainThread])
            return 2;
        NSBundle *bundle = NSBundle.mainBundle;
        if (![bundle.bundleIdentifier isEqualToString:@"com.dockpipe.launcher"])
            return 2;
        const auto hostPath = QFileInfo(QString::fromUtf8(bundle.executableURL.path.UTF8String)).canonicalFilePath();
        const auto bundlePath = QFileInfo(QString::fromUtf8(bundle.bundleURL.path.UTF8String)).canonicalFilePath();
        // A second Review.app CLI would reintroduce a separate app identity.
        // Packaging already owns this fixed bundled CLI location.
        if (descriptor.cliPath != QDir(bundlePath).filePath(QStringLiteral("Contents/Helpers/dockpipe")))
            return 2;
        QString error;
        if ((descriptor.trust == NativeWorkerHostDescriptor::Trust::ReviewAdhoc
                && !verifyNativeWorkerHostFile(hostPath, descriptor.hostSha256, true, error))
            || !signatureAllowed(bundlePath, descriptor, true) || !signatureAllowed(descriptor.cliPath, descriptor, false))
            return 2;
        QTemporaryDir directory;
        const auto imagePath = snapshotModule(descriptor, directory);
        if (imagePath.isEmpty() || !signatureAllowed(imagePath, descriptor, false))
            return 2;
        HostSignals hostSignals;
        if (!hostSignals.valid())
            return 2;
        StopDeadline deadline;
        // Intentionally no dlclose: code must remain mapped for late callbacks,
        // especially after unknown cleanup. One module per dedicated process.
        void *module = dlopen(QFile::encodeName(imagePath).constData(), RTLD_NOW | RTLD_LOCAL);
        if (!module)
            return 2;
        auto entry = reinterpret_cast<DockpipeNativeWorkerEntryV1>(dlsym(module, DOCKPIPE_NATIVE_WORKER_ENTRY_V1));
        if (!entry) {
            std::fputs("Native worker package entrypoint is unavailable.\n", stderr);
            std::_Exit(2);
        }
        const auto cli = descriptor.cliPath.toUtf8();
        const auto state = descriptor.stateDirectory.toUtf8();
        DockpipeNativeWorkerHostV1 host{DOCKPIPE_NATIVE_WORKER_ABI_V1, sizeof(DockpipeNativeWorkerHostV1),
            cli.constData(), state.constData(), descriptor.profileJson.constData(), size_t(descriptor.profileJson.size()), nullptr, nativeStopRequested};
        DockpipeNativeWorkerResultV1 result{sizeof(DockpipeNativeWorkerResultV1), DOCKPIPE_NATIVE_WORKER_FAILURE_V1, 0};
        try {
            entry(&host, &result);
        } catch (...) {
            std::fputs("Native worker package failed; cleanup is unknown.\n", stderr);
            std::_Exit(1);
        }
        if (!nativeWorkerHostResultIsClean(result)) {
            std::fputs("Native worker package cleanup is unknown.\n", stderr);
            std::_Exit(1);
        }
        return nativeWorkerHostResultExitCode(result);
    }
}
