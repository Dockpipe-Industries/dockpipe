#pragma once

#include "EnvironmentDefinition.h"

#include <QByteArray>
#include <QObject>
#include <QStringList>
#include <QVector>

class QProcess;

class EnvironmentDefinitionsClient final : public QObject {
    Q_OBJECT

public:
    using QObject::QObject;
    ~EnvironmentDefinitionsClient() override;

    void load(const QString &workdir, const QStringList &files);
    void cancel();
    bool isLoading() const { return m_process != nullptr; }

signals:
    void loaded(const QVector<EnvironmentDefinition> &definitions);
    void failed(const QString &error);

private:
    bool drain(QProcess *process);
    void fail(QProcess *process, const QString &error);

    QProcess *m_process = nullptr;
    QByteArray m_output;
};
