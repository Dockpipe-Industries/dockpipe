#include "DashboardPage.h"
#include "DashboardDock.h"
#include <QCursor>
#include <QScreen>
#include "DashboardFilesWidget.h"
#include <QSignalSpy>
#include "DashboardTourDialog.h"
#include "LauncherSettings.h"
#include <QRadioButton>
#include <QScopeGuard>
#include "DashboardStore.h"
#include "DashboardWidgetFactory.h"
#include "DashboardWorkspace.h"
#include "CanvasPanel.h"
#include "ContextStore.h"
#include "SessionManager.h"
#include "Theme.h"
#include "DesktopWindowWidget.h"
#include "DesktopApplicationCatalog.h"

#include <QApplication>
#include <QCheckBox>
#include <QComboBox>
#include <QDialog>
#include <QDialogButtonBox>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QFileInfo>
#include <QImage>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLabel>
#include <QLineEdit>
#include <QMenu>
#include <QPointer>
#include <QProcess>
#include <QPushButton>
#include <QTemporaryDir>
#include <QTest>
#include <QTimer>
#include <QToolButton>
#include <QTabBar>
#include <QTabWidget>
#include <memory>

#ifdef DOCKPIPE_X11_BILLBOARDS
#include <X11/Xatom.h>
#include <X11/Xlib.h>
#endif

namespace {
DashboardDefinition emptyDashboard()
{
    DashboardDefinition definition;
    definition.id = DashboardStore::newId();
    definition.name = QStringLiteral("Development workspace");
    return definition;
}

void drainEvents()
{
    QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
    QCoreApplication::processEvents(QEventLoop::AllEvents, 50);
}

bool captureIfRequested(QWidget &widget, const QString &name)
{
    const QString directory = qEnvironmentVariable("DASHBOARD_PAGE_SCREENSHOT_DIR");
    if (directory.isEmpty())
        return true;
    if (!QDir().mkpath(directory))
        return false;
    drainEvents();
    return widget.grab().save(QDir(directory).filePath(name + ".png"));
}

QColor pixelAt(const QImage &image, const QPoint &position)
{
    return image.pixelColor(position * image.devicePixelRatio());
}

bool isolatedXvfbDisplay()
{
#ifdef Q_OS_LINUX
    const QByteArray display = qgetenv("DISPLAY").split('.').first();
    if (!display.startsWith(':') || display.size() < 2)
        return false;
    bool numeric = false;
    display.mid(1).toInt(&numeric);
    if (!numeric)
        return false;
    // An opt-in alone must not allow this fixture to acquire real desktop apps.
    // Match the selected local X display to an actual Xvfb server process.
    for (const QString &entry : QDir("/proc").entryList(QDir::Dirs | QDir::NoDotAndDotDot)) {
        entry.toUInt(&numeric);
        if (!numeric)
            continue;
        QFile command("/proc/" + entry + "/cmdline");
        if (!command.open(QIODevice::ReadOnly))
            continue;
        const auto arguments = command.read(8192).split('\0');
        if (!arguments.isEmpty() && QFileInfo(QString::fromLocal8Bit(arguments.first())).fileName() == "Xvfb"
            && arguments.contains(display))
            return true;
    }
#endif
    return false;
}

class FixtureProcess final : public QProcess {
public:
    ~FixtureProcess() override
    {
        if (state() == NotRunning)
            return;
        terminate();
        if (!waitForFinished(2000)) {
            kill();
            waitForFinished(1000);
        }
    }
};

#ifdef Q_OS_LINUX
class IsolatedDesktopLaunch final {
public:
    IsolatedDesktopLaunch()
    {
        for (const QByteArray &key : {QByteArray("PATH"), QByteArray("XDG_DATA_HOME"),
                 QByteArray("XDG_DATA_DIRS"), QByteArray("DOCKPIPE_TEST_LAUNCH_MARKER")}) {
            m_environment.append({key, qgetenv(key)});
        }
    }

    ~IsolatedDesktopLaunch()
    {
        for (const auto &entry : m_environment) {
            if (entry.second.isNull())
                qunsetenv(entry.first.constData());
            else
                qputenv(entry.first.constData(), entry.second);
        }
    }

    bool initialize()
    {
        if (!m_directory.isValid() || !QDir().mkpath(m_directory.filePath("applications")))
            return false;
        QFile desktop(m_directory.filePath("applications/dashboard-launch-fixture.desktop"));
        if (!desktop.open(QIODevice::WriteOnly))
            return false;
        desktop.write("[Desktop Entry]\nType=Application\nName=Dashboard launch fixture\n"
            "Exec=/bin/true\nStartupWMClass=DockpipeLaunchFixture\n");
        desktop.close();
        if (!configureLauncher())
            return false;
        qputenv("PATH", m_directory.path().toUtf8());
        qputenv("XDG_DATA_HOME", m_directory.path().toUtf8());
        qputenv("XDG_DATA_DIRS", m_directory.filePath("empty").toUtf8());
        qputenv("DOCKPIPE_TEST_LAUNCH_MARKER", markerPath().toUtf8());
        QString error;
        const auto entries = DesktopApplicationCatalog::enumerate(&error);
        if (!error.isEmpty() || entries.size() != 1)
            return false;
        application = entries.first();
        return true;
    }

    bool configureLauncher(int exitCode = 0, int delayMilliseconds = 0)
    {
        QFile launcher(m_directory.filePath("gio"));
        if (!launcher.open(QIODevice::WriteOnly | QIODevice::Truncate))
            return false;
        // Dispatch metadata is the only effect. Native fixture processes are
        // owned separately; this launcher cannot start a real desktop app.
        QByteArray script = "#!/bin/sh\nprintf '%s\\n' \"$2\" >> \"$DOCKPIPE_TEST_LAUNCH_MARKER\"\n";
        if (delayMilliseconds > 0)
            script += "/bin/sleep " + QByteArray::number(delayMilliseconds / 1000.0, 'f', 3) + '\n';
        script += "exit " + QByteArray::number(exitCode) + '\n';
        if (launcher.write(script) != script.size())
            return false;
        launcher.close();
        return launcher.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner);
    }

    int dispatchCount() const
    {
        QFile marker(markerPath());
        if (!marker.open(QIODevice::ReadOnly))
            return 0;
        return marker.read(4096).count('\n');
    }

    DesktopApplicationEntry application;

private:
    QString markerPath() const { return m_directory.filePath("dispatched"); }
    QTemporaryDir m_directory;
    QVector<QPair<QByteArray, QByteArray>> m_environment;
};
#endif

#ifdef DOCKPIPE_X11_BILLBOARDS
class IsolatedClientList final {
public:
    bool install(quint64 nativeId)
    {
        m_display = XOpenDisplay(nullptr);
        if (!m_display)
            return false;
        m_root = DefaultRootWindow(m_display);
        m_property = XInternAtom(m_display, "_NET_CLIENT_LIST", False);
        Atom type = None;
        int format = 0;
        unsigned long count = 0;
        unsigned long remaining = 0;
        unsigned char *data = nullptr;
        const int result = XGetWindowProperty(m_display, m_root, m_property, 0, 128, False,
            AnyPropertyType, &type, &format, &count, &remaining, &data);
        const bool valid = result == Success && remaining == 0
            && (type == None || (type == XA_WINDOW && format == 32));
        if (valid && type == XA_WINDOW) {
            m_hadProperty = true;
            const auto *windows = reinterpret_cast<unsigned long *>(data);
            for (unsigned long index = 0; index < count; ++index)
                m_previous.append(windows[index]);
        }
        if (data)
            XFree(data);
        if (!valid)
            return false;
        const unsigned long window = static_cast<unsigned long>(nativeId);
        XChangeProperty(m_display, m_root, m_property, XA_WINDOW, 32, PropModeReplace,
            reinterpret_cast<const unsigned char *>(&window), 1);
        XSync(m_display, False);
        m_installed = true;
        return true;
    }

    void replace(const QVector<quint64> &ids)
    {
        QVector<unsigned long> windows;
        for (quint64 id : ids)
            windows.append(static_cast<unsigned long>(id));
        XChangeProperty(m_display, m_root, m_property, XA_WINDOW, 32, PropModeReplace,
            reinterpret_cast<const unsigned char *>(windows.constData()), int(windows.size()));
        XSync(m_display, False);
    }

    void removeProperty()
    {
        XDeleteProperty(m_display, m_root, m_property);
        XSync(m_display, False);
    }

    bool setWindowClass(quint64 id, const QByteArray &name)
    {
        const QByteArray value = name + '\0' + name + '\0';
        XChangeProperty(m_display, static_cast<Window>(id), XInternAtom(m_display, "WM_CLASS", False),
            XA_STRING, 8, PropModeReplace, reinterpret_cast<const unsigned char *>(value.constData()), value.size());
        XSync(m_display, False);
        return true;
    }

    void removeProcessId(quint64 id)
    {
        XDeleteProperty(m_display, static_cast<Window>(id), XInternAtom(m_display, "_NET_WM_PID", False));
        XSync(m_display, False);
    }

    void setShadowMargins(quint64 id, bool invalid)
    {
        const unsigned long margins[] = {invalid ? 0xffffffffUL : 16, 16, 10, 32};
        XChangeProperty(m_display, static_cast<Window>(id), XInternAtom(m_display, "_GTK_FRAME_EXTENTS", False),
            XA_CARDINAL, 32, PropModeReplace, reinterpret_cast<const unsigned char *>(margins), 4);
        XSync(m_display, False);
    }

    QRect geometry(quint64 id) const
    {
        XWindowAttributes attributes{};
        if (!XGetWindowAttributes(m_display, static_cast<Window>(id), &attributes))
            return {};
        return QRect(attributes.x, attributes.y, attributes.width, attributes.height);
    }

