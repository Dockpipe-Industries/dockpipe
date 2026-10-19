#pragma once

#include <QByteArray>
#include <QString>

class BrowserExtensionCrx final {
public:
    // Extracts identity metadata only. This does not verify the CRX signature.
    // The caller must authenticate the source and validate the archive separately.
    static QByteArray extractPublicKey(const QByteArray &crx, const QString &expectedId,
                                       QString *error);
};
