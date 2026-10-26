#pragma once

class QApplication;

// Explicit opt-in qualification of the real installed desktop application.
// Returns false when no qualification flags were supplied.
bool preparePackageDisplayQualification(QApplication &application, int &exitCode);
int runPackageDisplayQualification(QApplication &application);
