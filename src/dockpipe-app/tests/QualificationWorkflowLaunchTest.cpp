#include "QualificationWorkflowLaunch.h"
#include "WorkflowLaunchDialog.h"

#include <QAction>
#include <QApplication>
#include <QCheckBox>
#include <QComboBox>
#include <QDialogButtonBox>
#include <QFileDialog>
#include <QLineEdit>
#include <QPushButton>
#include <QSignalSpy>
#include <QTest>
#include <QTimer>

class QualificationWorkflowLaunchTest final : public QObject {
    Q_OBJECT

private:
    static WorkflowMeta workflow()
    {
        WorkflowMeta result;
        result.workflowId = "fixture-start";
        result.displayName = "Qualification fixture";
        result.supported = true;
        WorkflowInputMeta text;
        text.fieldName = "Name";
        text.envName = "FIXTURE_NAME";
        text.type = "string";
        WorkflowInputMeta choice;
        choice.fieldName = "Mode";
        choice.envName = "FIXTURE_MODE";
        choice.attributes.insert("options", "[\"first\",\"second\"]");
        WorkflowInputMeta boolean;
        boolean.fieldName = "Enabled";
        boolean.envName = "FIXTURE_ENABLED";
        boolean.attributes.insert("control", "checkbox");
        result.inputs = {text, choice, boolean};
        return result;
    }

private slots:
    void initTestCase()
    {
        QCoreApplication::setAttribute(Qt::AA_DontUseNativeDialogs);
    }

    void realActionOpensDialogAndClicksLaunch()
    {
        const auto metadata = workflow();
        QAction action;
        int launches = 0;
        bool submitted = false;
        QObject::connect(&action, &QAction::triggered, this, [&]() {
            WorkflowLaunchDialog dialog(metadata, {});
            auto *buttons = dialog.findChild<QDialogButtonBox *>();
            QSignalSpy clicked(buttons, &QDialogButtonBox::accepted);
            QTimer::singleShot(0, &dialog, [&]() {
                QString error;
                const auto outcome = QualificationWorkflowLaunch::submit(dialog, metadata,
                    {{"FIXTURE_NAME", "unicode λ / exact"}, {"FIXTURE_MODE", "second"},
                     {"FIXTURE_ENABLED", "1"}}, error);
                submitted = outcome == QualificationWorkflowLaunch::Outcome::Accepted;
                if (!submitted)
                    dialog.reject();
                QVERIFY2(submitted, qPrintable(error));
            });
            if (dialog.exec() == QDialog::Accepted) {
                ++launches;
                QCOMPARE(dialog.values().value("FIXTURE_NAME"), "unicode λ / exact");
                QCOMPARE(dialog.values().value("FIXTURE_MODE"), "second");
                QCOMPARE(dialog.values().value("FIXTURE_ENABLED"), "1");
            }
            QCOMPARE(clicked.size(), 1);
        });
        action.trigger();
        QVERIFY(submitted);
        QCOMPARE(launches, 1);
    }

    void invalidInputsNeverClickLaunch_data()
    {
        QTest::addColumn<QJsonObject>("inputs");
        QTest::newRow("unknown") << QJsonObject{{"UNDECLARED", "value"}};
        QTest::newRow("choice") << QJsonObject{{"FIXTURE_MODE", "undeclared"}};
        QTest::newRow("boolean") << QJsonObject{{"FIXTURE_ENABLED", "yes"}};
        QTest::newRow("oversized") << QJsonObject{{"FIXTURE_NAME", QString(4097, 'x')}};
        QTest::newRow("wrong-type") << QJsonObject{{"FIXTURE_NAME", 5}};
    }

    void invalidInputsNeverClickLaunch()
    {
        QFETCH(QJsonObject, inputs);
        const auto metadata = workflow();
        WorkflowLaunchDialog dialog(metadata, {});
        dialog.show();
        QSignalSpy accepted(&dialog, &QDialog::accepted);
        QString error;
        QVERIFY(QualificationWorkflowLaunch::submit(dialog, metadata, inputs, error) == QualificationWorkflowLaunch::Outcome::Failure);
        QVERIFY(!error.isEmpty());
        QCOMPARE(accepted.size(), 0);
        QVERIFY(!error.contains("undeclared"));
    }

    void secretAndMissingControlsNeverLaunch()
    {
        auto metadata = workflow();
        metadata.inputs[0].attributes.insert("secret", "true");
        WorkflowLaunchDialog secretDialog(metadata, {});
        secretDialog.show();
        QString error;
        QVERIFY(QualificationWorkflowLaunch::submit(secretDialog, metadata, {{"FIXTURE_NAME", "private-value"}}, error) == QualificationWorkflowLaunch::Outcome::Failure);
        QVERIFY(!error.contains("private-value"));
        metadata = workflow();
        WorkflowLaunchDialog missingDialog(metadata, {});
        missingDialog.show();
        missingDialog.findChild<QLineEdit *>("workflowInput_FIXTURE_NAME")->hide();
        QVERIFY(QualificationWorkflowLaunch::submit(missingDialog, metadata, {{"FIXTURE_NAME", "value"}}, error) == QualificationWorkflowLaunch::Outcome::Failure);
        QCOMPARE(missingDialog.result(), int(QDialog::Rejected));
    }

