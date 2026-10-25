#include "DashboardLogWidget.h"

#include "ContextStore.h"
#include "SessionManager.h"

#include <QComboBox>
#include <QDir>
#include <QFile>
#include <QLabel>
#include <QPlainTextEdit>
#include <QScrollBar>
#include <QSignalBlocker>
#include <QTextCursor>
#include <QTimer>
#include <QVBoxLayout>

DashboardLogWidget::DashboardLogWidget(SessionManager &sessions, ContextStore &store, QWidget *parent)
    : QWidget(parent), m_sessions(sessions), m_store(store)
{
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    m_runs = new QComboBox;
    m_runs->setObjectName("dashboardLogRun");
    m_runs->setSizeAdjustPolicy(QComboBox::AdjustToMinimumContentsLengthWithIcon);
    m_runs->setMinimumContentsLength(12);
    layout->addWidget(m_runs);
    m_log = new QPlainTextEdit;
    m_log->setObjectName("dashboardLogTail");
    m_log->setReadOnly(true);
    m_log->setMaximumBlockCount(400);
    m_log->setPlaceholderText(tr("Logs from workflows launched in this desktop session appear here."));
    layout->addWidget(m_log, 1);
    m_status = new QLabel;
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    layout->addWidget(m_status);
    connect(m_runs, qOverload<int>(&QComboBox::currentIndexChanged), this, &DashboardLogWidget::loadTail);
    connect(&sessions, &SessionManager::sessionStarted, this, &DashboardLogWidget::refresh);
    connect(&sessions, &SessionManager::sessionStopped, this, &DashboardLogWidget::refresh);
    connect(&sessions, &SessionManager::sessionFailed, this, &DashboardLogWidget::refresh);
    auto *timer = new QTimer(this);
    timer->setInterval(2000);
    connect(timer, &QTimer::timeout, this, [this]() {
        if (isVisible())
            refresh();
    });
    timer->start();
    refresh();
}

void DashboardLogWidget::setWorkdir(const QString &workdir)
{
    m_workdir = workdir;
    refresh();
}

void DashboardLogWidget::refresh()
{
    const QString selected = m_runs->currentData().toString();
    QVector<QPair<QString, QString>> runs;
    for (const auto &context : m_store.contexts) {
        if (!m_workdir.isEmpty() && QDir::cleanPath(context.workdir) != QDir::cleanPath(m_workdir))
            continue;
        if (!m_sessions.info(context.id).logPath.isEmpty())
            runs.append({context.id, context.label});
    }
    bool changed = m_runs->count() != runs.size();
    for (int index = 0; !changed && index < runs.size(); ++index)
        changed = m_runs->itemData(index).toString() != runs[index].first || m_runs->itemText(index) != runs[index].second;
    if (changed) {
        const QSignalBlocker blocker(m_runs);
        m_runs->clear();
        for (const auto &run : runs)
            m_runs->addItem(run.second, run.first);
        const int selectedIndex = m_runs->findData(selected);
        if (selectedIndex >= 0)
            m_runs->setCurrentIndex(selectedIndex);
    }
    loadTail();
}

void DashboardLogWidget::loadTail()
{
    const QString id = m_runs->currentData().toString();
    if (id.isEmpty()) {
        m_log->clear();
        m_lastTail.clear();
        m_status->setText(tr("No desktop session logs for this project."));
        return;
    }
    const auto session = m_sessions.info(id);
    QFile file(session.logPath);
    if (!file.open(QIODevice::ReadOnly)) {
        m_log->clear();
        m_lastTail.clear();
        m_status->setText(tr("This run's log is unavailable."));
        return;
    }
    // Keep the compact view bounded even for a long-running build or service.
    constexpr qint64 tailBytes = 64 * 1024;
    const bool clipped = file.size() > tailBytes;
    if (clipped && !file.seek(file.size() - tailBytes)) {
        m_log->clear();
        m_lastTail.clear();
        m_status->setText(tr("Could not read the log tail."));
        return;
    }
    QByteArray bytes = file.read(tailBytes);
    if (file.error() != QFile::NoError) {
        m_log->clear();
        m_lastTail.clear();
        m_status->setText(tr("Could not read the log tail."));
        return;
    }
    if (clipped) {
        const auto newline = bytes.indexOf('\n');
        if (newline >= 0)
            bytes.remove(0, newline + 1);
    }
    const QString text = QString::fromUtf8(bytes);
    if (m_lastTail != text) {
        m_lastTail = text;
        auto *scroll = m_log->verticalScrollBar();
        const bool atBottom = scroll->value() >= scroll->maximum() - 2;
        const int position = scroll->value();
        m_log->setPlainText(text);
        scroll->setValue(atBottom ? scroll->maximum() : position);
    }
    m_status->setText(tr("%1 · latest 64 KiB / 400 lines").arg(m_sessions.isRunning(id) ? tr("Live") : tr("Finished")));
}
