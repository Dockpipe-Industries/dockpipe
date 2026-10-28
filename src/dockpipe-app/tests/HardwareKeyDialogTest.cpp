#include "HardwareKeyDialog.h"

#include <QApplication>
#include <QComboBox>
#include <QJsonArray>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QPlainTextEdit>

int main(int argc, char **argv)
{
    QApplication application(argc, argv);
    HardwareKeyDialog dialog;
    auto *pin = dialog.findChild<QLineEdit *>("hardwareKeyPin");
    auto *devices = dialog.findChild<QComboBox *>("hardwareKeyDevice");
    auto *confirm = dialog.findChild<QPushButton *>("hardwareKeyConfirm");
    if (!pin || !devices || !confirm || pin->echoMode() != QLineEdit::Password)
        return 1;
    QJsonObject prompt{{"id", QString(32, 'a')}, {"title", "Approve pairing"},
        {"details", "<b>literal fingerprint</b>"}, {"pin_required", true},
        {"devices", QJsonArray{QJsonObject{{"id", "0"}, {"name", "Physical key"}}}}};
    dialog.showPrompt(prompt);
    if (confirm->isEnabled() || !pin->isEnabled() || devices->count() != 1)
        return 2;
    for (auto *label : dialog.findChildren<QLabel *>()) {
        if (label->textFormat() != Qt::PlainText)
            return 3;
    }
    const auto *details = dialog.findChild<QPlainTextEdit *>();
    if (!details || !details->isReadOnly() || details->toPlainText() != "<b>literal fingerprint</b>")
        return 7;
    int responses = 0;
    bool correctResponse = false;
    QObject::connect(&dialog, &HardwareKeyDialog::responseReady, &dialog,
                     [&](const QString &id, const QString &device, const QString &value) {
        ++responses;
        correctResponse = id == QString(32, 'a') && device == "0" && value == "private-pin";
    });
    pin->setText("private-pin");
    confirm->click();
    if (!correctResponse || responses != 1 || !pin->text().isEmpty()
        || pin->isEnabled() || devices->isEnabled() || confirm->isEnabled())
        return 4;
    dialog.showStatus("Touch your physical security key.");
    prompt["id"] = QString(32, 'b');
    dialog.showPrompt(prompt);
    if (!pin->text().isEmpty() || !pin->isEnabled() || confirm->isEnabled())
        return 5;
    pin->setText("second-private-pin");
    dialog.reject();
    if (!pin->text().isEmpty() || responses != 1)
        return 6;
    return 0;
}
