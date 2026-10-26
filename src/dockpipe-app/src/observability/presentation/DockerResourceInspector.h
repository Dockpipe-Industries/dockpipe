#pragma once

#include <QWidget>
#include <QStringList>

class QLabel;
class QPushButton;
class QPlainTextEdit;
class QStackedWidget;
class QTabWidget;

struct DockerInspectorSelection {
    QString key;
    QString name;
    QString status;
    QString detailsHtml;
    QString relatedHtml;
    int kind = -1; // Container, network, volume match the inventory tabs.
    bool canExec = false;
    bool canStart = false;
    bool canStop = false;
};

class DockerResourceInspector : public QWidget {
    Q_OBJECT
public:
    DockerResourceInspector(QPlainTextEdit *logs, const QList<QPlainTextEdit *> &inspect, QWidget *parent = nullptr);
    void setSelection(const DockerInspectorSelection &selection);
    void setStatus(const QString &text);

signals:
    void resourceRequested(const QString &key);
    void refreshRequested();
    void pauseChanged(bool paused);
    void inventoryRequested();
    void startRequested();
    void stopRequested();
    void execRequested(const QString &shell);

private:
    QString m_key;
    QLabel *m_title;
    QLabel *m_state;
    QLabel *m_details;
    QLabel *m_related;
    QLabel *m_status;
    QPushButton *m_start;
    QPushButton *m_stop;
    QPushButton *m_exec;
    QTabWidget *m_tabs;
    QStackedWidget *m_inspect;
};
