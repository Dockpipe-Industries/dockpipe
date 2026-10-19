#pragma once
#include <QJsonObject>
#include <QVector>
#include <QString>

// No scene or provider operations: the same comparison and route can carry
// working changes, commit/branch comparisons, or a provider-backed PR revision.
enum class GitReviewSource { WorkingTree, Commit, Branch, PullRequest };
enum class GitComparison { Unstaged, Staged, Combined };

struct GitReviewFile {
    QString path;
    QString originalPath;
    QString status;
    QString revision;
    bool binary = false;
    bool conflict() const;
    bool staged() const;
    bool unstaged() const;
    int requiredReviews() const;
};

struct GitReviewRow {
    QString before;
    QString after;
    int beforeLine = 0;
    int afterLine = 0;
    bool removed = false;
    bool added = false;
};

struct GitReviewHunk {
    QString heading;
    QVector<GitReviewRow> rows;
    int additions = 0;
    int deletions = 0;
};

struct GitReviewDiff {
    QString path;
    QString revision;
    QString beforeLabel;
    QString afterLabel;
    QString notice;
    GitComparison comparison = GitComparison::Unstaged;
    QVector<GitReviewHunk> hunks;
    bool binary = false;
    bool conflict = false;
    bool limited = false;
    static bool parse(const QJsonObject &object, GitReviewDiff &result, QString &error);
};

struct GitReviewModel {
    static constexpr int maximumFiles = 2048;
    GitReviewSource source = GitReviewSource::WorkingTree;
    QString root;
    QString branch;
    QString head;
    QString snapshot;
    QVector<GitReviewFile> files;
    static bool parse(const QJsonObject &object, GitReviewModel &result, QString &error);
    static QString comparisonName(GitComparison comparison);
    static int comparisonBit(GitComparison comparison);
};
