#include "SpatialNeuralService.h"
#include "SpatialWebWorld.h"
#include "SpatialInteriorGeometry.h"
#include "WorkspaceFlightView.h"
#include <QApplication>
#include <QElapsedTimer>
#include <QJsonArray>
#include <QThread>
#include <QPainter>
#include <QTemporaryDir>
#include <QDir>
#include <QFile>
#include <QStandardPaths>
#include <cstdio>

#define VERIFY_INTELLIGENCE(value) do { if (!(value)) { std::fprintf(stderr, "Intelligence assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

namespace {
bool waitFor(const std::function<bool()> &ready, int milliseconds = 15000)
{
    QElapsedTimer elapsed;
    elapsed.start();
    while (!ready() && elapsed.elapsed() < milliseconds) {
        QApplication::processEvents();
        QThread::msleep(5);
    }
    return ready();
}
SpatialWebDocument fixture()
{
    SpatialWebDocument document;
    document.url = QUrl("https://example.org/camping");
    document.title = "Woodland equipment and fantasy books";
    document.evidence = "Tents and hiking boots for mountain trails, plus fantasy novels about dragons and knights";
    document.venue = "store";
    for (int index = 0; index < 12; ++index) {
        SpatialWebItem item;
        item.id = QString::number(index);
        item.title = index % 2 ? "Dragon novels about knights and magical castles" : "Tent for camping in the woods";
        item.category = index % 2 ? "Books" : "Camping";
        item.url = QUrl("https://example.org/product/" + item.id);
        item.kind = "product";
        document.items.append(item);
    }
    return document;
}
QJsonObject proposal(const SpatialWebDocument &document, int feature)
{
    QJsonArray scores;
    for (int index = 0; index < 8; ++index)
        scores.append(index == feature ? 1.0 : 0.0);
    QJsonArray items;
    for (const auto &item : document.items)
        items.append(QJsonObject{{"id", item.id}, {"features", scores}});
    return {{"features", scores}, {"items", items}};
}
}

int spatialIntelligenceTest()
{
    const auto source = fixture();
    for (int feature = 0; feature < 8; ++feature) {
        auto document = source;
        VERIFY_INTELLIGENCE(SpatialNeuralService::apply(proposal(document, feature), document));
        auto options = SpatialWebWorld::optionsFor(document);
        const auto plan = SpatialWebWorld::plan(document, SpatialWebGraph::fromDocument(document), options);
        const auto result = SpatialWebWorld::compile(plan);
        if (!result.success || result.fallback)
            std::fprintf(stderr, "Feature %d rejected: %s\n", feature, qPrintable(result.diagnostic));
        VERIFY_INTELLIGENCE(result.success && !result.fallback);
        VERIFY_INTELLIGENCE(result.interior.solids.size() <= 96);
        VERIFY_INTELLIGENCE(result.interior.displays.size() == 12);
        VERIFY_INTELLIGENCE(document.items[0].url == source.items[0].url);
    }
    auto unchanged = source;
    auto invalid = proposal(source, 0);
    invalid["features"] = QJsonArray{1, 2, 3};
    VERIFY_INTELLIGENCE(!SpatialNeuralService::apply(invalid, unchanged));
    VERIFY_INTELLIGENCE(unchanged.worldFeatures.isEmpty());
    invalid = proposal(source, 0);
    auto items = invalid["items"].toArray();
    auto first = items[0].toObject();
    first["id"] = "99";
    items[0] = first;
    invalid["items"] = items;
    VERIFY_INTELLIGENCE(!SpatialNeuralService::apply(invalid, unchanged));
    VERIFY_INTELLIGENCE(unchanged.worldFeatures.isEmpty());

    auto animal = source;
    animal.items[0].kind = "animal";
    auto animalProposal = proposal(animal, 0);
    auto animalItems = animalProposal["items"].toArray();
    auto animalItem = animalItems[0].toObject();
    animalItem["animalForm"] = "spider";
    animalItems[0] = animalItem;
    animalProposal["items"] = animalItems;
    VERIFY_INTELLIGENCE(!SpatialNeuralService::apply(animalProposal, unchanged));
    VERIFY_INTELLIGENCE(SpatialNeuralService::apply(animalProposal, animal));
    VERIFY_INTELLIGENCE(animal.items[0].animalForm == "spider" && animal.items[0].url == source.items[0].url);
    animalItem["animalForm"] = "arbitrary-code-or-geometry";
    animalItems[0] = animalItem;
    animalProposal["items"] = animalItems;
    VERIFY_INTELLIGENCE(!SpatialNeuralService::apply(animalProposal, animal));
    VERIFY_INTELLIGENCE(animal.items[0].animalForm == "spider");

#ifdef Q_OS_LINUX
    const QByteArray priorRoot = qgetenv("DOCKPIPE_COCKPIT_NEURAL_ROOT");
    {
        QTemporaryDir fixture;
        VERIFY_INTELLIGENCE(fixture.isValid());
        QDir root(fixture.path());
        VERIFY_INTELLIGENCE(root.mkpath("runtime/bin") && root.mkpath("models"));
        for (const QString &path : {"ready.json", "models/semantic.onnx", "models/tokenizer.json", "models/cutout.onnx"}) {
            QFile file(root.filePath(path));
            VERIFY_INTELLIGENCE(file.open(QIODevice::WriteOnly));
        }
        QFile helper(root.filePath("runtime/bin/python"));
        VERIFY_INTELLIGENCE(helper.open(QIODevice::WriteOnly));
        helper.write("#!" + QStandardPaths::findExecutable("python3").toUtf8() + "\n"
            "import sys\n"
            "sys.stdin.readline()\n"
            "sys.stdout.write('x' * (1024 * 1024 + 1) + '\\n')\n"
            "sys.stdout.flush()\n");
        helper.close();
        VERIFY_INTELLIGENCE(helper.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner));
        qputenv("DOCKPIPE_COCKPIT_NEURAL_ROOT", fixture.path().toUtf8());
        SpatialNeuralService malformed;
        QString failure;
        bool leaked = false;
        QObject::connect(&malformed, &SpatialNeuralService::statusChanged, &malformed, [&](const QString &text) { failure = text; });
        QObject::connect(&malformed, &SpatialNeuralService::planReady, &malformed, [&](const QJsonObject &) { leaked = true; });
        malformed.plan(source);
        VERIFY_INTELLIGENCE(waitFor([&]() { return !failure.isEmpty(); }));
        VERIFY_INTELLIGENCE(!leaked);
        malformed.cancel();
    }
    if (priorRoot.isEmpty())
        qunsetenv("DOCKPIPE_COCKPIT_NEURAL_ROOT");
    else
        qputenv("DOCKPIPE_COCKPIT_NEURAL_ROOT", priorRoot);
