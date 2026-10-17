#pragma once
#include <QPlainTextEdit>

// Shared native editing behavior for project files and workflow YAML.
class SourceEditor : public QPlainTextEdit {
public:
    explicit SourceEditor(QWidget *parent = nullptr);
    void setFileName(const QString &path);
    QString languageName() const;
protected:
    void resizeEvent(QResizeEvent *event) override;
    bool eventFilter(QObject *object, QEvent *event) override;
    void changeEvent(QEvent *event) override;
    void keyPressEvent(QKeyEvent *event) override;
private:
    int gutterWidth() const;
    void updateGutter();
    void highlightCurrentLine();
    QWidget *m_gutter;
};
