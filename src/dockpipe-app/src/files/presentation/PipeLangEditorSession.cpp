#include "PipeLangEditorSession.h"
#include "FileEditorWorkspace.h"
#include "SourceEditor.h"

#include <QDialog>
#include <QDialogButtonBox>
#include <QEvent>
#include <QFileDialog>
#include <QFileInfo>
#include <QFormLayout>
#include <QHelpEvent>
#include <QHBoxLayout>
#include <QJsonObject>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QTextBlock>
#include <QTextDocument>
#include <QToolButton>
#include <QToolTip>
#include <QVBoxLayout>

namespace {
// LSP positions and QTextCursor offsets are both UTF-16. Reject invalid ranges
// rather than clipping them onto a different source location.
int documentOffset(QTextDocument *document, const QJsonValue &position)
{
    const auto object = position.toObject();
    const auto line = object.value("line");
    const auto character = object.value("character");
    if (!line.isDouble() || !character.isDouble() || line.toDouble() < 0 || character.toDouble() < 0
        || line.toDouble() != line.toInt(-1) || character.toDouble() != character.toInt(-1))
        return -1;

    const auto block = document->findBlockByNumber(line.toInt());
    if (!block.isValid() || character.toInt() > block.text().size())
        return -1;

    return block.position() + character.toInt();
}

QString hoverText(const QJsonValue &value)
{
    if (value.isString())
        return value.toString();
    if (value.isObject())
        return value.toObject().value("value").toString();
    if (value.isArray()) {
        QStringList parts;
        for (const auto &part : value.toArray()) {
            if (part.isString())
                parts.append(part.toString());
            else if (part.isObject())
                parts.append(part.toObject().value("value").toString());
        }
        return parts.join(QStringLiteral("\n\n"));
    }
    return {};
}
}

PipeLangEditorSession::PipeLangEditorSession(FileEditorWorkspace *workspace)
    : QObject(workspace), m_workspace(workspace), m_client(this)
{
    setObjectName("pipeLangEditorSession");
    connect(workspace, &FileEditorWorkspace::documentsChanged, this, &PipeLangEditorSession::synchronize);
    connect(workspace, &FileEditorWorkspace::currentChanged, this, [this] {
        m_client.cancelHover();
        QToolTip::hideText();
        refreshEditors();
    });
    connect(&m_client, &PipeLangLanguageClient::statusChanged, this, [this](const QString &status) {
        if (m_status) {
            m_status->setText(status);
            m_status->setToolTip(status);
        }
    });
    connect(&m_client, &PipeLangLanguageClient::documentVersionChanged, this,
        [this](const QString &path, int version) {
            const auto binding = m_bindings.value(path);
            if (binding.document)
                binding.document->setProperty("pipeLangDocumentVersion", version);
            QToolTip::hideText();
        });
    connect(&m_client, &PipeLangLanguageClient::diagnostics,
        this, &PipeLangEditorSession::applyDiagnostics);
    connect(&m_client, &PipeLangLanguageClient::hoverResult,
        this, &PipeLangEditorSession::showHover);
}

PipeLangEditorSession::~PipeLangEditorSession()
{
    disconnect(&m_client, nullptr, this, nullptr);
}

QWidget *PipeLangEditorSession::createControls(QWidget *parent)
{
    auto *controls = new QWidget(parent);
    auto *layout = new QHBoxLayout(controls);
    layout->setContentsMargins(6, 2, 6, 2);
    auto *configureButton = new QToolButton(controls);
    configureButton->setObjectName("configurePipeLangServer");
    configureButton->setText(tr("PipeLang server…"));
    configureButton->setToolTip(tr("Connect an explicitly selected installed language server for .pipe diagnostics and hover"));
    connect(configureButton, &QToolButton::clicked, this, &PipeLangEditorSession::configure);
    auto *stopButton = new QToolButton(controls);
    stopButton->setText(tr("Stop"));
    stopButton->setObjectName("stopPipeLangServer");
    connect(stopButton, &QToolButton::clicked, &m_client, &PipeLangLanguageClient::stop);
    m_status = new QLabel(tr("PipeLang language server disconnected."), controls);
    m_status->setObjectName("pipeLangServerStatus");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    layout->addWidget(configureButton);
    layout->addWidget(stopButton);
    layout->addWidget(m_status, 1);
    return controls;
}

void PipeLangEditorSession::configure()
{
    QDialog dialog(m_workspace);
    dialog.setWindowTitle(tr("Connect PipeLang language server"));
    auto *layout = new QVBoxLayout(&dialog);
    auto *explanation = new QLabel(tr("Select trusted installed tools. Connecting runs the selected Node and server on this host. "
        "Paths are used only for this session; project files cannot configure executable paths. "
        "An optional source root authorizes compiler analysis of imports beneath that directory."), &dialog);
    explanation->setWordWrap(true);
    layout->addWidget(explanation);
    auto *form = new QFormLayout;
    layout->addLayout(form);
    auto addPath = [&](const QString &label, const QString &name, bool directory) {
        auto *row = new QWidget(&dialog);
        auto *rowLayout = new QHBoxLayout(row);
        rowLayout->setContentsMargins(0, 0, 0, 0);
        auto *edit = new QLineEdit(row);
        edit->setObjectName(name);
        edit->setMinimumWidth(360);
        auto *browse = new QPushButton(tr("Browse…"), row);
        rowLayout->addWidget(edit, 1);
        rowLayout->addWidget(browse);
        connect(browse, &QPushButton::clicked, &dialog, [&, edit, directory] {
            const QString path = directory ? QFileDialog::getExistingDirectory(&dialog, tr("Select source root"))
                : QFileDialog::getOpenFileName(&dialog, tr("Select installed tool"));
            if (!path.isEmpty())
                edit->setText(path);
        });
        form->addRow(label, row);
        return edit;
    };
    auto *node = addPath(tr("Node executable"), "pipeLangNodePath", false);
    auto *server = addPath(tr("Server JavaScript entrypoint"), "pipeLangServerPath", false);
    auto *compiler = addPath(tr("PipeLang compiler executable"), "pipeLangCompilerPath", false);
    auto *root = addPath(tr("Source root (optional)"), "pipeLangSourceRoot", true);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Ok | QDialogButtonBox::Cancel, &dialog);
    buttons->button(QDialogButtonBox::Ok)->setText(tr("Connect"));
    layout->addWidget(buttons);
    connect(buttons, &QDialogButtonBox::accepted, &dialog, &QDialog::accept);
    connect(buttons, &QDialogButtonBox::rejected, &dialog, &QDialog::reject);
    if (dialog.exec() != QDialog::Accepted)
        return;

    synchronize();
    m_client.start({node->text(), server->text(), compiler->text(), root->text()});
}

