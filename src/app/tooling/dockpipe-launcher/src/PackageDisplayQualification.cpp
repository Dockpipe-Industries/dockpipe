#include "PackageDisplayQualification.h"
#include "ContextStore.h"
#include "DashboardAppsDialog.h"
#include "DashboardPage.h"
#include "DashboardWorkspace.h"
#include "DashboardWidgetFactory.h"
#include "DockpipeChoices.h"
#include "LauncherDisplayPackage.h"
#include "LauncherSettings.h"
#include "MainWindow.h"
#include "PackageDisplayWidget.h"
#include "PackageSurfacePage.h"

#include <QAbstractButton>
#include <QApplication>
#include <QComboBox>
#include <QDialogButtonBox>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QFileInfo>
#include <QImage>
#include <QJsonDocument>
#include <QJsonArray>
#include <QKeyEvent>
#include <QListWidget>
#include <QPointer>
#include <QPushButton>
#include <QRegularExpression>
#include <QSaveFile>
#include <QStackedWidget>
#include <QStandardPaths>
#include <QTextStream>
#include <QTimer>

namespace {
QJsonObject configuration;

bool qualificationRequested(const QStringList &arguments)
{
    return arguments.contains("--qualify-package-display")
        || arguments.contains("--qualification-profile-path");
}

bool loadConfiguration(const QStringList &arguments, QString &error)
{
    const int position = arguments.indexOf("--qualify-package-display");
    if (position < 0 || position + 1 >= arguments.size()) {
        error = "Qualification requires --qualify-package-display with an absolute configuration file.";
        return false;
    }
    const QFileInfo path(arguments.at(position + 1));
    QFile input(path.absoluteFilePath());
    if (!path.isAbsolute() || !input.open(QIODevice::ReadOnly) || input.size() > 32768) {
        error = "Cannot read bounded qualification configuration.";
        return false;
    }
    QJsonParseError parse;
    const auto document = QJsonDocument::fromJson(input.readAll(), &parse);
    if (parse.error != QJsonParseError::NoError || !document.isObject()) {
        error = "Qualification configuration must be a JSON object.";
        return false;
    }
    configuration = document.object();
    const QSet<QString> keys{"schema", "workspace", "surface_id", "resource_id", "output",
                             "profile_id", "allow_input", "timeout_ms"};
    for (auto entry = configuration.begin(); entry != configuration.end(); ++entry) {
        if (!keys.contains(entry.key())) {
            error = "Unknown qualification configuration field.";
            return false;
        }
    }
    const QRegularExpression identity("^[A-Za-z0-9_.:-]{1,128}$");
    const QRegularExpression profile("^[A-Za-z0-9_-]{1,64}$");
    const QFileInfo workspace(configuration.value("workspace").toString());
    const QFileInfo output(configuration.value("output").toString());
    const auto timeout = configuration.value("timeout_ms");
    if (configuration.value("schema") != 1 || !workspace.isAbsolute() || !workspace.isDir()
        || !output.isAbsolute() || configuration.value("output").toString().size() > 4096
        || !identity.match(configuration.value("surface_id").toString()).hasMatch()
        || !identity.match(configuration.value("resource_id").toString()).hasMatch()
        || !profile.match(configuration.value("profile_id").toString()).hasMatch()
        || !configuration.value("allow_input").isBool()
        || !timeout.isDouble() || timeout.toInt() < 30000 || timeout.toInt() > 180000
        || timeout.toDouble() != timeout.toInt()) {
        error = "Invalid qualification paths, identities, input policy or timeout.";
        return false;
    }
    return true;
}

bool nonuniform(const QImage &image)
{
    if (image.isNull() || image.width() < 32 || image.height() < 32)
        return false;
    const auto rgb = image.convertToFormat(QImage::Format_RGB32);
    const QRgb first = rgb.pixel(0, 0);
    int changed = 0;
    for (int y = 0; y < rgb.height(); ++y) {
        const auto *line = reinterpret_cast<const QRgb *>(rgb.constScanLine(y));
        for (int x = 0; x < rgb.width(); ++x) {
            if (line[x] != first && ++changed >= 256)
                return true;
        }
    }
    return false;
}
}

