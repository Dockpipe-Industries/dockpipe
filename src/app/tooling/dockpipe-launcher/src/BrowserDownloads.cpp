#include "BrowserDownloads.h"
#include "BrowserPreferences.h"

#include <QDesktopServices>
#include <QDialog>
#include <QDir>
#include <QFileDialog>
#include <QFile>
#include <QFileInfo>
#include <QHeaderView>
#include <QHBoxLayout>
#include <QLabel>
#include <QLocale>
#include <QPushButton>
#include <QStandardPaths>
#include <QTemporaryDir>
#include <QTreeWidget>
#include <QUrl>
#include <QVBoxLayout>
#include <QWebEngineDownloadRequest>
#include <QWebEngineProfile>

namespace {
// Transfers share the browser process. Retain only the most recent session
// records, while preserving every active transfer until it reaches an outcome.
constexpr size_t maximumSessionDownloads = 100;
constexpr int maximumActiveDownloads = 4;
}

BrowserDownloads::BrowserDownloads(QWebEngineProfile *profile, QWidget *host)
    : QObject(host), m_host(host)
{
    setObjectName("browserDownloads");
    connect(profile, &QWebEngineProfile::downloadRequested,
        this, &BrowserDownloads::requestDownload);
}

BrowserDownloads::~BrowserDownloads()
{
    for (auto &[id, entry] : m_entries) {
        if (!entry.request)
            continue;
        disconnect(entry.request, nullptr, this, nullptr);
        entry.request->cancel();
        entry.request->deleteLater();
    }
    delete m_panel;
}

void BrowserDownloads::showPanel()
{
    if (!m_host)
        return;
    if (!m_panel) {
        m_panel = new QDialog(m_host);
        m_panel->setObjectName("browserDownloadsPanel");
        m_panel->setWindowTitle(tr("Downloads"));
        m_panel->resize(700, 400);
        connect(m_panel, &QObject::destroyed, this, [this]() {
            m_list = nullptr;
            for (auto &[id, entry] : m_entries)
                entry.row = nullptr;
        });
        auto *layout = new QVBoxLayout(m_panel);
        auto *description = new QLabel(tr("Downloads from this session. Closing this window keeps downloads running."));
        description->setWordWrap(true);
        layout->addWidget(description);
        m_list = new QTreeWidget;
        m_list->setObjectName("browserDownloadsList");
        m_list->setHeaderLabels({tr("File"), tr("Status"), tr("Progress")});
        m_list->setRootIsDecorated(false);
        m_list->setSelectionMode(QAbstractItemView::SingleSelection);
        m_list->header()->setSectionResizeMode(0, QHeaderView::Stretch);
        m_list->header()->setSectionResizeMode(1, QHeaderView::ResizeToContents);
        m_list->header()->setSectionResizeMode(2, QHeaderView::ResizeToContents);
        layout->addWidget(m_list, 1);
        auto *actions = new QHBoxLayout;
        m_pause = new QPushButton(tr("Pause"));
        m_pause->setObjectName("browserDownloadPause");
        m_cancel = new QPushButton(tr("Cancel download"));
        m_cancel->setObjectName("browserDownloadCancel");
        m_reveal = new QPushButton(tr("Show folder"));
        m_reveal->setObjectName("browserDownloadReveal");
        m_clear = new QPushButton(tr("Clear finished"));
        m_clear->setObjectName("browserDownloadsClear");
        m_clear->setToolTip(tr("Remove finished entries from this list. Files are kept."));
        actions->addWidget(m_pause);
        actions->addWidget(m_cancel);
        actions->addWidget(m_reveal);
        actions->addStretch();
        actions->addWidget(m_clear);
        layout->addLayout(actions);
        connect(m_list, &QTreeWidget::itemSelectionChanged, this, &BrowserDownloads::updateActions);
        connect(m_pause, &QPushButton::clicked, this, [this]() {
            Entry *entry = selectedEntry();
            if (!entry || !entry->request || entry->finished)
                return;
            if (entry->request->isPaused())
                entry->request->resume();
            else
                entry->request->pause();
        });
        connect(m_cancel, &QPushButton::clicked, this, [this]() {
            Entry *entry = selectedEntry();
            if (entry && entry->request && !entry->finished)
                entry->request->cancel();
        });
        connect(m_reveal, &QPushButton::clicked, this, [this]() {
            const Entry *entry = selectedEntry();
            if (!entry || (!entry->completed && !entry->retainedForRecovery))
                return;
            if (!QDesktopServices::openUrl(QUrl::fromLocalFile(QFileInfo(entry->destination).absolutePath())))
                emit statusChanged(tr("Could not open the downloads folder."));
        });
        connect(m_clear, &QPushButton::clicked, this, &BrowserDownloads::clearFinished);
        for (auto &[id, entry] : m_entries) {
            entry.row = new QTreeWidgetItem(m_list);
            entry.row->setData(0, Qt::UserRole, QVariant::fromValue(id));
            updateRow(entry);
        }
    }
    updateActions();
    m_panel->show();
    m_panel->raise();
    m_panel->activateWindow();
}

