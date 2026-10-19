#pragma once
#include "GitReviewModel.h"
#include <QJsonObject>

// User-local review metadata, never repository code or a remote review approval.
class GitReviewProgress final {
public:
    void load(const QString &root);
    void save();
    bool reviewed(const GitReviewFile &file) const;
    bool stale(const GitReviewFile &file) const;
    bool mark(const GitReviewFile &file, GitComparison comparison, int hunk);
    void visit(const GitReviewFile &file, int hunk, GitComparison comparison = GitComparison::Unstaged);
    bool setNote(const GitReviewFile &file, const QString &note);
    QString note(const GitReviewFile &file) const;
    QString lastPath() const { return m_data.value("lastPath").toString(); }
    GitComparison lastComparison() const { return GitComparison(qBound(0, m_data.value("lastComparison").toInt(), 2)); }
    int lastHunk() const { return m_data.value("lastHunk").toInt(); }
private:
    QString key(const GitReviewFile &file) const;
    QString m_rootKey;
    QJsonObject m_data;
};
