#pragma once

#include <QJsonObject>

class QObject;
class ProjectGitWidget;

// Private composition state preserves the published Git client/widget layouts.
// A remote selection stays remote when its catalog entry becomes unavailable.
namespace RemoteGitContext {
bool configure(ProjectGitWidget *widget, const QJsonObject &target, const QString &root,
               const QJsonObject &grant = {}, QString *error = nullptr);
bool configureLocal(ProjectGitWidget *widget, const QString &workdir, QString *error = nullptr);
bool isRemote(const QObject *object);
bool canWrite(const QObject *object);
bool canPublish(const QObject *object);
QJsonObject target(const QObject *object);
QString root(const QObject *object);
bool validTarget(const QJsonObject &target, const QString &root);
void bind(QObject *object, bool remote, const QJsonObject &target, const QString &root,
          const QJsonObject &grant = {});
}
