#include "WorkspaceGraphPage.h"
#include "WorkspaceGraphTimestamp.h"
#include "WorkspaceFiles.h"
#include <QCoreApplication>
#include <QCryptographicHash>
#include <QDateTime>
#include <QElapsedTimer>
#include <QFileInfo>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QTextStream>
#include <QTranslator>
#include <QRegularExpression>
#include <QStringDecoder>
#include <algorithm>
#include <filesystem>
#include <queue>

namespace {
std::filesystem::path nativePath(const QString &path)
{
#ifdef Q_OS_WIN
    return std::filesystem::path(path.toStdWString());
#else
    return std::filesystem::path(QFile::encodeName(path).constData());
#endif
}
QString fileName(const std::filesystem::path &path)
{
#ifdef Q_OS_WIN
    return QString::fromStdWString(path.filename().native());
#else
    return QFile::decodeName(path.filename().native().c_str());
#endif
}
QJsonObject failure(const QString &message)
{
    return {{"error", message}};
}
}

QJsonObject WorkspaceGraphPage::read(const QJsonObject &request)
{
    const QString root = request.value("root").toString();
    if (request.value("operation").toString() == "preview") {
        const QString path = request.value("path").toString();
        if (root.size() > 32768 || path.size() > 32768 || !WorkspaceFiles::contains(root, path))
            return failure(QCoreApplication::translate("WorkspaceGraphPage", "Preview is outside this workspace or is a link."));
        const QFileInfo info(path);
        if (!info.isFile())
            return failure(QCoreApplication::translate("WorkspaceGraphPage", "Only regular files can be previewed."));
        QFile file(path);
        if (!file.open(QIODevice::ReadOnly))
            return failure(QCoreApplication::translate("WorkspaceGraphPage", "Cannot read this file."));
        // A hologram is a small excerpt, not a second editor. Read at most 8 KiB
        // in the cancellable helper; never load a whole image/archive or run code.
        const QByteArray bytes = file.read(8192);
        if (file.error() != QFile::NoError)
            return failure(QCoreApplication::translate("WorkspaceGraphPage", "File preview read failed."));
        if (bytes.contains('\0'))
            return {{"text", QCoreApplication::translate("WorkspaceGraphPage", "Binary file · Open in the file viewer for details.")}};
        QStringDecoder decoder(QStringDecoder::Utf8);
        const QString text = decoder(bytes);
        if (decoder.hasError())
            return {{"text", QCoreApplication::translate("WorkspaceGraphPage", "Non-UTF-8 file · Text preview unavailable.")}};
        const auto lines = text.split('\n');
        return {{"text", lines.mid(0, 128).join('\n')},
            {"truncated", info.size() > bytes.size() || lines.size() > 128}};
    }
    const QString directory = request.value("directory").toString();
    const QString after = request.value("after").toString();
    const int limit = request.value("limit").toInt();
    const bool hidden = request.value("hidden").toBool(true);
    if (root.size() > 32768 || directory.size() > 32768 || after.size() > 32768
        || limit < 1 || limit > maximumPageNodes || !WorkspaceFiles::contains(root, directory))
        return failure(QCoreApplication::translate("WorkspaceGraphPage", "Choose a directory inside this workspace. Links and Git metadata are not traversed."));
    const QFileInfo before(directory);
    if (!before.isDir())
        return failure(QCoreApplication::translate("WorkspaceGraphPage", "This directory no longer exists. Return to the workspace root or refresh."));
    std::error_code error;
    const auto nativeDirectory = nativePath(directory);
    const auto timestamp = std::filesystem::last_write_time(nativeDirectory, error);
    if (error)
        return failure(QCoreApplication::translate("WorkspaceGraphPage", "Cannot inspect this directory. Check permissions and retry."));
    const QString timestampText = WorkspaceGraphTimestamp::decimalCount(timestamp.time_since_epoch().count());
    const QString requestedStamp = request.value("stamp").toString();
    if (!requestedStamp.isEmpty() && requestedStamp.section(':', 0, 0) != timestampText)
        return failure(QCoreApplication::translate("WorkspaceGraphPage", "Directory changed between pages. Refresh this branch before loading more."));
    const QString canonical = before.canonicalFilePath();
    std::filesystem::directory_iterator iterator(nativeDirectory, error);
    if (error)
        return failure(QCoreApplication::translate("WorkspaceGraphPage", "Cannot read directory: %1").arg(QString::fromStdString(error.message())));
    // Keep only the lexically smallest page after the cursor. Even a million-entry
    // directory uses O(page size) retained names; each page rescans this directory
    // because filesystems do not provide sorted cursors. Never recurse or stat every entry.
    std::priority_queue<QString> names;
    QElapsedTimer elapsed;
    elapsed.start();
    quint64 scanned = 0;
    // Directory timestamps can repeat across rapid changes. Include every name
    // in an order-independent fingerprint without retaining the whole listing.
    QByteArray namesFingerprint(32, '\0');
    while (iterator != std::filesystem::directory_iterator()) {
        const QString name = fileName(iterator->path());
        const QByteArray nameDigest = QCryptographicHash::hash(name.toUtf8(), QCryptographicHash::Sha256);
        for (qsizetype index = 0; index < namesFingerprint.size(); ++index) {
            namesFingerprint[index] = static_cast<char>(
                static_cast<unsigned char>(namesFingerprint.at(index))
                ^ static_cast<unsigned char>(nameDigest.at(index)));
        }
        if (name != ".git" && (hidden || !name.startsWith('.')) && name > after) {
            if (names.size() < static_cast<size_t>(limit + 1)) {
                names.push(name);
            } else if (name < names.top()) {
                names.pop();
                names.push(name);
            }
        }
        iterator.increment(error);
        if (error)
            return failure(QCoreApplication::translate("WorkspaceGraphPage", "Directory changed or became unreadable: %1. Refresh to retry.").arg(QString::fromStdString(error.message())));
        if (++scanned % 256 == 0 && elapsed.elapsed() > 20000)
            return failure(QCoreApplication::translate("WorkspaceGraphPage", "Directory listing exceeded 20 seconds. Choose a smaller directory or retry when the filesystem responds."));
    }
    const QFileInfo current(directory);
    const auto afterTimestamp = std::filesystem::last_write_time(nativeDirectory, error);
    if (error || !current.exists() || current.isSymLink() || current.canonicalFilePath() != canonical
        || afterTimestamp != timestamp)
        return failure(QCoreApplication::translate("WorkspaceGraphPage", "Directory changed while loading. Refresh to start a new listing."));
    const QString stamp = QStringLiteral("%1:%2:%3")
        .arg(timestampText).arg(scanned)
        .arg(QString::fromLatin1(namesFingerprint.toHex()));
    if (!requestedStamp.isEmpty() && requestedStamp != stamp)
        return failure(QCoreApplication::translate("WorkspaceGraphPage", "Directory changed between pages. Refresh this branch before loading more."));
    const bool more = names.size() > static_cast<size_t>(limit);
    if (more)
        names.pop();
    QStringList ordered;
    while (!names.empty()) {
        ordered.prepend(names.top());
        names.pop();
    }
    QJsonArray entries;
    for (const QString &name : ordered) {
        const QFileInfo info(QDir(directory).filePath(name));
        const bool link = info.isSymLink();
        QString kind = "file";
        if (link)
            kind = "link";
        else if (info.isDir())
            kind = "directory";
        else if (!info.isFile())
            kind = "unavailable";
        QJsonObject entry{{"name", name}, {"kind", kind}};
        // Reuse this page's cached stat data. Never read contents or total a
        // directory tree; decimal text preserves large byte counts in JSON.
        if (kind == "file")
            entry.insert("size_bytes", QString::number(info.size()));
        entries.append(entry);
    }
    return {{"entries", entries}, {"more", more}, {"after", ordered.isEmpty() ? after : ordered.last()},
        {"stamp", stamp}, {"scanned", static_cast<qint64>(scanned)}, {"elapsed_ms", elapsed.elapsed()}};
}

