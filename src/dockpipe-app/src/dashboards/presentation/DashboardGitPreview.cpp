#include "DashboardGitPreview.h"
#include "GitDiffWidget.h"
#include "ProjectGitClient.h"
#include "RemoteGitContext.h"

#include <QDir>
#include <QFileInfo>
#include <QHideEvent>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QLabel>
#include <QPushButton>
#include <QShowEvent>
#include <QTimer>
#include <QVBoxLayout>

namespace {
QString previewRoot(const QString &repository, const QJsonObject &catalog)
{
    if (!catalog.isEmpty())
        return QStringLiteral("/");
    return QFileInfo(repository).canonicalFilePath();
}
}

DashboardGitPreview::DashboardGitPreview(const QString &repository, const QString &path, bool staged, QWidget *parent)
    : DashboardGitPreview(repository, path, staged, {}, {}, parent)
{
}

DashboardGitPreview::DashboardGitPreview(const QString &repository, const QString &path, bool staged,
                                       const QJsonObject &catalog, const QString &rootId, QWidget *parent)
    : QWidget(parent), m_root(previewRoot(repository, catalog)), m_path(path), m_staged(staged),
      m_client(std::make_unique<ProjectGitClient>()), m_diff(new GitDiffWidget(this)),
      m_status(new QLabel(this)), m_refresh(new QPushButton(tr("Refresh"), this))
{
    auto target = catalog.value("target").toObject();
    if (!catalog.isEmpty() && target.isEmpty())
        target.insert("node", "unavailable");
    RemoteGitContext::bind(m_client.get(), !catalog.isEmpty(), target, rootId);
    QJsonObject machine{{"mode", catalog.isEmpty() ? "local" : "remote"}};
    if (!catalog.isEmpty()) {
        machine.insert("target", target);
        machine.insert("rootId", rootId);
    }
    setProperty("dashboardMachineContext", machine);
    setObjectName("dashboardGitPreview");
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(8, 8, 8, 8);
    auto *header = new QHBoxLayout;
    auto *title = new QLabel(tr("%1 · %2").arg(path, staged ? tr("Staged") : tr("Unstaged")), this);
    title->setTextFormat(Qt::PlainText);
    title->setWordWrap(true);
    title->setToolTip(QDir::toNativeSeparators(m_root));
    header->addWidget(title, 1);
    m_refresh->setObjectName("dashboardGitPreviewRefresh");
    header->addWidget(m_refresh);
    layout->addLayout(header);
    m_diff->setFileSelectorVisible(false);
    layout->addWidget(m_diff, 1);
    m_status->setObjectName("dashboardGitPreviewStatus");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    layout->addWidget(m_status);
    connect(m_refresh, &QPushButton::clicked, this, &DashboardGitPreview::refresh);
    connect(m_client.get(), &ProjectGitClient::busyChanged, this, [this](bool busy) {
        m_refresh->setEnabled(!busy);
        if (!busy && m_refreshOnIdle) {
            m_refreshOnIdle = false;
            QTimer::singleShot(0, this, [this]() {
                if (isVisible())
                    refresh();
            });
        }
    });
}

DashboardGitPreview::~DashboardGitPreview()
{
    ++m_generation;
    m_refreshOnIdle = false;
    // Destroying the transport clears its completion before bounded process cleanup.
    m_client.reset();
}

void DashboardGitPreview::refresh()
{
    if (m_client->busy())
        return;
    const QString cleanPath = QDir::cleanPath(m_path);
    if (m_root.isEmpty()) {
        m_status->setText(tr("This repository folder is unavailable."));
        return;
    }
    if (m_path.isEmpty() || QDir::isAbsolutePath(m_path) || cleanPath == ".." || cleanPath.startsWith("../")) {
        m_status->setText(tr("Choose a changed file inside this repository."));
        return;
    }
    const quint64 generation = ++m_generation;
    m_diff->clear();
    m_status->setText(tr("Loading saved changes in %1…").arg(m_path));
    // Compare requires the runtime's fresh snapshot; the runtime also detects
    // changes between these two read-only operations and reports them clearly.
    m_client->run(m_root, {{"action", "status"}},
        [this, generation](bool ok, const QJsonObject &snapshot, const QString &error) {
            if (generation != m_generation)
                return;
            if (!ok) {
                m_status->setText(error);
                return;
            }
            m_client->run(m_root, {{"action", "compare"}, {"snapshot", snapshot.value("snapshot")},
                {"paths", QJsonArray{m_path}}, {"comparison", m_staged ? "staged" : "unstaged"}},
                [this, generation](bool compared, const QJsonObject &reply, const QString &failure) {
                    if (generation != m_generation)
                        return;
                    if (!compared) {
                        m_status->setText(failure);
                        return;
                    }
                    const auto review = reply.value("review").toObject();
                    m_diff->setReview(review);
                    m_status->setText(review.value("notice").toString(
                        tr("Reviewing saved changes in %1").arg(m_path)));
                });
        });
}

void DashboardGitPreview::hideEvent(QHideEvent *event)
{
    ++m_generation;
    m_refreshOnIdle = false;
    m_client->cancel();
    QWidget::hideEvent(event);
}

void DashboardGitPreview::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    m_refreshOnIdle = m_client->busy();
    if (!m_refreshOnIdle) {
        QTimer::singleShot(0, this, [this]() {
            if (isVisible())
                refresh();
        });
    }
}