    bool pointerTargets(quint64 id) const
    {
        Window current = m_root;
        // Walk only the hit-tested ancestor chain. Match the native control,
        // rather than QWidget visibility, to detect occlusion by the app.
        for (int depth = 0; depth < 32; ++depth) {
            Window root = 0;
            Window child = 0;
            int rootX = 0;
            int rootY = 0;
            int localX = 0;
            int localY = 0;
            unsigned int mask = 0;
            if (!XQueryPointer(m_display, current, &root, &child,
                    &rootX, &rootY, &localX, &localY, &mask) || !child)
                return false;
            if (child == static_cast<Window>(id))
                return true;
            if (child == current)
                return false;
            current = child;
        }
        return false;
    }

    quint64 parentWindow(quint64 id) const
    {
        Window root = 0;
        Window parent = 0;
        Window *children = nullptr;
        unsigned int count = 0;
        const bool found = XQueryTree(m_display, static_cast<Window>(id), &root, &parent, &children, &count);
        if (children)
            XFree(children);
        return found ? parent : 0;
    }

    bool viewable(quint64 id) const
    {
        XWindowAttributes attributes{};
        return XGetWindowAttributes(m_display, static_cast<Window>(id), &attributes)
            && attributes.map_state == IsViewable;
    }

    bool descendantOf(quint64 id, quint64 ancestor) const
    {
        for (int depth = 0; depth < 32 && id; ++depth) {
            if (id == ancestor)
                return true;
            const quint64 parent = parentWindow(id);
            if (parent == id)
                break;
            id = parent;
        }
        return false;
    }

    ~IsolatedClientList()
    {
        if (!m_display)
            return;
        if (m_installed) {
            if (m_hadProperty) {
                XChangeProperty(m_display, m_root, m_property, XA_WINDOW, 32, PropModeReplace,
                    reinterpret_cast<const unsigned char *>(m_previous.constData()), int(m_previous.size()));
            } else {
                XDeleteProperty(m_display, m_root, m_property);
            }
            XSync(m_display, False);
        }
        XCloseDisplay(m_display);
    }

private:
    Display *m_display = nullptr;
    Window m_root = 0;
    Atom m_property = 0;
    QVector<unsigned long> m_previous;
    bool m_hadProperty = false;
    bool m_installed = false;
};
#endif
}

class DashboardPageTest final : public QObject {
    Q_OBJECT
private slots:
    void independentEnvironmentsAndRoundTrip();
    void optionsAndSettingsControls();
    void repeatedSwitchesRetainContentAndObjects();
    void autoAcquireLifecycle_data();
    void autoAcquireLifecycle();
    void retainsAndReleasesInactiveForeignWindow();
    void dockPreferenceChangesReservedSpace();
    void desktopLaunchAdmission();
    void desktopLaunchAcquisition();
    void staleDesktopPanelsAreNotRestored();
    void tourLifecycle();
    void fileScopesAndCanvasProjectPicker();
    void desktopBackgroundLifecycle();
};

void DashboardPageTest::independentEnvironmentsAndRoundTrip()
{
    QTemporaryDir project;
    QVERIFY(project.isValid());
    QFile file(project.filePath("README.txt"));
    QVERIFY(file.open(QIODevice::WriteOnly));
    QVERIFY(file.write("Dashboard fixture project\n") > 0);
    file.close();
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    factory.setWorkdir(project.path());
    WorkflowMeta workflow;
    workflow.workflowId = "build-fixture";
    workflow.displayName = "Build project";
    factory.setWorkflows({workflow});
    DashboardPage page(emptyDashboard(), &factory);
    page.resize(1100, 780);
    page.show();
    drainEvents();
    auto *canvas = page.findChild<DashboardWorkspace *>("dashboardCanvasWorkspace");
    auto *tiling = page.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    QVERIFY(canvas && tiling);
    QCOMPARE(page.environment(), QString("canvas"));
    QVERIFY(!page.addWidgetKind("desktop-window"));
    QCOMPARE(page.panelCount(), 0);
    QVERIFY(page.addWidgetKind("files"));
    QVERIFY(page.addWidgetKind("send-input"));
    drainEvents();
    const auto canvasDefinition = page.snapshot();
    QCOMPARE(canvasDefinition.panels.size(), 2);
    for (const auto &panel : canvasDefinition.panels)
        QCOMPARE(panel.environment, QString("canvas"));
    QPointer<QWidget> files = canvas->findChild<QWidget *>("dashboardWidget_files");
    QPointer<QWidget> canvasInput = canvas->findChild<QWidget *>("dashboardWidget_send-input");
    QVERIFY(files && canvasInput);
    const auto canvasLayout = canvas->saveLayout();
    QVERIFY(captureIfRequested(page, "canvas"));
    page.setFullscreen(true);
    QVERIFY(captureIfRequested(page, "fullscreen-top-bar"));
    page.setFullscreen(false);
    drainEvents();

    page.setEnvironment("tiling");
    drainEvents();
    QCOMPARE(page.environment(), QString("tiling"));
    QVERIFY(tiling->findChildren<CanvasPanel *>().isEmpty());
    QVERIFY(!files->isVisible() && !canvasInput->isVisible());
    QVERIFY(page.addWidgetKind("workflows"));
    QVERIFY(page.addWidgetKind("send-input"));
    const auto panels = page.snapshot().panels;
    const QString workflowId = panels.at(2).id;
    const QString inputId = panels.at(3).id;
    QVERIFY(tiling->dockPanel(inputId, workflowId, "right"));
    drainEvents();
    QPointer<QWidget> workflows = tiling->findChild<QWidget *>("dashboardWidget_workflows");
    QPointer<QWidget> tilingInput = tiling->findChild<QWidget *>("dashboardWidget_send-input");
    QVERIFY(workflows && tilingInput && tilingInput != canvasInput);
    auto *bar = page.findChild<QWidget *>("dashboardTopBar");
    auto *environments = page.findChild<QWidget *>("dashboardEnvironments");
    QCOMPARE(environments->geometry().top(), bar->geometry().bottom() + 1);
    QVERIFY(captureIfRequested(page, "tiling"));
    page.setEnvironment("canvas");
    drainEvents();
    QCOMPARE(canvas->findChild<QWidget *>("dashboardWidget_files"), files.data());
    QCOMPARE(canvas->findChild<QWidget *>("dashboardWidget_send-input"), canvasInput.data());
    QCOMPARE(canvas->saveLayout(), canvasLayout);
    QVERIFY(files->isVisible() && canvasInput->isVisible());
    QVERIFY(!workflows->isVisible() && !tilingInput->isVisible());
    page.setEnvironment("invalid");
    QCOMPARE(page.environment(), QString("canvas"));
    page.setEnvironment("tiling");
    drainEvents();
    const auto saved = page.snapshot();
    QCOMPARE(saved.layout.value("version").toInt(), 2);
    QCOMPARE(saved.layout.value("mode").toString(), QString("tiling"));
    QCOMPARE(saved.layout.value("canvas").toObject(), canvasLayout);
    QCOMPARE(saved.layout.value("tiling").toObject().value("tree").toObject().value("type").toString(), QString("split"));

    DashboardStore store;
    QVector<DashboardDefinition> existing;
    QString error;
    QVERIFY2(store.load(existing, error), qPrintable(error));
    QVERIFY2(store.save({saved}, error), qPrintable(error));
    DashboardStore restoredStore;
    QVector<DashboardDefinition> readback;
    QVERIFY2(restoredStore.load(readback, error), qPrintable(error));
    QCOMPARE(readback.size(), 1);
    QCOMPARE(readback.first().layout, saved.layout);
    QCOMPARE(readback.first().panels.size(), saved.panels.size());
    for (int index = 0; index < saved.panels.size(); ++index) {
        QCOMPARE(readback.first().panels[index].id, saved.panels[index].id);
        QCOMPARE(readback.first().panels[index].kind, saved.panels[index].kind);
        QCOMPARE(readback.first().panels[index].environment, saved.panels[index].environment);
    }
    DashboardPage restored(readback.first(), &factory);
    restored.resize(page.size());
    restored.show();
    drainEvents();
    QCOMPARE(restored.environment(), QString("tiling"));
    auto *restoredCanvas = restored.findChild<DashboardWorkspace *>("dashboardCanvasWorkspace");
    auto *restoredTiling = restored.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    QCOMPARE(restoredCanvas->findChildren<CanvasPanel *>().size(), 2);
    QCOMPARE(restoredTiling->findChildren<CanvasPanel *>().size(), 2);
    QVERIFY(restoredCanvas->findChild<QWidget *>("dashboardWidget_files"));
    QVERIFY(!restoredTiling->findChild<QWidget *>("dashboardWidget_files"));
    QVERIFY(restoredTiling->findChild<QWidget *>("dashboardWidget_workflows"));
    QVERIFY(!restoredCanvas->findChild<QWidget *>("dashboardWidget_workflows"));
    // A restored inactive Canvas must keep its saved positions while Tiling is
    // shown, then apply them when its real viewport becomes available.
    QCOMPARE(restoredCanvas->saveLayout().value("canvas"), canvasLayout.value("canvas"));
    restored.setEnvironment("canvas");
    drainEvents();
    QCOMPARE(restoredCanvas->saveLayout().value("canvas"), canvasLayout.value("canvas"));
    QCOMPARE(restoredCanvas->saveLayout().value("canvasOrder"), canvasLayout.value("canvasOrder"));
}

