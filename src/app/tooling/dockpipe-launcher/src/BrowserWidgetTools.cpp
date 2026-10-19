#include "BrowserWidget.h"
#include "BrowserAnnotationDialog.h"
#include "BrowserDownloads.h"
#include "BrowserExtensionsDialog.h"
#include "BrowserExtensionPackage.h"
#include "BrowserRuntime.h"
#include "BrowserSettingsDialog.h"
#include "BrowserPreferences.h"
#include "BrowserLinkRouter.h"
#include "CanvasIcons.h"

#include <QAction>
#include <QCheckBox>
#include <QDesktopServices>
#include <QDialog>
#include <QDialogButtonBox>
#include <QFileDialog>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QMenu>
#include <QMessageBox>
#include <QPrintDialog>
#include <QPrinter>
#include <QPushButton>
#include <QSaveFile>
#include <QTabBar>
#include <QStackedWidget>
#include <QVBoxLayout>
#include <QWebEngineFindTextResult>
#include <QWebEnginePage>
#include <QWebEngineProfile>
#include <QWebEngineSettings>
#include <QWebEngineView>
#include <QWidgetAction>

QMenu *BrowserWidget::createMenu()
{
    auto *menu = new QMenu(this);
    menu->setObjectName("browserMenu");
    menu->setMinimumWidth(260);
    menu->setStyleSheet(R"CSS(
        QMenu { padding: 7px; border: 1px solid palette(mid); border-radius: 12px; }
        QMenu::item { padding: 7px 12px; border-radius: 5px; }
        QMenu::separator { height: 1px; background: palette(mid); margin: 5px 8px; }
    )CSS");
    menu->addAction(tr("Find in page"), this, &BrowserWidget::showFind);
    menu->addAction(tr("Print…"), this, &BrowserWidget::printPage);
    menu->addSeparator();
    auto *zoomRow = new QWidget;
    auto *zoomLayout = new QHBoxLayout(zoomRow);
    auto *zoomLabel = new QLabel;
    zoomLabel->setObjectName("browserZoomValue");
    auto *zoomOut = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Minus), {});
    auto *zoomIn = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Add), {});
    auto *reset = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Refresh), {});
    zoomOut->setAccessibleName(tr("Zoom out"));
    zoomIn->setAccessibleName(tr("Zoom in"));
    reset->setAccessibleName(tr("Reset zoom"));
    for (auto *button : {zoomOut, zoomIn, reset})
        button->setFixedSize(30, 30);
    zoomLayout->addWidget(new QLabel(tr("Zoom")));
    zoomLayout->addStretch();
    zoomLayout->addWidget(zoomOut);
    zoomLayout->addWidget(zoomLabel);
    zoomLayout->addWidget(zoomIn);
    zoomLayout->addWidget(reset);
    auto *zoomAction = new QWidgetAction(menu);
    zoomAction->setDefaultWidget(zoomRow);
    menu->addAction(zoomAction);
    const auto updateZoom = [this, zoomLabel]() {
        zoomLabel->setText(QString::number(qRound((m_view ? m_view->zoomFactor() : 1.0) * 100)) + "%");
    };
    connect(menu, &QMenu::aboutToShow, this, updateZoom);
    connect(zoomOut, &QPushButton::clicked, this, [this, updateZoom]() {
        if (m_view)
            setZoom(m_view->zoomFactor() - 0.1);
        updateZoom();
    });
    connect(zoomIn, &QPushButton::clicked, this, [this, updateZoom]() {
        if (m_view)
            setZoom(m_view->zoomFactor() + 0.1);
        updateZoom();
    });
    connect(reset, &QPushButton::clicked, this, [this, updateZoom]() {
        setZoom(1.0);
        updateZoom();
    });
    menu->addSeparator();
    auto *devices = menu->addMenu(tr("Device toolbar"));
    devices->addAction(tr("Full width"), this, [this]() { setViewportWidth(0); });
    devices->addAction(tr("Tablet · 820 px"), this, [this]() { setViewportWidth(820); });
    devices->addAction(tr("Phone · 390 px"), this, [this]() { setViewportWidth(390); });
    menu->addAction(tr("Take a screenshot…"), this, [this]() { capturePage(false); });
    auto *annotate = menu->addAction(tr("Annotate page…"), this, [this]() { capturePage(true); });
    annotate->setObjectName("browserAnnotateAction");
    menu->addSeparator();
    menu->addAction(tr("Passwords and autofill…"), this, [this]() {
        QPointer<BrowserWidget> owner(this);
        QPointer<QMessageBox> message = new QMessageBox(this);
        message->setWindowTitle(tr("Passwords and autofill"));
        message->setText(tr("Use a compatible password-manager extension for password storage and autofill. DockPipe does not store passwords. Extension compatibility depends on the browser runtime."));
        auto *manage = message->addButton(tr("Manage extensions"), QMessageBox::AcceptRole);
        message->addButton(QMessageBox::Close);
        message->exec();
        if (!owner || !message)
            return;
        const bool manageRequested = message->clickedButton() == manage;
        delete message;
        if (manageRequested)
            showExtensions();
    });
    menu->addAction(tr("Extensions…"), this, &BrowserWidget::showExtensions);
    menu->addAction(tr("Downloads…"), this, [this]() {
        if (m_downloads) {
            m_downloads->setHost(this);
            m_downloads->showPanel();
        }
    });
    menu->addAction(tr("History…"), this, &BrowserWidget::showHistory);
    menu->addAction(tr("Clear browsing data…"), this, &BrowserWidget::clearBrowsingData);
    menu->addSeparator();
    menu->addAction(tr("Open in default browser"), this, [this]() {
        if (m_view && BrowserRuntime::allowsUrl(m_view->url()))
            BrowserLinkRouter::openInSystemBrowser(m_view->url());
    });
    menu->addAction(tr("Browser settings…"), this, &BrowserWidget::showSettings);
    return menu;
}

