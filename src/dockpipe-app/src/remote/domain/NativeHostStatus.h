#pragma once

#include <QByteArray>
#include <QString>

struct NativeHostStatus {
    bool available = false;
    QString node;
    QString username;
    quint32 uid = 0;
    quint32 gid = 0;
    QString home;
    QString generation;

    QString identity() const;
    static bool decode(const QByteArray &output, NativeHostStatus *status);
};
