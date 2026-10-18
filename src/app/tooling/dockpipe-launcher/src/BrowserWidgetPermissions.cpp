#include "BrowserWidget.h"
#include "BrowserPreferences.h"

#include <QCheckBox>
#include <QHideEvent>
#include <QMessageBox>
#include <QStackedWidget>
#include <QWebEngineSettings>
#include <QWebEngineView>

namespace {
QStringList permissionKeys(QWebEnginePage::Feature feature)
{
    switch (feature) {
    case QWebEnginePage::MediaVideoCapture:
        return {"camera"};
    case QWebEnginePage::MediaAudioCapture:
        return {"microphone"};
    case QWebEnginePage::MediaAudioVideoCapture:
        return {"camera", "microphone"};
    case QWebEnginePage::Geolocation:
        return {"location"};
    case QWebEnginePage::Notifications:
        return {"notifications"};
    default:
        return {};
    }
}
}

void BrowserWidget::cancelPermissionPrompts()
{
    for (auto *prompt : findChildren<QMessageBox *>("browserPermissionPrompt")) {
        if (prompt->isVisible())
            prompt->reject();
    }
}

void BrowserWidget::hideEvent(QHideEvent *event)
{
    cancelPermissionPrompts();
    QWidget::hideEvent(event);
}

void BrowserWidget::applyPreferences()
{
    const auto settings = BrowserPreferences::instance()->settings();
    m_javascript = settings.javascript;
    m_images = settings.images;
    m_showFullUrl = settings.showFullUrl;
    // A changed policy supersedes any unanswered prompt from the old policy.
    cancelPermissionPrompts();
    for (auto *view : m_pages->findChildren<QWebEngineView *>()) {
        view->settings()->setAttribute(QWebEngineSettings::JavascriptEnabled, m_javascript);
        view->settings()->setAttribute(QWebEngineSettings::AutoLoadImages, m_images);
        auto origins = m_permissionOrigins.value(view);
        const QString currentOrigin = BrowserPreferences::originKey(view->url());
        if (!currentOrigin.isEmpty())
            origins.insert(currentOrigin);
        for (const QString &originKey : origins) {
            const QUrl origin(originKey);
            for (const auto feature : {QWebEnginePage::MediaVideoCapture, QWebEnginePage::MediaAudioCapture,
                     QWebEnginePage::MediaAudioVideoCapture, QWebEnginePage::Geolocation, QWebEnginePage::Notifications}) {
                bool blocked = false;
                for (const auto &key : permissionKeys(feature))
                    blocked = blocked || BrowserPreferences::permission(settings, origin, key) == "block";
                view->page()->setFeaturePermission(origin, feature,
                    blocked ? QWebEnginePage::PermissionDeniedByUser : QWebEnginePage::PermissionUnknown);
            }
        }
    }
    updateNavigation();
}

void BrowserWidget::requestSitePermission(QWebEngineView *view, const QUrl &origin, QWebEnginePage::Feature feature)
{
    const auto keys = permissionKeys(feature);
    const QString originKey = BrowserPreferences::originKey(origin);
    auto *page = view->page();
    if (keys.isEmpty() || originKey.isEmpty()) {
        page->setFeaturePermission(origin, feature, QWebEnginePage::PermissionDeniedByUser);
        return;
    }
    if (!m_permissionOrigins.contains(view)) {
        connect(view, &QObject::destroyed, this, [this, view]() { m_permissionOrigins.remove(view); });
        m_permissionOrigins.insert(view, {});
    }
    auto &origins = m_permissionOrigins[view];
    // Retain requested frame origins for policy changes, with a finite per-tab
    // session bound. New origins beyond this bound fail closed until tab closure.
    if (!origins.contains(originKey) && origins.size() >= 200) {
        page->setFeaturePermission(origin, feature, QWebEnginePage::PermissionDeniedByUser);
        return;
    }
    origins.insert(originKey);
    const auto settings = BrowserPreferences::instance()->settings();
    bool ask = false;
    for (const auto &key : keys) {
        const QString policy = BrowserPreferences::permission(settings, origin, key);
        if (policy == "block") {
            page->setFeaturePermission(origin, feature, QWebEnginePage::PermissionDeniedByUser);
            return;
        }
        ask = ask || policy == "ask";
    }
    if (!ask) {
        page->setFeaturePermission(origin, feature, QWebEnginePage::PermissionGrantedByUser);
        return;
    }
    // Prompt only for the visible selected page. Background spatial capture and
    // inactive tabs cannot interrupt another page with permission dialogs.
    if (view != m_view || !isVisible() || !m_chromeVisible || m_backgroundSuspended
        || findChild<QMessageBox *>("browserPermissionPrompt")) {
        page->setFeaturePermission(origin, feature, QWebEnginePage::PermissionDeniedByUser);
        return;
    }
    auto *prompt = new QMessageBox(QMessageBox::Question, tr("Site permission"),
        tr("Allow %1 to use %2?").arg(originKey, keys.join(tr(" and "))),
        QMessageBox::Yes | QMessageBox::No, this);
    prompt->setObjectName("browserPermissionPrompt");
    prompt->setTextFormat(Qt::PlainText);
    prompt->setDefaultButton(QMessageBox::No);
    prompt->setAttribute(Qt::WA_DeleteOnClose);
    auto *remember = new QCheckBox(tr("Remember for this site"), prompt);
    prompt->setCheckBox(remember);
    QPointer<QWebEnginePage> pending(page);
    connect(prompt, &QMessageBox::finished, this, [this, view, pending, origin, originKey, feature, keys, remember](int result) {
        if (!pending)
            return;
        const bool current = view == m_view && isVisible() && m_chromeVisible && !m_backgroundSuspended;
        const bool allow = current && result == QMessageBox::Yes;
        if (current && remember->isChecked() && (result == QMessageBox::Yes || result == QMessageBox::No)) {
            auto updated = BrowserPreferences::instance()->settings();
            for (const auto &key : keys)
                updated.sitePermissions[originKey][key] = allow ? "allow" : "block";
            if (!BrowserPreferences::instance()->save(updated)) {
                pending->setFeaturePermission(origin, feature, QWebEnginePage::PermissionDeniedByUser);
                showStatus(tr("Could not save site permission. Access was not granted."));
                return;
            }
        }
        pending->setFeaturePermission(origin, feature,
            allow ? QWebEnginePage::PermissionGrantedByUser : QWebEnginePage::PermissionDeniedByUser);
    });
    connect(page, &QWebEnginePage::featurePermissionRequestCanceled, prompt,
        [prompt, origin, feature](const QUrl &cancelledOrigin, QWebEnginePage::Feature cancelledFeature) {
            if (origin == cancelledOrigin && feature == cancelledFeature)
                prompt->reject();
        });
    connect(page, &QWebEnginePage::loadStarted, prompt, &QMessageBox::reject);
    connect(page, &QObject::destroyed, prompt, &QMessageBox::reject);
    connect(this, &BrowserWidget::activeViewAboutToChange, prompt, [prompt, view](QWebEngineView *previous) {
        if (previous == view)
            prompt->reject();
    });
    prompt->open();
}
