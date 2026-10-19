#include <QApplication>
#include <QTimer>
#include <QWebEnginePage>
#include <QWebEngineView>
#include <cstdio>

int main(int argc, char **argv)
{
    QApplication application(argc, argv);
    QWebEngineView browser;
    QObject::connect(&browser, &QWebEngineView::loadFinished, &application, [&](bool loaded) {
        if (!loaded) {
            application.exit(1);
            return;
        }
        browser.page()->runJavaScript(QStringLiteral("6 * 7"), [&](const QVariant &result) {
            if (result.toInt() != 42) {
                application.exit(2);
                return;
            }
            std::puts("PASS: deployed WebEngine helper loaded a page and executed JavaScript");
            application.quit();
        });
    });
    QTimer::singleShot(30000, &application, [&]() { application.exit(3); });
    browser.resize(640, 480);
    browser.show();
    browser.setHtml(QStringLiteral("<!doctype html><title>Dockpipe runtime probe</title><p>Runtime probe</p>"));
    return application.exec();
}
