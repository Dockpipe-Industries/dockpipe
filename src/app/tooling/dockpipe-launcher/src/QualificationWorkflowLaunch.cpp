#include "QualificationWorkflowLaunch.h"
#include "WorkflowLaunchDialog.h"

#include <QCheckBox>
#include <QComboBox>
#include <QDialogButtonBox>
#include <QLineEdit>
#include <QPushButton>
#include <QTabBar>
#include <QTabWidget>

namespace {
bool collectInputs(const QVector<WorkflowInputMeta> &inputs, QMap<QString, WorkflowInputMeta> &leaves,
                   int depth, int &count)
{
    if (depth > 8)
        return false;
    for (const auto &input : inputs) {
        if (++count > 256)
            return false;
        if (!input.children.isEmpty()) {
            if (!collectInputs(input.children, leaves, depth + 1, count))
                return false;
            continue;
        }
        const QString name = input.envName.trimmed();
        if (name.isEmpty() || leaves.contains(name))
            return false;
        leaves.insert(name, input);
    }
    return true;
}

void revealInput(QWidget *editor, WorkflowLaunchDialog &dialog)
{
    // Change only real tab controls containing this declared editor. A hidden
    // or routed-away field without such a tab remains unavailable.
    for (auto *tabs : dialog.findChildren<QTabWidget *>()) {
        if (!tabs->isAncestorOf(editor))
            continue;
        for (int index = 0; index < tabs->count(); ++index) {
            auto *page = tabs->widget(index);
            if (page == editor || page->isAncestorOf(editor)) {
                if (auto *bar = tabs->findChild<QTabBar *>())
                    bar->setCurrentIndex(index);
                break;
            }
        }
    }
}

QString entryInputName(const QVector<WorkflowInputMeta> &inputs, const QString &path, const QString &prefix = {})
{
    for (const auto &input : inputs) {
        const QString current = prefix.isEmpty() ? input.fieldName : prefix + '.' + input.fieldName;
        if (current == path && input.children.isEmpty())
            return input.envName.trimmed();
        const QString nested = entryInputName(input.children, path, current);
        if (!nested.isEmpty())
            return nested;
    }
    return {};
}

bool sensitive(const WorkflowInputMeta &input)
{
    if (input.attributes.value("password").trimmed().toLower() == "true"
        || input.attributes.value("secret").trimmed().toLower() == "true")
        return true;
    const QString name = input.envName.toUpper();
    return name.contains("PASSWORD") || name.contains("TOKEN")
        || name.contains("SECRET") || name.contains("KEY");
}
}

bool QualificationWorkflowLaunch::validate(const WorkflowMeta &workflow, const QJsonObject &inputs,
                                          QString &error)
{
    QMap<QString, WorkflowInputMeta> leaves;
    int count = 0;
    if (inputs.size() > 32 || !collectInputs(workflow.inputs, leaves, 0, count)) {
        error = "Qualification workflow metadata exceeds its bounds or has ambiguous inputs.";
        return false;
    }
    for (auto entry = inputs.begin(); entry != inputs.end(); ++entry) {
        if (entry.key().toUtf8().size() > 128 || !entry.value().isString()
            || entry.value().toString().toUtf8().size() > 4096
            || entry.value().toString().contains(QChar::Null)
            || !leaves.contains(entry.key()) || sensitive(leaves.value(entry.key()))) {
            error = "Qualification input is unknown, sensitive or outside its bounds.";
            return false;
        }
    }
    return true;
}

