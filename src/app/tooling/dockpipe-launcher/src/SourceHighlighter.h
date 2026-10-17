#pragma once
#include <syntaxhighlighter.h>

class SourceHighlighter : public KSyntaxHighlighting::SyntaxHighlighter {
    Q_OBJECT
public:
    explicit SourceHighlighter(QTextDocument *document);
    void setFileName(const QString &path);
    void updateTheme(bool dark);
    QString languageName() const;
protected:
    void highlightBlock(const QString &text) override;
};
