#include "ProjectFilesWidget.h"
#include "Theme.h"
#include "MermaidRenderer.h"

#include <QApplication>
#include <QFile>
#include <QDir>
#include <QPlainTextEdit>
#include <QTemporaryDir>
#include <QTest>
#include <QTextBlock>
#include <QTextFragment>
#include <QTextBrowser>
#include <QToolButton>
#include <QStyle>
#include <QTimer>

namespace {
class RecordingMermaidRenderer final : public MermaidRenderer {
public:
    RecordingMermaidRenderer(QStringList *sources, QObject *parent, int delayMs = 0)
        : MermaidRenderer(parent), m_sources(sources), m_delayMs(delayMs) {}

    void render(const QString &source, bool) override
    {
        m_sources->append(source);
        const int generation = ++m_generation;
        QTimer::singleShot(m_delayMs, this, [this, source, generation]() {
            if (generation != m_generation)
                return;
            if (source.contains("invalid")) {
                emit finished({}, "Invalid Mermaid syntax");
                return;
            }
            QImage diagram(240, 100, QImage::Format_ARGB32);
            diagram.fill(Qt::darkCyan);
            emit finished(diagram, {});
        });
    }

    void cancel() override { ++m_generation; }

private:
    QStringList *m_sources;
    int m_generation = 0;
    int m_delayMs;
};

int diagramImages(const QTextDocument *document)
{
    int count = 0;
    for (auto block = document->begin(); block.isValid(); block = block.next()) {
        for (auto it = block.begin(); !it.atEnd(); ++it) {
            if (it.fragment().charFormat().isImageFormat())
                ++count;
        }
    }
    return count;
}
}

class ProjectFilesMarkdownTest final : public QObject {
    Q_OBJECT
private slots:
    void previewRendersDraftWithoutSaving_data();
    void previewRendersDraftWithoutSaving();
    void switchingFilesAndRootsClearsPreview();
    void repeatedSavesReleaseConflictCheckHandle();
    void previewDoesNotLoadResourcesOrFollowUnsafeLinks();
    void oversizedDraftHasVisibleLimit();
    void mermaidFencesRenderWithoutChangingDraft();
    void mermaidErrorsAndDocumentLimitStayVisible();
    void mermaidRenderingResumesAfterPageHide();
    void cleanup() { MermaidRenderer::setFactory({}); }
};

void ProjectFilesMarkdownTest::previewRendersDraftWithoutSaving_data()
{
    QTest::addColumn<bool>("dark");
    QTest::newRow("dark") << true;
    QTest::newRow("light") << false;
}

void ProjectFilesMarkdownTest::previewRendersDraftWithoutSaving()
{
    QFETCH(bool, dark);
    applyDockpipeLauncherTheme(*qApp);
    QFile shared(":/theme/pipeon.qss");
    QFile companion(dark ? ":/theme/pipeon-dark.qss" : ":/theme/pipeon-light.qss");
    QVERIFY(shared.open(QIODevice::ReadOnly));
    QVERIFY(companion.open(QIODevice::ReadOnly));
    qApp->setStyleSheet(QString());
    qApp->setPalette(qApp->style()->standardPalette());
    qApp->setStyleSheet(QString::fromUtf8(shared.readAll() + companion.readAll()));
    QTemporaryDir root;
    ProjectFilesWidget files;
    files.resize(1280, 780);
    files.show();
    QVERIFY(files.setRoot(root.path()));
    QVERIFY(files.createFile("README.md"));
    auto *editor = files.findChild<QPlainTextEdit *>("fileTextEditor");
    auto *toggle = files.findChild<QToolButton *>("fileMarkdownPreviewButton");
    auto *preview = files.findChild<QTextBrowser *>("fileMarkdownPreview");
    QVERIFY(editor && toggle && preview);
    QVERIFY(!toggle->isHidden());
    QVERIFY(preview->isHidden());
    editor->selectAll();
    editor->insertPlainText("# Workspace\n\nA **bold** draft.\n\n- One\n- Two\n\n```cpp\nint main() {}\n```\n\n| Key | Value |\n| --- | --- |\n| Example | Ready |\n");
    QTest::mouseClick(toggle, Qt::LeftButton);
    QTRY_VERIFY(preview->isVisible());
    QTRY_VERIFY(preview->toPlainText().contains("A bold draft."));
    QCOMPARE(preview->document()->begin().blockFormat().headingLevel(), 1);
    QVERIFY(preview->document()->toHtml().contains("<table"));
    QCOMPARE(preview->palette().color(QPalette::Base).lightness() < 128, dark);
    QVERIFY(qAbs(preview->palette().color(QPalette::Base).lightness()
        - preview->palette().color(QPalette::Text).lightness()) > 100);
    QVERIFY(!editor->isHidden());
    QVERIFY(files.hasUnsavedChanges());
    QFile disk(root.filePath("README.md"));
    QVERIFY(disk.open(QIODevice::ReadOnly));
    QVERIFY(disk.readAll().isEmpty());
    disk.close();
    editor->appendPlainText("Live update");
    QTRY_VERIFY(preview->toPlainText().contains("Live update"));
    QVERIFY(files.saveFile());
    QVERIFY(!files.hasUnsavedChanges());
    QVERIFY(disk.open(QIODevice::ReadOnly));
    QVERIFY(disk.readAll().contains("Live update"));
    const QString images = qEnvironmentVariable("DOCKPIPE_MARKDOWN_TEST_IMAGES");
    if (!images.isEmpty()) {
        QVERIFY(QDir().mkpath(images));
        QVERIFY(files.grab().save(QDir(images).filePath(dark ? "markdown-dark.png" : "markdown-light.png")));
    }
    QTest::mouseClick(toggle, Qt::LeftButton);
    QVERIFY(preview->isHidden());
    QVERIFY(editor->toPlainText().contains("Live update"));
}

