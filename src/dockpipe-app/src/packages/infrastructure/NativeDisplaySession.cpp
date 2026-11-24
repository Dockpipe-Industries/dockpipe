#include "NativeDisplaySession.h"

#include <QJsonDocument>
#include <QMetaMethod>
#include <QVariant>

NativeDisplaySession::NativeDisplaySession(QObject *parent) : QObject(parent)
{
    m_deadline.setSingleShot(true);
    connect(&m_deadline, &QTimer::timeout, this, [this] { finish({}); });
}

bool NativeDisplaySession::requestClose(QObject *surface)
{
    cancel();
    if (!surface)
        return false;
    const auto *meta = surface->metaObject();
    const auto budget = surface->property("closeCompletionBudgetMs");
    const auto reference = surface->property("closeReference");
    if (meta->indexOfMethod("requestClose()") < 0
        || meta->indexOfSignal("closeCompleted(QJsonObject)") < 0
        || budget.metaType().id() != QMetaType::Int || budget.toInt() < 1 || budget.toInt() > 12000
        || reference.metaType().id() != QMetaType::QJsonObject
        || QJsonDocument(reference.toJsonObject()).toJson(QJsonDocument::Compact).size() > 16 * 1024)
        return false;
    m_surface = surface;
    m_reference = reference.toJsonObject();
    m_pending = true;
    connect(surface, SIGNAL(closeCompleted(QJsonObject)), this, SLOT(acceptCompletion(QJsonObject)));
    connect(surface, &QObject::destroyed, this, [this] { finish({}); });
    m_deadline.start(budget.toInt());
    if (!QMetaObject::invokeMethod(surface, "requestClose", Qt::DirectConnection)) {
        cancel();
        return false;
    }
    return true;
}

void NativeDisplaySession::cancel()
{
    m_pending = false;
    m_deadline.stop();
    if (m_surface)
        disconnect(m_surface, nullptr, this, nullptr);
    m_surface.clear();
    m_reference = {};
}

void NativeDisplaySession::acceptCompletion(const QJsonObject &result)
{
    if (!m_pending || sender() != m_surface || result.value("reference").toObject() != m_reference
        || !result.value("reference").isObject()
        || QJsonDocument(result).toJson(QJsonDocument::Compact).size() > 16 * 1024)
        return;
    // No prepared reference can prove closure. It can still finish local teardown.
    if (m_reference.isEmpty() && result.value("outcome") != QJsonValue("unknown"))
        return;
    finish(result);
}

void NativeDisplaySession::finish(const QJsonObject &result)
{
    if (!m_pending)
        return;
    cancel();
    emit closeCompleted(result);
}
