#include "MermaidRenderer.h"

namespace {
MermaidRenderer::Factory rendererFactory;
}

void MermaidRenderer::setFactory(Factory factory)
{
    rendererFactory = std::move(factory);
}

MermaidRenderer *MermaidRenderer::create(QObject *parent)
{
    return rendererFactory ? rendererFactory(parent) : nullptr;
}
