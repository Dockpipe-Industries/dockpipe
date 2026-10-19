#pragma once

#include "MermaidRenderer.h"
#include <QPointer>
#include <QTimer>

class QWebEnginePage;
class QWebEngineProfile;

// Browser-only implementation. Each operation owns a fresh, isolated page.
class WebMermaidRenderer final : public MermaidRenderer {
    Q_OBJECT
public:
    explicit WebMermaidRenderer(QObject *parent = nullptr);
    ~WebMermaidRenderer() override;
    void render(const QString &source, bool dark) override;
    void cancel() override;

private:
    void poll(quint64 generation);
    void complete(const QImage &image, const QString &error);
    QWebEngineProfile *m_profile = nullptr;
    QPointer<QWebEnginePage> m_page;
    QPointer<QWebEnginePage> m_retiredPage;
    QTimer m_timeout;
    quint64 m_generation = 0;
};

void installWebMermaidRenderer();