bool preparePackageDisplayQualification(QApplication &application, int &exitCode)
{
    if (!qualificationRequested(application.arguments()))
        return false;
    QString error;
    if (!loadConfiguration(application.arguments(), error)) {
        QTextStream(stderr) << error << '\n';
        exitCode = 2;
        return true;
    }
    application.setOrganizationName("dockpipe-qualification");
    application.setApplicationName("dockpipe-qualification-" + configuration.value("profile_id").toString());
    if (application.arguments().contains("--qualification-profile-path")) {
        QTextStream(stdout) << QJsonDocument(QJsonObject{{"schema", 1},
            {"profile_path", QStandardPaths::writableLocation(QStandardPaths::AppConfigLocation)},
            {"application", application.applicationName()}, {"organization", application.organizationName()}})
            .toJson(QJsonDocument::Compact) << '\n';
        exitCode = 0;
        return true;
    }
    exitCode = -1;
    return true;
}

class PackageDisplayQualification final : public QObject {
public:
    PackageDisplayQualification(QApplication &application, MainWindow &window)
        : QObject(&window), m_application(application), m_window(window)
    {
        m_clock.start();
        m_receipt = {{"schema", 1}, {"scope", "full-installed-launcher"},
            {"qualifier_revision", 4}, {"real_main_window", true}, {"real_catalog_and_snapshot", true},
            {"resource_lifecycle_owned_by_qualifier", false},
            {"profile_path", ContextStore::configDir()},
            {"application_path", application.applicationFilePath()},
            {"cli_path", DockpipeChoices::preferredDockpipeBinary(configuration.value("workspace").toString())},
            {"input_method", "programmatic Qt native Shift press/release; transport acknowledgement required"}};
        m_timer.setInterval(200);
        connect(&m_timer, &QTimer::timeout, this, [this]() { advance(); });
        m_timer.start();
    }

private:
    enum class Phase { Discover, Surface, Input, CanvasPicker, Canvas, TilingPicker, Tiling, Return, Closed, Disconnected, Reopen };

    LauncherDisplaySurface *visibleSurface() const
    {
        LauncherDisplaySurface *selected = nullptr;
        for (auto *widget : m_window.findChildren<QWidget *>()) {
            auto *surface = dynamic_cast<LauncherDisplaySurface *>(widget);
            if (surface && surface->isVisible()) {
                if (selected)
                    return nullptr;
                selected = surface;
            }
        }
        return selected;
    }

    int loadedViews() const
    {
        int count = 0;
        for (auto *widget : m_window.findChildren<PackageDisplayWidget *>())
            count += widget->hasPackageSurface() ? 1 : 0;
        return count;
    }

    bool readyForCapture()
    {
        auto *surface = visibleSurface();
        if (!surface || !surface->property("displayReady").toBool()
            || surface->property("displayFrameRevision").toULongLong() == 0 || loadedViews() != 1) {
            m_readySince = -1;
            return false;
        }
        if (m_readySince < 0)
            m_readySince = m_clock.elapsed();
        return m_clock.elapsed() - m_readySince >= 6500;
    }

    bool capture(const QString &name)
    {
        auto *surface = visibleSurface();
        auto *input = surface ? surface->findChild<QWidget *>("dockpipeDisplayInput") : nullptr;
        const QImage image = input ? input->grab().toImage() : QImage();
        const QString output = configuration.value("output").toString();
        const bool pixels = nonuniform(image);
        const bool saved = pixels && image.save(QDir(output).filePath(name + "-frame.png"))
            && m_window.grab().save(QDir(output).filePath(name + "-window.png"));
        m_receipt[name + "_pixels"] = pixels;
        m_receipt[name + "_capture"] = saved;
        m_receipt[name + "_revision"] = surface ? double(surface->property("displayFrameRevision").toULongLong()) : 0;
        m_receipt[name + "_single_lease"] = loadedViews() == 1;
        return saved;
    }

