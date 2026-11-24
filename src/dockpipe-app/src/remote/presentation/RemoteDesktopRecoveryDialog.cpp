#include "RemoteDesktopRecoveryDialog.h"
#include "DockpipeChoices.h"
#include "RemoteCommand.h"

#include <QComboBox>
#include <QCoreApplication>
#include <QDialog>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLabel>
#include <QPushButton>
#include <QRegularExpression>
#include <QSettings>
#include <QTemporaryFile>
#include <QVBoxLayout>
#include <memory>

namespace {
constexpr auto SessionsKey = "remote/uncertainDesktopSessions";
QString text(const char *source) { return QCoreApplication::translate("RemoteDesktopRecoveryDialog", source); }

bool validReference(const QJsonObject &reference)
{
    const auto target = reference.value("target").toObject();
    static const QRegularExpression digest(QStringLiteral("^[0-9a-f]{64}$"));
    static const QRegularExpression node(QStringLiteral("^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$"));
    return reference.size() == 4 && target.size() == 3 && reference.value("schema") == QJsonValue("dockpipe.remote.desktop/v1")
        && digest.match(reference.value("id").toString()).hasMatch() && digest.match(reference.value("epoch").toString()).hasMatch()
        && digest.match(target.value("fingerprint").toString()).hasMatch() && digest.match(target.value("generation").toString()).hasMatch()
        && node.match(target.value("node").toString()).hasMatch();
}

bool loadSessions(QJsonArray &sessions)
{
    QSettings settings;
    settings.sync();
    const auto data = settings.value(SessionsKey).toByteArray();
    if (data.isEmpty()) { sessions = {}; return true; }
    if (data.size() > 256 * 1024)
        return false;
    QJsonParseError error;
    const auto document = QJsonDocument::fromJson(data, &error);
    if (error.error != QJsonParseError::NoError || !document.isArray() || document.array().size() > 64)
        return false;
    for (const auto &entry : document.array()) {
        if (!entry.isObject() || !validReference(entry.toObject()))
            return false;
    }
    sessions = document.array();
    return settings.status() == QSettings::NoError;
}

class RecoveryDialog final : public QDialog {
public:
    explicit RecoveryDialog(QWidget *parent) : QDialog(parent), m_command(this)
    {
        setAttribute(Qt::WA_DeleteOnClose);
        setObjectName(QStringLiteral("desktopRecoveryDialog"));
        setWindowTitle(text(QT_TRANSLATE_NOOP("RemoteDesktopRecoveryDialog", "Retained desktop sessions")));
        resize(660, 300);
        auto *layout = new QVBoxLayout(this);
        auto *hint = new QLabel(text(QT_TRANSLATE_NOOP("RemoteDesktopRecoveryDialog", "Inspect or close the original session. A close request is not native cleanup proof.")));
        hint->setWordWrap(true);
        layout->addWidget(hint);
        m_sessions = new QComboBox;
        m_sessions->setObjectName(QStringLiteral("desktopRecoverySessions"));
        layout->addWidget(m_sessions);
        m_identity = new QLabel;
        m_identity->setTextFormat(Qt::PlainText);
        m_identity->setWordWrap(true);
        m_identity->setTextInteractionFlags(Qt::TextSelectableByMouse);
        layout->addWidget(m_identity);
        m_status = new QLabel;
        m_status->setObjectName(QStringLiteral("desktopRecoveryStatus"));
        m_status->setTextFormat(Qt::PlainText);
        m_status->setWordWrap(true);
        layout->addWidget(m_status);
        auto *actions = new QHBoxLayout;
        m_inspect = new QPushButton(text(QT_TRANSLATE_NOOP("RemoteDesktopRecoveryDialog", "Inspect status")));
        m_inspect->setObjectName(QStringLiteral("desktopRecoveryInspect"));
        m_close = new QPushButton(text(QT_TRANSLATE_NOOP("RemoteDesktopRecoveryDialog", "Request closure")));
        m_close->setObjectName(QStringLiteral("desktopRecoveryRequestClose"));
        actions->addWidget(m_inspect);
        actions->addWidget(m_close);
        layout->addLayout(actions);
        auto *dismiss = new QPushButton(text(QT_TRANSLATE_NOOP("RemoteDesktopRecoveryDialog", "Close")));
        layout->addWidget(dismiss, 0, Qt::AlignRight);
        connect(dismiss, &QPushButton::clicked, this, &QDialog::close);
        connect(m_inspect, &QPushButton::clicked, this, [this] { run(false); });
        connect(m_close, &QPushButton::clicked, this, [this] { run(true); });
        connect(m_sessions, &QComboBox::currentIndexChanged, this, [this] {
            m_identity->setText(QString::fromUtf8(QJsonDocument(m_sessions->currentData().toJsonObject()).toJson(QJsonDocument::Indented)));
        });
        reload();
    }
private:
    void reload()
    {
        m_sessions->clear();
        QJsonArray sessions;
        m_valid = loadSessions(sessions);
        if (!m_valid)
            m_status->setText(text(QT_TRANSLATE_NOOP("RemoteDesktopRecoveryDialog", "Retained session metadata is invalid. It has been preserved.")));
        for (const auto &entry : sessions) {
            const auto reference = entry.toObject();
            m_sessions->addItem(reference.value("target").toObject().value("node").toString() + " · " + reference.value("id").toString().left(16), reference);
        }
        if (m_valid && sessions.isEmpty())
            m_status->setText(text(QT_TRANSLATE_NOOP("RemoteDesktopRecoveryDialog", "No retained desktop sessions.")));
        updateActions();
    }
    void updateActions()
    {
        const bool available = m_valid && m_sessions->currentIndex() >= 0 && !m_command.busy();
        m_inspect->setEnabled(available);
        m_close->setEnabled(available);
        m_sessions->setEnabled(!m_command.busy());
    }
    void run(bool close)
    {
        const auto reference = m_sessions->currentData().toJsonObject();
        if (!validReference(reference) || m_command.busy())
            return;
        QString error;
        const auto cli = DockpipeChoices::operatorDockpipeBinary(&error);
        if (cli.isEmpty()) { m_status->setText(error); return; }
        m_request = std::make_unique<QTemporaryFile>();
        const QJsonObject request = close ? reference : QJsonObject{{"reference", reference}, {"metadata_only", true}};
        const auto data = QJsonDocument(request).toJson(QJsonDocument::Compact);
        if (!m_request->open() || !m_request->setPermissions(QFileDevice::ReadOwner | QFileDevice::WriteOwner)
            || m_request->write(data) != data.size() || !m_request->flush()) {
            m_status->setText(text(QT_TRANSLATE_NOOP("RemoteDesktopRecoveryDialog", "Could not prepare a private recovery request.")));
            return;
        }
        m_command.run(cli, {"remote", close ? "desktop-close" : "desktop-status", "--request", m_request->fileName(), "--auth", "never"},
            [this, reference](bool success, const QByteArray &data) {
            m_request.reset();
            const auto result = data.size() <= 1024 * 1024 ? QJsonDocument::fromJson(data).object() : QJsonObject{};
            const auto state = result.value("state").toString();
            if (!success || result.value("reference").toObject() != reference
                || !QStringList{"queued", "opening", "running", "closing", "closed", "unknown"}.contains(state)) {
                m_status->setText(text(QT_TRANSLATE_NOOP("RemoteDesktopRecoveryDialog", "Status could not be verified. The reference is retained.")));
            } else if (state == "closed" && result.value("lease_millis").isDouble() && result.value("lease_millis").toDouble() == 0) {
                QJsonArray current;
                if (loadSessions(current)) {
                    QJsonArray next;
                    for (const auto &entry : current) {
                        if (entry.toObject() != reference)
                            next.append(entry);
                    }
                    QSettings settings;
                    settings.setValue(SessionsKey, QJsonDocument(next).toJson(QJsonDocument::Compact));
                    settings.sync();
                    if (settings.status() == QSettings::NoError) {
                        reload();
                        m_status->setText(text(QT_TRANSLATE_NOOP("RemoteDesktopRecoveryDialog", "Native cleanup verified for this exact session.")));
                    }
                }
            } else {
                m_status->setText(text(QT_TRANSLATE_NOOP("RemoteDesktopRecoveryDialog", "Session status: %1. The reference is retained.")).arg(state));
            }
            updateActions();
        }, 10000);
        updateActions();
    }
    QComboBox *m_sessions;
    QLabel *m_identity;
    QLabel *m_status;
    QPushButton *m_inspect;
    QPushButton *m_close;
    bool m_valid = false;
    std::unique_ptr<QTemporaryFile> m_request;
    RemoteCommand m_command;
};
}

QPushButton *RemoteDesktopRecoveryDialog::createButton(QWidget *parent)
{
    auto *button = new QPushButton(QCoreApplication::translate("RemoteDesktopRecoveryDialog", "Retained sessions…"), parent);
    button->setObjectName(QStringLiteral("desktopRecoveryButton"));
    QObject::connect(button, &QPushButton::clicked, parent, [parent] { (new RecoveryDialog(parent))->open(); });
    return button;
}