int WorkspaceGraphPage::run(int argc, char **argv)
{
    QCoreApplication app(argc, argv);
    QTranslator translator;
    const QString language = qEnvironmentVariable("DOCKPIPE_UI_LANGUAGE", "en");
    const QRegularExpression localePattern("\\A[a-z]{2}(?:_[A-Z]{2})?\\z");
    if (localePattern.match(language).hasMatch()
        && translator.load(":/i18n/dockpipe_" + language + ".qm"))
        app.installTranslator(&translator);
    // One bounded JSON request on stdin. The GUI closes the input channel after writing.
    QFile input;
    input.open(stdin, QIODevice::ReadOnly);
    const QByteArray bytes = input.read(256 * 1024 + 1);
    QJsonParseError parse;
    const auto document = QJsonDocument::fromJson(bytes, &parse);
    QJsonObject result;
    if (bytes.size() > 256 * 1024 || parse.error != QJsonParseError::NoError || !document.isObject())
        result = failure(QCoreApplication::translate("WorkspaceGraphPage", "Invalid directory-page request."));
    else
        result = read(document.object());
    QByteArray response = QJsonDocument(result).toJson(QJsonDocument::Compact);
    if (response.size() > maximumReplyBytes)
        response = QJsonDocument(failure(QCoreApplication::translate("WorkspaceGraphPage", "Directory page exceeds the 2 MiB limit. Reduce the page size."))).toJson(QJsonDocument::Compact);
    QFile output;
    output.open(stdout, QIODevice::WriteOnly);
    output.write(response);
    return 0;
}
