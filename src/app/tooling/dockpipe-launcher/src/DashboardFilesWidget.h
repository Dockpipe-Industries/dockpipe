#pragma once

#include <QWidget>

class QFileSystemModel;
class QLabel;
class QLineEdit;
class QModelIndex;
class QPushButton;
class QTreeView;
class DashboardFilesFilter;

class DashboardFilesWidget final : public QWidget {
    Q_OBJECT
public:
    explicit DashboardFilesWidget(QWidget *parent = nullptr);
    void setContext(const QString &scope, const QString &projectPath);
    QString scope() const { return m_scope; }
    QString currentDirectory() const { return m_directory; }
    bool navigateTo(const QString &path);

signals:
    void openFileRequested(const QString &path);

private:
    void activate(const QModelIndex &index);
    void updateControls();
    void clearProject();
    void showFilesystem();
    QString m_scope = QStringLiteral("project");
    QString m_projectRoot;
    QString m_directory;
    QFileSystemModel *m_model;
    DashboardFilesFilter *m_filter;
    QTreeView *m_tree;
    QLineEdit *m_location;
    QPushButton *m_up;
    QPushButton *m_home;
    QPushButton *m_filesystem;
    QLabel *m_status;
};
