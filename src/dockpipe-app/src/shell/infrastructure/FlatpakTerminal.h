#pragma once

#include <QString>

class FlatpakTerminal {
public:
    struct Result {
        bool success;
        QString message;
    };

    struct HostEnvironment {
        QString home;
        QString command;
        QString path;
        QString shell;
        QString configHome;
        QString zshHome;
    };

    static QString command();
    static QString wrapperPath(const QString &home);
    static Result install(const QString &home);
    static Result completeSetup(const HostEnvironment &environment);
    static Result setupForCurrentUser();
};