void BrowserWidget::showFind()
{
    m_findBar->show();
    m_findText->setFocus();
    m_findText->selectAll();
    findText();
}

void BrowserWidget::findText(bool backwards)
{
    if (!m_view)
        return;
    QPointer<QWebEngineView> view(m_view);
    QPointer<BrowserWidget> owner(this);
    const QString query = m_findText->text();
    m_view->page()->findText(query, backwards ? QWebEnginePage::FindBackward : QWebEnginePage::FindFlags(),
        [owner, view, query](const QWebEngineFindTextResult &result) {
            if (owner && view && view == owner->m_view && query == owner->m_findText->text())
                owner->m_findResult->setText(query.isEmpty() ? QString()
                    : tr("%1 of %2").arg(result.activeMatch()).arg(result.numberOfMatches()));
        });
}

void BrowserWidget::setZoom(qreal zoom)
{
    if (m_view)
        m_view->setZoomFactor(qBound(0.25, zoom, 5.0));
}

void BrowserWidget::setViewportWidth(int width)
{
    if (!m_view)
        return;
    m_view->setMinimumWidth(width > 0 ? width : 0);
    m_view->setMaximumWidth(width > 0 ? width : QWIDGETSIZE_MAX);
    m_view->parentWidget()->layout()->setAlignment(m_view, width > 0 ? Qt::AlignHCenter : Qt::Alignment());
    showStatus(width > 0 ? tr("Viewport preview: %1 px. This changes layout width, not the browser's device identity.").arg(width) : QString());
}

void BrowserWidget::showHistory()
{
    auto *dialog = new QDialog(this);
    dialog->setAttribute(Qt::WA_DeleteOnClose);
    dialog->setObjectName("browserHistoryDialog");
    dialog->setWindowTitle(tr("History"));
    dialog->resize(650, 440);
    auto *layout = new QVBoxLayout(dialog);
    layout->addWidget(new QLabel(tr("Recent pages in this browser session (up to 500).")));
    auto *list = new QListWidget;
    list->setObjectName("browserHistoryList");
    for (auto visit = m_history.crbegin(); visit != m_history.crend(); ++visit) {
        auto *item = new QListWidgetItem(visit->title + "\n" + visit->url.toDisplayString(), list);
        item->setData(Qt::UserRole, visit->url);
        item->setToolTip(visit->time.toString());
    }
    layout->addWidget(list);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Close);
    auto *clear = buttons->addButton(tr("Clear history"), QDialogButtonBox::ActionRole);
    layout->addWidget(buttons);
    connect(clear, &QPushButton::clicked, dialog, [this, list]() {
        m_history.clear();
        list->clear();
    });
    connect(buttons, &QDialogButtonBox::rejected, dialog, &QDialog::close);
    connect(list, &QListWidget::itemActivated, dialog, [this, dialog](QListWidgetItem *item) {
        navigate(item->data(Qt::UserRole).toUrl());
        dialog->close();
    });
    dialog->show();
}

void BrowserWidget::showExtensions()
{
    if (!m_profile)
        return;
    auto *dialog = new BrowserExtensionsDialog(m_profile, this);
    dialog->setAttribute(Qt::WA_DeleteOnClose);
    connect(dialog, &BrowserExtensionsDialog::openUrl, this, [this](const QUrl &url) {
        if (BrowserRuntime::allowsNavigation(url, m_profile))
            addTab(url);
    });
    dialog->show();
    if (m_view && !BrowserExtensionPackage::extensionId(m_view->url().toString()).isEmpty())
        dialog->addWebStoreExtension(m_view->url().toString());
}

