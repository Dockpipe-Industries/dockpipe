#pragma once

#include <QString>
#include <QIcon>
#include <QVector>

struct DesktopApplicationEntry {
    QString name;
    QString id;
    QString launchTarget;
    QString windowClass;
    QString iconName;
    bool hasDeclaredWindowClass = false;
};

// Shared desktop metadata and platform launch dispatch. Run both operations on
// a worker thread: filesystem access and platform launch services may block.
class DesktopApplicationCatalog final {
public:
    static QVector<DesktopApplicationEntry> enumerate(QString *error = nullptr);
    // Success means the platform accepted launch, not that a window appeared.
    // Only a current catalog entry is accepted; executable fields are not parsed.
    static bool launch(const DesktopApplicationEntry &entry, QString &error);
    // Resolve an already enumerated application's icon on the GUI thread.
    static QIcon applicationIcon(const DesktopApplicationEntry &entry);
};
