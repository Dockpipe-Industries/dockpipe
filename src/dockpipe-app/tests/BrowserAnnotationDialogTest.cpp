#include "BrowserAnnotationDialog.h"

#include <QApplication>
#include <QClipboard>
#include <QComboBox>
#include <QInputDialog>
#include <QPointer>
#include <QPushButton>
#include <QTimer>
#include <QtTest>

#include <algorithm>

class BrowserAnnotationDialogTest final : public QObject {
    Q_OBJECT
private slots:
    void preservesCaptureAndSupportsUndo();
    void mapsResizedHighDpiCanvasToImagePixels();
    void ignoresLetterboxClicks();
    void addsAndCancelsTextNotes();
    void survivesOwnerDestructionDuringNote();
    void survivesOwnerDestructionDuringSave();
};

void BrowserAnnotationDialogTest::preservesCaptureAndSupportsUndo()
{
    QImage capture(640, 480, QImage::Format_ARGB32_Premultiplied);
    capture.fill(Qt::white);
    BrowserAnnotationDialog dialog(capture, QUrl(QStringLiteral("https://example.test")));
    dialog.show();
    QCoreApplication::processEvents();
    auto *canvas = dialog.findChild<QWidget *>(QStringLiteral("browserAnnotationCanvas"));
    auto *undo = dialog.findChild<QPushButton *>(QStringLiteral("browserAnnotationUndo"));
    auto *copy = dialog.findChild<QPushButton *>(QStringLiteral("browserAnnotationCopy"));
    QVERIFY(canvas);
    QVERIFY(undo);
    QVERIFY(copy);
    QCOMPARE(dialog.renderedImage(), capture);
    QVERIFY(!undo->isEnabled());
    QTest::mouseClick(canvas, Qt::LeftButton, Qt::NoModifier, canvas->rect().center());
    QVERIFY(dialog.renderedImage() != capture);
    QCOMPARE(capture.pixelColor(320, 240), QColor(Qt::white));
    QVERIFY(undo->isEnabled());
    QTest::mouseClick(copy, Qt::LeftButton);
    QCOMPARE(QApplication::clipboard()->image(), dialog.renderedImage());
    QTest::mouseClick(undo, Qt::LeftButton);
    QCOMPARE(dialog.renderedImage(), capture);
    QVERIFY(!undo->isEnabled());
}

void BrowserAnnotationDialogTest::mapsResizedHighDpiCanvasToImagePixels()
{
    QImage capture(1200, 600, QImage::Format_ARGB32_Premultiplied);
    capture.fill(Qt::white);
    capture.setDevicePixelRatio(2);
    BrowserAnnotationDialog dialog(capture, {});
    dialog.resize(720, 600);
    dialog.show();
    QCoreApplication::processEvents();
    auto *canvas = dialog.findChild<QWidget *>(QStringLiteral("browserAnnotationCanvas"));
    auto *tool = dialog.findChild<QComboBox *>(QStringLiteral("browserAnnotationTool"));
    auto *clear = dialog.findChild<QPushButton *>(QStringLiteral("browserAnnotationClear"));
    QVERIFY(canvas);
    QVERIFY(tool);
    QVERIFY(clear);
    tool->setCurrentIndex(1);
    const qreal scale = std::min(qreal(canvas->width()) / capture.width(),
                                 qreal(canvas->height()) / capture.height());
    const QPointF origin((canvas->width() - capture.width() * scale) / 2,
                         (canvas->height() - capture.height() * scale) / 2);
    const QPoint start = (origin + QPointF(300, 150) * scale).toPoint();
    const QPoint end = (origin + QPointF(900, 450) * scale).toPoint();
    QTest::mousePress(canvas, Qt::LeftButton, Qt::NoModifier, start);
    QTest::mouseRelease(canvas, Qt::LeftButton, Qt::NoModifier, end);
    const QImage result = dialog.renderedImage();
    QCOMPARE(result.size(), capture.size());
    QCOMPARE(result.devicePixelRatio(), qreal(1));
    QVERIFY(result.pixelColor(600, 300) != QColor(Qt::white));
    QCOMPARE(result.pixelColor(100, 100), QColor(Qt::white));
    dialog.resize(1000, 750);
    QCoreApplication::processEvents();
    QCOMPARE(dialog.renderedImage(), result);
    QTest::mouseClick(clear, Qt::LeftButton);
    QCOMPARE(dialog.renderedImage().pixelColor(600, 300), QColor(Qt::white));
    QCOMPARE(capture.devicePixelRatio(), qreal(2));
}

