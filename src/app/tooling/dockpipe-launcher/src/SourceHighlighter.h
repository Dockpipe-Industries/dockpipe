#pragma once

#include "LauncherUiExport.h"

#include <syntaxhighlighter.h>

class DOCKPIPE_LAUNCHER_UI_EXPORT SourceHighlighter : public KSyntaxHighlighting::SyntaxHighlighter {
    Q_OBJECT
public:
    explicit SourceHighlighter(QTextDocument *document);
    void setFileName(const QString &path);
    void updateTheme(bool dark);
    QString languageName() const;
protected:
    void highlightBlock(const QString &text) override;
};
