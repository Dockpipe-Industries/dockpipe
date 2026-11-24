#pragma once

#include "LauncherUiExport.h"
#include <QDialog>
#include <QJsonObject>

class QCheckBox;
class QComboBox;
class QJsonArray;

class DOCKPIPE_LAUNCHER_UI_EXPORT RemoteDesktopSettingsDialog final : public QDialog {
    Q_OBJECT
public:
    explicit RemoteDesktopSettingsDialog(const QJsonObject &capture,
        const QJsonObject &display, const QString &decoder, QWidget *parent = nullptr);
    void setProviderRestrictions(bool clipboardUnavailable, bool virtualDesktopUnavailable, bool showLocalCursor);
    void setProviderRestrictions(bool clipboardUnavailable, bool virtualDesktopUnavailable, bool showLocalCursor,
        bool richClipboardUnavailable);
    void setProviderRestrictions(bool clipboardUnavailable, bool virtualDesktopUnavailable, bool showLocalCursor,
        bool clipboardImagesUnavailable, bool clipboardFilesUnavailable);
    void setPhysicalDisplays(const QJsonArray &displays, const QString &selectedSource);
    QJsonObject captureOptions() const;
    QJsonObject presentationOptions() const;
    QString decoderMode() const;

private:
    QComboBox *m_displaySource;
    QComboBox *m_quality;
    QComboBox *m_frameRate;
    QComboBox *m_scale;
    QComboBox *m_decoder;
    QCheckBox *m_hideCursor;
    QCheckBox *m_swapControlCommand;
    QCheckBox *m_shareClipboard;
    QCheckBox *m_shareImages;
    QCheckBox *m_shareFiles;
};