void BrowserAnnotationDialogTest::ignoresLetterboxClicks()
{
    QImage capture(800, 100, QImage::Format_ARGB32_Premultiplied);
    capture.fill(Qt::white);
    BrowserAnnotationDialog dialog(capture, {});
    dialog.show();
    QCoreApplication::processEvents();
    auto *canvas = dialog.findChild<QWidget *>(QStringLiteral("browserAnnotationCanvas"));
    QVERIFY(canvas);
    QTest::mouseClick(canvas, Qt::LeftButton, Qt::NoModifier, QPoint(5, 5));
    QCOMPARE(dialog.renderedImage(), capture);
}

void BrowserAnnotationDialogTest::addsAndCancelsTextNotes()
{
    QImage capture(640, 480, QImage::Format_ARGB32_Premultiplied);
    capture.fill(Qt::white);
    BrowserAnnotationDialog dialog(capture, {});
    dialog.show();
    QCoreApplication::processEvents();
    auto *canvas = dialog.findChild<QWidget *>(QStringLiteral("browserAnnotationCanvas"));
    auto *tool = dialog.findChild<QComboBox *>(QStringLiteral("browserAnnotationTool"));
    QVERIFY(canvas);
    QVERIFY(tool);
    tool->setCurrentIndex(2);
    QTimer::singleShot(0, &dialog, [] {
        auto *input = qobject_cast<QInputDialog *>(QApplication::activeModalWidget());
        QVERIFY(input);
        input->reject();
    });
    QTest::mouseClick(canvas, Qt::LeftButton, Qt::NoModifier, canvas->rect().center());
    QCOMPARE(dialog.renderedImage(), capture);
    QTimer::singleShot(0, &dialog, [] {
        auto *input = qobject_cast<QInputDialog *>(QApplication::activeModalWidget());
        QVERIFY(input);
        input->setTextValue(QStringLiteral("Review this button"));
        input->accept();
    });
    QTest::mouseClick(canvas, Qt::LeftButton, Qt::NoModifier, canvas->rect().center());
    QVERIFY(dialog.renderedImage() != capture);
}

void BrowserAnnotationDialogTest::survivesOwnerDestructionDuringNote()
{
    QImage capture(640, 480, QImage::Format_ARGB32_Premultiplied);
    capture.fill(Qt::white);
    QPointer<BrowserAnnotationDialog> dialog = new BrowserAnnotationDialog(capture, {});
    dialog->show();
    QCoreApplication::processEvents();
    auto *canvas = dialog->findChild<QWidget *>(QStringLiteral("browserAnnotationCanvas"));
    auto *tool = dialog->findChild<QComboBox *>(QStringLiteral("browserAnnotationTool"));
    QVERIFY(canvas);
    QVERIFY(tool);
    tool->setCurrentIndex(2);
    QTimer::singleShot(0, [dialog] { delete dialog.data(); });
    QTest::mousePress(canvas, Qt::LeftButton, Qt::NoModifier, canvas->rect().center());
    QVERIFY(dialog.isNull());
}

void BrowserAnnotationDialogTest::survivesOwnerDestructionDuringSave()
{
    QImage capture(640, 480, QImage::Format_ARGB32_Premultiplied);
    capture.fill(Qt::white);
    QPointer<BrowserAnnotationDialog> dialog = new BrowserAnnotationDialog(capture, {});
    dialog->show();
    QCoreApplication::processEvents();
    auto *save = dialog->findChild<QPushButton *>(QStringLiteral("browserAnnotationSave"));
    QVERIFY(save);
    QTimer::singleShot(0, [dialog] { delete dialog.data(); });
    save->click();
    QVERIFY(dialog.isNull());
}

QTEST_MAIN(BrowserAnnotationDialogTest)
#include "BrowserAnnotationDialogTest.moc"
