#include "BrowserAnnotationDialog.h"

#include <QApplication>
#include <QClipboard>
#include <QColor>
#include <QComboBox>
#include <QDialogButtonBox>
#include <QFileDialog>
#include <QFontMetricsF>
#include <QHBoxLayout>
#include <QImageWriter>
#include <QInputDialog>
#include <QKeySequence>
#include <QLabel>
#include <QMessageBox>
#include <QMouseEvent>
#include <QPainter>
#include <QPainterPath>
#include <QPointer>
#include <QPushButton>
#include <QSaveFile>
#include <QVBoxLayout>
#include <QVector>

#include <algorithm>
#include <functional>

namespace {

constexpr int MaximumAnnotations = 256;
constexpr int MaximumStrokePoints = 4096;
constexpr int MaximumNoteCharacters = 1000;

enum class AnnotationTool { Pen, Highlight, Note };

struct Annotation {
    AnnotationTool tool = AnnotationTool::Pen;
    QColor color;
    QVector<QPointF> points;
    QRectF rectangle;
    QString text;
    qreal width = 3;
    int fontPixels = 15;
};

void paintAnnotation(QPainter &painter, const Annotation &annotation)
{
    painter.save();
    painter.setPen(QPen(annotation.color, annotation.width, Qt::SolidLine,
                        Qt::RoundCap, Qt::RoundJoin));
    if (annotation.tool == AnnotationTool::Pen) {
        if (annotation.points.size() == 1) {
            painter.drawPoint(annotation.points.first());
        } else if (!annotation.points.isEmpty()) {
            QPainterPath path(annotation.points.first());
            for (qsizetype index = 1; index < annotation.points.size(); ++index)
                path.lineTo(annotation.points.at(index));
            painter.drawPath(path);
        }
    } else if (annotation.tool == AnnotationTool::Highlight) {
        QColor fill = annotation.color;
        fill.setAlpha(65);
        painter.setBrush(fill);
        painter.drawRect(annotation.rectangle);
    } else {
        QColor background = annotation.color;
        background.setAlpha(235);
        painter.setBrush(background);
        painter.setPen(Qt::NoPen);
        painter.drawRoundedRect(annotation.rectangle, annotation.width * 2,
                                annotation.width * 2);
        const int brightness = qGray(annotation.color.rgb());
        painter.setPen(brightness > 140 ? QColor(20, 20, 20) : Qt::white);
        QFont font = painter.font();
        font.setPixelSize(annotation.fontPixels);
        painter.setFont(font);
        const qreal padding = annotation.width * 3;
        painter.drawText(annotation.rectangle.adjusted(padding, padding, -padding, -padding),
                         Qt::AlignLeft | Qt::AlignTop | Qt::TextWordWrap, annotation.text);
    }
    painter.restore();
}

} // namespace

class BrowserAnnotationCanvas final : public QWidget {
public:
    explicit BrowserAnnotationCanvas(const QImage &capture, QWidget *parent)
        : QWidget(parent), m_original(capture)
    {
        // Use physical image pixels consistently for rendering and export, including HiDPI grabs.
        m_original.setDevicePixelRatio(1);
        setObjectName(QStringLiteral("browserAnnotationCanvas"));
        setMinimumSize(320, 220);
        setCursor(Qt::CrossCursor);
        setMouseTracking(false);
        setAccessibleName(tr("Screenshot annotation canvas"));
    }

    AnnotationTool tool = AnnotationTool::Pen;
    QColor color = QColor(QStringLiteral("#ffd166"));
    std::function<void(bool)> changed;

    QImage renderedImage() const
    {
        if (m_original.isNull())
            return {};
        QImage result = m_original.convertToFormat(QImage::Format_ARGB32_Premultiplied);
        QPainter painter(&result);
        painter.setRenderHint(QPainter::Antialiasing);
        for (const Annotation &annotation : m_annotations)
            paintAnnotation(painter, annotation);
        return result;
    }

    void undo()
    {
        m_drawing = false;
        if (!m_annotations.isEmpty())
            m_annotations.removeLast();
        notifyChanged();
    }

    void clear()
    {
        m_drawing = false;
        m_annotations.clear();
        notifyChanged();
    }

protected:
    void paintEvent(QPaintEvent *) override
    {
        QPainter painter(this);
        painter.fillRect(rect(), palette().color(QPalette::Dark));
        if (m_original.isNull())
            return;
        const QRectF target = imageRect();
        painter.drawImage(target, m_original);
        painter.save();
        painter.translate(target.topLeft());
        painter.scale(imageScale(), imageScale());
        painter.setClipRect(QRectF(QPointF(), m_original.size()));
        painter.setRenderHint(QPainter::Antialiasing);
        for (const Annotation &annotation : m_annotations)
            paintAnnotation(painter, annotation);
        if (m_drawing)
            paintAnnotation(painter, m_current);
        painter.restore();
    }

