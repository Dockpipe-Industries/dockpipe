#pragma once
#include <QCoreApplication>
#include <QObject>
#include <QString>
#include <QStringDecoder>
#include <QThread>
#include <cstdint>
#include <limits>
#include <optional>
#include <stdexcept>
#include <string>
#include <variant>

// Experimental resolver-side reference adapter. No Qt types cross into Core.
namespace pipelang_qt {
constexpr std::size_t maxTextBytes = 1 << 20; // Explicit adapter profile, not a language limit.
inline std::optional<QString> fromText(const std::string& text) {
    if (text.size() > maxTextBytes) return std::nullopt;
    QStringDecoder decoder(QStringDecoder::Utf8, QStringDecoder::Flag::Stateless | QStringDecoder::Flag::ConvertInitialBom);
    QString value = decoder(QByteArrayView(text.data(), qsizetype(text.size())));
    if (decoder.hasError()) return std::nullopt;
    return value;
}
inline std::optional<std::string> toText(const QString& text) {
    if (text.size() > qsizetype(maxTextBytes)) return std::nullopt;
    // QString permits unpaired UTF-16 surrogates; toUtf8 alone replaces them.
    for (qsizetype i = 0; i < text.size(); ++i) {
        const auto c = text[i];
        if (c.isHighSurrogate()) {
            if (++i == text.size() || !text[i].isLowSurrogate()) return std::nullopt;
        } else if (c.isLowSurrogate()) return std::nullopt;
    }
    const auto bytes = text.toUtf8();
    if (bytes.size() > qsizetype(maxTextBytes)) return std::nullopt;
    return std::string(bytes.constData(), std::size_t(bytes.size()));
}
struct Integer { std::int64_t value; };
struct Boolean { bool value; };
struct Text { std::string value; };
enum class DomainCode { ArithmeticOverflow };
struct DomainFailure { DomainCode code; };
// Host/infrastructure refusal cannot impersonate a language Result error.
enum class AdapterCode { InvalidText, UnsupportedError, HostFailure };
struct AdapterFailure { AdapterCode code; };
using Outcome = std::variant<Integer, Boolean, Text, DomainFailure, AdapterFailure>;
struct Completion { quint64 request; Outcome outcome; };

inline QString presentation(const Outcome& value) {
    return std::visit([](const auto& v) -> QString {
        using T = std::decay_t<decltype(v)>;
        if constexpr (std::is_same_v<T, Integer>) return QString::number(qlonglong(v.value));
        else if constexpr (std::is_same_v<T, Boolean>) return v.value ? "true" : "false";
        else if constexpr (std::is_same_v<T, Text>) return fromText(v.value).value_or(QString("Host result unavailable"));
        else if constexpr (std::is_same_v<T, DomainFailure>) return "Arithmetic overflow";
        else return "Host result unavailable";
    }, value);
}

} // namespace pipelang_qt
Q_DECLARE_METATYPE(pipelang_qt::Completion)
namespace pipelang_qt {
class Delivery final : public QObject {
    Q_OBJECT
    Q_PROPERTY(QString display READ display NOTIFY changed)
public:
    explicit Delivery(QObject* owner) : QObject(nullptr) {
        auto* app = QCoreApplication::instance();
        if (!app || !owner || owner->thread() != app->thread() || QThread::currentThread() != app->thread())
            throw std::invalid_argument("delivery requires a GUI-thread owner");
        setParent(owner);
        qRegisterMetaType<Completion>();
    }
    quint64 begin() {
        requireOwnerThread();
        if (request_ == std::numeric_limits<quint64>::max()) throw std::overflow_error("request identity exhausted");
        const auto request = ++request_;
        const bool hadResult = result_.has_value();
        result_.reset();
        if (hadResult) emit changed();
        return request;
    }
    const std::optional<Completion>& result() const { requireOwnerThread(); return result_; }
    QString display() const { requireOwnerThread(); return result_ ? presentation(result_->outcome) : QString(); }
public slots:
    bool accept(pipelang_qt::Completion completion) {
        // Release-build enforcement, before observing mutable owner state.
        if (QThread::currentThread() != thread()) return false;
        if (completion.request == 0 || completion.request != request_ || result_) return false;
        if (const auto* text = std::get_if<Text>(&completion.outcome); text && !fromText(text->value))
            completion.outcome = AdapterFailure{AdapterCode::InvalidText};
        if (const auto* error = std::get_if<DomainFailure>(&completion.outcome);
            error && error->code != DomainCode::ArithmeticOverflow)
            completion.outcome = AdapterFailure{AdapterCode::UnsupportedError};
        result_ = std::move(completion);
        emit changed();
        return true;
    }
signals:
    void changed();
private:
    void requireOwnerThread() const {
        if (QThread::currentThread() != thread()) throw std::logic_error("GUI thread required");
    }
    quint64 request_ = 0;
    std::optional<Completion> result_;
};
} // namespace pipelang_qt
