#include "PackageDisplayWidget.h"
#include "DesktopWindowWidget.h"
#include "LauncherDisplayPackage.h"
#include "LauncherNativeLibraryScope.h"

#include <QDir>
#include <QFileInfo>
#include <QHideEvent>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLabel>
#include <QPluginLoader>
#include <QPointer>
#include <QShowEvent>
#include <QSysInfo>
#include <QVBoxLayout>
#include <cmath>

namespace {
QMap<QString, QPointer<PackageDisplayWidget>> leases;

bool boundedValue(const QJsonValue &value, int depth, int &nodes)
{
    if (depth > 8 || ++nodes > 128)
        return false;
    if (value.isString())
        return value.toString().size() <= 4096 && !value.toString().contains(QChar(0));
    if (value.isDouble())
        return std::isfinite(value.toDouble());
    if (value.isArray()) {
        for (const auto &entry : value.toArray()) {
            if (!boundedValue(entry, depth + 1, nodes))
                return false;
        }
    } else if (value.isObject()) {
        const auto object = value.toObject();
        for (auto entry = object.begin(); entry != object.end(); ++entry) {
            if (entry.key().isEmpty() || entry.key().size() > 64 || entry.key().contains(QChar(0))
                || !boundedValue(entry.value(), depth + 1, nodes))
                return false;
        }
    }
    return true;
}
}

PackageDisplayWidget::PackageDisplayWidget(QWidget *parent) : QWidget(parent)
{
    m_layout = new QVBoxLayout(this);
    m_layout->setContentsMargins(0, 0, 0, 0);
    m_status = new QLabel;
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    m_layout->addWidget(m_status);
    m_native = new DesktopWindowWidget;
    m_native->setProperty("packageDisplay", true);
    m_layout->addWidget(m_native);
    m_native->hide();
    m_status->hide();
}

PackageDisplayWidget::~PackageDisplayWidget()
{
    release();
}

bool PackageDisplayWidget::validateOptions(const QJsonObject &options, QString &error)
{
    int nodes = 0;
    if (QJsonDocument(options).toJson(QJsonDocument::Compact).size() > 16 * 1024
        || !boundedValue(options, 0, nodes)) {
        error = tr("Display options exceed 16 KiB, depth 8, 128 values, key 64 or string 4096 limits.");
        return false;
    }
    error.clear();
    return true;
}

QString PackageDisplayWidget::platformName()
{
#if defined(Q_OS_WIN)
    return "windows";
#elif defined(Q_OS_MACOS)
    return "macos";
#elif defined(Q_OS_LINUX)
    return "linux";
#else
    return "unsupported";
#endif
}

QString PackageDisplayWidget::leaseKey() const
{
    return QFileInfo(m_display.nativePackage.packageRoot).canonicalFilePath() + QChar(0) + m_display.leaseId;
}

void PackageDisplayWidget::setDisplay(const PackageDisplayMeta &display)
{
    const bool unchanged = m_display.id == display.id && m_display.leaseId == display.leaseId
        && m_display.processId == display.processId && m_display.processStartToken == display.processStartToken
        && m_display.nativePackage.packageRoot == display.nativePackage.packageRoot
        && m_display.nativePackage.interfaceVersion == display.nativePackage.interfaceVersion
        && m_display.nativePackage.libraries == display.nativePackage.libraries && m_display.options == display.options;
    if (unchanged)
        return;
    const bool sameResource = m_display.leaseId == display.leaseId
        && QFileInfo(m_display.nativePackage.packageRoot).canonicalFilePath()
            == QFileInfo(display.nativePackage.packageRoot).canonicalFilePath();
    const bool keepHandedOff = m_handedOff && sameResource;
    release();
    m_display = display;
    m_handedOff = keepHandedOff;
    if (isVisible())
        acquire();
}

void PackageDisplayWidget::clearDisplay()
{
    release();
    m_display = {};
    m_handedOff = false;
}

void PackageDisplayWidget::release()
{
    if (leases.value(leaseKey()) == this)
        leases.remove(leaseKey());
    if (m_surface) {
        // Callbacks cannot outlive the host. Workers must finish before deletion.
        m_surface->statusChanged = {};
        m_surface->failed = {};
        m_surface->setActive(false);
        m_surface->closeDisplay();
        delete m_surface;
        m_surface = nullptr;
    }
    m_loader.reset();
    m_nativeLibrary.reset();
    m_native->setProcessDisplay(0, {});
    m_native->hide();
    m_status->hide();
}