    void transition(Phase phase)
    {
        m_phase = phase;
        m_phaseSince = m_clock.elapsed();
        m_lastPickerDiagnostic = {};
        m_readySince = -1;
        m_receipt["last_phase"] = int(phase);
        QTextStream(stdout) << "Dockpipe full launcher qualification phase " << int(phase) << '\n';
    }

    void finish(bool success, const QString &error = {})
    {
        if (m_finished)
            return;
        m_finished = true;
        m_timer.stop();
        m_receipt["elapsed_ms"] = double(m_clock.elapsed());
        if (m_phase == Phase::CanvasPicker || m_phase == Phase::TilingPicker)
            recordPickerDiagnostic(m_phase == Phase::TilingPicker, true);
        if (!error.isEmpty()) {
            m_receipt["error"] = error.left(1024);
            m_window.grab().save(QDir(configuration.value("output").toString()).filePath("failure-window.png"));
        }
        // Hiding releases transports, without asking the owning resource to stop.
        m_window.hide();
        success = success && loadedViews() == 0;
        m_receipt["success"] = success;
        m_receipt["all_views_disconnected"] = loadedViews() == 0;
        QSaveFile output(QDir(configuration.value("output").toString()).filePath("full-launcher-proof.json"));
        const QByteArray bytes = QJsonDocument(m_receipt).toJson();
        const bool written = output.open(QIODevice::WriteOnly)
            && output.write(bytes) == bytes.size() && output.commit();
        m_application.exit(success && written && loadedViews() == 0 ? 0 : 1);
    }

    bool chooseDashboard()
    {
        for (int index = 0; index < m_window.m_stack->count(); ++index) {
            if (m_window.m_stack->widget(index)->property("dashboardId").toString()
                != DashboardStore::defaultDashboardId())
                continue;
            m_window.selectPage(index);
            m_dashboard = qobject_cast<DashboardPage *>(m_window.m_stack->currentWidget());
            return m_dashboard != nullptr;
        }
        return false;
    }

