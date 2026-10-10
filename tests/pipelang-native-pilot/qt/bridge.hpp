#pragma once
#include <QObject>
#include <QString>
#include <QThread>
#include "qt_binding.hpp"

// An adapter for this pilot, not part of PipeLang Core or its managed runtime.
class PilotBridge final : public QObject {
  Q_OBJECT
  Q_PROPERTY(QString result READ result NOTIFY resultChanged)
public:
  explicit PilotBridge(QObject* parent=nullptr):QObject(parent){}
  QString result() const {return result_;}
public slots:
  void calculate(QString left,QString right) {
    Q_ASSERT(QThread::currentThread()==thread());
    bool leftOK=false,rightOK=false;
    const qlonglong a=left.toLongLong(&leftOK),b=right.toLongLong(&rightOK);
    if(!leftOK || !rightOK)result_="Invalid integer";
    else {
      const auto value=pilot_calculate(static_cast<std::int64_t>(a),static_cast<std::int64_t>(b));
      result_=value.ok?QString::number(static_cast<qlonglong>(value.value)):QString("Arithmetic overflow");
    }
    emit resultChanged(result_);
  }
signals:
  void resultChanged(QString result);
private:
  QString result_="Ready";
};
