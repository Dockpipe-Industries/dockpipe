#include "DockerResourceInspector.h"
#include "CanvasIcons.h"

#include <QHBoxLayout>
#include <QFontDatabase>
#include <QLabel>
#include <QMenu>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QScrollArea>
#include <QStackedWidget>
#include <QTabWidget>
#include <QVBoxLayout>

DockerResourceInspector::DockerResourceInspector(QPlainTextEdit *logs,
    const QList<QPlainTextEdit *> &inspect, QWidget *parent) : QWidget(parent)
{
    setAttribute(Qt::WA_StyledBackground, true);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(8, 8, 8, 8);
    layout->setSpacing(8);
    m_title = new QLabel(tr("Select a resource"));
    m_title->setObjectName("dockerResourceTitle");
    m_title->setTextFormat(Qt::PlainText);
    QFont titleFont = m_title->font();
    titleFont.setPointSize(16);
    titleFont.setBold(true);
    m_title->setFont(titleFont);
    m_title->setWordWrap(true);
    layout->addWidget(m_title);
    m_state = new QLabel(tr("Choose a card on the canvas to inspect it."));
    m_state->setTextFormat(Qt::PlainText);
    m_state->setWordWrap(true);
    layout->addWidget(m_state);

    auto *actions = new QHBoxLayout;
    m_start = new QPushButton(tr("Start"));
    m_stop = new QPushButton(tr("Stop"));
    m_start->setObjectName("dockerStart");
    m_stop->setObjectName("dockerStop");
    m_exec = new QPushButton(tr("Exec"));
    m_exec->setObjectName("dockerExec");
    m_exec->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Terminal));
    auto *shells = new QMenu(m_exec);
    for (const QString &shell : {QStringLiteral("/bin/sh"), QStringLiteral("/bin/bash")}) {
        shells->addAction(shell, this, [this, shell]() { emit execRequested(shell); });
    }
    m_exec->setMenu(shells);
    m_exec->setToolTip(tr("Open an interactive shell in your terminal. The chosen shell must exist in the container."));
    connect(m_start, &QPushButton::clicked, this, &DockerResourceInspector::startRequested);
    connect(m_stop, &QPushButton::clicked, this, &DockerResourceInspector::stopRequested);
    actions->addWidget(m_start);
    actions->addWidget(m_stop);
    actions->addWidget(m_exec);
    actions->addStretch();
    auto *pause = new QPushButton(tr("Pause updates"));
    pause->setObjectName("dockerPauseUpdates");
    pause->setCheckable(true);
    connect(pause, &QPushButton::toggled, this, [this, pause](bool paused) {
        pause->setText(paused ? tr("Resume updates") : tr("Pause updates"));
        emit pauseChanged(paused);
    });
    actions->addWidget(pause);
    auto *refresh = new QPushButton(tr("Refresh"));
    connect(refresh, &QPushButton::clicked, this, &DockerResourceInspector::refreshRequested);
    actions->addWidget(refresh);
    layout->addLayout(actions);
    m_related = new QLabel;
    m_related->setObjectName("dockerRelatedResources");
    m_related->setWordWrap(true);
    m_related->setTextFormat(Qt::RichText);
    m_related->setTextInteractionFlags(Qt::TextBrowserInteraction);
    m_related->setOpenExternalLinks(false);
    connect(m_related, &QLabel::linkActivated, this, &DockerResourceInspector::resourceRequested);
    layout->addWidget(m_related);

    m_tabs = new QTabWidget;
    m_tabs->setObjectName("dockerInspectorTabs");
    logs->setObjectName("dockerSelectedLogs");
    logs->setMinimumHeight(160);
    logs->setLineWrapMode(QPlainTextEdit::NoWrap);
    logs->setPlaceholderText(tr("No recent log output."));
    logs->setFont(QFontDatabase::systemFont(QFontDatabase::FixedFont));
    logs->setMaximumBlockCount(4000);
    m_tabs->addTab(logs, tr("Logs · last 200 lines"));
    m_details = new QLabel;
    m_details->setObjectName("dockerReadableDetails");
    m_details->setWordWrap(true);
    m_details->setTextFormat(Qt::RichText);
    m_details->setTextInteractionFlags(Qt::TextSelectableByMouse);
    m_details->setAlignment(Qt::AlignTop | Qt::AlignLeft);
    m_details->setMargin(12);
    auto *detailsScroll = new QScrollArea;
    detailsScroll->setObjectName("dockerDetailsScroll");
    detailsScroll->setWidgetResizable(true);
    detailsScroll->setFrameShape(QFrame::NoFrame);
    detailsScroll->setWidget(m_details);
    m_tabs->addTab(detailsScroll, tr("Details"));
    m_inspect = new QStackedWidget;
    for (auto *view : inspect)
        m_inspect->addWidget(view);
    m_tabs->addTab(m_inspect, tr("Raw inspect"));
    layout->addWidget(m_tabs, 1);
    auto *footer = new QHBoxLayout;
    m_status = new QLabel;
    m_status->setObjectName("dockerInspectorStatus");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    footer->addWidget(m_status, 1);
    auto *inventory = new QPushButton(tr("All resources…"));
    inventory->setToolTip(tr("Browse the complete inventory, including resources outside the canvas limit."));
    connect(inventory, &QPushButton::clicked, this, &DockerResourceInspector::inventoryRequested);
    footer->addWidget(inventory);
    layout->addLayout(footer);
    setSelection({});
}

void DockerResourceInspector::setSelection(const DockerInspectorSelection &selection)
{
    const bool changed = m_key != selection.key;
    m_key = selection.key;
    m_title->setText(selection.name.isEmpty() ? tr("Select a resource") : selection.name);
    m_state->setText(selection.status);
    m_details->setText(selection.detailsHtml);
    QString relatedHtml = selection.relatedHtml;
    const QString linkColor = palette().color(QPalette::Window).lightness() < 128 ? "#8ec5f4" : "#285f94";
    relatedHtml.replace("<a ", "<a style='color:" + linkColor + ";' ");
    m_related->setText(relatedHtml);
    const bool container = selection.kind == 0;
    m_start->setVisible(container);
    m_stop->setVisible(container);
    m_exec->setVisible(container);
    m_start->setEnabled(selection.canStart);
    m_stop->setEnabled(selection.canStop);
    m_exec->setEnabled(selection.canExec);
    m_tabs->setVisible(selection.kind >= 0);
    m_tabs->setTabEnabled(0, container);
    m_inspect->setCurrentIndex(qMax(0, selection.kind));
    if (changed)
        m_tabs->setCurrentIndex(container ? 0 : 1);
}

void DockerResourceInspector::setStatus(const QString &text)
{
    m_status->setText(text);
}