void DashboardPageTest::optionsAndSettingsControls()
{
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    page.resize(1000, 760);
    page.show();
    drainEvents();
    auto options = page.snapshot().options;
    QVERIFY(options.compactTiling && options.showPanelHeaders);
    QVERIFY(!options.autoAcquireApps);
    QCOMPARE(options.canvasBackground, QString("grid"));
    auto *canvas = page.findChild<DashboardWorkspace *>("dashboardCanvasWorkspace");
    const QImage grid = canvas->grab().toImage();
    QVERIFY(pixelAt(grid, QPoint(32, 48)) != pixelAt(grid, QPoint(33, 49)));
    auto *tiling = page.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    page.setEnvironment("tiling");
    const QImage tilingGrid = tiling->grab().toImage();
    QCOMPARE(pixelAt(tilingGrid, QPoint(32, 48)), pixelAt(grid, QPoint(32, 48)));
    QCOMPARE(pixelAt(tilingGrid, QPoint(33, 49)), pixelAt(grid, QPoint(33, 49)));
    page.setEnvironment("canvas");
    options.canvasBackground = "plain";
    QVERIFY(page.applyOptions(options));
    const QImage plain = canvas->grab().toImage();
    QCOMPARE(pixelAt(plain, QPoint(32, 48)), pixelAt(plain, QPoint(33, 49)));
    QCOMPARE(pixelAt(tiling->grab().toImage(), QPoint(32, 48)), pixelAt(plain, QPoint(32, 48)));
    QVERIFY(pixelAt(grid, QPoint(32, 48)) != pixelAt(plain, QPoint(32, 48)));
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    options.canvasBackground = "image";
    options.canvasImagePath = directory.filePath("missing.png");
    QVERIFY(!page.applyOptions(options));
    QCOMPARE(page.snapshot().options.canvasBackground, QString("plain"));
    QVERIFY(page.snapshot().options.canvasImagePath.isEmpty());
    options.canvasBackground = "unknown";
    QVERIFY(!page.applyOptions(options));
    QCOMPARE(page.snapshot().options.canvasBackground, QString("plain"));
    QVERIFY(!page.findChild<QLabel *>("dashboardSaveError")->text().isEmpty());
    page.showSaveError({});
    auto *settings = page.findChild<QPushButton *>("dashboardSettingsButton");
    QVERIFY(settings && !settings->icon().isNull());
    QVERIFY(settings->text().isEmpty());
    QCOMPARE(settings->accessibleName(), QString("Dashboard settings"));

    bool inspected = false;
    bool captured = false;
    QTimer::singleShot(0, &page, [&]() {
        auto *dialog = page.findChild<QDialog *>("dashboardSettingsDialog");
        if (!dialog)
            return;
        auto *compact = dialog->findChild<QCheckBox *>("dashboardSettingsCompact");
        auto *headers = dialog->findChild<QCheckBox *>("dashboardSettingsHeaders");
        auto *acquire = dialog->findChild<QCheckBox *>("dashboardSettingsAutoAcquire");
        auto *background = dialog->findChild<QComboBox *>("dashboardSettingsBackground");
        auto *image = dialog->findChild<QLineEdit *>("dashboardSettingsImage");
        auto *canvasFiles = dialog->findChild<QComboBox *>("dashboardSettingsCanvasFiles");
        auto *tilingFiles = dialog->findChild<QComboBox *>("dashboardSettingsTilingFiles");
        inspected = canvasFiles && tilingFiles
            && canvasFiles->currentData() == "project" && tilingFiles->currentData() == "system"
            && compact && headers && acquire && background && image
            && compact->isChecked() && headers->isChecked() && !acquire->isChecked()
            && background->currentData() == "plain" && !image->isEnabled();
        if (inspected) {
            compact->setChecked(false);
            headers->setChecked(false);
            background->setCurrentIndex(background->findData("dusk"));
            captured = captureIfRequested(*dialog, "settings");
        }
        dialog->reject();
    });
    settings->click();
    QVERIFY(inspected && captured);
    QVERIFY(page.snapshot().options.compactTiling);
    QCOMPARE(page.snapshot().options.canvasBackground, QString("plain"));

    bool saved = false;
    QTimer::singleShot(0, &page, [&]() {
        auto *dialog = page.findChild<QDialog *>("dashboardSettingsDialog");
        if (!dialog)
            return;
        dialog->findChild<QCheckBox *>("dashboardSettingsCompact")->setChecked(false);
        dialog->findChild<QCheckBox *>("dashboardSettingsAutoHideDock")->setChecked(false);
        dialog->findChild<QCheckBox *>("dashboardSettingsHeaders")->setChecked(false);
        auto *background = dialog->findChild<QComboBox *>("dashboardSettingsBackground");
        background->setCurrentIndex(background->findData("ocean"));
        auto *canvasFiles = dialog->findChild<QComboBox *>("dashboardSettingsCanvasFiles");
        auto *tilingFiles = dialog->findChild<QComboBox *>("dashboardSettingsTilingFiles");
        canvasFiles->setCurrentIndex(canvasFiles->findData("system"));
        tilingFiles->setCurrentIndex(tilingFiles->findData("project"));
        saved = true;
        dialog->findChild<QDialogButtonBox *>()->button(QDialogButtonBox::Save)->click();
    });
    settings->click();
    QVERIFY(saved);
    const auto selected = page.snapshot().options;
    QVERIFY(!selected.compactTiling && !selected.showPanelHeaders && !selected.autoAcquireApps);
    QVERIFY(!selected.autoHideDock);
    QCOMPARE(selected.canvasBackground, QString("ocean"));
    QCOMPARE(selected.canvasFileScope, QString("system"));
    QCOMPARE(selected.tilingFileScope, QString("project"));
    DashboardStore store;
    QVector<DashboardDefinition> definitions;
    QString error;
    QVERIFY2(store.load(definitions, error), qPrintable(error));
    QVERIFY2(store.save({page.snapshot()}, error), qPrintable(error));
    QVERIFY2(store.load(definitions, error), qPrintable(error));
    const auto restored = definitions.first().options;
    QCOMPARE(restored.compactTiling, selected.compactTiling);
    QCOMPARE(restored.showPanelHeaders, selected.showPanelHeaders);
    QCOMPARE(restored.autoAcquireApps, selected.autoAcquireApps);
    QCOMPARE(restored.canvasBackground, selected.canvasBackground);
    QCOMPARE(restored.canvasImagePath, selected.canvasImagePath);
    QCOMPARE(restored.canvasFileScope, selected.canvasFileScope);
    QCOMPARE(restored.tilingFileScope, selected.tilingFileScope);
}

void DashboardPageTest::repeatedSwitchesRetainContentAndObjects()
{
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    page.resize(1100, 780);
    QVERIFY(page.addWidgetKind("workflows"));
    QVERIFY(page.addWidgetKind("send-input"));
    page.setEnvironment("tiling");
    QVERIFY(page.addWidgetKind("workflows"));
    QVERIFY(page.addWidgetKind("send-input"));
    page.show();
    QList<QPointer<QWidget>> content;
    for (const QString &name : {QString("dashboardCanvasWorkspace"), QString("dashboardTilingWorkspace")}) {
        auto *workspace = page.findChild<DashboardWorkspace *>(name);
        content.append(workspace->findChild<QWidget *>("dashboardWidget_workflows"));
        content.append(workspace->findChild<QWidget *>("dashboardWidget_send-input"));
    }
    for (int warmup = 0; warmup < 4; ++warmup) {
        page.setEnvironment("canvas");
        drainEvents();
        page.setEnvironment("tiling");
        drainEvents();
    }
    const int tilingObjects = page.findChildren<QObject *>().size();
    page.setEnvironment("canvas");
    drainEvents();
    const int canvasObjects = page.findChildren<QObject *>().size();
    QList<int> contentCounts;
    for (const auto &widget : content) {
        QVERIFY(widget);
        contentCounts.append(widget->findChildren<QObject *>().size());
    }
    QElapsedTimer elapsed;
    elapsed.start();
    for (int pair = 0; pair < 40; ++pair) {
        page.setEnvironment("tiling");
        drainEvents();
        QCOMPARE(page.findChildren<QObject *>().size(), tilingObjects);
        page.setEnvironment("canvas");
        drainEvents();
        QCOMPARE(page.findChildren<QObject *>().size(), canvasObjects);
    }
    const qint64 duration = elapsed.elapsed();
    qInfo("Dashboard Page: 80 environment switches in %lld ms; canvas objects=%d tiling objects=%d",
        static_cast<long long>(duration), canvasObjects, tilingObjects);
    QVERIFY2(duration < 5000, "80 lightweight Page environment switches exceeded the five-second regression budget");
    for (int index = 0; index < content.size(); ++index) {
        QVERIFY(content[index]);
        QCOMPARE(content[index]->findChildren<QObject *>().size(), contentCounts[index]);
        QVERIFY(page.isAncestorOf(content[index]));
    }
    QCOMPARE(page.panelCount(), 4);
}

void DashboardPageTest::dockPreferenceChangesReservedSpace()
{
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    page.resize(1000, 740);
    page.show();
    drainEvents();
    auto *dock = page.findChild<DashboardDock *>();
    auto *environments = page.findChild<QWidget *>("dashboardEnvironments");
    QVERIFY(dock && environments);
    QVERIFY(dock->isHidden());
    QCOMPARE(dock->parentWidget(), environments);
    const int fullHeight = environments->height();
    auto options = page.snapshot().options;
    options.autoHideDock = false;
    QVERIFY(page.applyOptions(options));
    drainEvents();
    QVERIFY(dock->isVisible());
    QCOMPARE(dock->parentWidget(), &page);
    QVERIFY(environments->height() < fullHeight);
    options.autoHideDock = true;
    QVERIFY(page.applyOptions(options));
    drainEvents();
    QCOMPARE(environments->height(), fullHeight);
    dock->reveal(true);
    QTRY_VERIFY(dock->isVisible());
    QVERIFY(captureIfRequested(page, "dock-overlay"));
}