void BrowserDownloads::requestDownload(QWebEngineDownloadRequest *request)
{
    int activeDownloads = 0;
    for (const auto &[id, entry] : m_entries) {
        if (!entry.finished)
            ++activeDownloads;
    }
    if (m_promptOpen || activeDownloads >= maximumActiveDownloads || !m_host) {
        request->cancel();
        request->deleteLater();
        emit statusChanged(tr("Finish the current save dialog or an active download before downloading another file."));
        return;
    }

    // The native save dialog runs a nested event loop. Either owner can be
    // destroyed or the request cancelled while the user chooses a destination.
    QPointer<BrowserDownloads> owner(this);
    QPointer<QWebEngineDownloadRequest> pending(request);
    const BrowserSettings settings = BrowserPreferences::instance()->settings();
    QString suggestedName = QFileInfo(request->suggestedFileName()).fileName();
    if (suggestedName.isEmpty() || suggestedName == "." || suggestedName == "..")
        suggestedName = QStringLiteral("download");
    QString destination;
    std::unique_ptr<QTemporaryDir> stagingDirectory;
    if (settings.askWhereToSave) {
        m_promptOpen = true;
        destination = QFileDialog::getSaveFileName(m_host, tr("Save download"),
            QDir(settings.downloadDirectory).filePath(suggestedName));
    } else {
        if (!QDir().mkpath(settings.downloadDirectory)) {
            request->cancel();
            request->deleteLater();
            emit statusChanged(tr("Could not create the configured downloads folder."));
            return;
        }
        // Stage automatic transfers in a unique directory on the destination
        // filesystem. Final rename refuses replacement, including races with
        // another transfer or application. Never reserve by overwriting a file.
        stagingDirectory = std::make_unique<QTemporaryDir>(
            QDir(settings.downloadDirectory).filePath(".dockpipe-download-XXXXXX"));
        if (!stagingDirectory->isValid()) {
            request->cancel();
            request->deleteLater();
            emit statusChanged(tr("Could not prepare a download in the configured folder."));
            return;
        }
        destination = stagingDirectory->filePath(suggestedName);
    }
    if (!owner) {
        if (pending) {
            pending->cancel();
            pending->deleteLater();
        }
        return;
    }
    m_promptOpen = false;
    if (!pending)
        return;
    if (request->state() != QWebEngineDownloadRequest::DownloadRequested) {
        request->deleteLater();
        return;
    }
    if (destination.isEmpty() || QFileInfo(destination).fileName().isEmpty()) {
        request->cancel();
        request->deleteLater();
        emit statusChanged(tr("Download cancelled."));
        return;
    }

    if (m_entries.size() >= maximumSessionDownloads)
        removeOldestFinished();
    const QFileInfo file(destination);
    request->setDownloadDirectory(file.absolutePath());
    request->setDownloadFileName(file.fileName());
    const quint64 id = ++m_nextId;
    Entry &entry = m_entries[id];
    entry.request = request;
    entry.fileName = file.fileName();
    entry.destination = file.absoluteFilePath();
    if (stagingDirectory) {
        entry.automaticDirectory = settings.downloadDirectory;
        entry.stagingDirectory = std::move(stagingDirectory);
    }
    if (m_panel) {
        entry.row = new QTreeWidgetItem;
        entry.row->setData(0, Qt::UserRole, QVariant::fromValue(id));
        m_list->insertTopLevelItem(0, entry.row);
    }
    connect(request, &QWebEngineDownloadRequest::receivedBytesChanged, this, [this, id]() { updateDownload(id); });
    connect(request, &QWebEngineDownloadRequest::totalBytesChanged, this, [this, id]() { updateDownload(id); });
    connect(request, &QWebEngineDownloadRequest::stateChanged, this, [this, id]() { updateDownload(id); });
    connect(request, &QWebEngineDownloadRequest::isPausedChanged, this, [this, id]() { updateDownload(id); });
    connect(request, &QObject::destroyed, this, [this, id]() {
        auto found = m_entries.find(id);
        if (found == m_entries.end())
            return;
        Entry &entry = found->second;
        entry.request = nullptr;
        if (!entry.retainedForRecovery)
            entry.stagingDirectory.reset();
        if (!entry.finished) {
            entry.finished = true;
            entry.status = tr("Cancelled");
            updateRow(entry);
            emit statusChanged(tr("Download cancelled: %1").arg(entry.fileName));
        }
        updateActions();
    });
    request->accept();
    if (!owner)
        return;
    updateDownload(id);
    if (owner && pending && !pending->isFinished())
        emit statusChanged(tr("Downloading %1…").arg(file.fileName()));
}

