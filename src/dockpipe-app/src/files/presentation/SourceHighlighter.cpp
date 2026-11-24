#include "SourceHighlighter.h"
#include <definition.h>
#include <repository.h>
#include <KSyntaxHighlighting/Theme>
#include <QApplication>
#include <QDir>
#include <QResource>
#include <QPalette>

namespace {
KSyntaxHighlighting::Repository &syntaxRepository()
{
    static const bool loaded = []() {
        const QDir executable(QCoreApplication::applicationDirPath());
        for (const QString &path : {executable.filePath("dockpipe-syntax.rcc"),
                 executable.filePath("../share/dockpipe/launcher/dockpipe-syntax.rcc")}) {
            if (QResource::registerResource(path))
                return true;
        }
        qWarning("Dockpipe syntax definitions are missing; reinstall the desktop package.");
        return false;
    }();
    Q_UNUSED(loaded);
    static KSyntaxHighlighting::Repository repository;
    return repository;
}
}

SourceHighlighter::SourceHighlighter(QTextDocument *document)
    : KSyntaxHighlighting::SyntaxHighlighter(document)
{
    updateTheme(QApplication::palette().color(QPalette::Window).lightness() < 128);
}

void SourceHighlighter::setFileName(const QString &path)
{
    setDefinition(syntaxRepository().definitionForFileName(path));
}

void SourceHighlighter::updateTheme(bool dark)
{
    const auto next = syntaxRepository().theme(dark ? QStringLiteral("Dockpipe Dark") : QStringLiteral("Dockpipe Light"));
    if (theme().name() == next.name())
        return;
    setTheme(next);
    rehighlight();
}

QString SourceHighlighter::languageName() const
{
    return definition().isValid() ? definition().name() : tr("Plain text");
}

void SourceHighlighter::highlightBlock(const QString &text)
{
    // Huge generated/minified lines remain editable without running grammar regexes.
    if (text.size() > 16384) {
        setCurrentBlockState(-1);
        setCurrentBlockUserData(nullptr);
        return;
    }
    KSyntaxHighlighting::SyntaxHighlighter::highlightBlock(text);
}
