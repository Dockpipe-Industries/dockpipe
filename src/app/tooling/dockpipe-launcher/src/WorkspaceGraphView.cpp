#include "WorkspaceGraphView.h"
#include <QGuiApplication>
#include <QOffscreenSurface>
#include <QOpenGLContext>
#include <QOpenGLFunctions>
#include <QOpenGLWidget>
#include <QPointer>
#include <QTimer>

WorkspaceGraphView::WorkspaceGraphView(QWidget *parent) : CanvasView(parent)
{
    updateCameraBasis();
    m_hardwareRequested = !qEnvironmentVariableIsSet("DOCKPIPE_GRAPH_SOFTWARE");
    m_rendererDescription = tr("Software · 2D view");
}

void WorkspaceGraphView::setHardwareAcceleration(bool enabled, bool deferUntilShown)
{
    m_hardwareRequested = enabled;
    m_hardwareFailed = false;
    if (!deferUntilShown)
        updateRenderer();
}

void WorkspaceGraphView::showEvent(QShowEvent *event)
{
    CanvasView::showEvent(event);
    updateRenderer();
}

void WorkspaceGraphView::useSoftware(const QString &reason)
{
    if (qobject_cast<QOpenGLWidget *>(viewport()))
        setViewport(new QWidget);
    setViewportUpdateMode(MinimalViewportUpdate);
    m_hardwareActive = false;
    m_rendererDescription = reason;
    if (rendererChanged)
        rendererChanged();
    viewport()->update();
}

void WorkspaceGraphView::updateRenderer()
{
    if (!m_spatial || !m_hardwareRequested) {
        useSoftware(m_spatial ? tr("Software · acceleration disabled") : tr("Software · 2D view"));
        return;
    }
    if (m_hardwareActive || m_hardwareFailed)
        return;
    const QString platform = QGuiApplication::platformName();
    if (platform == "offscreen" || platform == "minimal") {
        m_hardwareFailed = true;
        useSoftware(tr("Software · this display does not support OpenGL widgets"));
        return;
    }

    // Probe only when 3D is requested. A software GL implementation is not
    // advertised as hardware acceleration, and failed probes retain raster UI.
    QOpenGLContext probe;
    QOffscreenSurface surface;
    if (!probe.create()) {
        m_hardwareFailed = true;
        useSoftware(tr("Software · OpenGL context unavailable"));
        return;
    }
    surface.setFormat(probe.format());
    surface.create();
    if (!surface.isValid() || !probe.makeCurrent(&surface)) {
        m_hardwareFailed = true;
        useSoftware(tr("Software · OpenGL surface unavailable"));
        return;
    }
    const auto *rendererBytes = probe.functions()->glGetString(GL_RENDERER);
    const QString renderer = rendererBytes ? QString::fromLatin1(reinterpret_cast<const char *>(rendererBytes)) : QString();
    probe.doneCurrent();
    const QString normalized = renderer.toLower();
    if (renderer.isEmpty() || normalized.contains("llvmpipe") || normalized.contains("softpipe")
        || normalized.contains("software") || normalized.contains("basic render")) {
        m_hardwareFailed = true;
        useSoftware(tr("Software · no hardware OpenGL renderer detected"));
        return;
    }

    auto *gl = new QOpenGLWidget;
    gl->setFormat(probe.format());
    gl->setUpdateBehavior(QOpenGLWidget::NoPartialUpdate);
    setViewportUpdateMode(FullViewportUpdate);
    setViewport(gl);
    m_hardwareActive = true;
    m_rendererDescription = tr("OpenGL · initializing %1").arg(renderer);
    if (rendererChanged)
        rendererChanged();
    // One startup validation, not a render timer. The scene remains input-driven.
    const QPointer<QOpenGLWidget> guarded(gl);
    QTimer::singleShot(250, this, [this, guarded, renderer]() {
        if (!guarded || viewport() != guarded)
            return;
        if (!guarded->isValid()) {
            m_hardwareFailed = true;
            useSoftware(tr("Software · OpenGL viewport initialization failed"));
            return;
        }
        m_rendererDescription = tr("GPU · %1").arg(renderer);
        if (rendererChanged)
            rendererChanged();
    });
}