void BrowserDownloads::updateDownload(quint64 id)
{
    auto found = m_entries.find(id);
    if (found == m_entries.end() || !found->second.request || found->second.finished)
        return;
    Entry &entry = found->second;
    QWebEngineDownloadRequest *request = entry.request;
    entry.receivedBytes = request->receivedBytes();
    entry.totalBytes = request->totalBytes();
    const bool wasFinished = entry.finished;
    switch (request->state()) {
    case QWebEngineDownloadRequest::DownloadRequested:
        entry.status = tr("Starting…");
        break;
    case QWebEngineDownloadRequest::DownloadInProgress:
        entry.status = request->isPaused() ? tr("Paused") : tr("Downloading");
        break;
    case QWebEngineDownloadRequest::DownloadCompleted:
        entry.status = tr("Saved");
        entry.finished = true;
        entry.completed = !entry.stagingDirectory || finalizeAutomaticDownload(entry);
        break;
    case QWebEngineDownloadRequest::DownloadCancelled:
        entry.status = tr("Cancelled");
        entry.finished = true;
        break;
    case QWebEngineDownloadRequest::DownloadInterrupted:
        entry.status = tr("Interrupted: %1").arg(request->interruptReasonString());
        entry.finished = true;
        break;
    }
    updateRow(entry);
    updateActions();
    if (entry.finished && !wasFinished) {
        request->deleteLater();
        if (entry.completed)
            emit statusChanged(tr("Download saved: %1").arg(entry.fileName));
        else
            emit statusChanged(tr("%1 — %2").arg(entry.fileName, entry.status));
    }
}

bool BrowserDownloads::finalizeAutomaticDownload(Entry &entry)
{
    const QFileInfo original(entry.fileName);
    const QString suffix = original.completeSuffix();
    const QString extension = suffix.isEmpty() ? QString() : "." + suffix;
    const QString base = original.baseName();
    QFile downloaded(entry.destination);
    // A finite filename search prevents unbounded UI work in a crowded folder.
    // Exhaustion retains the completed bytes for recovery instead of replacing
    // any existing file or silently deleting a successful network transfer.
    for (int attempt = 0; attempt < 1000; ++attempt) {
        const QString name = attempt == 0 ? entry.fileName
            : QStringLiteral("%1 (%2)%3").arg(base).arg(attempt).arg(extension);
        const QString destination = QDir(entry.automaticDirectory).filePath(name);
        if (QFileInfo::exists(destination))
            continue;
        if (downloaded.rename(destination)) {
            entry.fileName = name;
            entry.destination = destination;
            entry.stagingDirectory.reset();
            return true;
        }
        if (!QFileInfo::exists(destination))
            break;
    }
    entry.stagingDirectory->setAutoRemove(false);
    entry.retainedForRecovery = true;
    entry.status = tr("Could not save to the chosen folder. File retained at %1").arg(entry.destination);
    return false;
}

void BrowserDownloads::updateRow(Entry &entry)
{
    if (!entry.row)
        return;
    entry.row->setText(0, entry.fileName);
    entry.row->setToolTip(0, entry.destination);
    entry.row->setText(1, entry.status);
    const QLocale locale;
    QString progress = locale.formattedDataSize(entry.receivedBytes);
    if (entry.totalBytes > 0)
        progress += tr(" / %1").arg(locale.formattedDataSize(entry.totalBytes));
    entry.row->setText(2, progress);
}

BrowserDownloads::Entry *BrowserDownloads::selectedEntry()
{
    if (!m_list || !m_list->currentItem())
        return nullptr;
    const quint64 id = m_list->currentItem()->data(0, Qt::UserRole).toULongLong();
    auto found = m_entries.find(id);
    return found == m_entries.end() ? nullptr : &found->second;
}

void BrowserDownloads::updateActions()
{
    if (!m_panel)
        return;
    const Entry *entry = selectedEntry();
    const bool active = entry && entry->request && !entry->finished;
    m_pause->setEnabled(active);
    m_pause->setText(active && entry->request->isPaused() ? tr("Resume") : tr("Pause"));
    m_cancel->setEnabled(active);
    m_reveal->setEnabled(entry && (entry->completed || entry->retainedForRecovery));
    bool hasFinished = false;
    for (const auto &[id, candidate] : m_entries) {
        if (candidate.finished) {
            hasFinished = true;
            break;
        }
    }
    m_clear->setEnabled(hasFinished);
}

void BrowserDownloads::clearFinished()
{
    for (auto entry = m_entries.begin(); entry != m_entries.end();) {
        if (entry->second.finished) {
            delete entry->second.row;
            entry = m_entries.erase(entry);
        } else {
            ++entry;
        }
    }
    updateActions();
}

void BrowserDownloads::removeOldestFinished()
{
    for (auto entry = m_entries.begin(); entry != m_entries.end(); ++entry) {
        if (entry->second.finished) {
            delete entry->second.row;
            m_entries.erase(entry);
            break;
        }
    }
}
