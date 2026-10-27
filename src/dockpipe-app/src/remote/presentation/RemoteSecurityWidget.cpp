#include "RemoteSecurityWidget.h"

#include "DockpipeChoices.h"

#include <QCheckBox>
#include <QDateTime>
#include <QFormLayout>
#include <QGroupBox>
#include <QHBoxLayout>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QLocale>
#include <QPushButton>
#include <QSignalBlocker>
#include <QSpinBox>
#include <QVBoxLayout>

#include <utility>

namespace {

QLabel *securityLabel(const QString &text, const QString &name)
{
    auto *label = new QLabel(text);
    label->setObjectName(name);
    label->setWordWrap(true);
    label->setTextFormat(Qt::PlainText);
    return label;
}

QString capabilityText(const QString &name, bool available, const QString &state = {})
{
    if (!available)
        return QObject::tr("%1: unavailable on this host").arg(name);
    if (state.isEmpty())
        return QObject::tr("%1: available").arg(name);
    return QObject::tr("%1: %2").arg(name, state);
}

} // namespace

RemoteSecurityWidget::RemoteSecurityWidget(QWidget *parent)
    : QWidget(parent), m_command(this)
{
    setObjectName(QStringLiteral("remoteSecurityWidget"));

    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(16, 16, 16, 16);
    layout->setSpacing(12);

    layout->addWidget(securityLabel(tr("Remote access"), QStringLiteral("remoteSecurityTitle")));
    m_status = securityLabel(tr("Checking remote access security…"), QStringLiteral("remoteSecurityStatus"));
    m_status->setProperty("securityState", QStringLiteral("unknown"));
    layout->addWidget(m_status);

    m_capabilities = securityLabel(QString(), QStringLiteral("remoteSecurityCapabilities"));
    layout->addWidget(m_capabilities);
    m_detail = securityLabel(QString(), QStringLiteral("remoteSecurityDetail"));
    m_detail->hide();
    layout->addWidget(m_detail);

    auto *actions = new QHBoxLayout;
    m_unlock = new QPushButton(tr("Unlock remote access…"));
    m_unlock->setObjectName(QStringLiteral("remoteSecurityUnlock"));
    m_lock = new QPushButton(tr("Lock now"));
    m_lock->setObjectName(QStringLiteral("remoteSecurityLock"));
    m_refresh = new QPushButton(tr("Refresh"));
    m_refresh->setObjectName(QStringLiteral("remoteSecurityRefresh"));
    actions->addWidget(m_unlock);
    actions->addWidget(m_lock);
    actions->addStretch();
    actions->addWidget(m_refresh);
    layout->addLayout(actions);

    auto *policy = new QGroupBox(tr("Automatic remote access lock"));
    policy->setObjectName(QStringLiteral("remoteSecurityPolicy"));
    auto *policyLayout = new QVBoxLayout(policy);
    auto *form = new QFormLayout;
    m_idleSeconds = new QSpinBox;
    m_idleSeconds->setObjectName(QStringLiteral("remoteSecurityIdleSeconds"));
    m_idleSeconds->setRange(60, 3600);
    m_idleSeconds->setSuffix(tr(" seconds"));
    form->addRow(tr("Lock after idle"), m_idleSeconds);
    policyLayout->addLayout(form);

    m_lockOnScreenLock = new QCheckBox(tr("Lock when the screen locks"));
    m_lockOnScreenLock->setObjectName(QStringLiteral("remoteSecurityLockOnScreenLock"));
    m_lockOnSleep = new QCheckBox(tr("Lock when the computer sleeps"));
    m_lockOnSleep->setObjectName(QStringLiteral("remoteSecurityLockOnSleep"));
    policyLayout->addWidget(m_lockOnScreenLock);
    policyLayout->addWidget(m_lockOnSleep);
    m_savePolicy = new QPushButton(tr("Save security policy"));
    m_savePolicy->setObjectName(QStringLiteral("remoteSecuritySavePolicy"));
    policyLayout->addWidget(m_savePolicy, 0, Qt::AlignLeft);
    layout->addWidget(policy);
    layout->addStretch();

    connect(m_refresh, &QPushButton::clicked, this, [this]() {
        clearOperationError();
        refresh();
    });
    connect(m_unlock, &QPushButton::clicked, this, [this]() {
        clearOperationError();
        run({QStringLiteral("unlock")}, 125000, [this](bool succeeded, const QByteArray &output) {
            if (!succeeded)
                m_operationError = tr("Unlock was not completed. %1")
                    .arg(QString::fromUtf8(output.isEmpty() ? m_commandDiagnostics : output).trimmed());
            refresh();
        });
    });
    connect(m_lock, &QPushButton::clicked, this, [this]() {
        clearOperationError();
        run({QStringLiteral("lock")}, 30000, [this](bool succeeded, const QByteArray &output) {
            if (!succeeded)
                m_operationError = tr("Remote access could not be locked. %1")
                    .arg(QString::fromUtf8(output.isEmpty() ? m_commandDiagnostics : output).trimmed());
            refresh();
        });
    });
    connect(m_savePolicy, &QPushButton::clicked, this, &RemoteSecurityWidget::savePolicy);
    connect(&m_command, &RemoteCommand::busyChanged, this, &RemoteSecurityWidget::setBusy);
    connect(&m_command, &RemoteCommand::output, this, [this](const QString &output) {
        constexpr qsizetype maximumDiagnostics = 64 * 1024;
        const QByteArray bytes = output.toUtf8();
        const qsizetype remaining = maximumDiagnostics - m_commandDiagnostics.size();
        if (remaining > 0)
            m_commandDiagnostics.append(bytes.left(remaining));
    });

    updateActions();
}