void DashboardPageTest::retainsAndReleasesInactiveForeignWindow()
{
    if (qEnvironmentVariable("DOCKPIPE_ISOLATED_DESKTOP_TEST") != "1")
        QSKIP("Desktop acquisition requires explicit opt-in on a disposable Xvfb display.");
#ifndef DOCKPIPE_X11_BILLBOARDS
    QSKIP("This build has no X11 native-window backend.");
#else
    QVERIFY2(QGuiApplication::platformName() == "xcb" && isolatedXvfbDisplay(),
        "Refusing native acquisition: the selected display must be a verified local Xvfb server");
    FixtureProcess fixture;
    fixture.setProcessChannelMode(QProcess::ForwardedErrorChannel);
    fixture.start(QCoreApplication::applicationDirPath() + "/desktop-app-window-test", {"--fixture"});
    QVERIFY(fixture.waitForStarted(3000));
    QByteArray output;
    QTRY_VERIFY_WITH_TIMEOUT((output += fixture.readAllStandardOutput()).startsWith("WINDOW:")
        && output.contains('\n'), 5000);
    const quint64 nativeId = output.split('\n').first().mid(7).toULongLong();
    QVERIFY(nativeId);
    IsolatedClientList clients;
    QVERIFY(clients.install(nativeId));
    const quint64 originalParent = clients.parentWindow(nativeId);
    QVERIFY(originalParent);
    QTRY_VERIFY_WITH_TIMEOUT(clients.viewable(nativeId), 5000);

    auto ownedWorkspace = std::make_unique<DashboardWorkspace>("deferred-native-regression");
    QPointer<DashboardWorkspace> workspace = ownedWorkspace.get();
    workspace->resize(1100, 760);
    QPointer<DesktopWindowWidget> native = new DesktopWindowWidget;
    QVERIFY(workspace->addPanel("native", "Native fixture", native));
    QVERIFY(workspace->addPanel("other", "Ordinary panel", new QLabel("Retained ordinary panel")));
    workspace->show();
    QString error;
    QVERIFY2(native->acquireWindow(nativeId, error), qPrintable(error));
    QTRY_VERIFY_WITH_TIMEOUT(native->hasInteractiveWindow(), 5000);
    QTRY_VERIFY_WITH_TIMEOUT(clients.descendantOf(nativeId, workspace->winId()), 5000);
    workspace->setMode("tiling");
    QVERIFY(workspace->dockPanel("native", "other", "tab"));
    QVERIFY(workspace->activatePanel("other"));
    workspace->setMode("canvas");
    workspace->setMode("tiling");
    QVERIFY(workspace->findChild<QWidget *>("dashboardDeferredTab"));
    QVERIFY(!native->isVisible());
    QCOMPARE(native->acquiredWindowId(), nativeId);
    QTRY_VERIFY_WITH_TIMEOUT(!clients.viewable(nativeId), 5000);
    auto *monitor = native->findChild<QTimer *>("desktopNativeMonitor");
    QVERIFY(monitor);
    QTRY_VERIFY_WITH_TIMEOUT(!monitor->isActive(), 5000);
    QCOMPARE(fixture.state(), QProcess::Running);

    auto *group = workspace->findChild<QTabWidget *>("dashboardTabGroup");
    QVERIFY(group && group->count() == 2);
    int nativeIndex = -1;
    for (int index = 0; index < group->count(); ++index) {
        if (group->tabText(index) == "Native fixture")
            nativeIndex = index;
    }
    QVERIFY(nativeIndex >= 0 && nativeIndex != group->currentIndex());
    QElapsedTimer activation;
    activation.start();
    QTest::mouseClick(group->tabBar(), Qt::LeftButton, {}, group->tabBar()->tabRect(nativeIndex).center());
    QTRY_VERIFY_WITH_TIMEOUT(native->isVisible() && native->hasInteractiveWindow()
        && clients.viewable(nativeId) && clients.descendantOf(nativeId, workspace->winId()), 5000);
    qInfo() << "Deferred native first activation ms" << activation.elapsed();
    QCOMPARE(native->acquiredWindowId(), nativeId);
    QCOMPARE(fixture.state(), QProcess::Running);

    for (int cycle = 0; cycle < 2; ++cycle) {
        for (const QString &mode : {QStringLiteral("canvas"), QStringLiteral("tiling")}) {
            workspace->setMode(mode);
            QTRY_VERIFY_WITH_TIMEOUT(native->isVisible() && native->hasInteractiveWindow()
                && clients.viewable(nativeId) && clients.descendantOf(nativeId, workspace->winId()), 5000);
            QCOMPARE(native->acquiredWindowId(), nativeId);
            QCOMPARE(fixture.state(), QProcess::Running);
        }
        QVERIFY(workspace->minimizePanel("native"));
        QTRY_VERIFY_WITH_TIMEOUT(!native->isVisible() && !clients.viewable(nativeId), 5000);
        QCOMPARE(native->acquiredWindowId(), nativeId);
        QCOMPARE(fixture.state(), QProcess::Running);
        QVERIFY(workspace->activatePanel("native"));
        QTRY_VERIFY_WITH_TIMEOUT(native->isVisible() && native->hasInteractiveWindow()
            && clients.viewable(nativeId), 5000);
        QCOMPARE(native->acquiredWindowId(), nativeId);
    }
    workspace->setAttribute(Qt::WA_DeleteOnClose);
    QVERIFY(workspace->close());
    ownedWorkspace.release();
    QTRY_VERIFY_WITH_TIMEOUT(workspace.isNull() && native.isNull(), 5000);
    QTRY_VERIFY_WITH_TIMEOUT(clients.parentWindow(nativeId) == originalParent
        && clients.viewable(nativeId), 5000);
    QCOMPARE(fixture.state(), QProcess::Running);
    qInfo() << "Native client survived deferred tab activation, modes, minimize/restore and workspace close";
#endif
}

void DashboardPageTest::autoAcquireLifecycle_data()
{
    QTest::addColumn<bool>("omitProcessId");
    QTest::addColumn<int>("shadowMode");
    QTest::newRow("published-process-id") << false << 0;
#ifdef DOCKPIPE_X11_RESOURCE_IDENTITIES
    QTest::newRow("server-process-id") << true << 0;
#endif
    QTest::newRow("client-shadow-margins") << false << 1;
    QTest::newRow("invalid-shadow-margins") << false << 2;
}

