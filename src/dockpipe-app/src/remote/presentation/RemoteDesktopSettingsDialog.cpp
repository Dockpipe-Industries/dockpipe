#include "RemoteDesktopSettingsDialog.h"

#include <QCheckBox>
#include <QComboBox>
#include <QDialogButtonBox>
#include <QFormLayout>
#include <QLabel>
#include <QVBoxLayout>

RemoteDesktopSettingsDialog::RemoteDesktopSettingsDialog(const QJsonObject &capture,
    const QJsonObject &display, const QString &decoder, QWidget *parent)
    : QDialog(parent)
{
    setObjectName(QStringLiteral("desktopSettingsDialog"));
    setWindowTitle(tr("Remote desktop settings"));
    auto *layout = new QVBoxLayout(this);
    auto *form = new QFormLayout;
    m_displaySource = new QComboBox;
    m_displaySource->setObjectName(QStringLiteral("desktopDisplaySource"));
    m_displaySource->addItem(tr("Current desktop"), QStringLiteral("existing"));
    m_displaySource->addItem(tr("Virtual desktop"), QStringLiteral("virtual"));
    m_displaySource->setCurrentIndex(capture.value("display_source") == QJsonValue("virtual") ? 1 : 0);
    form->addRow(tr("Desktop"), m_displaySource);

    m_quality = new QComboBox;
    m_quality->setObjectName(QStringLiteral("desktopQuality"));
    m_quality->addItem(tr("Low — 720p, 1 Mbps"), QStringLiteral("low"));
    m_quality->addItem(tr("Balanced — 900p, 2.5 Mbps"), QStringLiteral("balanced"));
    m_quality->addItem(tr("High — 1080p, 4 Mbps"), QStringLiteral("high"));
    m_quality->addItem(tr("4K — 2160p, 16 Mbps"), QStringLiteral("ultra"));
    const int qualityIndex = m_quality->findData(capture.value("quality").toString());
    m_quality->setCurrentIndex(qualityIndex >= 0 ? qualityIndex : 2);
    form->addRow(tr("Quality"), m_quality);

    m_frameRate = new QComboBox;
    m_frameRate->setObjectName(QStringLiteral("desktopFrameRate"));
    for (const int framesPerSecond : {10, 15, 24, 30, 60})
        m_frameRate->addItem(tr("%1 fps").arg(framesPerSecond), framesPerSecond);
    const int frameRateIndex = m_frameRate->findData(capture.value("frame_rate").toInt());
    m_frameRate->setCurrentIndex(frameRateIndex >= 0 ? frameRateIndex : 3);
    form->addRow(tr("Frame rate"), m_frameRate);

    m_decoder = new QComboBox;
    m_decoder->setObjectName(QStringLiteral("desktopDecoder"));
    m_decoder->addItem(tr("Automatic"), QStringLiteral("auto"));
    m_decoder->addItem(tr("Hardware only"), QStringLiteral("hardware"));
    m_decoder->addItem(tr("Software"), QStringLiteral("software"));
    const int decoderIndex = m_decoder->findData(decoder);
    m_decoder->setCurrentIndex(decoderIndex >= 0 ? decoderIndex : 2);
    form->addRow(tr("Video decoding"), m_decoder);

    m_scale = new QComboBox;
    m_scale->setObjectName(QStringLiteral("desktopScale"));
    m_scale->addItem(tr("Fit to window"), QStringLiteral("fit"));
    m_scale->addItem(tr("Actual size"), QStringLiteral("actual"));
    m_scale->setCurrentIndex(display.value("scale") == QJsonValue("actual") ? 1 : 0);
    form->addRow(tr("Display size"), m_scale);

    m_hideCursor = new QCheckBox(tr("Hide local cursor over the desktop"));
    m_hideCursor->setObjectName(QStringLiteral("desktopHideCursor"));
    m_hideCursor->setChecked(display.value("hide_cursor").toBool(true));
    form->addRow(m_hideCursor);

    m_swapControlCommand = new QCheckBox(tr("Swap Control and Command"));
    m_swapControlCommand->setObjectName(QStringLiteral("desktopSwapControlCommand"));
    m_swapControlCommand->setChecked(display.value("swap_control_command").toBool());
    m_swapControlCommand->setToolTip(tr("Control sends Command; Windows/Super sends Control. Option/Alt is unchanged."));
    form->addRow(m_swapControlCommand);

    m_shareClipboard = new QCheckBox(tr("Share text clipboard"));
    m_shareClipboard->setObjectName(QStringLiteral("desktopShareClipboard"));
    m_shareClipboard->setChecked(capture.value("clipboard_text").toBool());
    m_shareClipboard->setToolTip(tr("Share copied text while the control desktop is focused. Use the remote computer's copy and paste shortcuts."));
    form->addRow(m_shareClipboard);

    m_shareImages = new QCheckBox(tr("Share clipboard images"));
    m_shareImages->setObjectName(QStringLiteral("desktopShareImages"));
    m_shareImages->setChecked(capture.value("clipboard_images").toBool());
    form->addRow(m_shareImages);

    m_shareFiles = new QCheckBox(tr("Share clipboard files and file drops"));
    m_shareFiles->setObjectName(QStringLiteral("desktopShareFiles"));
    m_shareFiles->setChecked(capture.value("clipboard_files").toBool());
    m_shareFiles->setToolTip(tr("Drop files onto the desktop to transfer them, then paste in the remote app. Folders are not supported."));
    form->addRow(m_shareFiles);
    layout->addLayout(form);

    auto *hint = new QLabel;
    hint->setWordWrap(true);
    const auto updateHint = [this, hint] {
        hint->setText(m_displaySource->currentData().toString() == "virtual"
            ? tr("Virtual desktop uses the selected resolution. Changes apply on the next connection.")
            : tr("Quality and frame rate apply on the next connection. Resolution is a maximum; bitrate is a target."));
    };
    connect(m_displaySource, &QComboBox::currentIndexChanged, this, updateHint);
    updateHint();
    layout->addWidget(hint);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Save | QDialogButtonBox::Cancel);
    connect(buttons, &QDialogButtonBox::accepted, this, &QDialog::accept);
    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
    layout->addWidget(buttons);
}

QJsonObject RemoteDesktopSettingsDialog::captureOptions() const
{
    QJsonObject options{{"quality", m_quality->currentData().toString()},
        {"frame_rate", m_frameRate->currentData().toInt()}};
    if (m_displaySource->currentData().toString() == "virtual")
        options.insert("display_source", "virtual");
    if (m_shareClipboard->isChecked())
        options.insert("clipboard_text", true);
    if (m_shareImages->isChecked())
        options.insert("clipboard_images", true);
    if (m_shareFiles->isChecked())
        options.insert("clipboard_files", true);
    return options;
}

QJsonObject RemoteDesktopSettingsDialog::presentationOptions() const
{
    QJsonObject options{{"scale", m_scale->currentData().toString()},
        {"hide_cursor", m_hideCursor->isChecked()}};
    if (m_swapControlCommand->isChecked())
        options.insert("swap_control_command", true);
    return options;
}

QString RemoteDesktopSettingsDialog::decoderMode() const
{
    return m_decoder->currentData().toString();
}