    void mousePressEvent(QMouseEvent *event) override
    {
        if (event->button() != Qt::LeftButton || m_original.isNull()
            || !imageRect().contains(event->position()))
            return;
        if (m_annotations.size() >= MaximumAnnotations) {
            QMessageBox::information(this, tr("Annotation limit"),
                                     tr("Undo or clear an annotation before adding another."));
            return;
        }
        m_current = {};
        m_current.tool = tool;
        m_current.color = color;
        m_current.width = 3 / imageScale();
        m_start = imagePoint(event->position());
        if (tool == AnnotationTool::Note) {
            addNote();
            return;
        }
        m_current.points.append(m_start);
        m_current.rectangle = QRectF(m_start, m_start);
        m_drawing = true;
        update();
    }

    void mouseMoveEvent(QMouseEvent *event) override
    {
        if (!m_drawing)
            return;
        extendStroke(imagePoint(event->position()));
        update();
    }

    void mouseReleaseEvent(QMouseEvent *event) override
    {
        if (event->button() != Qt::LeftButton || !m_drawing)
            return;
        extendStroke(imagePoint(event->position()));
        m_annotations.append(m_current);
        m_drawing = false;
        notifyChanged();
    }

private:
    qreal imageScale() const
    {
        if (m_original.isNull())
            return 1;
        return std::min(qreal(width()) / m_original.width(),
                        qreal(height()) / m_original.height());
    }

    QRectF imageRect() const
    {
        const QSizeF size = QSizeF(m_original.size()) * imageScale();
        return QRectF(QPointF((width() - size.width()) / 2,
                             (height() - size.height()) / 2), size);
    }

    QPointF imagePoint(const QPointF &position) const
    {
        const QPointF point = (position - imageRect().topLeft()) / imageScale();
        return QPointF(std::clamp(point.x(), qreal(0), qreal(m_original.width() - 1)),
                       std::clamp(point.y(), qreal(0), qreal(m_original.height() - 1)));
    }

    void extendStroke(const QPointF &point)
    {
        if (m_current.tool == AnnotationTool::Highlight) {
            m_current.rectangle = QRectF(m_start, point).normalized();
            return;
        }
        if (!m_current.points.isEmpty() && m_current.points.last() == point)
            return;
        if (m_current.points.size() < MaximumStrokePoints) {
            m_current.points.append(point);
        } else {
            // Preserve the endpoint while bounding memory for a very long drag.
            m_current.points.last() = point;
        }
    }

    void addNote()
    {
        QPointer<BrowserAnnotationCanvas> owner(this);
        QPointer<QInputDialog> input = new QInputDialog(this);
        input->setWindowTitle(tr("Add note"));
        input->setLabelText(tr("Note text"));
        input->setInputMode(QInputDialog::TextInput);
        input->setOption(QInputDialog::UsePlainTextEditForTextInput);
        const int result = input->exec();
        if (!owner || !input)
            return;
        const QString text = input->textValue().trimmed();
        input->deleteLater();
        if (result != QDialog::Accepted || text.isEmpty())
            return;
        if (text.size() > MaximumNoteCharacters) {
            QMessageBox::information(this, tr("Note too long"),
                                     tr("Keep each note to 1,000 characters or fewer."));
            return;
        }
        m_current.text = text;
        m_current.fontPixels = std::max(1, qRound(15 / imageScale()));
        QFont font = this->font();
        font.setPixelSize(m_current.fontPixels);
        const qreal padding = m_current.width * 3;
        const qreal noteWidth = std::min(qreal(m_original.width()), 260 / imageScale());
        const QRectF bounds = QFontMetricsF(font).boundingRect(
            QRectF(0, 0, std::max(qreal(1), noteWidth - padding * 2), m_original.height()),
            Qt::TextWordWrap, text);
        const qreal noteHeight = bounds.height() + padding * 2;
        if (noteHeight > m_original.height()) {
            QMessageBox::information(this, tr("Note does not fit"),
                                     tr("Use a shorter note so all of its text fits on the screenshot."));
            return;
        }
        const QPointF origin(std::min(m_start.x(), m_original.width() - noteWidth),
                             std::min(m_start.y(), m_original.height() - noteHeight));
        m_current.rectangle = QRectF(origin, QSizeF(noteWidth, noteHeight));
        m_annotations.append(m_current);
        notifyChanged();
    }

    void notifyChanged()
    {
        update();
        if (changed)
            changed(!m_annotations.isEmpty());
    }

    QImage m_original;
    QVector<Annotation> m_annotations;
    Annotation m_current;
    QPointF m_start;
    bool m_drawing = false;
};

