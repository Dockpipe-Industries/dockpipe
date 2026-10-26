#include "WorkflowCatalog.h"

#include "DockpipeChoices.h"

#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QProcess>
#include <algorithm>

namespace {

constexpr int kCatalogDiscoveryTimeoutMs = 30000;

QStringList sortedUnique(QStringList values)
{
    values.removeDuplicates();
    std::sort(values.begin(), values.end());
    return values;
}

WorkflowInputMeta parseWorkflowInputMeta(const QJsonObject &io)
{
    WorkflowInputMeta input;
    input.fieldName = io.value(QStringLiteral("field_name")).toString().trimmed();
    input.envName = io.value(QStringLiteral("env_name")).toString().trimmed();
    input.type = io.value(QStringLiteral("type")).toString().trimmed();
    input.elementType = io.value(QStringLiteral("element_type")).toString().trimmed();
    input.description = io.value(QStringLiteral("description")).toString().trimmed();
    input.defaultValue = io.value(QStringLiteral("default_value")).toString().trimmed();
    const QJsonObject attrs = io.value(QStringLiteral("attributes")).toObject();
    for (auto ait = attrs.begin(); ait != attrs.end(); ++ait)
        input.attributes.insert(ait.key().trimmed().toLower(), ait.value().toString().trimmed());
    const QJsonArray children = io.value(QStringLiteral("children")).toArray();
    for (const QJsonValue &childValue : children) {
        if (!childValue.isObject())
            continue;
        const WorkflowInputMeta child = parseWorkflowInputMeta(childValue.toObject());
        if (!child.envName.isEmpty() || !child.fieldName.isEmpty() || !child.children.isEmpty())
            input.children.append(child);
    }
    return input;
}

WorkflowViewMeta parseWorkflowViewMeta(const QJsonObject &io)
{
    WorkflowViewMeta view;
    const QJsonObject entryObject = io.value(QStringLiteral("entry")).toObject();
    view.entry.type = entryObject.value(QStringLiteral("type")).toString().trimmed();
    view.entry.field = entryObject.value(QStringLiteral("field")).toString().trimmed();
    view.entry.title = entryObject.value(QStringLiteral("title")).toString().trimmed();
    view.entry.description = entryObject.value(QStringLiteral("description")).toString().trimmed();
    const QJsonArray options = entryObject.value(QStringLiteral("options")).toArray();
    for (const QJsonValue &optionValue : options) {
        if (!optionValue.isObject())
            continue;
        const QJsonObject optionObject = optionValue.toObject();
        WorkflowViewEntryOptionMeta option;
        option.value = optionObject.value(QStringLiteral("value")).toString().trimmed();
        option.label = optionObject.value(QStringLiteral("label")).toString().trimmed();
        option.next = optionObject.value(QStringLiteral("next")).toString().trimmed();
        const QJsonArray pagesArray = optionObject.value(QStringLiteral("pages")).toArray();
        for (const QJsonValue &pageValue : pagesArray) {
            const QString page = pageValue.toString().trimmed();
            if (!page.isEmpty())
                option.pages.append(page);
        }
        if (!option.value.isEmpty())
            view.entry.options.append(option);
    }
    const QJsonArray pages = io.value(QStringLiteral("pages")).toArray();
    for (const QJsonValue &pageValue : pages) {
        if (!pageValue.isObject())
            continue;
        const QJsonObject pageObject = pageValue.toObject();
        WorkflowViewPageMeta page;
        page.id = pageObject.value(QStringLiteral("id")).toString().trimmed();
        page.title = pageObject.value(QStringLiteral("title")).toString().trimmed();
        page.description = pageObject.value(QStringLiteral("description")).toString().trimmed();
        const QJsonArray sections = pageObject.value(QStringLiteral("sections")).toArray();
        for (const QJsonValue &sectionValue : sections) {
            if (!sectionValue.isObject())
                continue;
            const QJsonObject sectionObject = sectionValue.toObject();
            WorkflowViewSectionMeta section;
            section.id = sectionObject.value(QStringLiteral("id")).toString().trimmed();
            section.title = sectionObject.value(QStringLiteral("title")).toString().trimmed();
            section.description = sectionObject.value(QStringLiteral("description")).toString().trimmed();
            const QJsonArray fields = sectionObject.value(QStringLiteral("fields")).toArray();
            for (const QJsonValue &fieldValue : fields) {
                const QString field = fieldValue.toString().trimmed();
                if (!field.isEmpty())
                    section.fields.append(field);
            }
            if (!section.fields.isEmpty())
                page.sections.append(section);
        }
        if (!page.sections.isEmpty())
            view.pages.append(page);
    }
    return view;
}

bool runCatalogProcess(const QString &program, const QStringList &args, QByteArray *stdoutData)
{
    if (program.trimmed().isEmpty())
        return false;

    QProcess proc;
    proc.start(program, args);
    if (!proc.waitForFinished(kCatalogDiscoveryTimeoutMs)) {
        proc.kill();
        proc.waitForFinished();
        return false;
    }
    if (proc.exitCode() != 0)
        return false;

    if (stdoutData)
        *stdoutData = proc.readAllStandardOutput();
    return true;
}

} // namespace

