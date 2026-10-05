#include "boundary.hpp"
#include "pilot_binding_adapter.hpp"
#include <QPointer>
#include <QTimer>
#include <iostream>
#include <thread>
#include <vector>
using namespace pipelang_qt;
static void check(bool condition, const char* what) {
    if (!condition) throw std::runtime_error(what);
}
class Producer final : public QObject {
    Q_OBJECT
signals:
    void completed(pipelang_qt::Completion value);
};
int main(int argc, char** argv) {
    QCoreApplication app(argc, argv);
    try {
        const std::vector<std::string> texts = {"", std::string("a\0b",3), u8"é", u8"é", u8"😀", u8"﻿prefix", u8"﻿", std::string(1<<20,'x')};
        for (auto text : texts) {
            const auto expected = text;
            auto converted = fromText(text);
            check(bool(converted), "valid text rejected");
            text.assign("mutated");
            auto owned = toText(*converted);
            check(owned && *owned == expected, "text ownership or scalar bytes changed");
            converted->fill('z');
            check(*owned == expected, "reverse text ownership");
        }
        for (auto text : {std::string("\x80"), std::string("\xc0\xaf"), std::string("\xed\xa0\x80"),
                          std::string("\xf4\x90\x80\x80"), std::string("\xe2\x82"), std::string((1<<20)+1,'x')})
            check(!fromText(text), "malformed or oversized UTF-8 accepted");
        check(!toText(QString(QChar(0xd800))), "unpaired high surrogate");
        check(!toText(QString(QChar(0xdc00))), "unpaired low surrogate");
        check(!toText(QString((1<<20)+1,'x')), "oversized UTF-16");
        QObject owner;
        auto* delivery = new Delivery(&owner);
        int updates = 0;
        QObject::connect(delivery, &Delivery::changed, &owner, [&] {
            check(QThread::currentThread() == app.thread(), "notification on worker");
            ++updates;
        });
        check(!delivery->accept({0,Integer{9}}), "unsolicited completion");
        for (const auto number : {std::numeric_limits<std::int64_t>::min(), std::int64_t(9007199254740993LL), std::numeric_limits<std::int64_t>::max()}) {
            check(delivery->accept({delivery->begin(),Integer{number}}), "integer delivery");
            check(std::get<Integer>(delivery->result()->outcome).value == number, "integer precision");
        }
        delivery->accept({delivery->begin(),Boolean{false}});
        check(delivery->display() == "false", "Boolean value");
        const auto overflow = pilot_calculate(std::numeric_limits<std::int64_t>::max(),1);
        check(!overflow.ok, "generated overflow oracle");
        delivery->accept({delivery->begin(), fromArithmetic(overflow)});
        check(delivery->display() == "Arithmetic overflow" && std::holds_alternative<DomainFailure>(delivery->result()->outcome), "typed error erased");
        delivery->accept({delivery->begin(),Text{std::string("\x80")}});
        check(std::get<AdapterFailure>(delivery->result()->outcome).code == AdapterCode::InvalidText, "invalid text error domain");
        delivery->accept({delivery->begin(),DomainFailure{static_cast<DomainCode>(99)}});
        check(std::get<AdapterFailure>(delivery->result()->outcome).code == AdapterCode::UnsupportedError, "unknown domain error");
        check(std::holds_alternative<AdapterFailure>(fromArithmetic({true, 42, pipelang::Error::overflow})), "noncanonical success");
        check(std::holds_alternative<AdapterFailure>(fromArithmetic({false, 42, pipelang::Error::overflow})), "noncanonical failure");
        check(std::get<AdapterFailure>(fromArithmetic({false, 0, pipelang::Error::division_by_zero})).code == AdapterCode::UnsupportedError, "unmapped generated error");
        Producer producer;
        QObject::connect(&producer, &Producer::completed, delivery, &Delivery::accept, Qt::QueuedConnection);
        const int beforeReset = updates;
        const auto stale = delivery->begin();
        check(delivery->display().isEmpty() && updates == beforeReset + 1, "property reset notification");
        const auto current = delivery->begin();
        const int before = updates;
        std::thread worker([&] {
            check(!delivery->accept({current,Integer{99}}), "direct worker call accepted");
            emit producer.completed({stale,Integer{1}});
            const auto generated = pilot_calculate(19,23);
            check(generated.ok && generated.value == 42, "generated success oracle");
            emit producer.completed({current,fromArithmetic(generated)});
            emit producer.completed({current,Integer{100}}); // Duplicate terminal result.
        });
        worker.join();
        check(updates == before, "queued delivery ran inline");
        QCoreApplication::sendPostedEvents(delivery, QEvent::MetaCall);
        check(updates == before+1 && delivery->display() == "42", "stale/duplicate/queued arbitration");
        QPointer<Delivery> weak;
        int destroyed = 0, late = 0;
        {
            QObject temporaryOwner;
            auto* temporary = new Delivery(&temporaryOwner);
            weak = temporary;
            QObject::connect(temporary, &QObject::destroyed, &owner, [&] { ++destroyed; });
            QObject::connect(temporary, &Delivery::changed, &owner, [&] { ++late; });
            QObject::connect(&producer, &Producer::completed, temporary, &Delivery::accept, Qt::QueuedConnection);
            emit producer.completed({temporary->begin(),Text{"late"}});
        }
        QCoreApplication::sendPostedEvents(nullptr, QEvent::MetaCall);
        check(weak.isNull() && destroyed == 1 && late == 0, "owner destruction failed to cancel queued delivery");
        bool refused = false;
        try { Delivery invalid(nullptr); } catch (const std::invalid_argument&) { refused = true; }
        check(refused, "unowned QObject accepted");
        std::cout << "{\"passed\":true,\"checks\":\"owned scalar/text; strict encoding; typed errors; generated values; worker queue; stale/duplicate; destruction\"}\n";
        return 0;
    } catch (const std::exception& e) {
        std::cerr << e.what() << '\n';
        return 1;
    }
}
#include "boundary_test.moc"