    void recordPickerDiagnostic(bool tiling, bool capture = false)
    {
        const QString name = tiling ? "dashboardAppsDialog" : "dashboardWidgetPicker";
        auto *dialog = m_dashboard ? m_dashboard->findChild<QDialog *>(name) : nullptr;
        auto *list = dialog ? dialog->findChild<QListWidget *>(tiling ? "dashboardAppList" : "dashboardWidgetChoices") : nullptr;
        auto *buttons = dialog ? dialog->findChild<QDialogButtonBox *>() : nullptr;
        auto *button = buttons ? buttons->button(tiling ? QDialogButtonBox::Open : QDialogButtonBox::Ok) : nullptr;
        QJsonObject diagnostic{{"schema", 1}, {"phase", int(m_phase)},
            {"expected_id", m_displayId}, {"dialog_exists", dialog != nullptr},
            {"dialog_visible", dialog && dialog->isVisible()}, {"list_exists", list != nullptr},
            {"button_exists", button != nullptr}, {"button_enabled", button && button->isEnabled()},
            {"button_visible", button && button->isVisible()},
            {"dashboard_visible", m_dashboard && m_dashboard->isVisible()},
            {"dashboard_environment", m_dashboard ? m_dashboard->environment() : QString()}};
        diagnostic["configured_workspace"] = configuration.value("workspace").toString().left(4096);
        diagnostic["settings_workspace"] = m_window.m_settings.projectFolder.left(4096);
        diagnostic["factory_workspace"] = m_window.m_dashboardFactory->workdir().left(4096);
        QJsonArray inventory;
        for (const auto &display : m_window.m_dashboardFactory->packageDisplays()) {
            if (inventory.size() >= 32)
                break;
            inventory.append(QJsonObject{{"id", display.id.left(260)}, {"lease", display.leaseId.left(128)}});
        }
        diagnostic["factory_displays"] = inventory;
        if (auto *page = m_window.m_surfacePages.value(configuration.value("surface_id").toString()).data())
            diagnostic["surface_inventory"] = page->inventoryDiagnostic();
        QJsonArray items;
        if (list) {
            diagnostic["row_count"] = list->count();
            int expectedRow = -1;
            for (int index = 0; index < qMin(4096, list->count()); ++index) {
                const auto *item = list->item(index);
                const QString identity = item->data(tiling ? Qt::UserRole + 1 : Qt::UserRole).toString();
                if (identity == (tiling ? m_displayId : "package-display:" + m_displayId))
                    expectedRow = index;
            }
            diagnostic["expected_row"] = expectedRow;
            for (int index = 0; index < qMin(32, list->count()); ++index) {
                const auto *item = list->item(index);
                items.append(QJsonObject{{"row", index},
                    {"role", item->data(Qt::UserRole).toString().left(260)},
                    {"identity", item->data(Qt::UserRole + 1).toString().left(260)}});
            }
        }
        diagnostic["items"] = items;
        QJsonArray ownDialogs;
        for (auto *widget : m_window.findChildren<QDialog *>()) {
            if (ownDialogs.size() >= 16)
                break;
            ownDialogs.append(QJsonObject{{"name", widget->objectName().left(128)},
                {"class", QString::fromLatin1(widget->metaObject()->className()).left(128)},
                {"visible", widget->isVisible()}});
        }
        diagnostic["own_dialogs"] = ownDialogs;
        if (!capture && diagnostic == m_lastPickerDiagnostic)
            return;
        m_lastPickerDiagnostic = diagnostic;
        const QString stem = tiling ? "tiling-picker" : "canvas-picker";
        m_receipt[stem + "_diagnostic"] = diagnostic;
        QSaveFile output(QDir(configuration.value("output").toString()).filePath(stem + "-diagnostic.json"));
        const QByteArray bytes = QJsonDocument(diagnostic).toJson();
        if (output.open(QIODevice::WriteOnly) && output.write(bytes) == bytes.size())
            output.commit();
        if (capture && dialog)
            dialog->grab().save(QDir(configuration.value("output").toString()).filePath(stem + "-window.png"));
    }

    bool chooseDisplayFromPicker(bool tiling)
    {
        QDialog *dialog = tiling ? m_dashboard->findChild<DashboardAppsDialog *>()
            : m_dashboard->findChild<QDialog *>("dashboardWidgetPicker");
        auto *list = dialog ? dialog->findChild<QListWidget *>(tiling ? "dashboardAppList" : "dashboardWidgetChoices") : nullptr;
        if (!list)
            return false;
        for (int index = 0; index < list->count(); ++index) {
            auto *item = list->item(index);
            const bool matches = tiling
                ? item->data(Qt::UserRole) == "package-display"
                    && item->data(Qt::UserRole + 1).toString() == m_displayId
                : item->data(Qt::UserRole).toString() == "package-display:" + m_displayId;
            if (!matches)
                continue;
            list->setCurrentItem(item);
            auto *buttons = dialog->findChild<QDialogButtonBox *>();
            auto *button = buttons ? buttons->button(tiling ? QDialogButtonBox::Open : QDialogButtonBox::Ok) : nullptr;
            if (!button)
                return false;
            button->click();
            return true;
        }
        return false;
    }

    bool closeDisplayPanel()
    {
        for (auto *display : m_dashboard->findChildren<PackageDisplayWidget *>()) {
            if (!display->isVisible() || !display->hasPackageSurface())
                continue;
            for (auto *frame = display->parentWidget(); frame && frame != m_dashboard; frame = frame->parentWidget()) {
                if (!frame->objectName().startsWith("dashboardPanel_"))
                    continue;
                auto *button = frame->findChild<QAbstractButton *>("dashboardPanelClose");
                if (!button)
                    return false;
                button->click();
                return true;
            }
        }
        return false;
    }

