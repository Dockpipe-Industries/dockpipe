#pragma once

#include <QPointF>
#include <algorithm>
#include <cmath>

// View-only offsets: looking around never changes steering or rider balance.
// Simulation time pauses with the view, so menus cannot consume the hold delay.
class DirtBikeCameraLook {
public:
    void reset()
    {
        m_offset = {};
        m_holdRemaining = 0;
    }

    void nudge(const QPointF &radians)
    {
        if (!std::isfinite(radians.x()) || !std::isfinite(radians.y()) || radians.isNull())
            return;
        m_offset.setX(std::clamp(m_offset.x() + radians.x(), -3.0, 3.0));
        m_offset.setY(std::clamp(m_offset.y() + radians.y(), -.65, .9));
        m_holdRemaining = 2;
    }

    void advance(const QPointF &stick, double seconds)
    {
        if (!std::isfinite(seconds) || seconds <= 0)
            return;
        seconds = std::min(seconds, .05);
        // SDL already applies its hardware dead zone; this small threshold also
        // keeps numerical noise from postponing the return indefinitely.
        const auto axis = [](double value) {
            return std::isfinite(value) && std::abs(value) > .02
                ? std::clamp(value, -1.0, 1.0) : 0.0;
        };
        const QPointF input(axis(stick.x()), axis(stick.y()));
        if (!input.isNull()) {
            nudge(QPointF(input.x() * 1.8, input.y() * 1.2) * seconds);
            return;
        }
        const double held = std::min(seconds, m_holdRemaining);
        m_holdRemaining -= held;
        seconds -= held;
        m_offset *= std::exp(-4.0 * seconds);
        if (m_holdRemaining == 0 && std::hypot(m_offset.x(), m_offset.y()) < .001)
            m_offset = {};
    }

    QPointF offset() const { return m_offset; }
    bool pending() const { return m_holdRemaining > 0 || !m_offset.isNull(); }

private:
    QPointF m_offset;
    double m_holdRemaining = 0;
};