void BrowserWidget::showSettings()
{
    if (auto *existing = findChild<BrowserSettingsDialog *>()) {
        existing->show();
        existing->raise();
        existing->activateWindow();
        return;
    }
    auto *dialog = new BrowserSettingsDialog(this);
    dialog->setAttribute(Qt::WA_DeleteOnClose);
    connect(dialog, &BrowserSettingsDialog::historyRequested, this, &BrowserWidget::showHistory);
    connect(dialog, &BrowserSettingsDialog::clearDataRequested, this, &BrowserWidget::clearBrowsingData);
    connect(dialog, &BrowserSettingsDialog::extensionsRequested, this, &BrowserWidget::showExtensions);
    connect(dialog, &BrowserSettingsDialog::downloadsRequested, this, [this]() {
        if (m_downloads) {
            m_downloads->setHost(this);
            m_downloads->showPanel();
        }
    });
    dialog->show();
}

void BrowserWidget::showSiteInfo()
{
    if (!m_view)
        return;
    const auto url = m_view->url();
    QMessageBox::information(this, tr("Site information"),
        tr("%1\n\nProtocol: %2\nCamera: %3\nMicrophone: %4\nLocation: %5\nNotifications: %6\n\nManage site permissions in Browser settings.")
            .arg(url.host(), url.scheme().toUpper(),
                BrowserPreferences::permission(BrowserPreferences::instance()->settings(), url, "camera"),
                BrowserPreferences::permission(BrowserPreferences::instance()->settings(), url, "microphone"),
                BrowserPreferences::permission(BrowserPreferences::instance()->settings(), url, "location"),
                BrowserPreferences::permission(BrowserPreferences::instance()->settings(), url, "notifications")));
}

void BrowserWidget::clearBrowsingData()
{
    if (m_printing) {
        showStatus(tr("Wait for printing to finish before clearing browsing data."));
        return;
    }
    if (hasSharedSession()) {
        showStatus(tr("Detached tabs share this browsing session. Close the other browser widgets before clearing its data."));
        return;
    }
    QPointer<BrowserWidget> owner(this);
    const auto answer = QMessageBox::question(this, tr("Clear browsing data"),
        tr("Close browser tabs, cancel downloads and clear this session's cookies, website storage, cache and history? Installed extensions and saved files will remain available."));
    if (!owner || answer != QMessageBox::Yes)
        return;
    for (auto *dialog : findChildren<QDialog *>(QString(), Qt::FindDirectChildrenOnly))
        delete dialog;
    destroySession();
    m_history.clear();
    createSession();
    showStatus(tr("Browsing data cleared."));
}

void BrowserWidget::capturePage(bool annotate)
{
    if (!m_view || m_view->url().isEmpty()) {
        showStatus(tr("Open a page before taking a capture."));
        return;
    }
    const QImage capture = m_view->grab().toImage();
    if (annotate) {
        auto *dialog = new BrowserAnnotationDialog(capture, m_view->url(), this);
        dialog->setAttribute(Qt::WA_DeleteOnClose);
        dialog->show();
        return;
    }
    QPointer<BrowserWidget> owner(this);
    const QString path = QFileDialog::getSaveFileName(this, tr("Save page screenshot"), "page.png", tr("PNG image (*.png)"));
    if (!owner || path.isEmpty())
        return;
    QSaveFile file(path);
    if (!file.open(QIODevice::WriteOnly) || !capture.save(&file, "PNG") || !file.commit())
        showStatus(tr("Could not save the screenshot."));
    else
        showStatus(tr("Screenshot saved."));
}

void BrowserWidget::printPage()
{
    if (!m_view || m_printing)
        return;
    QPointer<QWebEngineView> view(m_view);
    QPointer<BrowserWidget> owner(this);
    auto printer = std::make_unique<QPrinter>(QPrinter::HighResolution);
    QPointer<QPrintDialog> dialog = new QPrintDialog(printer.get(), this);
    m_printing = true;
    const int result = dialog->exec();
    if (!owner)
        return;
    delete dialog;
    if (result != QDialog::Accepted || !view) {
        m_printing = false;
        return;
    }
    m_printer = std::move(printer);
    const auto completed = [this](bool success) {
        m_printing = false;
        showStatus(success ? tr("Page sent to printer.") : tr("Printing failed."));
    };
    connect(view, &QWebEngineView::printFinished, this, completed, Qt::SingleShotConnection);
    view->print(m_printer.get());
}
