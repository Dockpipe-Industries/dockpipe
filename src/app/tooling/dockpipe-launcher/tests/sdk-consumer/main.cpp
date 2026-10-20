#include "BrowserWidget.h"
#include "LauncherWorkspacePackage.h"
#include "SourceHighlighter.h"
#include "WorkspaceFiles.h"

#include <QApplication>
#include <QTextDocument>

int main(int argc, char **argv)
{
    QApplication application(argc, argv);
    QTextDocument document;
    SourceHighlighter highlighter(&document);
    BrowserWidget browser;
    browser.navigate(QUrl("about:blank"));
    return 0;
}
