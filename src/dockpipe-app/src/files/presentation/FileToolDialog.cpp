#include "FileToolDialog.h"
#include "ContextStore.h"
#include "MarkdownPreview.h"

#include <QDialogButtonBox>
#include <QFileInfo>
#include <QHBoxLayout>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QScreen>
#include <QTimer>
#include <QToolButton>
#include <QVBoxLayout>

namespace {
const QByteArray resultPrefix("::dockpipe-file::");
constexpr int maximumEventBytes = 1024 * 1024;
constexpr int maximumOutputBytes = 2 * maximumEventBytes;
}

FileToolDialog::FileToolDialog(const Context &context, const QString &title,
                               const QString &provider, const QString &file, QWidget *parent)
    : QDialog(parent), m_context(context), m_sessions(this)
{
    setObjectName("fileToolDialog");
    setWindowTitle(title);
    setAttribute(Qt::WA_DeleteOnClose);
    setWindowModality(Qt::WindowModal);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(24, 20, 24, 20);
    layout->setSpacing(12);
    auto *subtitle = new QLabel(tr("%1 · %2").arg(QFileInfo(file).fileName(), provider));
    subtitle->setObjectName("filesPageSubtitle");
    subtitle->setTextFormat(Qt::PlainText);
    subtitle->setWordWrap(true);
    layout->addWidget(subtitle);
    m_status = new QLabel(tr("Running file tool…"));
    m_status->setObjectName("fileToolStatus");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    layout->addWidget(m_status);
    m_preview = new MarkdownPreview;
    m_preview->setAccessibleName(tr("File tool result"));
    layout->addWidget(m_preview, 1);
    auto *detailsToggle = new QToolButton;
    detailsToggle->setObjectName("fileToolDetailsToggle");
    detailsToggle->setText(tr("Details"));
    detailsToggle->setToolButtonStyle(Qt::ToolButtonTextBesideIcon);
    detailsToggle->setArrowType(Qt::RightArrow);
    detailsToggle->setCheckable(true);
    m_details = new QPlainTextEdit;
    m_details->setObjectName("fileToolDetails");
    m_details->setReadOnly(true);
    m_details->setMaximumBlockCount(100);
    m_details->setMaximumHeight(140);
    m_details->hide();
    connect(detailsToggle, &QToolButton::toggled, m_details, &QWidget::setVisible);
    connect(detailsToggle, &QToolButton::toggled, this, [detailsToggle](bool checked) {
        detailsToggle->setArrowType(checked ? Qt::DownArrow : Qt::RightArrow);
    });
    layout->addWidget(m_details);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Close);
    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
    auto *footer = new QHBoxLayout;
    footer->addWidget(detailsToggle);
    footer->addStretch();
    footer->addWidget(buttons);
    layout->addLayout(footer);
    const QSize available = screen()->availableGeometry().size() - QSize(40, 60);
    resize(QSize(760, 600).boundedTo(available));
    m_deadline = new QTimer(this);
    m_deadline->setSingleShot(true);
    connect(m_deadline, &QTimer::timeout, this, [this]() {
        fail(tr("The file tool exceeded its 30-second deadline."));
    });
    connect(&m_sessions, &SessionManager::sessionOutput, this, [this](const QString &, const QString &text) {
        receive(text);
    });
    connect(&m_sessions, &SessionManager::sessionFailed, this, [this](const QString &, const QString &error) {
        fail(error);
    });
    connect(&m_sessions, &SessionManager::sessionStopped, this,
        [this](const QString &, int exitCode, QProcess::ExitStatus status) { finish(exitCode, status); });
    connect(&m_sessions, &SessionManager::sessionPrompt, this, [this]() {
        fail(tr("File tools must finish without interactive prompts. Run this workflow from Workflows if it needs input."));
    });
    QTimer::singleShot(0, this, [this]() {
        if (m_finished)
            return;
        m_deadline->start(30000);
        if (!m_sessions.launch(m_context, ContextStore::logsDir()))
            fail(tr("Could not start the file tool."));
    });
}

FileToolDialog::~FileToolDialog()
{
    m_finished = true;
    m_sessions.stop(m_context.id);
}

void FileToolDialog::done(int result)
{
    m_finished = true;
    m_deadline->stop();
    if (m_sessions.isRunning(m_context.id)) {
        if (property("fileToolClosePending").toBool())
            return;
        setProperty("fileToolClosePending", true);
        // The member SessionManager must outlive its asynchronous stop/reap.
        // Keep the hidden dialog alive through the existing process kill deadline.
        setAttribute(Qt::WA_DeleteOnClose, false);
        connect(&m_sessions, &SessionManager::sessionStopped, this,
            [this, result](const QString &contextId, int, QProcess::ExitStatus) {
            if (contextId != m_context.id)
                return;
            setProperty("fileToolClosePending", false);
            QDialog::done(result);
            deleteLater();
        });
        hide();
        m_sessions.stop(m_context.id);
        return;
    }
    m_sessions.stop(m_context.id);
    QDialog::done(result);
}

void FileToolDialog::receive(const QString &text)
{
    if (m_finished)
        return;
    const QByteArray bytes = text.toUtf8();
    if (bytes.size() > maximumOutputBytes - m_received) {
        fail(tr("The file tool exceeded the 2 MiB output limit."));
        return;
    }
    m_received += bytes.size();
    m_pending += bytes;
    while (m_pending.contains('\n')) {
        const int newline = m_pending.indexOf('\n');
        const QByteArray line = m_pending.left(newline);
        m_pending.remove(0, newline + 1);
        if (line.size() > maximumEventBytes) {
            fail(tr("The file tool exceeded the 1 MiB line limit."));
            return;
        }
        if (!line.startsWith(resultPrefix)) {
            m_details->appendPlainText(QString::fromUtf8(line).left(4096));
            continue;
        }
        QJsonParseError error;
        const auto document = QJsonDocument::fromJson(line.mid(resultPrefix.size()), &error);
        const auto value = document.object();
        if (error.error != QJsonParseError::NoError || !document.isObject()
            || value.value("schema").toDouble() != 1 || value.value("kind").toString() != "markdown"
            || !value.value("text").isString() || m_hasResult) {
            fail(tr("The file tool returned an invalid or duplicate result."));
            return;
        }
        m_result = value.value("text").toString();
        m_hasResult = true;
    }
    if (m_pending.size() > maximumEventBytes)
        fail(tr("The file tool exceeded the 1 MiB line limit."));
}

void FileToolDialog::fail(const QString &message)
{
    if (m_finished)
        return;
    m_finished = true;
    m_deadline->stop();
    m_status->setText(message);
    m_status->setProperty("failed", true);
    m_sessions.stop(m_context.id);
}

void FileToolDialog::finish(int exitCode, QProcess::ExitStatus status)
{
    if (m_finished)
        return;
    if (!m_pending.isEmpty())
        receive("\n");
    if (m_finished)
        return;
    if (exitCode != 0 || status != QProcess::NormalExit) {
        fail(tr("The file tool failed. Open Details for its output."));
        return;
    }
    if (!m_hasResult) {
        fail(tr("The file tool finished without a result."));
        return;
    }
    m_finished = true;
    m_deadline->stop();
    m_preview->render(m_result);
    m_status->setText(tr("Report complete · saved file"));
}