#endif
    SpatialNeuralService service;
    service.setPreferGpu(qEnvironmentVariableIsSet("DOCKPIPE_TEST_NEURAL_GPU"));
    // Real-model checks opt in with an isolated downloaded runtime. All domain
    // and failure-boundary tests above remain self-contained and network-free.
    if (qEnvironmentVariableIsEmpty("DOCKPIPE_COCKPIT_NEURAL_ROOT"))
        return 0;
    if (qEnvironmentVariableIsSet("DOCKPIPE_TEST_NEURAL_INSTALL")) {
        bool finished = false;
        bool installed = false;
        QObject::connect(&service, &SpatialNeuralService::installationFinished, &service, [&](bool success) {
            finished = true;
            installed = success;
        });
        service.install(false);
        VERIFY_INTELLIGENCE(waitFor([&]() { return finished; }, 610000));
        VERIFY_INTELLIGENCE(installed);
    }
    VERIFY_INTELLIGENCE(service.available());
    QJsonObject answer;
    QImage cutout;
    QString failure;
    QObject::connect(&service, &SpatialNeuralService::planReady, &service, [&](const QJsonObject &value) { answer = value; });
    QObject::connect(&service, &SpatialNeuralService::imageReady, &service, [&](const QString &, const QImage &value) { cutout = value; });
    QObject::connect(&service, &SpatialNeuralService::statusChanged, &service, [&](const QString &value) { failure = value; });
    service.plan(source);
    VERIFY_INTELLIGENCE(waitFor([&]() { return !answer.isEmpty() || !failure.isEmpty(); }));
    if (!failure.isEmpty())
        std::fprintf(stderr, "%s\n", qPrintable(failure));
    VERIFY_INTELLIGENCE(!answer.isEmpty());
    if (qEnvironmentVariableIsSet("DOCKPIPE_TEST_NEURAL_GPU"))
        VERIFY_INTELLIGENCE(answer.value("provider") == "CUDAExecutionProvider");
    auto interpreted = source;
    VERIFY_INTELLIGENCE(SpatialNeuralService::apply(answer, interpreted));
    VERIFY_INTELLIGENCE(interpreted.items[0].worldFeatures[0] > interpreted.items[0].worldFeatures[1]);
    VERIFY_INTELLIGENCE(interpreted.items[1].worldFeatures[1] > interpreted.items[1].worldFeatures[0]);
    const auto world = SpatialWebWorld::construct(interpreted, SpatialWebWorld::optionsFor(interpreted));
    VERIFY_INTELLIGENCE(world.success && !world.fallback);
    QImage product(320, 240, QImage::Format_ARGB32);
    product.fill(Qt::white);
    {
        QPainter painter(&product);
        painter.setPen(Qt::NoPen);
        painter.setBrush(QColor("#b54632"));
        painter.drawRoundedRect(QRect(100, 45, 120, 160), 15, 15);
        painter.setBrush(QColor("#292f36"));
        painter.drawRoundedRect(QRect(125, 25, 70, 30), 8, 8);
    }
    service.cutout("0", product);
    VERIFY_INTELLIGENCE(waitFor([&]() { return !cutout.isNull() || !failure.isEmpty(); }));
    VERIFY_INTELLIGENCE(!cutout.isNull());
    VERIFY_INTELLIGENCE(cutout.pixelColor(0, 0).alpha() < 32);
    VERIFY_INTELLIGENCE(cutout.pixelColor(cutout.width() / 2, cutout.height() / 2).alpha() > 200);
    // A queued cache result must not leak across navigation/cancellation.
    answer = {};
    service.plan(source);
    service.cancel();
    QElapsedTimer settling;
    settling.start();
    while (settling.elapsed() < 100) {
        QApplication::processEvents();
        QThread::msleep(5);
    }
    VERIFY_INTELLIGENCE(answer.isEmpty());
    service.plan(source);
    VERIFY_INTELLIGENCE(waitFor([&]() { return !answer.isEmpty(); }));
    const QString screenshot = qEnvironmentVariable("DOCKPIPE_INTELLIGENCE_SCREENSHOT");
    if (!screenshot.isEmpty()) {
        WorkspaceFlightView scene({}, false);
        scene.resize(1280, 800);
        scene.setInterior(world.interior, source.title);
        for (int index = 0; index < interpreted.items.size(); ++index)
            scene.setInteriorImage(interpreted.items[index].id, SpatialWebDocument::poster(interpreted.items[index], cutout, index));
        scene.show();
        QApplication::processEvents();
        VERIFY_INTELLIGENCE(scene.grab().save(screenshot));
    }
    std::fprintf(stderr, "Real semantic/cutout models passed using %s\n", qPrintable(answer["provider"].toString()));
    return 0;
}
