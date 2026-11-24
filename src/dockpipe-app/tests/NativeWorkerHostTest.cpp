#include "NativeWorkerHost.h"

#include <QCryptographicHash>
#include <QDir>
#include <QFile>
#include <QJsonDocument>
#include <QJsonObject>
#include <QTemporaryDir>
#include <QtTest>
#include <sys/stat.h>

class NativeWorkerHostTest final : public QObject {
    Q_OBJECT
private:
    QTemporaryDir m_root;

    QString path(const QString &name) const { return QDir(QDir(m_root.path()).canonicalPath()).filePath(name); }

    void write(const QString &name, const QByteArray &content, mode_t mode)
    {
        QFile file(path(name));
        QVERIFY(file.open(QIODevice::WriteOnly));
        QCOMPARE(file.write(content), content.size());
        file.close();
        QCOMPARE(::chmod(QFile::encodeName(path(name)).constData(), mode), 0);
    }

    QJsonObject descriptor(bool preview = false) const
    {
        const auto hash = QCryptographicHash::hash(QByteArrayLiteral("owned fixture image"), QCryptographicHash::Sha256).toHex();
        const QJsonObject module{{"path", path("module")}, {"sha256", QString::fromLatin1(hash)}};
        const QJsonObject cli{{"path", path("cli")}, {"sha256", QString::fromLatin1(hash)}};
        const QJsonObject trust = preview ? QJsonObject{{"mode", "review-adhoc"}, {"host_sha256", QString(64, 'a')}}
                                         : QJsonObject{{"mode", "production"}, {"team_identifier", "ABCDE12345"}};
        return {{"schema", "dockpipe.native-worker-host/v1"}, {"module", module}, {"cli", cli},
            {"state_directory", QDir(m_root.path()).canonicalPath()}, {"profile", QJsonObject{{"opaque_operation", "fixture"}}}, {"trust", trust}};
    }

    std::optional<NativeWorkerHostDescriptor> admit(const QJsonObject &object, QString &error)
    {
        write("descriptor.json", QJsonDocument(object).toJson(), 0600);
        return readNativeWorkerHostDescriptor(path("descriptor.json"), error);
    }

private slots:
    void init()
    {
        QVERIFY(m_root.isValid());
        QCOMPARE(::chmod(QFile::encodeName(m_root.path()).constData(), 0700), 0);
        write("module", "owned fixture image", 0600);
        write("cli", "owned fixture image", 0700);
    }

    void modeArgumentsCannotFallThroughToGui()
    {
        auto parse = [](QList<QByteArray> values) {
            QList<char *> pointers;
            for (auto &value : values)
                pointers.append(value.data());
            return parseNativeWorkerHostArguments(pointers.size(), pointers.data());
        };
        QVERIFY(!parse({"DockPipe", "--version"}).requested);
        QVERIFY(!parse({"DockPipe", "--allow-second-instance"}).requested);
        QVERIFY(parse({"DockPipe", "--native-worker-host", "--request", "/private/owner.json"}).valid);
        for (const auto &arguments : QList<QList<QByteArray>>{
                 {"DockPipe", "--native-worker-host"},
                 {"DockPipe", "--native-worker-host", "--request", "relative.json"},
                 {"DockPipe", "--version", "--native-worker-host", "--request", "/private/owner.json"},
                 {"DockPipe", "--native-worker-host", "--request", "/private/owner.json", "--allow-second-instance"},
                 {"DockPipe", "--native-worker-host", "--native-worker-host", "/private/owner.json"}}) {
            const auto result = parse(arguments);
            QVERIFY(result.requested && !result.valid);
        }
    }

    void exactOwnedDescriptorPreservesOpaqueProfile()
    {
        for (bool preview : {false, true}) {
            QString error;
            const auto admitted = admit(descriptor(preview), error);
            QVERIFY2(admitted.has_value(), qPrintable(error));
            QCOMPARE(admitted->modulePath, path("module"));
            QCOMPARE(admitted->cliPath, path("cli"));
            QCOMPARE(admitted->profileJson, QByteArrayLiteral("{\"opaque_operation\":\"fixture\"}"));
            QCOMPARE(admitted->trust, preview ? NativeWorkerHostDescriptor::Trust::ReviewAdhoc : NativeWorkerHostDescriptor::Trust::Production);
        }
    }

    void rejectsAuthorityAndResourceChanges()
    {
        const auto base = descriptor();
        QList<QJsonObject> invalid;
        auto object = base;
        object.insert("unexpected", true);
        invalid.append(object);
        object = base;
        object["profile"] = QStringLiteral("not an object");
        invalid.append(object);
        object = base;
        object["profile"] = QJsonObject{{"bytes", QString(DOCKPIPE_NATIVE_WORKER_PROFILE_LIMIT_V1, 'x')}};
        invalid.append(object);
        object = base;
        object["trust"] = QJsonObject{{"mode", "production"}, {"team_identifier", "wrong-team"}};
        invalid.append(object);
        object = base;
        object["trust"] = QJsonObject{{"mode", "review-adhoc"}};
        invalid.append(object);
        object = base;
        object["trust"] = QJsonObject{{"mode", "none"}, {"host_sha256", QString(64, 'a')}};
        invalid.append(object);
        object = base;
        auto module = object["module"].toObject();
        module["sha256"] = QString(64, 'f');
        object["module"] = module;
        invalid.append(object);
        for (const auto &candidate : invalid) {
            QString error;
            QVERIFY(!admit(candidate, error));
            QVERIFY(!error.isEmpty());
        }
    }