    bool selectEnvironment(const QString &environment)
    {
        auto *button = m_dashboard->findChild<QAbstractButton *>(
            environment == "canvas" ? "dashboardCanvasMode" : "dashboardTilingMode");
        if (!button)
            return false;
        button->click();
        return m_dashboard->environment() == environment;
    }

    void advance()
    {
        if (m_clock.elapsed() > configuration.value("timeout_ms").toInt()) {
            finish(false, "Qualification deadline exceeded; installed catalog, display readiness or lifecycle did not satisfy the current phase.");
            return;
        }
        const QString resource = configuration.value("resource_id").toString();
        switch (m_phase) {
        case Phase::Discover:
            m_surfacePage = m_window.m_surfacePages.value(configuration.value("surface_id").toString());
            if (m_surfacePage) {
                m_window.selectPage(m_window.m_stack->indexOf(m_surfacePage));
                transition(Phase::Surface);
            }
            break;
        case Phase::Surface: {
            auto *selector = m_surfacePage->findChild<QComboBox *>("surfaceResourceSelector");
            const int index = selector ? selector->findData(resource) : -1;
            if (index < 0)
                break;
            selector->setCurrentIndex(index);
            if (!readyForCapture())
                break;
            if (!capture("surface")) {
                finish(false, "Installed surface did not render nonuniform native pixels.");
                return;
            }
            auto *surface = visibleSurface();
            m_acknowledgements = surface->property("displayInputAcknowledgements").toULongLong();
            if (configuration.value("allow_input").toBool()) {
                auto *input = surface->findChild<QWidget *>("dockpipeDisplayInput");
                input->setFocus(Qt::OtherFocusReason);
#if defined(Q_OS_WIN)
                const quint32 scan = 0x2a;
                const quint32 nativeKey = 0x10;
#elif defined(Q_OS_MACOS)
                const quint32 scan = 0;
                const quint32 nativeKey = 56;
#else
                const quint32 scan = 50;
                const quint32 nativeKey = 0xffe1;
#endif
                QKeyEvent press(QEvent::KeyPress, Qt::Key_Shift, Qt::ShiftModifier, scan, nativeKey, 0);
                QKeyEvent release(QEvent::KeyRelease, Qt::Key_Shift, Qt::NoModifier, scan, nativeKey, 0);
                QApplication::sendEvent(input, &press);
                QApplication::sendEvent(input, &release);
            }
            transition(Phase::Input);
            break;
        }
        case Phase::Input: {
            auto *surface = visibleSurface();
            if (!surface)
                break;
            const bool acknowledged = surface->property("displayInputAcknowledgements").toULongLong() >= m_acknowledgements + 2;
            if (configuration.value("allow_input").toBool() && !acknowledged)
                break;
            m_receipt["input_enabled"] = configuration.value("allow_input");
            m_receipt["input_acknowledged"] = acknowledged;
            m_displayId.clear();
            const QString expected = configuration.value("surface_id").toString() + '/' + resource;
            for (const auto &display : m_window.m_dashboardFactory->packageDisplays()) {
                if (display.id == expected && display.leaseId == resource)
                    m_displayId = display.id;
            }
            if (m_displayId.isEmpty()) {
                finish(false, "The installed dashboard inventory has no matching qualified display identity.");
                return;
            }
            if (!chooseDashboard()) {
                finish(false, "Default dashboard was unavailable in the real MainWindow.");
                return;
            }
            if (!selectEnvironment("canvas")) {
                finish(false, "The Canvas mode control was unavailable.");
                return;
            }
            if (auto *button = m_dashboard->findChild<QAbstractButton *>("dashboardAddWidget"))
                button->click();
            transition(Phase::CanvasPicker);
            break;
        }
        case Phase::CanvasPicker:
            recordPickerDiagnostic(false);
            if (m_clock.elapsed() - m_phaseSince > 15000) {
                finish(false, "Canvas picker did not expose the requested live display within 15 seconds.");
                return;
            }
            if (chooseDisplayFromPicker(false)) {
                m_receipt["canvas_picker_used"] = true;
                transition(Phase::Canvas);
            }
            break;
        case Phase::Canvas:
            if (!readyForCapture())
                break;
            if (!capture("canvas")) {
                finish(false, "Canvas did not render nonuniform native pixels.");
                return;
            }
            if (!closeDisplayPanel() || !selectEnvironment("tiling")) {
                finish(false, "The Canvas panel close or Tiling mode control failed.");
                return;
            }
            if (auto *button = m_dashboard->findChild<QAbstractButton *>("dashboardDockApps"))
                button->click();
            transition(Phase::TilingPicker);
            break;
        case Phase::TilingPicker:
            recordPickerDiagnostic(true);
            if (m_clock.elapsed() - m_phaseSince > 15000) {
                finish(false, "Tiling Apps picker did not expose the requested live display within 15 seconds.");
                return;
            }
            if (chooseDisplayFromPicker(true)) {
                m_receipt["tiling_apps_picker_used"] = true;
                transition(Phase::Tiling);
            }
            break;
        case Phase::Tiling:
            if (!readyForCapture())
                break;
            if (!capture("tiling")) {
                finish(false, "Tiling did not render nonuniform native pixels.");
                return;
            }
            m_window.selectPage(m_window.m_stack->indexOf(m_surfacePage));
            transition(Phase::Return);
            break;
        case Phase::Return:
            if (!readyForCapture())
                break;
            if (!capture("returned")) {
                finish(false, "Returning to the provider page did not render native pixels.");
                return;
            }
            m_window.selectPage(m_window.m_stack->indexOf(m_dashboard));
            transition(Phase::Closed);
            break;
        case Phase::Closed:
            if (!readyForCapture())
                break;
            if (!closeDisplayPanel()) {
                finish(false, "The Tiling panel close control was unavailable.");
                return;
            }
            transition(Phase::Disconnected);
            break;
        case Phase::Disconnected:
            if (loadedViews() != 0)
                break;
            m_receipt["panel_close_disconnected"] = true;
            m_window.selectPage(m_window.m_stack->indexOf(m_surfacePage));
            transition(Phase::Reopen);
            break;
        case Phase::Reopen:
            if (!readyForCapture())
                break;
            if (!capture("reopened")) {
                finish(false, "Display resource did not survive panel close/reopen.");
                return;
            }
            finish(m_receipt.value("panel_close_disconnected").toBool());
            break;
        }
    }