void RemoteSecurityWidget::refresh()
{
    if (m_command.busy())
        return;

    run({QStringLiteral("security-status")}, 30000,
        [this](bool succeeded, const QByteArray &output) {
            if (!succeeded) {
                setUnavailable(tr("Dockpipe could not read the remote access security status."));
                return;
            }
            QJsonParseError parseError;
            const QJsonDocument document = QJsonDocument::fromJson(output, &parseError);
            if (parseError.error != QJsonParseError::NoError || !document.isObject()) {
                setUnavailable(tr("Dockpipe returned an invalid security status."));
                return;
            }
            applyStatus(document.object());
        });
}

void RemoteSecurityWidget::run(const QStringList &arguments, int timeoutMs,
                               RemoteCommand::Completion completion)
{
    if (m_command.busy())
        return;

    QString error;
    const QString program = DockpipeChoices::operatorDockpipeBinary(&error);
    if (program.isEmpty()) {
        setUnavailable(error);
        if (completion)
            completion(false, error.toUtf8());
        return;
    }

    QStringList commandArguments{QStringLiteral("remote")};
    commandArguments.append(arguments);
    m_commandDiagnostics.clear();
    m_command.run(program, commandArguments, std::move(completion), timeoutMs);
}

void RemoteSecurityWidget::applyStatus(const QJsonObject &status)
{
    m_hasStatus = status.value(QStringLiteral("supported")).isBool()
        && status.value(QStringLiteral("authority_available")).isBool()
        && status.value(QStringLiteral("locked")).isBool()
        && status.value(QStringLiteral("idle_seconds")).isDouble()
        && status.value(QStringLiteral("lock_on_screen_lock")).isBool()
        && status.value(QStringLiteral("lock_on_sleep")).isBool()
        && status.value(QStringLiteral("absolute_seconds")).isDouble()
        && status.value(QStringLiteral("absolute_seconds")).toInt() == 3600
        && status.value(QStringLiteral("expires_at")).isString()
        && status.value(QStringLiteral("capabilities")).isObject();
    if (!m_hasStatus) {
        setUnavailable(tr("Dockpipe returned an incomplete security status."));
        return;
    }

    const QJsonObject capabilities = status.value(QStringLiteral("capabilities")).toObject();
    m_hasStatus = capabilities.value(QStringLiteral("idle")).isBool()
        && capabilities.value(QStringLiteral("screen_lock")).isBool()
        && capabilities.value(QStringLiteral("sleep")).isBool();
    if (!m_hasStatus) {
        setUnavailable(tr("Dockpipe returned incomplete security capabilities."));
        return;
    }

    m_supported = status.value(QStringLiteral("supported")).toBool();
    m_authorityAvailable = status.value(QStringLiteral("authority_available")).toBool();
    m_locked = status.value(QStringLiteral("locked")).toBool();
    m_idleSupported = capabilities.value(QStringLiteral("idle")).toBool();
    m_screenLockSupported = capabilities.value(QStringLiteral("screen_lock")).toBool();
    m_sleepSupported = capabilities.value(QStringLiteral("sleep")).toBool();

    if (!m_authorityAvailable) {
        setUnavailable(status.value(QStringLiteral("detail")).toString(), m_locked);
        return;
    }

    const int idleSeconds = status.value(QStringLiteral("idle_seconds")).toInt();
    if (idleSeconds < 60 || idleSeconds > 3600) {
        setUnavailable(tr("Dockpipe returned an out-of-range idle lock policy."));
        return;
    }

    const QSignalBlocker idleBlocker(m_idleSeconds);
    const QSignalBlocker screenBlocker(m_lockOnScreenLock);
    const QSignalBlocker sleepBlocker(m_lockOnSleep);
    m_idleSeconds->setValue(idleSeconds);
    m_lockOnScreenLock->setChecked(status.value(QStringLiteral("lock_on_screen_lock")).toBool());
    m_lockOnSleep->setChecked(status.value(QStringLiteral("lock_on_sleep")).toBool());

    m_status->setProperty("securityState", m_locked ? QStringLiteral("locked") : QStringLiteral("unlocked"));
    QString state = m_locked ? tr("Remote access is locked.") : tr("Remote access is unlocked.");
    if (!m_supported)
        state += tr(" Required native lock capabilities are unavailable; unlock will be refused.");
    if (!m_locked && status.value(QStringLiteral("expires_at")).isString()) {
        const QDateTime expiry = QDateTime::fromString(
            status.value(QStringLiteral("expires_at")).toString(), Qt::ISODate);
        if (expiry.isValid())
            state += tr(" Current authorization expires at %1.")
                .arg(QLocale().toString(expiry.toLocalTime(), QLocale::ShortFormat));
    }
    m_status->setText(state);

    const QString idleCapability = m_idleSupported
        ? capabilityText(tr("Idle lock"), true, tr("after %1 seconds").arg(idleSeconds))
        : tr("Idle lock: native idle detection unavailable; conservative duration cap applies.");
    m_capabilities->setText(QStringList{
        idleCapability,
        capabilityText(tr("Screen lock"), m_screenLockSupported,
                       m_screenLockSupported
                           ? (m_lockOnScreenLock->isChecked() ? tr("enabled") : tr("disabled"))
                           : QString()),
        capabilityText(tr("Sleep lock"), m_sleepSupported,
                       m_sleepSupported
                           ? (m_lockOnSleep->isChecked() ? tr("enabled") : tr("disabled"))
                           : QString())
    }.join(QStringLiteral("\n")));
    showDetail(status.value(QStringLiteral("detail")).toString());

    // The authority owns policy validation and re-authenticates any weakening.
    // Keep policy editable when native event sources are missing so the shared
    // values can still be set; idle enforcement falls back to the duration cap.
    m_idleSeconds->setEnabled(m_authorityAvailable);
    m_lockOnScreenLock->setEnabled(m_authorityAvailable);
    m_lockOnSleep->setEnabled(m_authorityAvailable);
    updateActions();
}

