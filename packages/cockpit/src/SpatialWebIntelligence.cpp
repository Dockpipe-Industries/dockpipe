#include "SpatialWebBrowser.h"
#include "SpatialSiteExplorer.h"
#include "SpatialNeuralService.h"
#include "SpatialWebWorld.h"
#include "WorkspaceFlightView.h"
#include <QActionGroup>
#include <QComboBox>
#include <QHBoxLayout>
#include <QMenu>
#include <QPushButton>
#include <QSettings>
#include <QSignalBlocker>
#include <QToolButton>

void SpatialWebBrowser::configureIntelligence(QHBoxLayout *toolbar)
{
    m_neural = new SpatialNeuralService(this);
    const bool gpu = QSettings().value("cockpit/neuralGpu", false).toBool();
    m_neural->setPreferGpu(gpu);
    m_enhance = new QToolButton;
    m_enhance->setObjectName("spatialWebEnhance");
    m_enhance->setText(tr("Enhance"));
    m_enhance->setCheckable(true);
    m_enhance->setToolTip(tr("Build more distinctive surroundings and cut out product images locally.\nFirst use downloads approximately 95 MB of models plus an isolated runtime."));
    m_enhance->setChecked(m_neural->available() && QSettings().value("cockpit/neuralEnabled", false).toBool());
    auto *menu = new QMenu(m_enhance);
    auto *backends = new QActionGroup(menu);
    for (bool accelerated : {false, true}) {
        auto *action = menu->addAction(accelerated ? tr("Prefer GPU (about 2 GB with CUDA libraries)") : tr("CPU (smaller download)"));
        action->setCheckable(true);
        action->setChecked(gpu == accelerated);
        backends->addAction(action);
        connect(action, &QAction::triggered, this, [this, accelerated]() {
            m_neural->setPreferGpu(accelerated);
            m_enhance->setChecked(false);
            m_applyDesign->hide();
            m_proposal = {};
            QSettings().setValue("cockpit/neuralGpu", accelerated);
            QSettings().setValue("cockpit/neuralEnabled", false);
            setStatus(tr("Enhancement backend selected. Click Enhance to enable it."));
        });
    }
    m_enhance->setMenu(menu);
    m_enhance->setPopupMode(QToolButton::MenuButtonPopup);
    toolbar->addWidget(m_enhance);
    m_applyDesign = new QPushButton(tr("Enter new design"));
    m_applyDesign->setObjectName("spatialWebApplyDesign");
    m_applyDesign->setToolTip(tr("Enter the prepared world at its entrance. Destinations remain the same."));
    m_applyDesign->hide();
    toolbar->addWidget(m_applyDesign);
    connect(m_applyDesign, &QPushButton::clicked, this, [this]() { applyNeuralDesign(false); });
    connect(m_enhance, &QToolButton::clicked, this, [this](bool enabled) {
        QSettings().setValue("cockpit/neuralEnabled", false);
        if (!enabled) {
            m_neural->cancel();
            m_proposal = {};
            m_applyDesign->hide();
            setStatus(tr("Local enhancement stopped. Your current world is preserved."));
        } else if (!m_neural->available()) {
            m_neural->install(QSettings().value("cockpit/neuralGpu", false).toBool());
        } else {
            QSettings().setValue("cockpit/neuralEnabled", true);
            requestEnhancement();
        }
    });
    connect(m_neural, &SpatialNeuralService::installationFinished, this, [this](bool success) {
        m_enhance->setChecked(success);
        QSettings().setValue("cockpit/neuralEnabled", success);
        if (success)
            requestEnhancement();
    });
    connect(m_neural, &SpatialNeuralService::statusChanged, this, &SpatialWebBrowser::setStatus);
    connect(m_neural, &SpatialNeuralService::planReady, this, [this](const QJsonObject &proposal) {
        if (!m_enhance->isChecked() || m_document.items.isEmpty())
            return;
        m_proposal = proposal;
        // Never move an exploring user or replace their car underneath them.
        // A late design is ready on demand; texture cutouts can arrive in place.
        if (m_autoDesign && !m_scene->moving()
            && (m_scene->cameraPosition() - m_planOrigin).lengthSquared() < 1
            && m_scene->activeVehicleId().isEmpty()) {
            applyNeuralDesign(true);
        } else {
            m_applyDesign->show();
            setStatus(tr("A content-inspired world is ready. Enter new design when you want to explore it."));
        }
    });
    connect(m_neural, &SpatialNeuralService::imageReady, this, [this](const QString &id, const QImage &image) {
        if (!m_enhance->isChecked())
            return;
        for (int index = 0; index < m_document.items.size(); ++index) {
            auto &item = m_document.items[index];
            if (item.id != id)
                continue;
            item.poster = SpatialWebDocument::poster(item, image, index);
            m_scene->setInteriorImage(id, item.poster);
            return;
        }
    });
}

void SpatialWebBrowser::requestEnhancement()
{
    if (m_passage || !m_enhance->isChecked() || !m_neural->available() || m_neural->installing()
        || m_document.items.isEmpty() || !isVisible() || !spatial())
        return;
    m_neural->cancel();
    m_planOrigin = m_scene->cameraPosition();
    m_autoDesign = true;
    if (m_document.venue != "cinema" && m_document.venue != "city" && !(m_document.stableLayout && m_document.worldFeatures.size() == 8))
        m_neural->plan(m_document);
    for (const auto &item : m_document.items) {
        if (item.kind == "product" && !item.visual.isNull())
            m_neural->cutout(item.id, item.visual);
    }
}

void SpatialWebBrowser::applyNeuralDesign(bool preserveLook)
{
    if (m_passage || m_headquarters || m_document.venue == "cinema" || m_document.venue == "city")
        return;
    auto candidate = m_document;
    if (!SpatialNeuralService::apply(m_proposal, candidate)) {
        m_proposal = {};
        m_applyDesign->hide();
        setStatus(tr("The local model returned an invalid design. Your current world is preserved."));
        return;
    }
    const QString backend = m_proposal.value("provider").toString();
    const auto result = SpatialWebWorld::construct(candidate, SpatialWebWorld::optionsFor(candidate));
    m_proposal = {};
    m_applyDesign->hide();
    if (!result.success || result.fallback) {
        setStatus(tr("The enhanced design could not preserve all paths. Your current world is preserved."));
        return;
    }
    m_document = candidate;
    m_currentInterior = result.interior;
    m_currentInterior.environmentKey = candidate.environmentKey();
    m_site->rememberDesign(candidate);
    m_scene->setInterior(result.interior, m_document.title, preserveLook, preserveLook, candidate.environmentKey());
    m_scene->setShipVisible(result.interior.locomotion == "fly");
    for (const auto &item : m_document.items)
        m_scene->setInteriorImage(item.id, item.poster);
    const QSignalBlocker mobilityBlocker(m_mobility);
    const QSignalBlocker layoutBlocker(m_layout);
    m_mobility->setCurrentIndex(m_mobility->findData(result.interior.locomotion));
    m_layout->setCurrentIndex(0);
    const bool accelerated = backend == "CUDAExecutionProvider" || backend == "CoreMLExecutionProvider";
    m_worldNotice = accelerated ? tr("Local semantic design · GPU acceleration · product cutouts arrive in place.")
        : tr("Local semantic design · CPU · product cutouts arrive in place.");
    setStatus(m_worldNotice);
    m_scene->setFocus();
}
