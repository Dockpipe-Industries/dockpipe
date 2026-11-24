#pragma once

#include "DockpipeNativeWorkerPackage.h"
#include <QByteArray>
#include <QString>
#include <optional>

struct NativeWorkerHostArguments {
    bool requested = false;
    bool valid = false;
    QString descriptorPath;
};

struct NativeWorkerHostDescriptor {
    enum class Trust { Production, ReviewAdhoc };
    QString modulePath;
    QByteArray moduleSha256;
    QString cliPath;
    QByteArray cliSha256;
    QString stateDirectory;
    QByteArray profileJson;
    Trust trust = Trust::Production;
    QString teamIdentifier;
    QByteArray hostSha256;
};

NativeWorkerHostArguments parseNativeWorkerHostArguments(int argc, char *argv[]);
std::optional<NativeWorkerHostDescriptor> readNativeWorkerHostDescriptor(const QString &path, QString &error);
bool verifyNativeWorkerHostFile(const QString &path, const QByteArray &sha256, bool executable, QString &error);
// No value means normal GUI startup; a value is this dedicated mode's exit code.
std::optional<int> dispatchNativeWorkerHost(int argc, char *argv[]);

bool nativeWorkerHostResultIsClean(const DockpipeNativeWorkerResultV1 &result);
int nativeWorkerHostResultExitCode(const DockpipeNativeWorkerResultV1 &result);

struct NativeWorkerHostSignature {
    bool valid = false;
    bool adhoc = false;
    bool developerId = false;
    bool hardenedRuntime = false;
    bool libraryValidationDisabled = false;
    QString teamIdentifier;
    QString identifier;
};
bool nativeWorkerHostSignatureAllowed(const NativeWorkerHostDescriptor &descriptor,
    const NativeWorkerHostSignature &signature, bool host);
