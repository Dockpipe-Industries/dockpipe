#include "SpatialWebBrowser.h"
#include "SpatialSiteExplorer.h"
#include "SpatialWebMedia.h"
#include "WorkspaceFlightView.h"
#include <QFrame>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QShortcut>
#include <QStackedWidget>
#include <QUrlQuery>
#include <QVBoxLayout>
#ifdef DOCKPIPE_SPATIAL_WEB
#include <QWebEngineView>
#endif

void SpatialWebBrowser::configureSearch()
{
    m_search = new QWidget;
    m_search->setObjectName("spatialCityTerminal");
    m_search->setProperty("flightOverlay", true);
    auto *layout = new QVBoxLayout(m_search);
    layout->addStretch();
    auto *card = new QFrame;
    card->setObjectName("citySearchCard");
    card->setMinimumWidth(560);
    card->setMaximumWidth(680);
    card->setStyleSheet(
        "QFrame#citySearchCard { background: #142232; border: 1px solid #678c99; border-radius: 14px; }"
        "QLabel { color: #eaf4f4; background: transparent; border: none; }"
        "QLineEdit { background: #0b1520; color: #ffffff; padding: 16px; border: 1px solid #789f9e; border-radius: 6px; font-size: 19px; }"
        "QPushButton { min-height: 42px; padding: 4px 20px; }");
    auto *fields = new QVBoxLayout(card);
    fields->setContentsMargins(32, 28, 32, 28);
    fields->setSpacing(16);
    auto *eyebrow = new QLabel(tr("DOCKPIPE  /  CITY DIRECTORY"));
    fields->addWidget(eyebrow);
    auto *title = new QLabel(tr("Where do you want to go?"));
    title->setStyleSheet("font-size: 29px; font-weight: bold;");
    title->setWordWrap(true);
    fields->addWidget(title);
    auto *description = new QLabel(tr("Search a topic. Explore the results as a street of destinations."));
    description->setWordWrap(true);
    fields->addWidget(description);
    m_searchQuery = new QLineEdit;
    m_searchQuery->setObjectName("spatialCityQuery");
    m_searchQuery->setPlaceholderText(tr("Rainforest animals, architecture, places to explore…"));
    m_searchQuery->setMaxLength(512);
    m_searchQuery->setAccessibleName(tr("Search query"));
    fields->addWidget(m_searchQuery);
    auto *submit = new QPushButton(tr("Explore search →"));
    submit->setObjectName("spatialCitySubmit");
    fields->addWidget(submit);
    m_searchStatus = new QLabel;
    m_searchStatus->setTextFormat(Qt::PlainText);
    m_searchStatus->setWordWrap(true);
    fields->addWidget(m_searchStatus);
    auto *back = new QPushButton(tr("Return to world · Esc"));
    back->setObjectName("spatialCityCancel");
    fields->addWidget(back);
    layout->addWidget(card, 0, Qt::AlignHCenter);
    layout->addStretch();
    m_pages->addWidget(m_search);
    m_search->hide();
    const auto search = [this]() {
        const QString text = m_searchQuery->text().trimmed();
        if (text.isEmpty()) {
            m_searchStatus->setText(tr("Enter a topic to explore."));
            m_searchQuery->setFocus();
            return;
        }
        QUrl url("https://www.google.com/search");
        QUrlQuery query;
        query.addQueryItem("q", text);
        url.setQuery(query);
        navigate(url, false);
        setStatus(tr("Finding destinations for “%1”… Your street will open here.").arg(text));
    };
    connect(submit, &QPushButton::clicked, this, search);
    connect(m_searchQuery, &QLineEdit::returnPressed, this, search);
    connect(back, &QPushButton::clicked, this, &SpatialWebBrowser::closeSearch);
    auto *escape = new QShortcut(QKeySequence(Qt::Key_Escape), m_search);
    escape->setContext(Qt::WidgetWithChildrenShortcut);
    connect(escape, &QShortcut::activated, this, &SpatialWebBrowser::closeSearch);
}

void SpatialWebBrowser::showSearch()
{
#ifdef DOCKPIPE_SPATIAL_WEB
    if (!m_browser)
        return;
#else
    setStatus(tr("City search requires the Cockpit browser runtime."));
    return;
#endif
    cancelPassage();
    cancelProjection();
    m_site->leaveArea();
    m_site->pause();
    if (m_media)
        m_media->stop();
    m_scene->setPilotInput({}, {}, false);
#ifdef DOCKPIPE_SPATIAL_WEB
    if (m_browser)
        m_browser->stop();
#endif
    m_loading = false;
    m_liveMode = false;
    m_preserveWorld = true;
    freezeBackgroundBrowser();
    m_searchOpen = true;
    m_pages->setCurrentWidget(m_search);
    m_search->raise();
    m_searchStatus->setText(tr("Powered by Google results. Up to twelve destinations per street."));
    m_searchQuery->setFocus();
    m_searchQuery->selectAll();
}

void SpatialWebBrowser::closeSearch()
{
    cancelProjection();
    m_loading = false;
    m_preserveWorld = true;
#ifdef DOCKPIPE_SPATIAL_WEB
    if (m_browser)
        m_browser->stop();
#endif
    m_searchOpen = false;
    m_headquarters = m_document.url.isEmpty();
    m_search->hide();
    m_pages->setCurrentWidget(m_scene);
    m_scene->show();
    m_scene->raise();
    freezeBackgroundBrowser();
    m_scene->setFocus();
    setStatus(tr("Returned to your world. Search city opens the directory again."));
}