BrowserAnnotationDialog::BrowserAnnotationDialog(const QImage &capture, const QUrl &url,
                                                 QWidget *parent)
    : QDialog(parent)
{
    setObjectName(QStringLiteral("browserAnnotationDialog"));
    setWindowTitle(tr("Annotate screenshot"));
    resize(1100, 760);
    auto *layout = new QVBoxLayout(this);
    auto *tools = new QHBoxLayout;
    auto *tool = new QComboBox(this);
    tool->setObjectName(QStringLiteral("browserAnnotationTool"));
    tool->setAccessibleName(tr("Annotation tool"));
    tool->addItems({tr("Pen"), tr("Highlight rectangle"), tr("Text note")});
    tools->addWidget(tool);
    auto *colors = new QComboBox(this);
    colors->setObjectName(QStringLiteral("browserAnnotationColor"));
    colors->setAccessibleName(tr("Annotation color"));
    const QList<QPair<QString, QString>> choices = {
        {tr("Yellow"), QStringLiteral("#ffd166")},
        {tr("Red"), QStringLiteral("#ef476f")},
        {tr("Green"), QStringLiteral("#06d6a0")},
        {tr("Blue"), QStringLiteral("#118ab2")},
        {tr("White"), QStringLiteral("#ffffff")}
    };
    for (const auto &choice : choices) {
        colors->addItem(choice.first, QColor(choice.second));
        colors->setItemData(colors->count() - 1, QColor(choice.second), Qt::DecorationRole);
    }
    tools->addWidget(colors);
    tools->addStretch();
    auto *undo = new QPushButton(tr("Undo"), this);
    undo->setObjectName(QStringLiteral("browserAnnotationUndo"));
    undo->setShortcut(QKeySequence::Undo);
    undo->setEnabled(false);
    tools->addWidget(undo);
    auto *clear = new QPushButton(tr("Clear"), this);
    clear->setObjectName(QStringLiteral("browserAnnotationClear"));
    clear->setEnabled(false);
    tools->addWidget(clear);
    layout->addLayout(tools);
    m_canvas = new BrowserAnnotationCanvas(capture, this);
    layout->addWidget(m_canvas, 1);
    auto *hint = new QLabel(tr("Draw on the screenshot. Your edits stay here until you copy or save."), this);
    hint->setWordWrap(true);
    hint->setToolTip(url.toDisplayString(QUrl::RemoveUserInfo));
    layout->addWidget(hint);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Close, this);
    auto *copy = buttons->addButton(tr("Copy image"), QDialogButtonBox::ActionRole);
    copy->setObjectName(QStringLiteral("browserAnnotationCopy"));
    auto *save = buttons->addButton(tr("Save PNG…"), QDialogButtonBox::ActionRole);
    save->setObjectName(QStringLiteral("browserAnnotationSave"));
    copy->setEnabled(!capture.isNull());
    save->setEnabled(!capture.isNull());
    layout->addWidget(buttons);
    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
    connect(tool, &QComboBox::currentIndexChanged, this, [this](int index) {
        m_canvas->tool = static_cast<AnnotationTool>(index);
    });
    connect(colors, &QComboBox::currentIndexChanged, this, [this, colors] {
        m_canvas->color = colors->currentData().value<QColor>();
    });
    connect(undo, &QPushButton::clicked, this, [this] { m_canvas->undo(); });
    connect(clear, &QPushButton::clicked, this, [this] { m_canvas->clear(); });
    m_canvas->changed = [undo, clear](bool hasAnnotations) {
        undo->setEnabled(hasAnnotations);
        clear->setEnabled(hasAnnotations);
    };
    connect(copy, &QPushButton::clicked, this, [this, hint] {
        QApplication::clipboard()->setImage(renderedImage());
        hint->setText(tr("Annotated image copied to the clipboard."));
    });
    connect(save, &QPushButton::clicked, this, &BrowserAnnotationDialog::saveImage);
}

QImage BrowserAnnotationDialog::renderedImage() const
{
    return m_canvas->renderedImage();
}

void BrowserAnnotationDialog::saveImage()
{
    const QImage image = renderedImage();
    QPointer<BrowserAnnotationDialog> owner(this);
    QPointer<QFileDialog> dialog = new QFileDialog(this, tr("Save annotated screenshot"));
    dialog->setAcceptMode(QFileDialog::AcceptSave);
    dialog->setNameFilter(tr("PNG image (*.png)"));
    dialog->setDefaultSuffix(QStringLiteral("png"));
    dialog->selectFile(QStringLiteral("annotated-screenshot.png"));
    const int result = dialog->exec();
    if (!owner || !dialog)
        return;
    const QStringList files = dialog->selectedFiles();
    dialog->deleteLater();
    if (result != QDialog::Accepted || files.isEmpty())
        return;
    QSaveFile file(files.first());
    if (!file.open(QIODevice::WriteOnly)) {
        QMessageBox::warning(this, tr("Could not save screenshot"), file.errorString());
        return;
    }
    QImageWriter writer(&file, "png");
    if (!writer.write(image)) {
        file.cancelWriting();
        QMessageBox::warning(this, tr("Could not save screenshot"), writer.errorString());
        return;
    }
    if (!file.commit())
        QMessageBox::warning(this, tr("Could not save screenshot"), file.errorString());
}
