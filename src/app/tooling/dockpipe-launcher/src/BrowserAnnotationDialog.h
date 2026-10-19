#pragma once

#include <QDialog>
#include <QImage>
#include <QUrl>

class BrowserAnnotationCanvas;

/// Keeps screenshot edits in memory until the user copies or saves the result.
class BrowserAnnotationDialog final : public QDialog {
    Q_OBJECT
public:
    explicit BrowserAnnotationDialog(const QImage &capture, const QUrl &url,
                                     QWidget *parent = nullptr);

    QImage renderedImage() const;

private:
    void saveImage();

    BrowserAnnotationCanvas *m_canvas = nullptr;
};