void PackageDisplayWidget::acquire()
{
    if (m_handedOff || !isVisible())
        return;
    if (m_display.processId > 0) {
        m_native->show();
        m_native->setProcessDisplay(m_display.processId, m_display.processStartToken);
        return;
    }
    if (m_display.nativePackage.interfaceVersion == 0 || m_surface)
        return;
    QString error;
    const auto fail = [this](const QString &message) {
        m_status->setText(message.left(4096));
        m_status->show();
        emit displayFailed(message.left(4096));
    };
    const auto &descriptor = m_display.nativePackage;
    if (descriptor.interfaceVersion != 1 || m_display.leaseId.isEmpty()
        || !validateOptions(m_display.options, error)) {
        fail(error.isEmpty() ? tr("Incompatible package display descriptor.") : error);
        return;
    }
    const QFileInfo root(descriptor.packageRoot);
    const QFileInfo library(descriptor.libraries.value(platformName()));
    const QString canonicalRoot = root.canonicalFilePath();
    const QString canonicalLibrary = library.canonicalFilePath();
    const QString relative = QDir(canonicalRoot).relativeFilePath(canonicalLibrary);
    if (!root.isAbsolute() || !root.isDir() || !library.isAbsolute() || !library.isFile()
        || canonicalRoot.isEmpty() || canonicalLibrary.isEmpty() || relative == "."
        || QDir::isAbsolutePath(relative) || relative.split('/').contains("..")) {
        fail(tr("This package has no contained display library for this platform."));
        return;
    }
    m_loader = std::make_unique<QPluginLoader>(canonicalLibrary);
    m_loader->setLoadHints(QLibrary::PreventUnloadHint);
    const auto metadata = m_loader->metaData();
    const auto contract = metadata.value("MetaData").toObject();
    if (metadata.value("IID") != QJsonValue(LauncherDisplayPackage_iid)
        || contract.value("display_abi") != QJsonValue(1)
        || contract.value("qt_version") != QJsonValue(QString(QT_VERSION_STR))
        || contract.value("build_abi") != QJsonValue(QSysInfo::buildAbi())
        || contract.value("compiler_id") != QJsonValue(QString(DOCKPIPE_DISPLAY_COMPILER_ID))
        || contract.value("compiler_version") != QJsonValue(QString(DOCKPIPE_DISPLAY_COMPILER_VERSION))) {
        fail(tr("The package display does not match this host interface and Qt kit."));
        m_loader.reset();
        return;
    }
    m_nativeLibrary = std::make_unique<LauncherNativeLibraryScope>();
    if (!m_nativeLibrary->load(canonicalLibrary, error)) {
        fail(error);
        m_loader.reset();
        m_nativeLibrary.reset();
        return;
    }
    auto *package = qobject_cast<LauncherDisplayPackage *>(m_loader->instance());
    if (!package) {
        fail(tr("The package display could not load: %1").arg(m_loader->errorString()));
        m_loader.reset();
        m_nativeLibrary.reset();
        return;
    }
    // Only after validation/load succeeds may another view surrender its lease.
    if (auto *previous = leases.value(leaseKey()).data(); previous && previous != this) {
        previous->release();
        previous->m_handedOff = true;
    }
    package->initialize(canonicalRoot);
    m_surface = package->createDisplay(m_display.options, this);
    if (!m_surface) {
        fail(tr("The package did not create a display surface."));
        m_loader.reset();
        m_nativeLibrary.reset();
        return;
    }
    leases.insert(leaseKey(), this);
    m_layout->addWidget(m_surface);
    m_surface->statusChanged = [this](const QString &message) {
        m_status->setText(message.left(4096));
        m_status->setVisible(!message.isEmpty());
    };
    m_surface->failed = fail;
    m_surface->show();
    m_surface->setActive(true);
}

void PackageDisplayWidget::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    m_handedOff = false;
    acquire();
}

void PackageDisplayWidget::hideEvent(QHideEvent *event)
{
    release();
    QWidget::hideEvent(event);
}
