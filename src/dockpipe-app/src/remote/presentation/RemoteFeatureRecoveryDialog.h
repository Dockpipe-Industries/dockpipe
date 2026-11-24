#pragma once

#include <QJsonObject>
#include <functional>

class QPushButton;
class QWidget;

// Presentation of core receipt operations. No original payload or retry action.
namespace RemoteFeatureRecoveryDialog {
using VerifiedRecovery = std::function<void(const QJsonObject &, const QJsonObject &)>;
QPushButton *createButton(QWidget *parent, VerifiedRecovery verifiedRecovery = {});
}
