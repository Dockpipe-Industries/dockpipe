#include <QApplication>
#include <QCloseEvent>
#include <QIcon>
#include <QLabel>
#include <QLineEdit>
#include <QPalette>
#include <QPixmap>
#include <QTimer>
#include <QVBoxLayout>
#include <QWidget>
#include <cstdio>

namespace {
class DesktopWindowFixture final : public QWidget {
public:
    explicit DesktopWindowFixture(bool rejectFirstClose)
        : m_rejectFirstClose(rejectFirstClose)
    {
        setWindowTitle("Dockpipe isolated desktop window fixture");
        setWindowFlag(Qt::FramelessWindowHint);
        resize(640, 480);

        QPalette colors = palette();
        colors.setColor(QPalette::Window, QColor("#195a89"));
        colors.setColor(QPalette::WindowText, Qt::white);
        setPalette(colors);
        setAutoFillBackground(true);

        QPixmap icon(32, 32);
        icon.fill(QColor("#35b77c"));
        setWindowIcon(QIcon(icon));

        auto *contents = new QVBoxLayout(this);
        contents->addWidget(new QLabel("Task-owned desktop acquisition fixture", this));
        auto *input = new QLineEdit(this);
        input->setObjectName("fixtureInput");
        input->setMaxLength(128);
        contents->addWidget(input);
        contents->addStretch();
        connect(input, &QLineEdit::textEdited, this, [](const QString &value) {
            const QByteArray text = value.toUtf8();
            std::printf("TEXT:%s\n", text.constData());
            std::fflush(stdout);
        });
        input->setFocus();
    }

protected:
    void closeEvent(QCloseEvent *event) override
    {
        if (m_rejectFirstClose) {
            m_rejectFirstClose = false;
            event->ignore();
            std::puts("CLOSE:cancelled");
            std::fflush(stdout);
            return;
        }
        QWidget::closeEvent(event);
    }

private:
    bool m_rejectFirstClose;
};
}

int main(int argc, char **argv)
{
    QApplication application(argc, argv);
    application.setApplicationName("dockpipe-window-fixture");
    for (const QString &argument : application.arguments().mid(1)) {
        if (argument != "--fixture" && argument != "--reject-first-close") {
            std::fprintf(stderr, "Unsupported desktop window fixture argument\n");
            return 2;
        }
    }
    if (QGuiApplication::platformName() != "xcb") {
        std::fprintf(stderr, "Desktop window fixture requires an X11 display\n");
        return 2;
    }

    DesktopWindowFixture window(application.arguments().contains("--reject-first-close"));
    window.show();
    std::printf("WINDOW:%llu\n", static_cast<unsigned long long>(window.winId()));
    std::fflush(stdout);

    // The parent test normally owns termination. Bound an abandoned fixture too.
    QTimer::singleShot(120000, &application, [] {
        std::fprintf(stderr, "Desktop window fixture lifetime exceeded 120 seconds\n");
        QCoreApplication::exit(124);
    });
    return application.exec();
}
