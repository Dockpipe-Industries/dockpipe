#pragma once

#include "TerminalLaunchContext.h"

#include <QObject>
#include <QMap>
#include <QStringList>
#include <QVector>

class QProcess;
class QTimer;

struct TerminalShellProfile {
    QString id;
    QString label;
    QString program;
    QStringList arguments;
    QString workingDirectoryArgument;
    QMap<QString, QString> environment;
};

class TerminalShellCatalog final : public QObject {
    Q_OBJECT
public:
    // Explicit discovery inputs keep filesystem and process fixtures independent
    // of the machine running the tests. Production uses systemDiscovery().
    struct Discovery {
        bool windows = false;
        bool flatpak = false;
        QString windowsDirectory;
        QStringList powerShellRoots;
        QStringList gitRoots;
        QStringList searchDirectories;
        QString loginShell;
        QString shellsFile;
        QString wslProgram;
        QString flatpakSpawnProgram;
    };

    explicit TerminalShellCatalog(QObject *parent = nullptr);
    explicit TerminalShellCatalog(const Discovery &discovery, QObject *parent = nullptr);
    ~TerminalShellCatalog() override;

    QVector<TerminalShellProfile> profiles() const;
    QString error() const;
    bool discovering() const;
    void refresh();
    void cancel();

    static Discovery systemDiscovery(TerminalLaunchContext context = TerminalLaunchContext::Automatic);

signals:
    void profilesChanged();
    void discoveryFinished();

private:
    void discoverLocal();
    void addExecutable(const QString &path, const QString &label, const QStringList &arguments = {},
                       const QMap<QString, QString> &environment = {});
    void startProbe(const QString &program, const QStringList &arguments);
    void readProbeOutput();
    void finishProbe();
    void publishFinished();

    Discovery m_discovery;
    QVector<TerminalShellProfile> m_profiles;
    QString m_error;
    QProcess *m_process = nullptr;
    QTimer *m_timeout = nullptr;
    QByteArray m_output;
    qsizetype m_diagnosticBytes = 0;
    bool m_cancelled = false;
    bool m_discovering = false;
};