void DashboardPageTest::autoAcquireLifecycle()
{
    QFETCH(bool, omitProcessId);
    QFETCH(int, shadowMode);
    if (qEnvironmentVariable("DOCKPIPE_ISOLATED_DESKTOP_TEST") != "1")
        QSKIP("Desktop acquisition requires explicit opt-in on a disposable Xvfb display.");
#ifndef DOCKPIPE_X11_BILLBOARDS
    QSKIP("This build has no X11 native-window backend.");
#else
    QVERIFY2(QGuiApplication::platformName() == "xcb" && isolatedXvfbDisplay(),
        "Refusing native acquisition: the selected display must be a verified local Xvfb server");
    const QString unavailable = DesktopWindowWidget::automaticAcquisitionUnavailableReason();
    QVERIFY2(unavailable.isEmpty(), qPrintable(unavailable));
    FixtureProcess fixture;
    fixture.setProcessChannelMode(QProcess::ForwardedErrorChannel);
    fixture.start(QCoreApplication::applicationDirPath() + "/desktop-app-window-test", {"--fixture", "--reject-first-close"});
    QVERIFY(fixture.waitForStarted(3000));
    QByteArray output;
    QTRY_VERIFY_WITH_TIMEOUT((output += fixture.readAllStandardOutput()).startsWith("WINDOW:") && output.contains('\n'), 5000);
    const quint64 nativeId = output.split('\n').first().mid(7).toULongLong();
    QVERIFY(nativeId);
    IsolatedClientList clients;
    QVERIFY(clients.install(nativeId));
    if (omitProcessId)
        clients.removeProcessId(nativeId);
    if (shadowMode)
        clients.setShadowMargins(nativeId, shadowMode == 2);
    const auto available = [nativeId]() {
        QString error;
        const auto windows = DesktopWindowWidget::availableWindows(error);
        return error.isEmpty() && windows.size() == 1 && windows.first().id == nativeId;
    };
    // Never opt into auto acquisition while another external window is present.
    QTRY_VERIFY_WITH_TIMEOUT(available(), 5000);
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    page.resize(1000, 740);
    page.show();
    page.setEnvironment("tiling");
    drainEvents();
    QVERIFY(!page.snapshot().options.autoAcquireApps);
    QCOMPARE(page.panelCount(), 0);
    QVERIFY(available());
    auto options = page.snapshot().options;
    options.autoAcquireApps = true;
    QVERIFY(page.applyOptions(options));
    QTRY_COMPARE_WITH_TIMEOUT(page.panelCount(), 1, 5000);
    QPointer<DesktopWindowWidget> acquired = page.findChild<DesktopWindowWidget *>();
    QVERIFY(acquired);
    QTRY_VERIFY_WITH_TIMEOUT(acquired->hasInteractiveWindow(), 5000);
    QCOMPARE(acquired->acquiredWindowId(), nativeId);
    auto *dock = page.findChild<DashboardDock *>();
    auto *edge = page.findChild<QWidget *>("dashboardDockRevealEdge");
    QVERIFY(dock && edge);
    QCursor::setPos(edge->mapToGlobal(edge->rect().center()));
    QTRY_VERIFY_WITH_TIMEOUT(dock->isVisible(), 1500);
    QTRY_VERIFY(dock->geometry().bottom() < dock->parentWidget()->height());
    const QString screenshotDirectory = qEnvironmentVariable("DASHBOARD_PAGE_SCREENSHOT_DIR");
    if (!screenshotDirectory.isEmpty()) {
        QDir().mkpath(screenshotDirectory);
        QTest::qWait(200);
        QVERIFY(page.screen()->grabWindow(0).save(screenshotDirectory + "/native-dock-overlay.png"));
    }
    QCursor::setPos(page.mapToGlobal(QPoint(20, 20)));
    QTRY_VERIFY_WITH_TIMEOUT(dock->isHidden(), 1500);
    auto *nativeContainer = acquired->findChild<QWidget *>("billboardNativeApp");
    QVERIFY(nativeContainer);
    QTRY_COMPARE(nativeContainer->size(), acquired->size());
    QCOMPARE(nativeContainer->mapTo(acquired, QPoint()), QPoint());
    const auto expectedGeometry = [&]() {
        const qreal scale = nativeContainer->devicePixelRatioF();
        const QSize size(qRound(nativeContainer->width() * scale), qRound(nativeContainer->height() * scale));
        return shadowMode == 1 ? QRect(QPoint(-16, -10), size + QSize(32, 42)) : QRect(QPoint(), size);
    };
    QTRY_COMPARE(clients.geometry(nativeId), expectedGeometry());
    QVERIFY(edge->isHidden());
    const QPoint bottomPixel = acquired->mapToGlobal(QPoint(acquired->width() / 2, acquired->height() - 2));
    const QColor bottomColor = page.screen()->grabWindow(0, bottomPixel.x(), bottomPixel.y(), 1, 1).toImage().pixelColor(0, 0);
    QVERIFY2(bottomColor.blue() > bottomColor.red(), "The dock hotspot must not paint a gray strip over the native app");
    auto *pane = qobject_cast<QWidget *>(acquired->parent());
    QVERIFY(pane);
    QCOMPARE(acquired->width(), pane->width());
    const QString panelId = pane->objectName().mid(QString("dashboardPanel_").size());
    auto *workspace = page.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    QVERIFY(page.addWidgetKind("workflows"));
    const QString neighborId = page.snapshot().panels.last().id;
    QTRY_COMPARE(clients.geometry(nativeId), expectedGeometry());
    const QSize splitSize = acquired->size();
    auto *tileActions = pane->findChild<QToolButton *>("dashboardTileActions");
    QVERIFY(tileActions);
    const auto openTileMenu = [&]() {
        QCursor::setPos(pane->mapToGlobal(QPoint(pane->width() / 2, tileActions->height() / 2)));
        QTest::mouseClick(tileActions, Qt::LeftButton);
        return qobject_cast<QMenu *>(QApplication::activePopupWidget());
    };
    const auto findAction = [](QMenu *menu, const QString &text) -> QAction * {
        if (!menu)
            return nullptr;
        for (QAction *action : menu->actions()) {
            if (action->text() == text)
                return action;
        }
        return nullptr;
    };
    QCursor::setPos(pane->mapToGlobal(QPoint(pane->width() / 2, tileActions->height() / 2)));
    QTRY_VERIFY_WITH_TIMEOUT(tileActions->isVisible(), 1500);
    QVERIFY(tileActions->testAttribute(Qt::WA_NativeWindow));
    QTRY_VERIFY_WITH_TIMEOUT(clients.pointerTargets(tileActions->winId()), 1500);
    QMenu *tileMenu = openTileMenu();
    QVERIFY(tileMenu);
    auto *expand = findAction(tileMenu, "Expand panel");
    QVERIFY(expand);
    tileMenu->close();
    expand->trigger();
    QTRY_VERIFY(acquired->width() > splitSize.width());
    QTRY_COMPARE(clients.geometry(nativeId), expectedGeometry());
    tileMenu = openTileMenu();
    QVERIFY(tileMenu);
    auto *restore = findAction(tileMenu, "Restore panel");
    QVERIFY(restore);
    tileMenu->close();
    restore->trigger();
    QTRY_COMPARE(acquired->size(), splitSize);
    QTRY_COMPARE(clients.geometry(nativeId), expectedGeometry());
    tileActions->clearFocus();
    QCursor::setPos(acquired->mapToGlobal(acquired->rect().center()));
    QTRY_VERIFY_WITH_TIMEOUT(tileActions->isHidden(), 1500);
    QTRY_VERIFY_WITH_TIMEOUT(clients.pointerTargets(nativeId), 1500);
    workspace->panelCloseRequested(neighborId);
    QTRY_COMPARE(clients.geometry(nativeId), expectedGeometry());
    QVERIFY(workspace->minimizePanel(panelId));
    QVERIFY(!acquired->isVisible());
    QVERIFY(!available());
    QVERIFY(workspace->activatePanel(panelId));
    QTRY_VERIFY(acquired->hasInteractiveWindow());
    QVERIFY(!acquired->acquiredWindowTitle().isEmpty());
    QVERIFY(!acquired->applicationIcon().isNull());
    QVERIFY(page.snapshot().panels.isEmpty());
    QVERIFY(page.snapshot().layout.value("tiling").toObject().value("tree").toObject().isEmpty());

    auto *monitor = acquired->findChild<QTimer *>("desktopNativeMonitor");
    QVERIFY(monitor);
    page.setEnvironment("canvas");
    QVERIFY(acquired && !acquired->isVisible());
    QCOMPARE(page.panelCount(), 1);
    QCOMPARE(acquired->acquiredWindowId(), nativeId);
    QTRY_VERIFY(!monitor->isActive());
    QVERIFY(!available());
    page.hide();
    page.show();
    drainEvents();
    QCOMPARE(page.environment(), QString("canvas"));
    QCOMPARE(page.panelCount(), 1);
    QVERIFY(!available());
    page.setEnvironment("tiling");
    QCOMPARE(page.findChild<DesktopWindowWidget *>(), acquired.data());
    QTRY_VERIFY_WITH_TIMEOUT(acquired->hasInteractiveWindow(), 5000);
    QCOMPARE(acquired->acquiredWindowId(), nativeId);
    // Navigating to another page retains it. Closing the launcher explicitly
    // releases all desktop panels before hiding or destroying the host.
    page.hide();
    QTRY_VERIFY(!monitor->isActive());
    QVERIFY(acquired && !available());
    page.show();
    QTRY_VERIFY_WITH_TIMEOUT(acquired->hasInteractiveWindow(), 5000);
    QCursor::setPos(pane->mapToGlobal(QPoint(pane->width() / 2, tileActions->height() / 2)));
    QTRY_VERIFY_WITH_TIMEOUT(tileActions->isVisible(), 1500);
    QTRY_VERIFY_WITH_TIMEOUT(clients.pointerTargets(tileActions->winId()), 1500);
    tileMenu = openTileMenu();
    QVERIFY(tileMenu);
    auto *release = findAction(tileMenu, "Release to desktop");
    QVERIFY(release);
    tileMenu->close();
    release->trigger();
    QTRY_VERIFY(acquired.isNull());
    QCOMPARE(page.panelCount(), 0);
    QTRY_VERIFY_WITH_TIMEOUT(available(), 5000);
    QCOMPARE(fixture.state(), QProcess::Running);
    page.setEnvironment("canvas");
    page.setEnvironment("tiling");
    acquired = page.findChild<DesktopWindowWidget *>();
    QVERIFY(acquired);
    QTRY_VERIFY_WITH_TIMEOUT(acquired->hasInteractiveWindow(), 5000);
    QVERIFY(page.addWidgetKind("workflows"));
    const QString retainedPanelId = page.snapshot().panels.last().id;
    page.releaseDesktopWindows();
    QTRY_VERIFY(acquired.isNull());
    QCOMPARE(page.panelCount(), 1);
    QCOMPARE(page.snapshot().panels.first().id, retainedPanelId);
    QTRY_VERIFY_WITH_TIMEOUT(available(), 5000);
    QCOMPARE(fixture.state(), QProcess::Running);
    // Shutdown can reach closeEvent, aboutToQuit, and the destructor. Releasing
    // more than once must keep the other app and ordinary dashboard panels.
    page.releaseDesktopWindows();
    QCOMPARE(page.panelCount(), 1);
    QVERIFY(available());
    page.hide();
    page.show();
    QTRY_COMPARE_WITH_TIMEOUT(page.panelCount(), 2, 5000);
    acquired = page.findChild<DesktopWindowWidget *>();
    QVERIFY(acquired);
    QTRY_VERIFY_WITH_TIMEOUT(acquired->hasInteractiveWindow(), 5000);
    workspace->panelCloseRequested(retainedPanelId);
    options.autoAcquireApps = false;
    QVERIFY(page.applyOptions(options));
    QTRY_VERIFY(acquired.isNull());
    QCOMPARE(page.panelCount(), 0);
    QTRY_VERIFY_WITH_TIMEOUT(available(), 5000);
    page.setEnvironment("canvas");
    page.setEnvironment("tiling");
    drainEvents();
    QCOMPARE(page.panelCount(), 0);
    QVERIFY(available());
    auto *closingPage = new DashboardPage(emptyDashboard(), &factory);
    closingPage->show();
    closingPage->setEnvironment("tiling");
    options.autoAcquireApps = true;
    QVERIFY(closingPage->applyOptions(options));
    QPointer<DesktopWindowWidget> closingWindow = closingPage->findChild<DesktopWindowWidget *>();
    QVERIFY(closingWindow);
    // Switching immediately, before native attachment finishes, must also keep
    // the app owned without consuming its visible attachment timeout.
    closingPage->setEnvironment("canvas");
    QTest::qWait(3200);
    QVERIFY(closingWindow);
    QCOMPARE(closingWindow->acquiredWindowId(), nativeId);
    QVERIFY(!closingWindow->findChild<QTimer *>("desktopNativeMonitor")->isActive());
    QVERIFY(!available());
    closingPage->setEnvironment("tiling");
    QTRY_VERIFY_WITH_TIMEOUT(closingWindow->hasInteractiveWindow(), 5000);
    closingPage->setEnvironment("canvas");
    QVERIFY(closingWindow && !closingWindow->isVisible());
    delete closingPage;
    QVERIFY(closingWindow.isNull());
    QTRY_VERIFY_WITH_TIMEOUT(available(), 5000);
    QCOMPARE(fixture.state(), QProcess::Running);
    // Only an explicit user close in managed Tiling sends the OS close request.
    // The first request is rejected by the fixture, like cancelling a save prompt.
    options.autoAcquireApps = true;
    QVERIFY(page.applyOptions(options));
    acquired = page.findChild<DesktopWindowWidget *>();
    QVERIFY(acquired);
    QTRY_VERIFY_WITH_TIMEOUT(acquired->hasInteractiveWindow(), 5000);
    auto *managedWorkspace = page.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    const QString managedId = acquired->parentWidget()->objectName().mid(QString("dashboardPanel_").size());
    managedWorkspace->panelCloseRequested(managedId);
    QTRY_VERIFY_WITH_TIMEOUT((output += fixture.readAllStandardOutput()).contains("CLOSE:cancelled"), 3000);
    QVERIFY(acquired && acquired->hasInteractiveWindow());
    QCOMPARE(page.panelCount(), 1);
    QCOMPARE(fixture.state(), QProcess::Running);
    managedWorkspace->panelCloseRequested(managedId);
    QTRY_COMPARE_WITH_TIMEOUT(fixture.state(), QProcess::NotRunning, 3000);
    QTRY_VERIFY_WITH_TIMEOUT(acquired.isNull(), 3000);
    QCOMPARE(page.panelCount(), 0);
    QVERIFY(page.findChild<QLabel *>("dashboardSaveError")->isHidden());
#endif
}

