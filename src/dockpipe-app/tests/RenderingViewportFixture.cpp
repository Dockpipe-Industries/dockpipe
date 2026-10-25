// Host ABI and late-loading qualification transferred from
// SpatialPipe/packages/cockpit/tests/RenderingViewportFixture.cpp.
// The package implementation and its feature tests remain owned by SpatialPipe.
#include <QOpenGLWidget>

// This test-only library is loaded after the host backing store and optional
// package exist. It never belongs to the workspace package's public assets.
extern "C" Q_DECL_EXPORT QWidget *dockpipe_test_create_gl_viewport(const QSurfaceFormat *format)
{
    if (!format)
        return nullptr;
    auto *viewport = new QOpenGLWidget;
    viewport->setFormat(*format);
    viewport->setUpdateBehavior(QOpenGLWidget::NoPartialUpdate);
    return viewport;
}

extern "C" Q_DECL_EXPORT bool dockpipe_test_gl_viewport_valid(const QWidget *viewport)
{
    const auto *gl = qobject_cast<const QOpenGLWidget *>(viewport);
    return gl && gl->isValid();
}