    void unprovidedOptionalSecretRemainsBlank()
    {
        auto metadata = workflow();
        metadata.inputs[0].attributes.insert("password", "true");
        WorkflowLaunchDialog dialog(metadata, {});
        dialog.show();
        QString error;
        QCOMPARE(QualificationWorkflowLaunch::submit(dialog, metadata,
                     {{"FIXTURE_MODE", "second"}}, error),
                 QualificationWorkflowLaunch::Outcome::Accepted);
        QVERIFY(dialog.values().value("FIXTURE_NAME").isEmpty());
    }

    void routedEntryChoiceUsesRealFormRebuild()
    {
        auto metadata = workflow();
        WorkflowInputMeta source;
        source.fieldName = "Source";
        source.envName = "FIXTURE_SOURCE";
        source.defaultValue = "first";
        metadata.inputs.append(source);
        metadata.view.entry.field = "Source";
        metadata.view.entry.options = {{"first", "First", "first-page", {}},
                                       {"second", "Second", {}, {"second-page", "shared-page"}}};
        WorkflowViewSectionMeta first;
        first.fields = {"Name"};
        WorkflowViewSectionMeta second;
        second.fields = {"Mode"};
        WorkflowViewSectionMeta shared;
        shared.fields = {"Enabled"};
        metadata.view.pages = {{"first-page", "First", {}, {first}},
                               {"second-page", "Second", {}, {second}},
                               {"shared-page", "Shared", {}, {shared}}};
        WorkflowLaunchDialog dialog(metadata, {});
        dialog.show();
        QSignalSpy accepted(&dialog, &QDialog::accepted);
        QString error;
        const QJsonObject inputs{{"FIXTURE_SOURCE", "second"}, {"FIXTURE_MODE", "second"},
                                 {"FIXTURE_ENABLED", "1"}};
        QCOMPARE(QualificationWorkflowLaunch::submit(dialog, metadata, inputs, error),
                 QualificationWorkflowLaunch::Outcome::Pending);
        QCOMPARE(accepted.size(), 0);
        QTest::qWait(1); // Real event turn disposes the previous routed page.
        QCOMPARE(QualificationWorkflowLaunch::submit(dialog, metadata, inputs, error),
                 QualificationWorkflowLaunch::Outcome::Accepted);
        QCOMPARE(accepted.size(), 1);
        QCOMPARE(dialog.values().value("FIXTURE_SOURCE"), "second");
        QCOMPARE(dialog.values().value("FIXTURE_MODE"), "second");

        WorkflowLaunchDialog defaultDialog(metadata, {});
        defaultDialog.show();
        QCOMPARE(QualificationWorkflowLaunch::submit(defaultDialog, metadata,
                     {{"FIXTURE_NAME", "default route"}}, error),
                 QualificationWorkflowLaunch::Outcome::Accepted);
        QCOMPARE(defaultDialog.values().value("FIXTURE_SOURCE"), "first");

        WorkflowLaunchDialog hiddenDialog(metadata, {});
        hiddenDialog.show();
        QCOMPARE(QualificationWorkflowLaunch::submit(hiddenDialog, metadata,
                     {{"FIXTURE_MODE", "second"}}, error),
                 QualificationWorkflowLaunch::Outcome::Failure);
        QCOMPARE(hiddenDialog.result(), int(QDialog::Rejected));
    }

    void explicitTextControlDoesNotInferAFilePicker()
    {
        auto metadata = workflow();
        metadata.inputs[0].envName = "FIXTURE_DISK_SIZE";
        metadata.inputs[0].attributes.insert("control", "text");
        WorkflowLaunchDialog dialog(metadata, {});
        auto *edit = dialog.findChild<QLineEdit *>("workflowInput_FIXTURE_DISK_SIZE");
        QVERIFY(edit);
        QVERIFY(!edit->parentWidget()->findChild<QPushButton *>(QString(), Qt::FindDirectChildrenOnly));
    }

    void explicitFileModeOverridesDiskNameGuess_data()
    {
        QTest::addColumn<QString>("fileMode");
        QTest::addColumn<int>("expectedAcceptMode");
        QTest::addColumn<int>("expectedFileMode");
        QTest::newRow("existing") << QString("open-file") << int(QFileDialog::AcceptOpen) << int(QFileDialog::ExistingFile);
        QTest::newRow("destination") << QString("save-file") << int(QFileDialog::AcceptSave) << int(QFileDialog::AnyFile);
    }

