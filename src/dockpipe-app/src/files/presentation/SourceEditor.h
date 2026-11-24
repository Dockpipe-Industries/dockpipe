#pragma once

#include <QCoreApplication>
#include <QPlainTextEdit>

// Shared native editing behavior for project files and workflow YAML.
class SourceEditor : public QPlainTextEdit {
    Q_DECLARE_TR_FUNCTIONS(SourceEditor)
public:
    explicit SourceEditor(QWidget *parent = nullptr);
    void setPlainText(const QString &text);
    void loadFileText(const QString &text, const QString &path);
    void setFileDocument(QTextDocument *document, const QString &path);
    void setFileName(const QString &path);
    QString languageName() const;
    void refreshLanguageDiagnostics();
    void setLineMapping(const QVector<int> &lines);
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
