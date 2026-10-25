#include "PackageDisplayWidget.h"
#include "LauncherDisplayPackage.h"
#include <QApplication>
#include <QDir>
#include <QFile>
#include <QJsonDocument>
#include <QJsonArray>
#include <QEvent>
#include <QElapsedTimer>
#include <QJsonObject>
#include <QTimer>
#include <QVariant>
#include <cstdio>

class DisplayLifecycleRecorder final : public QObject {
public:
    QJsonArray events;
    QElapsedTimer elapsed;

    DisplayLifecycleRecorder()
    {
        elapsed.start();
    }

    bool eventFilter(QObject *object, QEvent *event) override
    {
        if (events.size() < 64 && (event->type() == QEvent::Show
            || event->type() == QEvent::Hide || event->type() == QEvent::WindowStateChange)) {
            auto *widget = qobject_cast<QWidget *>(object);
            events.append(QJsonObject{{"view", object->objectName()},
                {"event", int(event->type())}, {"spontaneous", event->spontaneous()},
                {"elapsed_ms", elapsed.elapsed()},
                {"visible", widget && widget->isVisible()},
                {"window_state", widget ? int(widget->windowState()) : 0}});
        }
        return QObject::eventFilter(object, event);
    }
};

// Explicit local installed-plugin qualification. The caller owns the disposable
// resource lifecycle; this harness only acquires/releases its native widgets.
int main(int argc, char **argv)
{
    QApplication application(argc, argv);
    if (application.arguments().size() != 3)
        return 2;
    QFile input(application.arguments().at(1));
    if (!input.open(QIODevice::ReadOnly) || input.size() > 32 * 1024)
        return 2;
    QJsonParseError parse;
    const auto document = QJsonDocument::fromJson(input.readAll(), &parse);
    if (parse.error != QJsonParseError::NoError || !document.isObject())
        return 2;
    const auto config = document.object();
    const QString outputDirectory = application.arguments().at(2);
    if (!QDir().mkpath(outputDirectory))
        return 2;
    PackageDisplayMeta display;
    display.id = config.value("lease_id").toString();
    display.leaseId = display.id;
    display.nativePackage = {1, config.value("package_root").toString(),
        {{PackageDisplayWidget::platformName(), config.value("library").toString()}}};
    display.options = config.value("options").toObject();
    QString error;
    if (display.id.isEmpty() || !config.value("options").isObject()
        || !PackageDisplayWidget::validateOptions(display.options, error))
        return 2;
    PackageDisplayWidget first;
    PackageDisplayWidget second;
    DisplayLifecycleRecorder lifecycle;
    first.setObjectName("first");
    second.setObjectName("second");
    first.installEventFilter(&lifecycle);
    second.installEventFilter(&lifecycle);
    first.resize(960, 720);
    second.resize(960, 720);
    first.setWindowTitle("Dockpipe installed display proof: first view");
    second.setWindowTitle("Dockpipe installed display proof: handoff view");
    QJsonObject receipt{{"schema", 1}, {"resource_lifecycle_owned_by_harness", false}};
    bool failed = false;
    const auto recordFailure = [&](const QString &) {
        // Package diagnostics may contain endpoint details; do not log them.
        failed = true;
    };
    QObject::connect(&first, &PackageDisplayWidget::displayFailed, &application, recordFailure);
    QObject::connect(&second, &PackageDisplayWidget::displayFailed, &application, recordFailure);
    first.setDisplay(display);
    first.show();
    const auto capture = [&](PackageDisplayWidget &widget, const QString &name) {
        const QString path = QDir(outputDirectory).filePath(name + ".png");
        return widget.grab().save(path);
    };
    const auto displayProperty = [](PackageDisplayWidget &widget, const char *name) {
        for (auto *child : widget.findChildren<QWidget *>()) {
            if (auto *surface = dynamic_cast<LauncherDisplaySurface *>(child))
                return surface->property(name);
        }
        return QVariant();
    };
    // Each view remains attached beyond the package connection deadline, so
    // construction alone cannot hide a delayed transport failure.
    QTimer::singleShot(6500, &application, [&]() {
        receipt["first_loaded"] = first.hasPackageSurface();
        receipt["first_ready"] = displayProperty(first, "displayReady").toBool();
        receipt["first_transport"] = displayProperty(first, "displayTransportKind").toString().left(64);
        receipt["first_capture"] = capture(first, "first");
        second.setDisplay(display);
        second.show();
    });
    QTimer::singleShot(13000, &application, [&]() {
        receipt["handoff_loaded"] = second.hasPackageSurface();
        receipt["handoff_ready"] = displayProperty(second, "displayReady").toBool();
        receipt["handoff_transport"] = displayProperty(second, "displayTransportKind").toString().left(64);
        receipt["previous_disconnected"] = !first.hasPackageSurface();
        receipt["handoff_capture"] = capture(second, "handoff");
        first.setDisplay(display);
        receipt["background_did_not_steal"] = !first.hasPackageSurface();
        first.hide();
        first.show();
    });
    QTimer::singleShot(19500, &application, [&]() {
        receipt["return_loaded"] = first.hasPackageSurface();
        receipt["return_ready"] = displayProperty(first, "displayReady").toBool();
        receipt["return_transport"] = displayProperty(first, "displayTransportKind").toString().left(64);
        receipt["handoff_disconnected"] = !second.hasPackageSurface();
        receipt["return_capture"] = capture(first, "returned");
        first.clearDisplay();
        second.clearDisplay();
        receipt["closed_disconnected"] = !first.hasPackageSurface() && !second.hasPackageSurface();
        receipt["package_failure"] = failed;
        receipt["view_lifecycle"] = lifecycle.events;
        bool success = !failed;
        for (auto entry = receipt.begin(); entry != receipt.end(); ++entry) {
            if (entry.value().isBool() && entry.key() != "resource_lifecycle_owned_by_harness" && entry.key() != "package_failure")
                success = success && entry.value().toBool();
        }
        receipt["success"] = success;
        QFile output(QDir(outputDirectory).filePath("host-factory-proof.json"));
        if (!output.open(QIODevice::WriteOnly)) {
            application.exit(1);
            return;
        }
        output.write(QJsonDocument(receipt).toJson());
        output.close();
        application.exit(success ? 0 : 1);
    });
    return application.exec();
}