void ProjectFilesMarkdownTest::repeatedSavesReleaseConflictCheckHandle()
{
    QTemporaryDir root;
    ProjectFilesWidget files;
    QVERIFY(files.setRoot(root.path()));
    QVERIFY(files.createFile("notes.md"));
    auto *editor = files.findChild<QPlainTextEdit *>("fileTextEditor");
    for (const QString &text : {QString("First draft"), QString("Second draft")}) {
        editor->setPlainText(text);
        editor->document()->setModified(true);
        QVERIFY(files.saveFile());
        QVERIFY(!files.hasUnsavedChanges());
        QFile disk(root.filePath("notes.md"));
        QVERIFY(disk.open(QIODevice::ReadOnly));
        QCOMPARE(disk.readAll(), text.toUtf8());
    }
}

void ProjectFilesMarkdownTest::switchingFilesAndRootsClearsPreview()
{
    QTemporaryDir root;
    QTemporaryDir other;
    ProjectFilesWidget files;
    files.show();
    QVERIFY(files.setRoot(root.path()));
    QVERIFY(files.createFile("notes.MARKDOWN"));
    auto *toggle = files.findChild<QToolButton *>("fileMarkdownPreviewButton");
    auto *preview = files.findChild<QTextBrowser *>("fileMarkdownPreview");
    toggle->click();
    QVERIFY(preview->isVisible());
    QVERIFY(files.createFile("script.py"));
    QVERIFY(toggle->isHidden());
    QVERIFY(!toggle->isChecked());
    QVERIFY(preview->isHidden());
    QVERIFY(files.openFile(root.filePath("notes.MARKDOWN")));
    toggle->click();
    QVERIFY(files.setRoot(other.path()));
    QVERIFY(!toggle->isChecked());
    QVERIFY(toggle->isHidden());
    QVERIFY(preview->isHidden());
}

void ProjectFilesMarkdownTest::previewDoesNotLoadResourcesOrFollowUnsafeLinks()
{
    QTemporaryDir root;
    QTemporaryDir outside;
    QFile secret(outside.filePath("private.txt"));
    QVERIFY(secret.open(QIODevice::WriteOnly));
    secret.write("Outside content");
    secret.close();
    ProjectFilesWidget files;
    files.show();
    QVERIFY(files.setRoot(root.path()));
    QVERIFY(files.createFile("README.md"));
    auto *editor = files.findChild<QPlainTextEdit *>("fileTextEditor");
    auto *toggle = files.findChild<QToolButton *>("fileMarkdownPreviewButton");
    auto *preview = files.findChild<QTextBrowser *>("fileMarkdownPreview");
    editor->selectAll();
    editor->insertPlainText("# Safe\n\n<img src=\"file:///private.png\">\n\n![Remote](https://invalid.example/image.png)");
    toggle->click();
    QTRY_VERIFY(preview->toPlainText().contains("Safe"));
    QVERIFY(!preview->document()->resource(QTextDocument::ImageResource, QUrl::fromLocalFile(secret.fileName())).isValid());
    QVERIFY(!preview->document()->resource(QTextDocument::ImageResource, QUrl("https://invalid.example/image.png")).isValid());
    const QString originalPath = files.currentPath();
    emit preview->anchorClicked(QUrl::fromLocalFile(secret.fileName()));
    QCOMPARE(files.currentPath(), originalPath);
    emit preview->anchorClicked(QUrl("javascript:alert(1)"));
    QCOMPARE(files.currentPath(), originalPath);
    QVERIFY(files.hasUnsavedChanges());
    editor->document()->setModified(false);
    QVERIFY(files.createFile("other.md"));
    QVERIFY(files.openFile(originalPath));
    toggle->click();
    emit preview->anchorClicked(QUrl("other.md"));
    QCOMPARE(files.currentPath(), root.filePath("other.md"));
}

void ProjectFilesMarkdownTest::oversizedDraftHasVisibleLimit()
{
    QTemporaryDir root;
    ProjectFilesWidget files;
    files.show();
    QVERIFY(files.setRoot(root.path()));
    QVERIFY(files.createFile("large.md"));
    auto *editor = files.findChild<QPlainTextEdit *>("fileTextEditor");
    auto *toggle = files.findChild<QToolButton *>("fileMarkdownPreviewButton");
    auto *preview = files.findChild<QTextBrowser *>("fileMarkdownPreview");
    editor->selectAll();
    editor->insertPlainText(QString(1024 * 1024 + 1, QChar('a')));
    toggle->click();
    QTRY_VERIFY(preview->toPlainText().contains("1 MiB"));
    editor->selectAll();
    editor->insertPlainText("# Recovered");
    QTRY_COMPARE(preview->toPlainText(), QString("Recovered"));
    QVERIFY(files.hasUnsavedChanges());
}

