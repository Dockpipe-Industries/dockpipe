#include "WebMermaidRenderer.h"

#include <QApplication>
#include <QDir>
#include <QSignalSpy>
#include <QTest>
#include <QWebEnginePage>
#include <QWebEngineProfile>

#ifndef DOCKPIPE_MERMAID_BACKEND_ONLY
#include "ProjectFilesWidget.h"
#include "Theme.h"
#include <QFile>
#include <QPlainTextEdit>
#include <QTemporaryDir>
#include <QTextBlock>
#include <QTextBrowser>
#include <QTextFragment>
#include <QTextImageFormat>
#include <QToolButton>
#endif

class WebMermaidRendererTest final : public QObject {
    Q_OBJECT
private slots:
    void diagrams_data()
    {
        QTest::addColumn<QString>("source");
        QTest::addColumn<bool>("dark");
        QTest::newRow("flowchart-light") << QStringLiteral("flowchart LR\n A[Start] --> B[Finish]") << false;
        QTest::newRow("flowchart-dark") << QStringLiteral("flowchart TD\n A --> B --> C") << true;
        // Self-loop layout uses structuredClone in the pinned Mermaid runtime.
        QTest::newRow("self-loop") << QStringLiteral("flowchart LR\n A[Loop] --> A\n A --> B") << false;
        QTest::newRow("sequence") << QStringLiteral("sequenceDiagram\n Alice->>Bob: Hello\n Bob-->>Alice: Hi") << false;
        QTest::newRow("inert-link") << QStringLiteral("flowchart LR\n A --> B\n click A href \"https://127.0.0.1:9/forbidden\"") << false;
    }

    void diagrams()
    {
        QFETCH(QString, source);
        QFETCH(bool, dark);
        WebMermaidRenderer renderer;
        QSignalSpy completed(&renderer, &MermaidRenderer::finished);
        renderer.render(source, dark);
        QTRY_COMPARE_WITH_TIMEOUT(completed.size(), 1, 15000);
        const auto result = completed.takeFirst();
        QVERIFY2(result.at(1).toString().isEmpty(), qPrintable(result.at(1).toString()));
        const QImage image = qvariant_cast<QImage>(result.at(0));
        QVERIFY(!image.isNull());
        QVERIFY(image.width() > 30);
        QVERIFY(image.height() > 30);
        QVERIFY(qint64(image.width()) * image.height() <= 4000000);
        if (QString::fromLatin1(QTest::currentDataTag()) == QStringLiteral("flowchart-light")) {
            // A nonempty image alone misses labels accidentally stripped with
            // foreignObject HTML. Check the first node's interior for ink.
            int labelPixels = 0;
            for (int y = image.height() / 3; y < image.height() * 2 / 3; ++y) {
                for (int x = image.width() / 10; x < image.width() * 4 / 10; ++x) {
                    if (qGray(image.pixel(x, y)) < 120)
                        ++labelPixels;
                }
            }
            QVERIFY2(labelPixels > 30, "Flowchart node labels are missing from the rendered image");
        }
        const QString imageDirectory = qEnvironmentVariable("DOCKPIPE_MERMAID_TEST_IMAGES");
        if (!imageDirectory.isEmpty()) {
            QVERIFY(QDir().mkpath(imageDirectory));
            QVERIFY(image.save(QDir(imageDirectory).filePath(
                QString::fromLatin1(QTest::currentDataTag()) + QStringLiteral(".png"))));
        }
    }

    void protectedEdgeLimit()
    {
        WebMermaidRenderer renderer;
        QSignalSpy completed(&renderer, &MermaidRenderer::finished);
        QString diagram = QStringLiteral("%%{init: {'maxEdges': 10000}}%%\nflowchart LR\n");
        for (int index = 0; index < 501; ++index)
            diagram += QStringLiteral("A --> B\n");
        renderer.render(diagram, false);
        QTRY_COMPARE_WITH_TIMEOUT(completed.size(), 1, 15000);
        QVERIFY(qvariant_cast<QImage>(completed.first().at(0)).isNull());
        QVERIFY2(completed.first().at(1).toString().contains(QStringLiteral("edges"), Qt::CaseInsensitive),
                 qPrintable(completed.first().at(1).toString()));
    }

    void invalidAndOversized()
    {
        WebMermaidRenderer renderer;
        QSignalSpy completed(&renderer, &MermaidRenderer::finished);
        renderer.render(QStringLiteral("not a Mermaid diagram"), false);
        QTRY_COMPARE_WITH_TIMEOUT(completed.size(), 1, 15000);
        QVERIFY(qvariant_cast<QImage>(completed.first().at(0)).isNull());
        QVERIFY(!completed.takeFirst().at(1).toString().isEmpty());
        renderer.render(QString(50001, QLatin1Char('a')), false);
        QCOMPARE(completed.size(), 1);
        QVERIFY(completed.first().at(1).toString().contains(QStringLiteral("50,000")));
    }

    void cancellationAndReplacement()
    {
        WebMermaidRenderer renderer;
        QSignalSpy completed(&renderer, &MermaidRenderer::finished);
        renderer.render(QStringLiteral("flowchart LR\n Old --> Cancelled"), false);
        renderer.cancel();
        QTest::qWait(100);
        QCOMPARE(completed.size(), 0);
        renderer.render(QStringLiteral("invalid stale input"), false);
        renderer.render(QStringLiteral("flowchart LR\n Current --> Result"), true);
        QTRY_COMPARE_WITH_TIMEOUT(completed.size(), 1, 15000);
        QVERIFY2(completed.first().at(1).toString().isEmpty(),
                 qPrintable(completed.first().at(1).toString()));
        QTest::qWait(100);
        QCOMPARE(completed.size(), 1);
    }