void DashboardPageTest::desktopLaunchAdmission()
{
#ifndef Q_OS_LINUX
    QSKIP("Isolated gio dispatch is Linux-specific.");
#else
    IsolatedDesktopLaunch launch;
    QVERIFY(launch.initialize());
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    page.show();
    QVERIFY(!page.launchDesktopApplication(launch.application));
    QCOMPARE(page.panelCount(), 0);
    page.setEnvironment("tiling");
    page.setCanAddPanel([]() { return false; });
    QVERIFY(!page.launchDesktopApplication(launch.application));
    QCOMPARE(page.panelCount(), 0);
    page.setCanAddPanel({});
    for (int index = 0; index < DashboardStore::maximumPanelsPerDashboard; ++index)
        QVERIFY(page.addWidgetKind("workflows"));
    QVERIFY(!page.launchDesktopApplication(launch.application));
    QCOMPARE(page.panelCount(), DashboardStore::maximumPanelsPerDashboard);
    QTest::qWait(250);
    QCOMPARE(launch.dispatchCount(), 0);
    QVERIFY(page.findChildren<DesktopWindowWidget *>().isEmpty());
#endif
}

void DashboardPageTest::desktopLaunchAcquisition()
{
    if (qEnvironmentVariable("DOCKPIPE_ISOLATED_DESKTOP_TEST") != "1")
        QSKIP("Desktop launch acquisition requires opt-in on a disposable Xvfb display.");
#if !defined(DOCKPIPE_X11_BILLBOARDS) || !defined(Q_OS_LINUX)
    QSKIP("This fixture requires Linux X11 native window support.");
#else
    QVERIFY2(QGuiApplication::platformName() == "xcb" && isolatedXvfbDisplay(),
        "Refusing native acquisition outside a verified local Xvfb server");
    IsolatedDesktopLaunch launch;
    QVERIFY(launch.initialize());
    FixtureProcess fixture;
    fixture.setProcessChannelMode(QProcess::ForwardedErrorChannel);
    fixture.start(QCoreApplication::applicationDirPath() + "/desktop-app-window-test", {"--fixture"});
    QVERIFY(fixture.waitForStarted(3000));
    QByteArray output;
    QTRY_VERIFY_WITH_TIMEOUT((output += fixture.readAllStandardOutput()).startsWith("WINDOW:") && output.contains('\n'), 5000);
    const quint64 nativeId = output.split('\n').first().mid(7).toULongLong();
    QVERIFY(nativeId);
    IsolatedClientList clients;
    QVERIFY(clients.install(nativeId));
    QVERIFY(clients.setWindowClass(nativeId, "dockpipelaunchfixture"));
    clients.replace({});
    const auto windowIsSelectable = [nativeId]() {
        QString error;
        const auto choices = DesktopWindowWidget::availableWindows(error);
        return error.isEmpty() && choices.size() == 1 && choices.first().id == nativeId;
    };
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    page.resize(1000, 740);
    page.show();
    page.setEnvironment("tiling");
    QVERIFY(page.launchDesktopApplication(launch.application));
    QCOMPARE(page.panelCount(), 1);
    auto windows = page.findChildren<DesktopWindowWidget *>();
    QCOMPARE(windows.size(), 1);
    QPointer<DesktopWindowWidget> acquired = windows.first();
    QTRY_COMPARE_WITH_TIMEOUT(launch.dispatchCount(), 1, 3000);
    QVERIFY(!acquired->findChild<QDialog *>("desktopWindowPicker"));
    // Make the owned window discoverable only after dispatch, as a newly mapped
    // application would appear. The class intentionally differs only in case.
    clients.replace({nativeId});
    QTRY_VERIFY_WITH_TIMEOUT(acquired->hasInteractiveWindow(), 5000);
    QCOMPARE(acquired->acquiredWindowId(), nativeId);
    QVERIFY(page.snapshot().panels.isEmpty());
    QVERIFY(page.snapshot().layout.value("tiling").toObject().value("tree").toObject().isEmpty());
    QVERIFY(!acquired->findChild<QDialog *>("desktopWindowPicker"));
    auto *discovery = acquired->findChild<QTimer *>("desktopApplicationDiscovery");
    QVERIFY(discovery && !discovery->isActive());
    page.setCanAddPanel([]() { return false; });
    QVERIFY(page.launchDesktopApplication(launch.application));
    QCOMPARE(page.panelCount(), 1);
    QCOMPARE(launch.dispatchCount(), 1);
    page.setCanAddPanel({});
    auto closeDesktopPanel = [&page](DesktopWindowWidget *window) {
        auto *frame = window->parentWidget();
        const QString id = frame->objectName().mid(QString("dashboardPanel_").size());
        page.findChild<DashboardWorkspace *>("dashboardTilingWorkspace")->panelCloseRequested(id);
    };
    page.setEnvironment("canvas");
    QVERIFY(acquired && !acquired->isVisible());
    QCOMPARE(acquired->acquiredWindowId(), nativeId);
    closeDesktopPanel(acquired);
    QTRY_VERIFY_WITH_TIMEOUT(acquired.isNull(), 3000);
    QCOMPARE(page.panelCount(), 0);
    QTRY_VERIFY_WITH_TIMEOUT(windowIsSelectable(), 3000);
    QCOMPARE(fixture.state(), QProcess::Running);

    // Releasing a pending launch prevents a later matching window from being
    // acquired, including after returning to Tiling with automatic capture off.
    clients.replace({});
    QVERIFY(launch.configureLauncher(0, 500));
    page.setEnvironment("tiling");
    QVERIFY(page.launchDesktopApplication(launch.application));
    QTRY_COMPARE_WITH_TIMEOUT(launch.dispatchCount(), 2, 3000);
    bool responsive = false;
    QTimer::singleShot(0, &page, [&responsive]() { responsive = true; });
    QTRY_VERIFY_WITH_TIMEOUT(responsive, 200);
    page.setEnvironment("canvas");
    closeDesktopPanel(page.findChild<DesktopWindowWidget *>());
    clients.replace({nativeId});
    QTest::qWait(700);
    for (auto *window : page.findChildren<DesktopWindowWidget *>()) {
        auto *discovery = window->findChild<QTimer *>("desktopApplicationDiscovery");
        QVERIFY(discovery && !discovery->isActive());
        QVERIFY(!window->hasInteractiveWindow());
        QVERIFY(!window->findChild<QDialog *>("desktopWindowPicker"));
    }
    page.setEnvironment("tiling");
    QTest::qWait(700);
    QVERIFY(windowIsSelectable());
    for (auto *window : page.findChildren<DesktopWindowWidget *>())
        QVERIFY(!window->hasInteractiveWindow());

    QVERIFY(launch.configureLauncher());
    // Dock activation brings the sole existing exact-class window into Tiling
    // without dispatching another process or racing a delayed new window.
    QVERIFY(page.launchDesktopApplication(launch.application));
    QCOMPARE(launch.dispatchCount(), 2);
    const auto attachedWindow = [&page]() -> DesktopWindowWidget * {
        for (auto *window : page.findChildren<DesktopWindowWidget *>()) {
            if (window->hasInteractiveWindow())
                return window;
        }
        return nullptr;
    };
    QTRY_VERIFY_WITH_TIMEOUT(attachedWindow(), 5000);
    QCOMPARE(attachedWindow()->acquiredWindowId(), nativeId);
    QVERIFY(!attachedWindow()->findChild<QDialog *>("desktopWindowPicker"));
    page.setEnvironment("canvas");
    closeDesktopPanel(page.findChild<DesktopWindowWidget *>());
    QTRY_VERIFY_WITH_TIMEOUT(windowIsSelectable(), 3000);

    FixtureProcess secondFixture;
    secondFixture.setProcessChannelMode(QProcess::ForwardedErrorChannel);
    secondFixture.start(QCoreApplication::applicationDirPath() + "/desktop-app-window-test", {"--fixture"});
    QVERIFY(secondFixture.waitForStarted(3000));
    QByteArray secondOutput;
    QTRY_VERIFY_WITH_TIMEOUT((secondOutput += secondFixture.readAllStandardOutput()).startsWith("WINDOW:")
        && secondOutput.contains('\n'), 5000);
    const quint64 secondId = secondOutput.split('\n').first().mid(7).toULongLong();
    QVERIFY(secondId && secondId != nativeId);
    QVERIFY(clients.setWindowClass(secondId, "DockpipeLaunchFixture"));
    clients.replace({nativeId, secondId});
    page.setEnvironment("tiling");
    QVERIFY(page.launchDesktopApplication(launch.application));
    QCOMPARE(launch.dispatchCount(), 2);
    QVERIFY(page.findChild<QDialog *>("desktopWindowPicker"));
    QTest::qWait(200);
    QVERIFY(!attachedWindow());
    // Cancelling the chooser removes its pending tile without acquiring either
    // eligible window. The widgets belong to this session, never saved metadata.
    page.findChild<QDialog *>("desktopWindowPicker")->reject();
    QTRY_COMPARE_WITH_TIMEOUT(page.panelCount(), 0, 3000);
    QTRY_VERIFY_WITH_TIMEOUT(page.findChildren<DesktopWindowWidget *>().isEmpty(), 3000);
    page.setEnvironment("canvas");
    QTest::qWait(200);
    QVERIFY(!attachedWindow());
    QCOMPARE(fixture.state(), QProcess::Running);
    QCOMPARE(secondFixture.state(), QProcess::Running);
    // Missing desktop metadata is a refusal, not a reason to dispatch blindly.
    clients.removeProperty();
    page.setEnvironment("tiling");
    QVERIFY(page.launchDesktopApplication(launch.application));
    QTest::qWait(100);
    QCOMPARE(launch.dispatchCount(), 2);
    QVERIFY(!attachedWindow());
    auto *dashboardError = page.findChild<QLabel *>("dashboardSaveError");
    QVERIFY(dashboardError);
    QTRY_VERIFY_WITH_TIMEOUT(dashboardError->text().contains("No app was launched"), 3000);
    QTRY_COMPARE_WITH_TIMEOUT(page.panelCount(), 0, 3000);
    QTRY_VERIFY_WITH_TIMEOUT(page.findChildren<DesktopWindowWidget *>().isEmpty(), 3000);
    page.setEnvironment("canvas");

    clients.replace({});
    QVERIFY(launch.configureLauncher(9));
    page.setEnvironment("tiling");
    QVERIFY(page.launchDesktopApplication(launch.application));
    QTRY_COMPARE_WITH_TIMEOUT(launch.dispatchCount(), 3, 3000);
    QTRY_VERIFY_WITH_TIMEOUT(dashboardError->text().contains("exit 9"), 3000);
    QTRY_COMPARE_WITH_TIMEOUT(page.panelCount(), 0, 3000);
    QTRY_VERIFY_WITH_TIMEOUT(page.findChildren<DesktopWindowWidget *>().isEmpty(), 3000);
    clients.replace({nativeId});
    QTest::qWait(200);
    QVERIFY(!attachedWindow());
    for (auto *window : page.findChildren<DesktopWindowWidget *>()) {
        auto *timer = window->findChild<QTimer *>("desktopApplicationDiscovery");
        QVERIFY(timer && !timer->isActive());
    }
    // Closing an owned native fixture removes its panel and collapses the split
    // while preserving the neighboring ordinary widget and its content instance.
    QVERIFY(launch.configureLauncher());
    QVERIFY(page.addWidgetKind("workflows"));
    const QString ordinaryId = page.snapshot().panels.first().id;
    auto *tiling = page.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    QPointer<QWidget> ordinary = tiling->findChild<QWidget *>("dashboardWidget_workflows");
    QVERIFY(ordinary);
    QVERIFY(page.launchDesktopApplication(launch.application));
    QTRY_VERIFY_WITH_TIMEOUT(attachedWindow(), 5000);
    QPointer<DesktopWindowWidget> closing = attachedWindow();
    QPointer<QTimer> closingDiscovery = closing->findChild<QTimer *>("desktopApplicationDiscovery");
    QVERIFY(closingDiscovery && !closingDiscovery->isActive());
    QCOMPARE(page.panelCount(), 2);
    QCOMPARE(page.snapshot().panels.size(), 1);
    clients.replace({});
    fixture.terminate();
    QVERIFY(fixture.waitForFinished(3000));
    QTRY_VERIFY_WITH_TIMEOUT(closing.isNull(), 5000);
    QVERIFY(closingDiscovery.isNull());
    QCOMPARE(page.panelCount(), 1);
    QVERIFY(ordinary && ordinary->isVisible());
    QCOMPARE(tiling->findChild<QWidget *>("dashboardWidget_workflows"), ordinary.data());
    const auto remainingTree = page.snapshot().layout.value("tiling").toObject().value("tree").toObject();
    QCOMPARE(remainingTree.value("type").toString(), QString("tabs"));
    QCOMPARE(remainingTree.value("ids").toArray(), QJsonArray{ordinaryId});
    page.setEnvironment("canvas");
#endif
}