void RemoteSecurityWidget::setUnavailable(const QString &detail, bool locked)
{
    m_hasStatus = false;
    m_supported = false;
    m_authorityAvailable = false;
    m_status->setProperty("securityState", QStringLiteral("unavailable"));
    m_status->setText(locked
        ? tr("Remote access is locked; security controls are unavailable on this host.")
        : tr("Remote access security is unavailable on this host."));
    m_capabilities->setText(QStringList{
        capabilityText(tr("Idle lock"), false),
        capabilityText(tr("Screen lock"), false),
        capabilityText(tr("Sleep lock"), false)
    }.join(QStringLiteral("\n")));
    showDetail(detail);
    m_idleSeconds->setEnabled(false);
    m_lockOnScreenLock->setEnabled(false);
    m_lockOnSleep->setEnabled(false);
    updateActions();
}

void RemoteSecurityWidget::showDetail(const QString &detail)
{
    QStringList messages;
    if (!detail.trimmed().isEmpty())
        messages.append(detail.trimmed());
    if (!m_operationError.trimmed().isEmpty())
        messages.append(m_operationError.trimmed());
    m_detail->setText(messages.join(QLatin1Char('\n')));
    m_detail->setVisible(!messages.isEmpty());
}

void RemoteSecurityWidget::clearOperationError()
{
    m_operationError.clear();
    showDetail({});
}

void RemoteSecurityWidget::setBusy(bool busy)
{
    m_busy = busy;
    if (!busy && m_policySavePending) {
        m_policySavePending = false;
        if (m_hasStatus && m_authorityAvailable) {
            savePolicy();
            return;
        }
    }
    updateActions();
}

void RemoteSecurityWidget::updateActions()
{
    const bool busy = m_busy || m_command.busy();
    m_refresh->setEnabled(!busy);
    // Lock and OS-authenticated unlock remain reachable if status reporting is
    // unavailable; the CLI is the authority and reports unsupported actions.
    m_unlock->setEnabled(!busy);
    m_lock->setEnabled(!busy);
    m_savePolicy->setEnabled(!busy && m_hasStatus && m_authorityAvailable);
}

void RemoteSecurityWidget::savePolicy()
{
    if (!m_hasStatus || !m_authorityAvailable)
        return;
    clearOperationError();
    if (m_command.busy()) {
        m_policySavePending = true;
        return;
    }

    const auto booleanArgument = [](const QString &name, bool enabled) {
        return name + QLatin1Char('=') + (enabled ? QStringLiteral("true") : QStringLiteral("false"));
    };
    run({QStringLiteral("security-policy"),
         QStringLiteral("--idle-seconds"), QString::number(m_idleSeconds->value()),
         booleanArgument(QStringLiteral("--lock-on-screen-lock"), m_lockOnScreenLock->isChecked()),
         booleanArgument(QStringLiteral("--lock-on-sleep"), m_lockOnSleep->isChecked())},
        125000, [this](bool succeeded, const QByteArray &output) {
            if (!succeeded) {
                m_operationError = tr("Dockpipe could not update the shared remote access policy. %1")
                    .arg(QString::fromUtf8(output.isEmpty() ? m_commandDiagnostics : output).trimmed());
                showDetail({});
                return;
            }
            refresh();
        });
}
