#pragma once

#include <QString>
#include <QStringList>

// Opens interactive setup in a real terminal so sudo and browser login own their
// normal input channel. Arguments never become unquoted shell source.
bool openSetupTerminal(const QString &program, const QStringList &arguments, QString *error);
