#include "WorkflowCatalogData.h"

QString WorkflowMeta::sourceLabel() const
{
    if (source != QStringLiteral("vscode"))
        return QString();
    return sourceKind == QStringLiteral("launch") ? QStringLiteral("VS Code · launch.json")
                                                  : QStringLiteral("VS Code · tasks.json");
}

QString WorkflowMeta::catalogTitle() const
{
    const QString title = displayName.isEmpty() ? workflowId : displayName;
    return sourceLabel().isEmpty() ? title : title + QStringLiteral(" · ") + sourceLabel();
}

QString WorkflowMeta::catalogDescription() const
{
    QStringList details;
    if (!description.isEmpty())
        details.append(description);
    if (!sourceLabel().isEmpty())
        details.append(sourceLabel() + QStringLiteral(" — ") + sourcePath);
    if (!supported && !unsupportedReason.isEmpty())
        details.append(unsupportedReason);
    details.append(warnings);
    return details.join(QLatin1Char('\n'));
}
