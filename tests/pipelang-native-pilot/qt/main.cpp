#include "bridge.hpp"
#include <QApplication>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QVBoxLayout>
#include <QPointer>
#include <QTimer>
#include <QFile>
#include <QJsonDocument>
#include <QJsonObject>
#include <QPixmap>
#include <iostream>

int main(int argc,char** argv){
  QApplication app(argc,argv);
  if(argc!=2)return 2;
  const QString output=QString::fromLocal8Bit(argv[1]);
  bool passed=true;int updates=0,destroyed=0;
  auto* window=new QWidget;
  window->setWindowTitle("PipeLang · C++ / Qt pilot");window->resize(500,300);
  window->setStyleSheet("QWidget{background:#172333;color:#eff5ff;font-size:18px} QLineEdit{background:#24364d;padding:8px;border:1px solid #5a789c;border-radius:5px} QPushButton{background:#3477bd;padding:10px;border-radius:5px} QLabel{padding:8px}");
  auto* layout=new QVBoxLayout(window);
  auto* title=new QLabel("PipeLang logic, native Qt interface",window);layout->addWidget(title);
  auto* left=new QLineEdit("19",window);auto* right=new QLineEdit("23",window);
  left->setAccessibleName("Left integer");right->setAccessibleName("Right integer");layout->addWidget(left);layout->addWidget(right);
  auto* button=new QPushButton("Calculate",window);layout->addWidget(button);
  auto* result=new QLabel("Ready",window);result->setAccessibleName("Calculation result");layout->addWidget(result);
  auto* bridge=new PilotBridge(window);QPointer<PilotBridge> weakBridge=bridge;QPointer<QLabel> weakLabel=result;
  QObject::connect(bridge,&QObject::destroyed,[&]{destroyed++;});
  QObject::connect(bridge,&PilotBridge::resultChanged,result,[&](QString value){passed=passed && QThread::currentThread()==app.thread();result->setText(value);updates++;},Qt::QueuedConnection);
  QObject::connect(button,&QPushButton::clicked,bridge,[=]{bridge->calculate(left->text(),right->text());});
  window->show();
  QTimer::singleShot(100,[&]{button->click();});
  QTimer::singleShot(250,[&]{passed=passed && result->text()=="42" && bridge->property("result").toString()=="42" && updates==1;left->setText("9223372036854775807");right->setText("1");button->click();});
  QTimer::singleShot(400,[&]{passed=passed && result->text()=="Arithmetic overflow" && updates==2;left->setText("bad");button->click();});
  QTimer::singleShot(550,[&]{passed=passed && result->text()=="Invalid integer" && updates==3;left->setText("19");right->setText("23");button->click();});
  QTimer::singleShot(800,[&]{passed=passed && result->text()=="42" && updates==4 && window->isVisible();passed=passed && window->grab().save(output+"/qt-pilot.png");
    delete window;passed=passed && weakBridge.isNull() && weakLabel.isNull() && destroyed==1;
    QJsonObject receipt{{"passed",passed},{"result","42"},{"updates",updates},{"bridge_destroyed",destroyed},{"children_destroyed",weakLabel.isNull()},{"platform",QApplication::platformName()},{"qt_version",qVersion()},{"checks","button event, queued signal, property, overflow, invalid input, screenshot, ownership cleanup"}};
    QFile f(output+"/qt-result.json");if(!f.open(QIODevice::WriteOnly))passed=false;else f.write(QJsonDocument(receipt).toJson());app.exit(passed?0:1);
  });
  return app.exec();
}