WorkflowCatalogData WorkflowCatalog::discoverCatalog(const QString &hintWorkdir)
{
    WorkflowCatalogData out;
    QString program = DockpipeChoices::preferredDockpipeBinary(hintWorkdir);
    if (program.trimmed().isEmpty())
        program = QStringLiteral("dockpipe");

    QStringList args{
        QStringLiteral("catalog"),
        QStringLiteral("list"),
        QStringLiteral("--format"),
        QStringLiteral("json"),
    };
    if (!hintWorkdir.trimmed().isEmpty())
        args << QStringLiteral("--workdir") << hintWorkdir;

    QStringList programs{program};
    if (program != QStringLiteral("dockpipe"))
        programs.append(QStringLiteral("dockpipe"));
    programs.removeDuplicates();

    QByteArray stdoutData;
    bool ok = false;
    for (const QString &candidate : programs) {
        if (runCatalogProcess(candidate, args, &stdoutData)) {
            ok = true;
            break;
        }
    }
    if (!ok)
        return out;

    const QJsonDocument doc = QJsonDocument::fromJson(stdoutData);
    if (!doc.isObject())
        return out;

    const QJsonObject root = doc.object();
    const QJsonArray workflows = root.value(QStringLiteral("workflows")).toArray();
    for (const QJsonValue &value : workflows) {
        if (!value.isObject())
            continue;
        const QJsonObject o = value.toObject();
        WorkflowMeta meta;
        meta.workflowId = o.value(QStringLiteral("workflow_id")).toString().trimmed();
        meta.displayName = o.value(QStringLiteral("display_name")).toString().trimmed();
        meta.description = o.value(QStringLiteral("description")).toString().trimmed();
        meta.category = o.value(QStringLiteral("category")).toString().trimmed();
        meta.iconPath = o.value(QStringLiteral("icon_path")).toString().trimmed();
        meta.configPath = o.value(QStringLiteral("config_path")).toString().trimmed();
        meta.source = o.value("source").toString();
        meta.sourceKind = o.value("source_kind").toString();
        meta.sourcePath = o.value("source_path").toString();
        meta.sourceName = o.value("source_name").toString();
        meta.cloneable = o.value("cloneable").toBool();
        meta.supported = o.value("supported").toBool(true);
        meta.unsupportedReason = o.value("unsupported_reason").toString();
        for (const auto &warning : o.value("warnings").toArray())
            meta.warnings.append(warning.toString());
        const QJsonArray inputs = o.value(QStringLiteral("inputs")).toArray();
        for (const QJsonValue &iv : inputs) {
            if (!iv.isObject())
                continue;
            const WorkflowInputMeta input = parseWorkflowInputMeta(iv.toObject());
            if (!input.envName.isEmpty() || !input.fieldName.isEmpty() || !input.children.isEmpty())
                meta.inputs.append(input);
        }
        meta.view = parseWorkflowViewMeta(o.value(QStringLiteral("view")).toObject());
        const QJsonObject vars = o.value(QStringLiteral("vars")).toObject();
        for (auto it = vars.begin(); it != vars.end(); ++it)
            meta.vars.insert(it.key(), it.value().toString());
        if (meta.displayName.isEmpty())
            meta.displayName = meta.workflowId;
        if (!meta.workflowId.isEmpty())
            out.workflows.append(meta);
    }

    for (const QJsonValue &value : root.value(QStringLiteral("resolver_details")).toArray()) {
        const auto object = value.toObject();
        ResolverMeta resolver;
        resolver.name = object.value("name").toString();
        resolver.title = object.value("title").toString();
        resolver.version = object.value("version").toString();
        resolver.description = object.value("description").toString();
        resolver.capability = object.value("capability").toString();
        resolver.remoteSetup = object.value("remote_setup").toString();
        const auto surface = object.value("surface").toObject();
        if (surface.value("schema").toInt() == 1) {
            resolver.surface.packageRoot = surface.value("package_root").toString();
            resolver.surface.id = surface.value("id").toString();
            resolver.surface.title = surface.value("title").toString();
            resolver.surface.snapshot = surface.value("snapshot").toString();
            const auto nativeDisplay = surface.value("native_display").toObject();
            resolver.surface.nativeDisplay.interfaceVersion = nativeDisplay.value("interface").toInt();
            resolver.surface.nativeDisplay.packageRoot = nativeDisplay.value("package_root").toString();
            const auto libraries = nativeDisplay.value("libraries").toObject();
            for (auto entry = libraries.begin(); entry != libraries.end(); ++entry)
                resolver.surface.nativeDisplay.libraries.insert(entry.key(), entry.value().toString());
            for (const auto &entry : surface.value("actions").toArray()) {
                const auto action = entry.toObject();
                resolver.surface.actions.append({action.value("id").toString(), action.value("title").toString(),
                    action.value("scope").toString(), action.value("workflow").toString(),
                    action.value("workflow_id").toString()});
                auto &environment = resolver.surface.actions.last().launchEnvironment;
                const auto platforms = action.value("launch_environment").toObject();
                for (auto platform = platforms.begin(); platform != platforms.end(); ++platform) {
                    const auto bindings = platform.value().toObject();
                    for (auto binding = bindings.begin(); binding != bindings.end(); ++binding) {
                        const auto reference = binding.value().toObject();
                        environment[platform.key()][binding.key()] = {reference.value("asset").toString(), reference.value("inherit").toString()};
                    }
                }
            }
        }
        if (!resolver.name.isEmpty())
            out.resolverDetails.append(resolver);
    }
    for (const QJsonValue &value : root.value(QStringLiteral("resolvers")).toArray())
        out.resolvers.append(value.toString().trimmed());
    for (const QJsonValue &value : root.value(QStringLiteral("strategies")).toArray())
        out.strategies.append(value.toString().trimmed());
    for (const QJsonValue &value : root.value(QStringLiteral("runtimes")).toArray())
        out.runtimes.append(value.toString().trimmed());

    std::sort(out.workflows.begin(), out.workflows.end(), [](const WorkflowMeta &a, const WorkflowMeta &b) {
        return a.workflowId.localeAwareCompare(b.workflowId) < 0;
    });
    out.resolvers = sortedUnique(out.resolvers);
    out.strategies = sortedUnique(out.strategies);
    out.runtimes = sortedUnique(out.runtimes);
    return out;
}

QVector<WorkflowMeta> WorkflowCatalog::discoverAll(const QString &repoRoot, const QString &hintWorkdir)
{
    Q_UNUSED(repoRoot);
    return discoverCatalog(hintWorkdir).workflows;
}

QVector<WorkflowMeta> WorkflowCatalog::discoverAppWorkflows(const QString &repoRoot, const QString &hintWorkdir)
{
    Q_UNUSED(repoRoot);
    const QVector<WorkflowMeta> all = discoverCatalog(hintWorkdir).workflows;
    QVector<WorkflowMeta> out;
    QStringList seenDisplayNames;
    for (const WorkflowMeta &m : all) {
        if (m.category.compare(QStringLiteral("app"), Qt::CaseInsensitive) != 0)
            continue;
        const QString displayKey = m.source == "vscode" ? m.workflowId
            : m.displayName.trimmed().toLower();
        if (!displayKey.isEmpty() && seenDisplayNames.contains(displayKey))
            continue;
        out.append(m);
        if (!displayKey.isEmpty())
            seenDisplayNames.append(displayKey);
    }
    return out;
}