    void duplicateAuthorityAndEscapedAliasesAreRejected()
    {
        const auto compact = QJsonDocument(descriptor()).toJson(QJsonDocument::Compact);
        for (const auto &key : {QByteArrayLiteral("trust"), QByteArrayLiteral("tr\\u0075st")}) {
            auto duplicate = compact;
            duplicate.chop(1);
            duplicate += ",\"" + key + "\":{\"mode\":\"review-adhoc\",\"host_sha256\":\"" + QByteArray(64, 'a') + "\"}}";
            write("descriptor.json", duplicate, 0600);
            QString error;
            QVERIFY(!readNativeWorkerHostDescriptor(path("descriptor.json"), error));
        }
    }

    void rejectsSharedLinkedAndNonExecutableFiles()
    {
        QString error;
        QVERIFY(admit(descriptor(), error));
        QCOMPARE(::chmod(QFile::encodeName(path("descriptor.json")).constData(), 0644), 0);
        QVERIFY(!readNativeWorkerHostDescriptor(path("descriptor.json"), error));
        QCOMPARE(::chmod(QFile::encodeName(path("module")).constData(), 0660), 0);
        QVERIFY(!admit(descriptor(), error));
        write("module", "owned fixture image", 0600);
        QCOMPARE(::chmod(QFile::encodeName(path("cli")).constData(), 0600), 0);
        QVERIFY(!admit(descriptor(), error));
        write("cli", "owned fixture image", 0700);
        QFile::remove(path("module-link"));
        QVERIFY(QFile::link(path("module"), path("module-link")));
        auto linked = descriptor();
        auto module = linked["module"].toObject();
        module["path"] = path("module-link");
        linked["module"] = module;
        QVERIFY(!admit(linked, error));
        QCOMPARE(::chmod(QFile::encodeName(m_root.path()).constData(), 0755), 0);
        QVERIFY(!admit(descriptor(), error));
    }

    void signerPolicyNeverFallsBackOrDisablesValidation()
    {
        NativeWorkerHostDescriptor descriptor;
        descriptor.teamIdentifier = QStringLiteral("ABCDE12345");
        NativeWorkerHostSignature signature;
        signature.valid = true;
        signature.developerId = true;
        signature.hardenedRuntime = true;
        signature.teamIdentifier = descriptor.teamIdentifier;
        signature.identifier = QStringLiteral("com.dockpipe.launcher");
        QVERIFY(nativeWorkerHostSignatureAllowed(descriptor, signature, true));
        signature.teamIdentifier = QStringLiteral("OTHER12345");
        QVERIFY(!nativeWorkerHostSignatureAllowed(descriptor, signature, true));
        signature.teamIdentifier = descriptor.teamIdentifier;
        signature.hardenedRuntime = false;
        QVERIFY(!nativeWorkerHostSignatureAllowed(descriptor, signature, true));
        QVERIFY(nativeWorkerHostSignatureAllowed(descriptor, signature, false));
        signature.hardenedRuntime = true;
        signature.libraryValidationDisabled = true;
        QVERIFY(!nativeWorkerHostSignatureAllowed(descriptor, signature, true));
        signature.libraryValidationDisabled = false;
        signature.adhoc = true;
        QVERIFY(!nativeWorkerHostSignatureAllowed(descriptor, signature, true));
        descriptor.trust = NativeWorkerHostDescriptor::Trust::ReviewAdhoc;
        QVERIFY(nativeWorkerHostSignatureAllowed(descriptor, signature, true));
        signature.valid = false;
        QVERIFY(!nativeWorkerHostSignatureAllowed(descriptor, signature, true));
        signature.valid = true;
        signature.libraryValidationDisabled = true;
        QVERIFY(!nativeWorkerHostSignatureAllowed(descriptor, signature, true));
        signature.libraryValidationDisabled = false;
        signature.identifier = QStringLiteral("external.package.app");
        QVERIFY(!nativeWorkerHostSignatureAllowed(descriptor, signature, true));
    }

    void missingOrUnknownCleanupCannotBecomeSuccess()
    {
        DockpipeNativeWorkerResultV1 result{sizeof(result), DOCKPIPE_NATIVE_WORKER_SUCCESS_V1, 0};
        QVERIFY(!nativeWorkerHostResultIsClean(result));
        QCOMPARE(nativeWorkerHostResultExitCode(result), 1);
        result.cleanup_verified = 1;
        QVERIFY(nativeWorkerHostResultIsClean(result));
        QCOMPARE(nativeWorkerHostResultExitCode(result), 0);
        result.outcome = DOCKPIPE_NATIVE_WORKER_CANCELLED_V1;
        QCOMPARE(nativeWorkerHostResultExitCode(result), 130);
        result.outcome = DOCKPIPE_NATIVE_WORKER_FAILURE_V1;
        QCOMPARE(nativeWorkerHostResultExitCode(result), 1);
        result.outcome = 9;
        QVERIFY(!nativeWorkerHostResultIsClean(result));
        result.outcome = DOCKPIPE_NATIVE_WORKER_SUCCESS_V1;
        result.struct_size = 0;
        QVERIFY(!nativeWorkerHostResultIsClean(result));
    }
};

QTEST_APPLESS_MAIN(NativeWorkerHostTest)
#include "NativeWorkerHostTest.moc"