void DashboardPageTest::staleDesktopPanelsAreNotRestored()
{
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage original(emptyDashboard(), &factory);
    original.resize(1000, 740);
    original.show();
    QVERIFY(original.addWidgetKind("workflows"));
    original.setEnvironment("tiling");
    QVERIFY(original.addWidgetKind("workflows"));
    QVERIFY(original.addWidgetKind("send-input"));
    const auto ordinaryPanels = original.snapshot().panels;
    auto *originalTiling = original.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    QVERIFY(originalTiling->dockPanel(ordinaryPanels.at(2).id, ordinaryPanels.at(1).id, "tab"));
    drainEvents();
    auto saved = original.snapshot();
    const auto expectedCanvas = saved.layout.value("canvas").toObject();
    const auto expectedTiling = saved.layout.value("tiling").toObject();
    const auto expectedTree = expectedTiling.value("tree").toObject();
    const QString staleSplit = "stale-desktop-split";
    const QString staleFloating = "stale-desktop-floating";
    const QString staleCanvas = "stale-desktop-canvas";
    saved.panels.append({staleSplit, "desktop-window", "tiling"});
    saved.panels.append({staleFloating, "desktop-window", "tiling"});
    saved.panels.append({staleCanvas, "desktop-window", "canvas"});
    auto tiling = expectedTiling;
    const QJsonObject staleTabs{{"type", "tabs"}, {"ids", QJsonArray{staleSplit}}, {"active", staleSplit}};
    tiling["tree"] = QJsonObject{{"type", "split"}, {"orientation", "horizontal"},
        {"children", QJsonArray{expectedTree, staleTabs}}, {"sizes", QJsonArray{3, 1}}};
    tiling["floating"] = QJsonArray{staleFloating};
    tiling["floatingGeometry"] = QJsonObject{{staleFloating, QJsonArray{20, 20, 400, 300}}};
    saved.layout["tiling"] = tiling;
    auto canvas = expectedCanvas;
    auto order = canvas.value("canvasOrder").toArray();
    order.append(staleCanvas);
    canvas["canvasOrder"] = order;
    auto geometry = canvas.value("canvas").toObject();
    geometry[staleCanvas] = QJsonArray{30, 30, 400, 300};
    canvas["canvas"] = geometry;
    saved.layout["canvas"] = canvas;
    DashboardPage restored(saved, &factory);
    restored.resize(original.size());
    restored.show();
    drainEvents();
    QCOMPARE(restored.panelCount(), 3);
    QVERIFY(restored.findChildren<DesktopWindowWidget *>().isEmpty());
    QVERIFY(restored.findChild<QLabel *>("dashboardSaveError")->text().isEmpty());
    const auto clean = restored.snapshot();
    QCOMPARE(clean.panels.size(), 3);
    QCOMPARE(clean.layout.value("tiling").toObject().value("tree").toObject(), expectedTree);
    QVERIFY(clean.layout.value("tiling").toObject().value("floating").toArray().isEmpty());
    const QByteArray serialized = QJsonDocument(clean.layout).toJson(QJsonDocument::Compact);
    QVERIFY(!serialized.contains("stale-desktop"));
    restored.setEnvironment("canvas");
    restored.setEnvironment("tiling");
    QCOMPARE(restored.panelCount(), 3);
    QVERIFY(restored.findChildren<DesktopWindowWidget *>().isEmpty());
}