    void boundedReplacementAndDestruction()
    {
        auto *renderer = new WebMermaidRenderer;
        for (int index = 0; index < 10; ++index) {
            renderer->render(QStringLiteral("flowchart LR\n A --> B"), false);
            QVERIFY(renderer->findChildren<QWebEnginePage *>().size() <= 2);
        }
        delete renderer;
        QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
        QTest::qWait(100);
    }

    void completionAllowsImmediateReplacement()
    {
        WebMermaidRenderer renderer;
        QSignalSpy completed(&renderer, &MermaidRenderer::finished);
        connect(&renderer, &MermaidRenderer::finished, &renderer,
                [&renderer, &completed](const QImage &, const QString &) {
            renderer.cancel();
            if (completed.size() == 1)
                renderer.render(QStringLiteral("flowchart LR\n Next --> Result"), true);
        });
        renderer.render(QStringLiteral("flowchart LR\n First --> Result"), false);
        QTRY_COMPARE_WITH_TIMEOUT(completed.size(), 2, 20000);
        for (const auto &result : completed)
            QVERIFY2(result.at(1).toString().isEmpty(), qPrintable(result.at(1).toString()));
    }

#ifndef DOCKPIPE_MERMAID_BACKEND_ONLY
    void filesPreviewUsesRealRenderer()
    {
        QTemporaryDir directory;
        ProjectFilesWidget files;
        files.resize(1280, 780);
        files.show();
        QVERIFY(files.setRoot(directory.path()));
        QVERIFY(files.createFile(QStringLiteral("README.md")));
        auto *editor = files.findChild<QPlainTextEdit *>(QStringLiteral("fileTextEditor"));
        auto *toggle = files.findChild<QToolButton *>(QStringLiteral("fileMarkdownPreviewButton"));
        auto *preview = files.findChild<QTextBrowser *>(QStringLiteral("fileMarkdownPreview"));
        QVERIFY(editor && toggle && preview);
        const QString draft = QStringLiteral("# Live diagram\n\n```mermaid\nflowchart LR\n A[Draft] --> B[Preview]\n```\n");
        editor->selectAll();
        editor->insertPlainText(draft);
        toggle->click();
        const auto containsDiagram = [preview] {
            for (auto block = preview->document()->begin(); block.isValid(); block = block.next()) {
                for (auto fragment = block.begin(); !fragment.atEnd(); ++fragment) {
                    const auto format = fragment.fragment().charFormat();
                    if (!format.isImageFormat())
                        continue;
                    const QUrl resource(format.toImageFormat().name());
                    if (qvariant_cast<QImage>(preview->document()->resource(QTextDocument::ImageResource, resource)).isNull())
                        continue;
                    return true;
                }
            }
            return false;
        };
        QTRY_VERIFY_WITH_TIMEOUT(containsDiagram(), 15000);
        QCOMPARE(editor->toPlainText(), draft);
        QVERIFY(files.hasUnsavedChanges());
        QFile disk(directory.filePath(QStringLiteral("README.md")));
        QVERIFY(disk.open(QIODevice::ReadOnly));
        QVERIFY(disk.readAll().isEmpty());
        const QString imageDirectory = qEnvironmentVariable("DOCKPIPE_MERMAID_TEST_IMAGES");
        if (!imageDirectory.isEmpty()) {
            QVERIFY(QDir().mkpath(imageDirectory));
            QVERIFY(files.grab().save(QDir(imageDirectory).filePath(QStringLiteral("files-mermaid.png"))));
        }
        // Close without prompting over the intentionally unsaved draft.
        editor->document()->setModified(false);
    }
#endif

    void isolatedOfflinePage()
    {
        WebMermaidRenderer renderer;
        QSignalSpy completed(&renderer, &MermaidRenderer::finished);
        renderer.render(QStringLiteral("flowchart LR\n A --> B"), false);
        const auto profiles = renderer.findChildren<QWebEngineProfile *>();
        QCOMPARE(profiles.size(), 1);
        QVERIFY(profiles.first()->isOffTheRecord());
        QCOMPARE(profiles.first()->persistentCookiesPolicy(), QWebEngineProfile::NoPersistentCookies);
        auto *page = profiles.first()->findChild<QWebEnginePage *>();
        QVERIFY(page);
        bool blocked = false;
        // Exercise the real page's CSP and request boundary, without a server.
        // A new page has not finished loading yet. Run the probe once its
        // bundled document is ready, before the diagram render completes.
        connect(page, &QWebEnginePage::loadFinished, &renderer, [page, &blocked](bool loaded) {
            if (!loaded)
                return;
            page->runJavaScript(QStringLiteral(
                "window.offlineProbe = 'pending'; fetch('https://127.0.0.1:9/forbidden')"
                ".then(() => window.offlineProbe = 'allowed').catch(() => window.offlineProbe = 'blocked');"));
            QTimer::singleShot(20, page, [page, &blocked] {
                page->runJavaScript(QStringLiteral("window.offlineProbe"), [&blocked](const QVariant &value) {
                    blocked = value.toString() == QStringLiteral("blocked");
                });
            });
        });
        QTRY_VERIFY_WITH_TIMEOUT(blocked, 10000);
        QTRY_COMPARE_WITH_TIMEOUT(completed.size(), 1, 15000);
        QVERIFY2(completed.first().at(1).toString().isEmpty(),
                 qPrintable(completed.first().at(1).toString()));
    }
};

int main(int argc, char **argv)
{
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    QApplication application(argc, argv);
#ifndef DOCKPIPE_MERMAID_BACKEND_ONLY
    applyDockpipeLauncherTheme(application);
#endif
    installWebMermaidRenderer();
    WebMermaidRendererTest tests;
    return QTest::qExec(&tests, argc, argv);
}
#include "WebMermaidRendererTest.moc"