    QApplication &m_application;
    MainWindow &m_window;
    QTimer m_timer;
    QElapsedTimer m_clock;
    QJsonObject m_receipt;
    QJsonObject m_lastPickerDiagnostic;
    QPointer<PackageSurfacePage> m_surfacePage;
    QPointer<DashboardPage> m_dashboard;
    Phase m_phase = Phase::Discover;
    qint64 m_readySince = -1;
    qint64 m_phaseSince = 0;
    quint64 m_acknowledgements = 0;
    QString m_displayId;
    bool m_finished = false;
};

int runPackageDisplayQualification(QApplication &application)
{
    const QString output = configuration.value("output").toString();
    if (QFileInfo::exists(output) || !QDir().mkpath(output)) {
        QTextStream(stderr) << "Qualification output must be a fresh directory.\n";
        return 2;
    }
    LauncherSettings settings;
    settings.load();
    settings.projectFolder = configuration.value("workspace").toString();
    settings.defaultDashboardVisible = true;
    settings.sidebarVisible = true;
    settings.packageRemotes.clear();
    if (!settings.save() || !LauncherSettings::saveDashboardTourState(true, true)) {
        QTextStream(stderr) << "Cannot initialize isolated qualification settings.\n";
        return 2;
    }
    MainWindow window;
    window.resize(1280, 900);
    PackageDisplayQualification qualification(application, window);
    window.show();
    return application.exec();
}
