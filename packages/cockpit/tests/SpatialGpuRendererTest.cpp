#include "SpatialGpuRenderer.h"
#include "SpatialGpuFoliage.h"
#include <QSet>
#include <QGuiApplication>
#include <QOffscreenSurface>
#include <QOpenGLContext>
#include <QOpenGLFramebufferObject>
#include <QOpenGLFunctions>
#include <QOpenGLPaintDevice>
#include <QPainter>
#include <algorithm>
#include <cmath>
#include <cstdio>
#include <limits>

#define CHECK_GPU(condition) do { if (!(condition)) { std::fprintf(stderr, "Native geometry assertion at %d: %s\n", __LINE__, #condition); return 1; } } while (false)

int main(int argc, char **argv)
{
    QGuiApplication application(argc, argv);
    QSurfaceFormat format;
    format.setVersion(3, 3);
    format.setProfile(QSurfaceFormat::CoreProfile);
    QOpenGLContext context;
    context.setFormat(format);
    if (!context.create())
        return 77;
    QOffscreenSurface surface;
    surface.setFormat(context.format());
    surface.create();
    if (!surface.isValid() || !context.makeCurrent(&surface))
        return 77;
    auto *gl = context.functions();
    SpatialGpuRenderer renderer;
    SpatialGpuRenderer::Camera camera;
    camera.right = {1, 0, 0};
    camera.up = {0, 1, 0};
    camera.forward = {0, 0, 1};
    camera.viewport = {256, 256};
    camera.focalLength = 128;
    camera.sky = QColor("#303030");
    camera.fogDistance = 60000;
    const auto quad = [](float depth, float size, QColor color) {
        SpatialGpuRenderer::Face face;
        face.vertices = {{-size, size, depth}, {size, size, depth},
            {size, -size, depth}, {-size, -size, depth}};
        face.normal = {0, 0, -1};
        face.color = color;
        face.unlit = true;
        return face;
    };
    bool accepted = false;
    bool coverageStateRestored = true;
    const auto render = [&](const QVector<SpatialGpuRenderer::Face> &faces, int sourceSamples = 0,
                            const QVector<SpatialGpuRenderer::InstanceGroup> &instances = {}) {
        QOpenGLFramebufferObjectFormat targetFormat;
        targetFormat.setSamples(sourceSamples);
        QOpenGLFramebufferObject target(camera.viewport, targetFormat);
        if (!target.isValid() || !target.bind())
            return QImage();
        auto *activeGl = QOpenGLContext::currentContext()->functions();
        activeGl->glViewport(0, 0, camera.viewport.width(), camera.viewport.height());
        QOpenGLPaintDevice device(camera.viewport);
        QPainter painter(&device);
        painter.fillRect(QRect(QPoint(), camera.viewport), camera.sky);
        const bool previousCoverage = activeGl->glIsEnabled(GL_SAMPLE_ALPHA_TO_COVERAGE);
        const bool previousAlphaToOne = activeGl->glIsEnabled(GL_SAMPLE_ALPHA_TO_ONE);
        accepted = renderer.render(&painter, camera, faces, instances);
        coverageStateRestored = coverageStateRestored
            && bool(activeGl->glIsEnabled(GL_SAMPLE_ALPHA_TO_COVERAGE)) == previousCoverage
            && bool(activeGl->glIsEnabled(GL_SAMPLE_ALPHA_TO_ONE)) == previousAlphaToOne;
        // QPainter must remain usable after the native pass, including failures.
        painter.fillRect(QRect(0, 0, 12, 12), Qt::blue);
        painter.end();
        activeGl->glFinish();
        auto image = target.toImage();
        target.release();
        return image;
    };
    const auto near = quad(50, 15, Qt::green);
    const auto far = quad(100, 50, Qt::red);
    auto image = render({near, far}); // Deliberately wrong painter order.
    if (!accepted)
        std::fprintf(stderr, "%s\n", qPrintable(renderer.error()));
    CHECK_GPU(accepted && !image.isNull());
    CHECK_GPU(image.pixelColor(128, 128).green() > 240);
    CHECK_GPU(image.pixelColor(75, 128).red() > 240);
    CHECK_GPU(image.pixelColor(4, 4).blue() > 240);
    CHECK_GPU(renderer.drawCalls() == 1); // Per-vertex colour shares one batch.

    // Finishes sharing a texture must not collapse into one opaque batch.
    // Compare the grouped output to independently rendered materials.
    auto standardFinish = quad(100, 30, QColor(100, 75, 40));
    standardFinish.unlit = false;
    standardFinish.normal = QVector3D(-.3f, .3f, -1).normalized();
    auto paintedFinish = standardFinish;
    paintedFinish.finish = SpatialSurfaceFinish::Painted;
    auto metalFinish = standardFinish;
    metalFinish.finish = SpatialSurfaceFinish::Metal;
    const auto standardImage = render({standardFinish});
    CHECK_GPU(accepted);
    const auto paintedImage = render({paintedFinish});
    CHECK_GPU(accepted);
    const auto metalImage = render({metalFinish});
    CHECK_GPU(accepted);
    CHECK_GPU(standardImage.pixelColor(128, 128) != paintedImage.pixelColor(128, 128));
    CHECK_GPU(standardImage.pixelColor(128, 128) != metalImage.pixelColor(128, 128));
    image = render({standardFinish, paintedFinish, metalFinish});
    CHECK_GPU(accepted && renderer.drawCalls() == 3);
    auto invalidFinish = standardFinish;
    invalidFinish.finish = static_cast<SpatialSurfaceFinish>(100);
    image = render({invalidFinish});
    CHECK_GPU(!accepted && image.pixelColor(4, 4).blue() > 240);

    // Model UVs follow the model when it moves through the world. Move the
    // camera with it so the projected geometry is identical and compare texels.
    auto fabric = quad(100, 30, QColor(180, 180, 180));
    fabric.material = SpatialSurfaceMaterial::Fabric;
    fabric.uvs = {{0,0}, {1,0}, {1,1}, {0,1}};
    const auto stationaryFabric = render({fabric});
    CHECK_GPU(accepted);
    const QVector3D translation(137, 45, 53);
    for (auto &vertex : fabric.vertices)
        vertex += translation;
    camera.position = translation;
    const auto movingFabric = render({fabric});
    CHECK_GPU(accepted);
    for (int x : {110, 128, 145}) {
        for (int y : {110, 128, 145}) {
            const auto before = stationaryFabric.pixelColor(x, y);
            const auto after = movingFabric.pixelColor(x, y);
            CHECK_GPU(std::abs(before.red() - after.red()) <= 1);
            CHECK_GPU(std::abs(before.green() - after.green()) <= 1);
            CHECK_GPU(std::abs(before.blue() - after.blue()) <= 1);
        }
    }
    camera.position = {};

    const auto pine = SpatialGpuFoliage::pine();
    CHECK_GPU(pine == SpatialGpuFoliage::pine() && pine->size() == 1256);
    QSet<qint64> foliageImages;
    int foliageVertices = 0;
    for (const auto &face : *pine) {
        foliageVertices += (face.vertices.size() - 2) * 3;
        CHECK_GPU(!face.image.isNull());
        foliageImages.insert(face.image.cacheKey());
        CHECK_GPU(face.normal.lengthSquared() > .99f && face.normal.lengthSquared() < 1.01f);
        for (const auto &point : face.vertices) {
            CHECK_GPU(std::isfinite(point.x()) && std::isfinite(point.y()) && std::isfinite(point.z()));
            CHECK_GPU(std::hypot(point.x(), point.z()) < .5f && point.y() >= -.001f && point.y() <= 1.01f);
        }
    }
    CHECK_GPU(foliageImages.size() == 2 && foliageVertices <= SpatialGpuRenderer::maximumInstanceVertices);
    const auto needles = pine->last().image;
    int needlePixels = 0;
    for (int y = 0; y < needles.height(); ++y) {
        for (int x = 0; x < needles.width(); ++x) {
            const int alpha = needles.pixelColor(x, y).alpha();
            if (x == 0 || y == 0 || x == needles.width() - 1 || y == needles.height() - 1)
                CHECK_GPU(alpha == 0);
            if (alpha > 127)
                ++needlePixels;
        }
    }
    CHECK_GPU(needlePixels > 10000 && needlePixels < 35000);

    const auto grass = SpatialGpuFoliage::grassPatch();
    CHECK_GPU(grass == SpatialGpuFoliage::grassPatch() && grass->size() == 640);
    int grassVertices = 0;
    for (const auto &face : *grass) {
        grassVertices += (face.vertices.size() - 2) * 3;
        CHECK_GPU(face.image.isNull() && face.color.alpha() == 255 && !face.cutout);
        CHECK_GPU(face.normal.lengthSquared() > .99f && face.normal.lengthSquared() < 1.01f);
        for (const auto &point : face.vertices) {
            CHECK_GPU(std::hypot(point.x(), point.z()) < .5f);
            CHECK_GPU(point.y() >= 0 && point.y() <= 1);
        }
    }
    CHECK_GPU(grassVertices == 2880 && grassVertices <= SpatialGpuRenderer::maximumInstanceVertices);
    const QVector3D groundCoverCamera = camera.position;
    camera.position = {0, 30, 0};
    SpatialGpuRenderer::InstanceGroup groundCover{grass, {{{0, 0, 150}, {200, 50, 200}, 0, 1}}};
    const auto grassImage = render({}, 0, {groundCover});
    CHECK_GPU(accepted && renderer.drawCalls() == 1);
    int grassPixels = 0;
    for (int y = 16; y < grassImage.height(); ++y) {
        for (int x = 16; x < grassImage.width(); ++x) {
            const auto pixel = grassImage.pixelColor(x, y);
            if (pixel.green() > pixel.red() * 1.05 && pixel.green() > pixel.blue() * 1.2)
                ++grassPixels;
        }
    }
    CHECK_GPU(grassPixels > 100);
    camera.position = groundCoverCamera;

    auto sharedMesh = QSharedPointer<QVector<SpatialGpuRenderer::Face>>::create();
    sharedMesh->append(quad(0, 10, Qt::yellow));
    SpatialGpuRenderer::InstanceGroup group{sharedMesh, {
        {{-30, 0, 100}, {1, 1, 1}, 0, 1},
        {{30, 0, 100}, {1.5f, 1, 1}, 0, .5f}}};
    image = render({}, 0, {group});
    CHECK_GPU(accepted && renderer.drawCalls() == 1);
    CHECK_GPU(image.pixelColor(90, 128).red() > 240);
    CHECK_GPU(image.pixelColor(166, 128).red() > 110 && image.pixelColor(166, 128).red() < 145);
    image = render({quad(50, 25, Qt::green)}, 0, {group});
    CHECK_GPU(accepted && renderer.drawCalls() == 2 && image.pixelColor(90, 128).green() > 240);
    auto invalidGroup = group;
    invalidGroup.instances[0].scale.setX(0);
    image = render({}, 0, {invalidGroup});
    CHECK_GPU(!accepted && image.pixelColor(4, 4).blue() > 240);
    invalidGroup = group;
    invalidGroup.instances[0].heading = std::numeric_limits<float>::quiet_NaN();
    render({}, 0, {invalidGroup});
    CHECK_GPU(!accepted);
    invalidGroup = group;
    invalidGroup.instances[0].shade = 5;
    render({}, 0, {invalidGroup});
    CHECK_GPU(!accepted);
    invalidGroup = group;
    invalidGroup.instances[0].position.setZ(1000001);
    render({}, 0, {invalidGroup});
    CHECK_GPU(!accepted);
    invalidGroup = group;
    invalidGroup.instances.resize(SpatialGpuRenderer::maximumInstances + 1);
    render({}, 0, {invalidGroup});
    CHECK_GPU(!accepted);
    invalidGroup = group;
    invalidGroup.mesh.reset();
    render({}, 0, {invalidGroup});
    CHECK_GPU(!accepted);
    render({}, 0, QVector<SpatialGpuRenderer::InstanceGroup>(SpatialGpuRenderer::maximumInstanceGroups + 1, group));
    CHECK_GPU(!accepted);
    sharedMesh->first().uvs = {{0, 0}};
    render({}, 0, {group});
    CHECK_GPU(!accepted);
    sharedMesh->first().uvs.clear();
    sharedMesh->first().uvs = {{0, 0}, {1, 0}, {1, 1}, {0, std::numeric_limits<float>::infinity()}};
    render({}, 0, {group});
    CHECK_GPU(!accepted);
    sharedMesh->first().uvs.clear();
    sharedMesh->first().color.setAlpha(128);
    render({}, 0, {group});
    CHECK_GPU(!accepted);
    sharedMesh->first().color = Qt::yellow;
    render({}, 0, {group});
    CHECK_GPU(accepted);
    auto oversizedMesh = QSharedPointer<QVector<SpatialGpuRenderer::Face>>::create(
        SpatialGpuRenderer::maximumInstanceVertices / 6 + 1, sharedMesh->first());
    invalidGroup = group;
    invalidGroup.mesh = oversizedMesh;
    render({}, 0, {invalidGroup});
    CHECK_GPU(!accepted);

    auto cutout = quad(40, 20, Qt::white);
    cutout.image = QImage(8, 8, QImage::Format_RGBA8888);
    cutout.image.fill(Qt::transparent);
    {
        QPainter artwork(&cutout.image);
        artwork.fillRect(0, 0, 4, 8, Qt::yellow);
    }
    cutout.cutout = true;
    image = render({cutout, near, far});
    CHECK_GPU(accepted);
    CHECK_GPU(image.pixelColor(105, 128).red() > 230);
    CHECK_GPU(image.pixelColor(150, 128).green() > 230);
    CHECK_GPU(image.pixelColor(150, 128).red() < 20);

    // Signage boxes start at the bottom, unlike image billboards. QImage's
    // first row must still appear above its last row on that vertex ordering.
    auto sign = quad(40, 20, Qt::white);
    sign.vertices = {{-20,-20,40}, {20,-20,40}, {20,20,40}, {-20,20,40}};
    sign.uvs = {{0,1}, {1,1}, {1,0}, {0,0}};
    sign.image = QImage(8, 8, QImage::Format_RGBA8888);
    sign.image.fill(Qt::blue);
    {
        QPainter artwork(&sign.image);
        artwork.fillRect(0, 0, 8, 4, Qt::red);
    }
    image = render({sign});
    CHECK_GPU(accepted && image.pixelColor(128, 100).red() > 230);
    CHECK_GPU(image.pixelColor(128, 156).blue() > 230);

    // Custom UVs and yaw must apply to every instance, including alpha holes.
    auto instanceCutout = cutout;
    instanceCutout.vertices = quad(0, 10, Qt::white).vertices;
    sharedMesh->first() = instanceCutout;
    group.instances[1].heading = 3.14159265f;
    group.instances[1].shade = 1;
    image = render({far}, 0, {group});
    CHECK_GPU(accepted && renderer.drawCalls() == 2);
    CHECK_GPU(image.pixelColor(84, 128).green() > 230);
    CHECK_GPU(image.pixelColor(96, 128).green() < 20);
    CHECK_GPU(image.pixelColor(156, 128).green() < 20);
    CHECK_GPU(image.pixelColor(176, 128).green() > 230);
    sharedMesh->first().uvs = {{1, 0}, {0, 0}, {0, 1}, {1, 1}};
    image = render({far}, 0, {group});
    CHECK_GPU(accepted && image.pixelColor(96, 128).green() > 230);
    CHECK_GPU(image.pixelColor(84, 128).green() < 20);

    // The ridden bike has an eight-corner contact shadow. It must never silently
    // switch backends during boarding, motion or a crash.
    auto shadow = quad(30, 10, QColor(0, 0, 0, 90));
    shadow.vertices = {{-10, 5, 30}, {-5, 10, 30}, {5, 10, 30}, {10, 5, 30},
        {10, -5, 30}, {5, -10, 30}, {-5, -10, 30}, {-10, -5, 30}};
    image = render({near, shadow});
    std::printf("alpha_pass=%d green=%d error=%s\n", accepted, image.pixelColor(128, 128).green(), qPrintable(renderer.error()));
    CHECK_GPU(accepted && image.pixelColor(128, 128).green() < 200);

    // Environment orientation follows the same camera axes as scene geometry.
    camera.environment = QImage(64, 32, QImage::Format_RGB32);
    camera.environment.fill(Qt::cyan);
    {
        QPainter sky(&camera.environment);
        sky.fillRect(0, 0, 64, 16, Qt::yellow);
    }
    image = render({near});
    CHECK_GPU(accepted && image.pixelColor(128, 128).green() > 240);
    CHECK_GPU(image.pixelColor(200, 40).red() > 230);
    CHECK_GPU(image.pixelColor(200, 210).blue() > 230);
    camera.environment = {};

    // A real caster must darken a receiver, remain identical on repeated
    // frames, and avoid shadow-depth flicker under tiny camera translations.
    {
        const auto originalCamera = camera;
        camera.position = {0, 500, -800};
        camera.forward = QVector3D(0, -500, 800).normalized();
        camera.up = QVector3D::crossProduct(camera.forward, camera.right).normalized();
        SpatialGpuRenderer::Face ground;
        ground.vertices = {{-1200, 0, -1200}, {-1200, 0, 1200}, {1200, 0, 1200}, {1200, 0, -1200}};
        ground.normal = {0, 1, 0};
        ground.color = QColor(150, 150, 150);
        auto roof = ground;
        roof.vertices = {{-120, 220, -120}, {-120, 220, 120}, {120, 220, 120}, {120, 220, -120}};
        const auto litGround = render({ground});
        const auto shadowed = render({ground, roof});
        CHECK_GPU(accepted && shadowed == render({ground, roof}));
        int shadowPixels = 0;
        for (int y = 16; y < 240; ++y)
            for (int x = 16; x < 240; ++x)
                shadowPixels += qGray(litGround.pixel(x, y)) - qGray(shadowed.pixel(x, y)) > 20;
        CHECK_GPU(shadowPixels > 100);
        camera.position += QVector3D(.01f, 0, .01f);
        const auto nudged = render({ground, roof});
        int flashingPixels = 0;
        for (int y = 16; y < 240; ++y)
            for (int x = 16; x < 240; ++x)
                flashingPixels += std::abs(qGray(nudged.pixel(x, y)) - qGray(shadowed.pixel(x, y))) > 8;
        std::printf("shadow_receiver_pixels=%d subtexel_changed_pixels=%d\n", shadowPixels, flashingPixels);
        CHECK_GPU(flashingPixels < 40 && gl->glGetError() == GL_NO_ERROR);
        camera = originalCamera;
    }

    // Material data shares residency across faces. Its roughness alpha must
    // never make opaque terrain transparent. Compare lit/unlit images so the
    // check measures normal-driven lighting, not merely albedo contrast.
    CHECK_GPU(SpatialSurfaceTextures::nativeDetail(SpatialSurfaceMaterial::Plain).isNull());
    CHECK_GPU(SpatialSurfaceTextures::nativeDetail(static_cast<SpatialSurfaceMaterial>(99)).isNull());
    // Exercise the authored 1K detail at one texture texel per framebuffer
    // pixel. A minified photograph's lighting extrema depend on driver mip
    // generation/filtering and cannot define a backend-independent contrast
    // bound. Minification and terrain continuity have separate checks below.
    const auto detailCamera = camera;
    camera.viewport = {1024, 1024};
    camera.focalLength = 512;
    for (const auto material : {SpatialSurfaceMaterial::Dirt, SpatialSurfaceMaterial::Grass, SpatialSurfaceMaterial::Rock}) {
        const auto detail = SpatialSurfaceTextures::nativeDetail(material);
        CHECK_GPU(detail.size() == QSize(1024, 1024));
        CHECK_GPU(detail.sizeInBytes() == 4 * 1024 * 1024);
        CHECK_GPU(detail.cacheKey() == SpatialSurfaceTextures::nativeDetail(material).cacheKey());
        int roughnessMinimum = 255;
        int roughnessMaximum = 0;
        for (int y = 0; y < detail.height(); y += 16) {
            for (int x = 0; x < detail.width(); x += 16) {
                const int roughness = detail.pixelColor(x, y).alpha();
                roughnessMinimum = std::min(roughnessMinimum, roughness);
                roughnessMaximum = std::max(roughnessMaximum, roughness);
            }
        }
        CHECK_GPU(roughnessMaximum - roughnessMinimum > 30);
        auto materialFace = quad(100, 100, QColor(120, 120, 120));
        materialFace.material = material;
        materialFace.uvs = {{0, 0}, {1, 0}, {1, 1}, {0, 1}};
        const auto unlit = render({materialFace});
        CHECK_GPU(accepted);
        materialFace.unlit = false;
        const auto lit = render({materialFace});
        CHECK_GPU(accepted && renderer.drawCalls() == 1);
        double minimumLight = 100;
        double maximumLight = 0;
        for (int y = 24; y < camera.viewport.height() - 24; y += 3) {
            for (int x = 24; x < camera.viewport.width() - 24; x += 3) {
                const int base = qGray(unlit.pixel(x, y));
                if (base < 40)
                    continue;
                const double light = double(qGray(lit.pixel(x, y))) / base;
                minimumLight = std::min(minimumLight, light);
                maximumLight = std::max(maximumLight, light);
                CHECK_GPU(lit.pixelColor(x, y).alpha() == 255);
            }
        }
        std::printf("material=%d lighting_range=%.3f roughness=%d..%d\n",
            int(material), maximumLight - minimumLight, roughnessMinimum, roughnessMaximum);
        CHECK_GPU(maximumLight - minimumLight > .2);
        CHECK_GPU(gl->glGetError() == GL_NO_ERROR);
    }

    camera = detailCamera;

    // World-space terrain variation must be continuous across mesh splits,
    // stable on repeated renders, and avoid repeating at one source-tile step.
    // Explicit UVs isolate large-scale placement from photographed detail.
    auto terrain = quad(100, 100, QColor(120, 120, 120));
    terrain.material = SpatialSurfaceMaterial::Grass;
    terrain.uvs = {{0, 0}, {1, 0}, {1, 1}, {0, 1}};
    const auto wholeTerrain = render({terrain});
    CHECK_GPU(accepted);
    const auto repeatedTerrain = render({terrain});
    CHECK_GPU(accepted && wholeTerrain == repeatedTerrain);
    auto leftTerrain = terrain;
    leftTerrain.vertices[1].setX(0);
    leftTerrain.vertices[2].setX(0);
    leftTerrain.uvs[1].setX(.5f);
    leftTerrain.uvs[2].setX(.5f);
    auto rightTerrain = terrain;
    rightTerrain.vertices[0].setX(0);
    rightTerrain.vertices[3].setX(0);
    rightTerrain.uvs[0].setX(.5f);
    rightTerrain.uvs[3].setX(.5f);
    const auto splitTerrain = render({leftTerrain, rightTerrain});
    CHECK_GPU(accepted);
    for (int y = 32; y < 224; y += 7) {
        for (int x = 118; x < 138; ++x) {
            const auto whole = wholeTerrain.pixelColor(x, y);
            const auto split = splitTerrain.pixelColor(x, y);
            CHECK_GPU(std::abs(whole.red() - split.red()) <= 2);
            CHECK_GPU(std::abs(whole.green() - split.green()) <= 2);
            CHECK_GPU(std::abs(whole.blue() - split.blue()) <= 2);
        }
    }
    const QVector3D terrainOffset(5100, 0, 0);
    for (auto &point : terrain.vertices)
        point += terrainOffset;
    camera.position += terrainOffset;
    const auto distantTerrain = render({terrain});
    CHECK_GPU(accepted);
    int differentTerrainPixels = 0;
    for (int y = 32; y < 224; y += 8) {
        for (int x = 32; x < 224; x += 8) {
            if (std::abs(qGray(wholeTerrain.pixel(x, y)) - qGray(distantTerrain.pixel(x, y))) > 3)
                ++differentTerrainPixels;
        }
    }
    CHECK_GPU(differentTerrainPixels > 40);
    camera.position -= terrainOffset;

    // Native geometry receives actual multisample coverage, not a blurred or
    // lower-resolution intermediate. The surrounding painter image survives.
    auto triangle = near;
    triangle.vertices = {{-19, 17, 50}, {21, 6, 50}, {-10, -23, 50}};
    triangle.color = Qt::white;
    camera.samples = 4;
    image = render({triangle});
    CHECK_GPU(accepted && image.pixelColor(4, 4).blue() > 240);
    CHECK_GPU(renderer.samples() == 0 || renderer.samples() == 4);
    if (renderer.samples()) {
        int coveredEdges = 0;
        for (int y = 16; y < image.height(); ++y) {
            for (int x = 16; x < image.width(); ++x) {
                const auto pixel = image.pixelColor(x, y);
                if (pixel.red() > 80 && pixel.red() < 225 && std::abs(pixel.red() - pixel.green()) < 3)
                    ++coveredEdges;
            }
        }
        CHECK_GPU(coveredEdges > 20);
    }
    CHECK_GPU(gl->glGetError() == GL_NO_ERROR);
    image = render({near, far}, 4);
    CHECK_GPU(accepted && image.pixelColor(128, 128).green() > 240);
    CHECK_GPU(image.pixelColor(4, 4).blue() > 240 && gl->glGetError() == GL_NO_ERROR);
    camera.samples = 0;

    // Filtered leaf/text alpha should cover individual depth samples rather
    // than snap every texel to solid/absent. The resolved scene remains opaque.
    auto leaf = near;
    leaf.cutout = true;
    leaf.image = QImage(128, 1, QImage::Format_ARGB32);
    for (int x = 0; x < leaf.image.width(); ++x)
        leaf.image.setPixel(x, 0, qRgba(0, 255, 0, x * 255 / 127));
    const auto partialCoverage = [&](const QImage &result) {
        int partial = 0;
        for (int x = 92; x < 164; ++x) {
            const auto pixel = result.pixelColor(x, 128);
            if (pixel.alpha() != 255)
                return -1000;
            if (pixel.green() > 10 && pixel.green() < 230)
                ++partial;
        }
        return partial;
    };
    image = render({far, leaf});
    CHECK_GPU(accepted && partialCoverage(image) == 0);
    camera.samples = 4;
    image = render({far, leaf});
    CHECK_GPU(accepted && image.pixelColor(4, 4).blue() > 240);
    if (renderer.samples())
        CHECK_GPU(partialCoverage(image) > 15);
    CHECK_GPU(coverageStateRestored && gl->glGetError() == GL_NO_ERROR);
    gl->glEnable(GL_SAMPLE_ALPHA_TO_COVERAGE);
    gl->glEnable(GL_SAMPLE_ALPHA_TO_ONE);
    image = render({far, leaf});
    CHECK_GPU(accepted && coverageStateRestored);
    gl->glDisable(GL_SAMPLE_ALPHA_TO_COVERAGE);
    gl->glDisable(GL_SAMPLE_ALPHA_TO_ONE);
    camera.samples = 0;

    camera.focalLength = std::numeric_limits<float>::quiet_NaN();
    render({near});
    CHECK_GPU(!accepted && !renderer.error().isEmpty());
    camera.focalLength = 128;
    camera.up = camera.right;
    render({near});
    CHECK_GPU(!accepted);
    camera.up = {0, 1, 0};

    auto invalid = near;
    invalid.vertices[0].setX(std::numeric_limits<float>::quiet_NaN());
    image = render({invalid});
    CHECK_GPU(!accepted && !renderer.error().isEmpty());
    CHECK_GPU(image.pixelColor(4, 4).blue() > 240);
    invalid = near;
    invalid.normal.setX(std::numeric_limits<float>::infinity());
    render({invalid});
    CHECK_GPU(!accepted);
    invalid = near;
    invalid.normals = {{0, 0, 0}};
    render({invalid});
    CHECK_GPU(!accepted);
    QVector<SpatialGpuRenderer::Face> excessive(SpatialGpuRenderer::maximumVertices / 6 + 1, near);
    render(excessive);
    CHECK_GPU(!accepted);

    // A frame must not evict textures that it has already admitted for its own
    // batches. Exhaust the 64 MiB budget, preserve the painter, then verify that
    // later bounded frames can evict/reload assets and recover without a reset.
    {
        QVector<SpatialGpuRenderer::Face> textureBurst;
        for (int index = 0; index < 14; ++index) {
            auto textured = near;
            textured.image = QImage(1024, 1024, QImage::Format_RGB32);
            textured.image.fill(QColor(20 + index * 15, 220 - index * 10, 80));
            textureBurst.append(std::move(textured));
        }
        image = render(textureBurst);
        CHECK_GPU(!accepted && renderer.error().contains("residency"));
        CHECK_GPU(renderer.drawCalls() == 0 && image.pixelColor(4, 4).blue() > 240);
        CHECK_GPU(renderer.residentTextureBytes() > 0);
        CHECK_GPU(renderer.residentTextureBytes() <= SpatialGpuRenderer::maximumTextureBytes);
        for (const auto &textured : textureBurst) {
            image = render({textured});
            CHECK_GPU(accepted && renderer.error().isEmpty());
            CHECK_GPU(renderer.residentTextureBytes() <= SpatialGpuRenderer::maximumTextureBytes);
            const auto expected = textured.image.pixelColor(0, 0);
            const auto actual = image.pixelColor(128, 128);
            CHECK_GPU(std::abs(actual.red() - expected.red()) <= 2);
            CHECK_GPU(std::abs(actual.green() - expected.green()) <= 2);
        }
        image = render({textureBurst.first()});
        CHECK_GPU(accepted && image.pixelColor(128, 128).green() > 215);
        std::printf("native_texture_residency_bytes=%d budget=%d\n",
            renderer.residentTextureBytes(), SpatialGpuRenderer::maximumTextureBytes);
        CHECK_GPU(coverageStateRestored && gl->glGetError() == GL_NO_ERROR);
    }
    // Resize and explicit cleanup reconstruct resources without stale images.
    for (int pass = 0; pass < 3; ++pass) {
        context.doneCurrent();
        renderer.release();
        CHECK_GPU(renderer.residentTextureBytes() == 0);
        CHECK_GPU(QOpenGLContext::currentContext() == nullptr);
        CHECK_GPU(context.makeCurrent(&surface));
        camera.viewport = {256 + pass * 32, 256 + pass * 32};
        image = render({near, far}, 0, {group});
        CHECK_GPU(accepted && image.size() == camera.viewport);
        CHECK_GPU(image.pixelColor(image.width() / 2, image.height() / 2).green() > 230);
    }
    CHECK_GPU(gl->glGetError() == GL_NO_ERROR);
    // Fullscreen/reparenting can replace the viewport's context. Exercise two
    // independent contexts, with cleanup temporarily reacquiring the old one.
    QOpenGLContext secondContext;
    secondContext.setFormat(format);
    CHECK_GPU(secondContext.create());
    QOffscreenSurface secondSurface;
    secondSurface.setFormat(secondContext.format());
    secondSurface.create();
    CHECK_GPU(secondSurface.isValid() && secondContext.makeCurrent(&secondSurface));
    image = render({near, far}, 0, {group});
    CHECK_GPU(accepted && QOpenGLContext::currentContext() == &secondContext);
    CHECK_GPU(image.pixelColor(image.width() / 2, image.height() / 2).green() > 230);
    CHECK_GPU(secondContext.functions()->glGetError() == GL_NO_ERROR);
    CHECK_GPU(context.makeCurrent(&surface));
    image = render({near, far});
    CHECK_GPU(accepted && QOpenGLContext::currentContext() == &context);
    CHECK_GPU(image.pixelColor(image.width() / 2, image.height() / 2).green() > 230);
    CHECK_GPU(gl->glGetError() == GL_NO_ERROR);
    renderer.release();
    CHECK_GPU(renderer.residentTextureBytes() == 0);
    context.doneCurrent();
    std::printf("native_geometry: depth, cutouts, compositing, bounds, resize and cleanup passed\n");
    return 0;
}