void PipeLangEditorSession::attachEditor(SourceEditor *editor)
{
    m_editors.append(editor);
    editor->viewport()->installEventFilter(this);
    connect(editor, &QPlainTextEdit::cursorPositionChanged, this, [this, editor] {
        if (m_hoverEditor == editor) {
            m_client.cancelHover();
            QToolTip::hideText();
        }
    });
}

void PipeLangEditorSession::synchronize()
{
    QHash<QString, QPointer<QTextDocument>> current;
    for (const auto &path : m_workspace->paths()) {
        const auto *record = m_workspace->document(path);
        if (record && !record->workflow && record->document && QFileInfo(path).suffix() == QStringLiteral("pipe"))
            current.insert(path, record->document);
    }
    const auto previousPaths = m_bindings.keys();
    for (const auto &path : previousPaths) {
        if (!current.contains(path) || current.value(path) != m_bindings.value(path).document) {
            m_client.closeDocument(path);
            m_bindings.remove(path);
        }
    }
    for (auto it = current.cbegin(); it != current.cend(); ++it) {
        auto &binding = m_bindings[it.key()];
        if (binding.document == it.value() && binding.revision == it.value()->revision())
            continue;
        binding.document = it.value();
        binding.revision = it.value()->revision();
        m_client.setDocument(it.key(), it.value()->toPlainText());
    }
}

void PipeLangEditorSession::refreshEditors()
{
    for (const auto &editor : m_editors) {
        if (editor)
            editor->refreshLanguageDiagnostics();
    }
}

void PipeLangEditorSession::applyDiagnostics(const QString &path, int version, const QJsonArray &items)
{
    const auto document = m_bindings.value(path).document;
    if (!document || document->property("pipeLangDocumentVersion").toInt() != version)
        return;

    QVariantList diagnostics;
    for (const auto &value : items) {
        const auto item = value.toObject();
        const auto range = item.value("range").toObject();
        const int start = documentOffset(document, range.value("start"));
        const int end = documentOffset(document, range.value("end"));
        if (start < 0 || end < start)
            continue;
        diagnostics.append(QVariantMap{{"start", start}, {"end", end},
            {"severity", item.value("severity").toInt(1)}, {"message", item.value("message").toString()}});
    }
    document->setProperty("pipeLangDiagnostics", diagnostics);
    refreshEditors();
}

bool PipeLangEditorSession::eventFilter(QObject *object, QEvent *event)
{
    if (event->type() == QEvent::Leave || event->type() == QEvent::MouseMove) {
        m_client.cancelHover();
        m_hoverEditor.clear();
        QToolTip::hideText();
    }
    if (event->type() != QEvent::ToolTip)
        return QObject::eventFilter(object, event);

    SourceEditor *editor = nullptr;
    for (const auto &candidate : m_editors) {
        if (candidate && candidate->viewport() == object)
            editor = candidate;
    }
    if (!editor)
        return false;
    QString path;
    for (auto it = m_bindings.cbegin(); it != m_bindings.cend(); ++it) {
        if (it->document == editor->document()) {
            path = it.key();
            break;
        }
    }
    if (path.isEmpty())
        return false;

    auto *help = static_cast<QHelpEvent *>(event);
    const auto cursor = editor->cursorForPosition(help->pos());
    m_hoverEditor = editor;
    m_hoverOffset = cursor.position();
    m_hoverPosition = help->globalPos();
    QStringList messages;
    for (const auto &value : editor->document()->property("pipeLangDiagnostics").toList()) {
        const auto item = value.toMap();
        if (m_hoverOffset >= item.value("start").toInt() && m_hoverOffset <= item.value("end").toInt())
            messages.append(item.value("message").toString());
    }
    if (!messages.isEmpty())
        QToolTip::showText(m_hoverPosition, messages.join('\n').toHtmlEscaped(), editor);
    m_client.hover(path, cursor.blockNumber(), cursor.positionInBlock());
    return true;
}

void PipeLangEditorSession::showHover(const QString &path, int version, const QJsonValue &contents)
{
    const auto document = m_bindings.value(path).document;
    if (!document || !m_hoverEditor || m_hoverEditor->document() != document
        || document->property("pipeLangDocumentVersion").toInt() != version)
        return;
    const auto text = hoverText(contents);
    if (!text.isEmpty())
        QToolTip::showText(m_hoverPosition, QStringLiteral("<pre>%1</pre>").arg(text.toHtmlEscaped()), m_hoverEditor);
}