QualificationWorkflowLaunch::Outcome QualificationWorkflowLaunch::submit(
    WorkflowLaunchDialog &dialog, const WorkflowMeta &workflow, const QJsonObject &inputs, QString &error)
{
    if (!validate(workflow, inputs, error))
        return Outcome::Failure;
    // Route through the actual entry control first. Its callback rebuilds the
    // form using deleteLater; wait for another event-loop turn before editing.
    const QString entryName = entryInputName(workflow.inputs, workflow.view.entry.field);
    if (!entryName.isEmpty() && inputs.contains(entryName)) {
        const auto choices = dialog.findChildren<QComboBox *>("workflowInput_" + entryName);
        if (choices.size() != 1 || !choices.first()->isEnabled() || !choices.first()->isVisibleTo(&dialog)) {
            error = "Qualification entry choice is unavailable or ambiguous.";
            return Outcome::Failure;
        }
        auto *choice = choices.first();
        const int index = choice->findData(inputs.value(entryName).toString());
        if (index < 0) {
            error = "Qualification entry choice is not a declared option.";
            return Outcome::Failure;
        }
        if (choice->currentIndex() != index) {
            choice->setCurrentIndex(index);
            return Outcome::Pending;
        }
    }
    // Validate every requested control before editing or clicking anything.
    for (auto entry = inputs.begin(); entry != inputs.end(); ++entry) {
        const auto editors = dialog.findChildren<QWidget *>("workflowInput_" + entry.key());
        if (editors.size() == 1)
            revealInput(editors.first(), dialog);
        if (editors.size() != 1 || !editors.first()->isEnabled() || !editors.first()->isVisibleTo(&dialog)) {
            error = "Qualification input control is missing, ambiguous or unavailable.";
            return Outcome::Failure;
        }
        auto *editor = editors.first();
        if (auto *text = qobject_cast<QLineEdit *>(editor)) {
            if (text->echoMode() != QLineEdit::Normal || text->property("workflowSecretInput").toBool()) {
                error = "Qualification cannot edit a sensitive input control.";
                return Outcome::Failure;
            }
        } else if (auto *choice = qobject_cast<QComboBox *>(editor)) {
            if (choice->findData(entry.value().toString()) < 0) {
                error = "Qualification choice is not a declared option.";
                return Outcome::Failure;
            }
        } else if (qobject_cast<QCheckBox *>(editor)) {
            if (entry.value() != "0" && entry.value() != "1") {
                error = "Qualification boolean must be an explicit zero or one.";
                return Outcome::Failure;
            }
        } else {
            error = "Qualification input control is unsupported.";
            return Outcome::Failure;
        }
    }
    const auto boxes = dialog.findChildren<QDialogButtonBox *>();
    if (boxes.size() != 1 || !boxes.first()->button(QDialogButtonBox::Ok)
        || !boxes.first()->button(QDialogButtonBox::Ok)->isEnabled()) {
        error = "Qualification launch button is unavailable or ambiguous.";
        return Outcome::Failure;
    }
    for (auto entry = inputs.begin(); entry != inputs.end(); ++entry) {
        auto *editor = dialog.findChild<QWidget *>("workflowInput_" + entry.key());
        if (!editor) {
            error = "Qualification input disappeared after a routed control change.";
            return Outcome::Failure;
        }
        revealInput(editor, dialog);
        if (auto *text = qobject_cast<QLineEdit *>(editor))
            text->setText(entry.value().toString());
        else if (auto *choice = qobject_cast<QComboBox *>(editor))
            choice->setCurrentIndex(choice->findData(entry.value().toString()));
        else if (auto *check = qobject_cast<QCheckBox *>(editor))
            check->setChecked(entry.value() == "1");
    }
    const auto actual = dialog.values();
    for (auto entry = inputs.begin(); entry != inputs.end(); ++entry) {
        if (actual.value(entry.key()) != entry.value().toString().trimmed()) {
            error = "Qualification input control did not retain its admitted value.";
            return Outcome::Failure;
        }
    }
    boxes.first()->button(QDialogButtonBox::Ok)->click();
    return dialog.result() == QDialog::Accepted ? Outcome::Accepted : Outcome::Failure;
}

QString QualificationWorkflowLaunch::resolve(const QString &launchId,
                                             const QSet<QString> &previousResources,
                                             const QVector<Resource> &resources, QString &error)
{
    if (launchId.isEmpty() || resources.size() > 256) {
        error = "Qualification launch correlation is unavailable or outside its bounds.";
        return {};
    }
    QString selected;
    for (const auto &resource : resources) {
        if (resource.launchId != launchId)
            continue;
        if (resource.id.isEmpty() || previousResources.contains(resource.id)
            || !resource.hasDisplay || !selected.isEmpty()) {
            error = "Qualification launch resource is stale, ambiguous or lacks a display.";
            return {};
        }
        selected = resource.id;
    }
    return selected;
}