void ProjectFilesMarkdownTest::mermaidFencesRenderWithoutChangingDraft()
{
    QStringList sources;
    MermaidRenderer::setFactory([&sources](QObject *parent) {
        return new RecordingMermaidRenderer(&sources, parent);
    });
    QTemporaryDir root;
    ProjectFilesWidget files;
    files.show();
    QVERIFY(files.setRoot(root.path()));
    QVERIFY(files.createFile("diagrams.md"));
    auto *editor = files.findChild<QPlainTextEdit *>("fileTextEditor");
    auto *toggle = files.findChild<QToolButton *>("fileMarkdownPreviewButton");
    auto *preview = files.findChild<QTextBrowser *>("fileMarkdownPreview");
    const QString draft = "# Architecture\n\n```mermaid\nflowchart TD\nA-->B\n```\n\n"
        "~~~mermaid\nsequenceDiagram\nAlice->>Bob: Hello\n~~~\n\n"
        "````text\n```mermaid\nNot a diagram\n```\n````\n\n"
        "```mermaid\nAn unfinished diagram\n";
    editor->insertPlainText(draft);
    toggle->click();
    QTRY_COMPARE(diagramImages(preview->document()), 2);
    QCOMPARE(sources, QStringList({"flowchart TD\nA-->B", "sequenceDiagram\nAlice->>Bob: Hello"}));
    QVERIFY(preview->toPlainText().contains("Not a diagram"));
    QVERIFY(preview->toPlainText().contains("An unfinished diagram"));
    QVERIFY(!preview->toPlainText().contains("dockpipe-mermaid-"));
    QCOMPARE(editor->toPlainText(), draft);
    QVERIFY(files.hasUnsavedChanges());
    QVERIFY(files.saveFile());
    QFile disk(root.filePath("diagrams.md"));
    QVERIFY(disk.open(QIODevice::ReadOnly));
    QCOMPARE(disk.readAll(), draft.toUtf8());
}

void ProjectFilesMarkdownTest::mermaidErrorsAndDocumentLimitStayVisible()
{
    QStringList sources;
    MermaidRenderer::setFactory([&sources](QObject *parent) {
        return new RecordingMermaidRenderer(&sources, parent);
    });
    QTemporaryDir root;
    ProjectFilesWidget files;
    files.show();
    QVERIFY(files.setRoot(root.path()));
    QVERIFY(files.createFile("many.md"));
    auto *editor = files.findChild<QPlainTextEdit *>("fileTextEditor");
    auto *toggle = files.findChild<QToolButton *>("fileMarkdownPreviewButton");
    auto *preview = files.findChild<QTextBrowser *>("fileMarkdownPreview");
    editor->insertPlainText("```mermaid\ninvalid\n```\n");
    toggle->click();
    QTRY_VERIFY(preview->toPlainText().contains("Invalid Mermaid syntax"));
    QVERIFY(preview->toPlainText().contains("invalid"));
    sources.clear();
    editor->selectAll();
    editor->insertPlainText(QString("```mermaid\nflowchart TD\nA-->B\n```\n\n").repeated(17));
    QTRY_COMPARE(diagramImages(preview->document()), 16);
    QCOMPARE(sources.size(), 16);
    QVERIFY(preview->toPlainText().contains("after the first 16"));
    QVERIFY(preview->toPlainText().contains("flowchart TD"));
    QVERIFY(files.hasUnsavedChanges());
}

void ProjectFilesMarkdownTest::mermaidRenderingResumesAfterPageHide()
{
    QStringList sources;
    MermaidRenderer::setFactory([&sources](QObject *parent) {
        return new RecordingMermaidRenderer(&sources, parent, 400);
    });
    QTemporaryDir root;
    ProjectFilesWidget files;
    files.show();
    QVERIFY(files.setRoot(root.path()));
    QVERIFY(files.createFile("pending.md"));
    auto *editor = files.findChild<QPlainTextEdit *>("fileTextEditor");
    auto *toggle = files.findChild<QToolButton *>("fileMarkdownPreviewButton");
    auto *preview = files.findChild<QTextBrowser *>("fileMarkdownPreview");
    editor->insertPlainText("```mermaid\nflowchart TD\nA-->B\n```\n");
    toggle->click();
    QTRY_COMPARE(sources.size(), 1);
    files.hide();
    files.show();
    QTRY_COMPARE(diagramImages(preview->document()), 1);
    QCOMPARE(sources.size(), 2);
    QVERIFY(files.hasUnsavedChanges());
}

QTEST_MAIN(ProjectFilesMarkdownTest)
#include "ProjectFilesMarkdownTest.moc"