    void explicitFileModeOverridesDiskNameGuess()
    {
        QFETCH(QString, fileMode);
        QFETCH(int, expectedAcceptMode);
        QFETCH(int, expectedFileMode);
        auto metadata = workflow();
        metadata.inputs[0].envName = "FIXTURE_DISK";
        metadata.inputs[0].attributes.insert("control", "file");
        metadata.inputs[0].attributes.insert("filemode", fileMode);
        WorkflowLaunchDialog dialog(metadata, {});
        dialog.show();
        auto *edit = dialog.findChild<QLineEdit *>("workflowInput_FIXTURE_DISK");
        QVERIFY(edit);
        auto *browse = edit->parentWidget()->findChild<QPushButton *>(QString(), Qt::FindDirectChildrenOnly);
        QVERIFY(browse);
        int actualAcceptMode = -1;
        int actualFileMode = -1;
        QTimer::singleShot(0, &dialog, [&]() {
            for (auto *widget : QApplication::topLevelWidgets()) {
                if (auto *picker = qobject_cast<QFileDialog *>(widget)) {
                    actualAcceptMode = picker->acceptMode();
                    actualFileMode = picker->fileMode();
                    picker->reject();
                }
            }
        });
        QTest::mouseClick(browse, Qt::LeftButton);
        QCOMPARE(actualAcceptMode, expectedAcceptMode);
        QCOMPARE(actualFileMode, expectedFileMode);
    }

    void routedInputsPreserveEditsAndClearedDefaults()
    {
        auto metadata = workflow();
        metadata.inputs[0].defaultValue = "initial";
        WorkflowInputMeta source;
        source.fieldName = "Source";
        source.envName = "FIXTURE_SOURCE";
        source.defaultValue = "first";
        metadata.inputs.append(source);
        metadata.view.entry.field = "Source";
        metadata.view.entry.options = {{"first", "First", "first-page", {}},
                                       {"second", "Second", "second-page", {}}};
        WorkflowViewSectionMeta first;
        first.fields = {"Name"};
        WorkflowViewSectionMeta second;
        second.fields = {"Mode"};
        metadata.view.pages = {{"first-page", "First", {}, {first}},
                               {"second-page", "Second", {}, {second}}};
        WorkflowLaunchDialog dialog(metadata, {});
        dialog.show();
        auto *choice = dialog.findChild<QComboBox *>("workflowInput_FIXTURE_SOURCE");
        auto *name = dialog.findChild<QLineEdit *>("workflowInput_FIXTURE_NAME");
        QVERIFY(choice);
        QVERIFY(name);
        for (const QString &value : {QString("edited"), QString()}) {
            name->setText(value);
            choice->setCurrentIndex(1);
            QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
            QVERIFY(!dialog.values().contains("FIXTURE_NAME"));
            choice->setCurrentIndex(0);
            QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
            name = dialog.findChild<QLineEdit *>("workflowInput_FIXTURE_NAME");
            QVERIFY(name);
            QCOMPARE(name->text(), value);
        }
    }

    void cancellationDoesNotLaunch()
    {
        const auto metadata = workflow();
        WorkflowLaunchDialog dialog(metadata, {});
        QTimer::singleShot(0, &dialog, &QDialog::reject);
        QCOMPARE(dialog.exec(), int(QDialog::Rejected));
    }

    void onlyFreshExactLaunchResourceIsAdmitted()
    {
        QString error;
        const QVector<QualificationWorkflowLaunch::Resource> resources{
            {"old", "previous", true}, {"unrelated", "other", true}, {"new", "admitted", true}};
        QCOMPARE(QualificationWorkflowLaunch::resolve("admitted", {"old"}, resources, error), "new");
        QVERIFY(error.isEmpty());
        QCOMPARE(QualificationWorkflowLaunch::resolve("pending", {"old"}, resources, error), QString());
        QVERIFY(error.isEmpty());
    }

    void staleAmbiguousOrDisplaylessResourcesFail_data()
    {
        QTest::addColumn<int>("scenario");
        QTest::newRow("stale") << 0;
        QTest::newRow("ambiguous") << 1;
        QTest::newRow("displayless") << 2;
        QTest::newRow("cancelled-without-invocation") << 3;
    }

    void staleAmbiguousOrDisplaylessResourcesFail()
    {
        QFETCH(int, scenario);
        QVector<QualificationWorkflowLaunch::Resource> resources{{"new", "admitted", true}};
        QSet<QString> previous;
        QString launchId = "admitted";
        if (scenario == 0)
            previous.insert("new");
        else if (scenario == 1)
            resources.append({"second", "admitted", true});
        else if (scenario == 2)
            resources[0].hasDisplay = false;
        else
            launchId.clear();
        QString error;
        QVERIFY(QualificationWorkflowLaunch::resolve(launchId, previous, resources, error).isEmpty());
        QVERIFY(!error.isEmpty());
    }
};

QTEST_MAIN(QualificationWorkflowLaunchTest)
#include "QualificationWorkflowLaunchTest.moc"