void DashboardPageTest::tourLifecycle()
{
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    page.show();
    auto respond = [&page](bool finish, bool suppress) {
        QTimer::singleShot(0, &page, [finish, suppress]() {
            auto *dialog = qobject_cast<DashboardTourDialog *>(QApplication::activeModalWidget());
            QVERIFY(dialog);
            dialog->findChild<QComboBox *>("dashboardTourBackground")->setCurrentIndex(
                dialog->findChild<QComboBox *>("dashboardTourBackground")->findData("ocean"));
            dialog->findChild<QRadioButton *>("dashboardTourTiling")->setChecked(true);
            dialog->findChild<QCheckBox *>("dashboardTourDoNotShowAgain")->setChecked(suppress);
            if (finish)
                dialog->accept();
            else
                dialog->reject();
        });
    };
    bool completed = true;
    bool suppressed = true;
    respond(false, false);
    page.showTour(true);
    QCOMPARE(page.environment(), QString("canvas"));
    QCOMPARE(page.snapshot().options.canvasBackground, QString("grid"));
    QVERIFY(LauncherSettings::loadDashboardTourState(completed, suppressed));
    QVERIFY(!completed && !suppressed);
    respond(false, true);
    page.showTour(true);
    QVERIFY(LauncherSettings::loadDashboardTourState(completed, suppressed));
    QVERIFY(!completed && suppressed);
    // Suppressed automatic invocation must return without opening a dialog.
    page.showTour(true);
    respond(true, false);
    page.showTour();
    QCOMPARE(page.environment(), QString("tiling"));
    QCOMPARE(page.snapshot().options.canvasBackground, QString("ocean"));
    QVERIFY(!page.snapshot().options.autoAcquireApps);
    QVERIFY(LauncherSettings::loadDashboardTourState(completed, suppressed));
    QVERIFY(completed && !suppressed);
    page.showTour(true);
    // Manual replay remains available after completion, with cancellation inert.
    respond(false, false);
    page.showTour();
    QCOMPARE(page.environment(), QString("tiling"));
    QCOMPARE(page.snapshot().options.canvasBackground, QString("ocean"));
    auto options = page.snapshot().options;
    options.canvasBackground = "desktop";
    QVERIFY(page.applyOptions(options));
    QString error;
    QVERIFY2(DashboardStore::validate({page.snapshot()}, error), qPrintable(error));
    QCOMPARE(page.snapshot().options.canvasBackground, QString("desktop"));
}

void DashboardPageTest::desktopBackgroundLifecycle()
{
#ifndef Q_OS_LINUX
    QSKIP("Mock gsettings lifecycle is Linux-specific.");
#else
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QByteArray oldPath = qgetenv("PATH");
    const QByteArray oldDesktop = qgetenv("XDG_CURRENT_DESKTOP");
    const QByteArray oldImage = qgetenv("DOCKPIPE_TEST_WALLPAPER");
    const auto restoreEnvironment = qScopeGuard([&]() {
        qputenv("PATH", oldPath);
        qputenv("XDG_CURRENT_DESKTOP", oldDesktop);
        if (oldImage.isNull())
            qunsetenv("DOCKPIPE_TEST_WALLPAPER");
        else
            qputenv("DOCKPIPE_TEST_WALLPAPER", oldImage);
    });
    const QString imagePath = directory.filePath("wallpaper.png");
    QImage image(80, 80, QImage::Format_RGB32);
    image.fill(QColor("#38c862"));
    QVERIFY(image.save(imagePath));
    QFile program(directory.filePath("gsettings"));
    QVERIFY(program.open(QIODevice::WriteOnly));
    const QByteArray script = "#!/bin/sh\n/bin/sleep 0.15\n"
        "if [ \"$1\" = get ]; then exit 1; fi\n"
        "printf \"org.gnome.desktop.background picture-uri '%s'\\n\" \"$DOCKPIPE_TEST_WALLPAPER\"\n";
    QCOMPARE(program.write(script), script.size());
    program.close();
    QVERIFY(program.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner));
    qputenv("PATH", directory.path().toUtf8() + ":" + oldPath);
    qputenv("XDG_CURRENT_DESKTOP", "GNOME");
    qputenv("DOCKPIPE_TEST_WALLPAPER", imagePath.toUtf8());
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    page.resize(900, 700);
    page.show();
    auto *canvas = page.findChild<DashboardWorkspace *>("dashboardCanvasWorkspace");
    auto *tiling = page.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    auto *status = page.findChild<QLabel *>("dashboardBackgroundStatus");
    QVERIFY(canvas && tiling && status);
    page.setEnvironment("tiling");
    auto options = page.snapshot().options;
    options.canvasBackground = "desktop";
    QElapsedTimer response;
    response.start();
    QVERIFY(page.applyOptions(options));
    QVERIFY2(response.elapsed() < 100, "Wallpaper lookup blocked the UI thread.");
    options.canvasBackground = "plain";
    QVERIFY(page.applyOptions(options));
    QTest::qWait(500);
    QVERIFY(pixelAt(canvas->grab().toImage(), QPoint(20, 20)) != QColor("#38c862"));
    QCOMPARE(page.snapshot().options.canvasBackground, QString("plain"));
    options.canvasBackground = "desktop";
    QVERIFY(page.applyOptions(options));
    QTRY_COMPARE_WITH_TIMEOUT(pixelAt(canvas->grab().toImage(), QPoint(20, 20)), QColor("#38c862"), 3000);
    QCOMPARE(pixelAt(tiling->grab().toImage(), QPoint(20, 20)), QColor("#38c862"));
    QVERIFY(status->text().isEmpty());
    // A wallpaper changed in place refreshes on returning to Canvas.
    image.fill(QColor("#d63fd9"));
    QVERIFY(image.save(imagePath));
    page.setEnvironment("canvas");
    page.setEnvironment("tiling");
    QTRY_COMPARE_WITH_TIMEOUT(pixelAt(tiling->grab().toImage(), QPoint(20, 20)), QColor("#d63fd9"), 3000);
    QCOMPARE(pixelAt(canvas->grab().toImage(), QPoint(20, 20)), QColor("#d63fd9"));
    qputenv("DOCKPIPE_TEST_WALLPAPER", directory.filePath("missing.png").toUtf8());
    page.setEnvironment("tiling");
    page.setEnvironment("canvas");
    QTRY_VERIFY_WITH_TIMEOUT(!status->text().isEmpty(), 3000);
    QCOMPARE(pixelAt(canvas->grab().toImage(), QPoint(20, 20)), QColor("#d63fd9"));
    QCOMPARE(pixelAt(tiling->grab().toImage(), QPoint(20, 20)), QColor("#d63fd9"));
    qInfo("Wallpaper lookup stayed asynchronous; stale results discarded, refresh and failure fallback passed.");
#endif
}

void DashboardPageTest::fileScopesAndCanvasProjectPicker()
{
    QTemporaryDir firstProject;
    QTemporaryDir secondProject;
    QVERIFY(firstProject.isValid() && secondProject.isValid());
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    factory.setWorkdir(firstProject.path());
    DashboardPage page(emptyDashboard(), &factory);
    page.resize(1100, 780);
    page.show();
    QVERIFY(page.addWidgetKind("files"));
    auto *canvas = page.findChild<DashboardWorkspace *>("dashboardCanvasWorkspace");
    auto *tiling = page.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    auto *canvasFiles = canvas->findChild<DashboardFilesWidget *>();
    QVERIFY(canvasFiles);
    QCOMPARE(canvasFiles->scope(), QString("project"));
    QCOMPARE(canvasFiles->currentDirectory(), QFileInfo(firstProject.path()).canonicalFilePath());
    auto *picker = page.findChild<QPushButton *>("dashboardCanvasProject");
    QVERIFY(picker && picker->isVisible());
    QSignalSpy projectRequested(&page, &DashboardPage::projectChangeRequested);
    picker->click();
    QCOMPARE(projectRequested.size(), 1);
    page.setEnvironment("tiling");
    QVERIFY(picker->isHidden());
    QVERIFY(page.addWidgetKind("files"));
    auto *systemFiles = tiling->findChild<DashboardFilesWidget *>();
    QVERIFY(systemFiles);
    QCOMPARE(systemFiles->scope(), QString("system"));
    QCOMPARE(systemFiles->currentDirectory(), QFileInfo(QDir::homePath()).canonicalFilePath());
    QVERIFY(systemFiles->navigateTo(firstProject.path()));
    factory.setWorkdir(secondProject.path());
    QCOMPARE(canvasFiles->currentDirectory(), QFileInfo(secondProject.path()).canonicalFilePath());
    QCOMPARE(systemFiles->currentDirectory(), QFileInfo(firstProject.path()).canonicalFilePath());
    QVERIFY(picker->toolTip().contains(QDir::toNativeSeparators(secondProject.path())));
    auto options = page.snapshot().options;
    options.canvasFileScope = "system";
    options.tilingFileScope = "project";
    QVERIFY(page.applyOptions(options));
    QCOMPARE(systemFiles->scope(), QString("project"));
    QCOMPARE(systemFiles->currentDirectory(), QFileInfo(secondProject.path()).canonicalFilePath());
    QCOMPARE(canvasFiles->scope(), QString("system"));
    QCOMPARE(canvasFiles->currentDirectory(), QFileInfo(QDir::homePath()).canonicalFilePath());
    page.setEnvironment("canvas");
    QVERIFY(picker->isVisible());
    QCOMPARE(canvas->findChild<DashboardFilesWidget *>(), canvasFiles);
    QCOMPARE(tiling->findChild<DashboardFilesWidget *>(), systemFiles);
    QVERIFY(captureIfRequested(page, "file-scopes"));
}

int main(int argc, char **argv)
{
    QTemporaryDir config;
    if (!config.isValid())
        return 1;
    qputenv("XDG_CONFIG_HOME", config.path().toUtf8());
    qputenv("XDG_CACHE_HOME", config.path().toUtf8());
    qputenv("APPDATA", config.path().toUtf8());
    QApplication application(argc, argv);
    application.setOrganizationName("DockpipeDashboardPageTest");
    application.setApplicationName("DashboardPageFixture");
    application.setStyle("Fusion");
    applyDockpipeLauncherTheme(application);
    if (application.styleSheet().isEmpty())
        return 1;
    DashboardPageTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "DashboardPageTest.moc"
