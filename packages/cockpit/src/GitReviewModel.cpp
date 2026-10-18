#include "GitReviewModel.h"
#include <QJsonArray>
#include <QRegularExpression>
#include <QSet>
#include <algorithm>

bool GitReviewFile::conflict() const
{
    return status.contains('U') || status == "AA" || status == "DD";
}
bool GitReviewFile::staged() const
{
    return status != "??" && status.size() == 2 && status[0] != ' ';
}
bool GitReviewFile::unstaged() const
{
    return status == "??" || (status.size() == 2 && status[1] != ' ');
}
int GitReviewFile::requiredReviews() const
{
    return (staged() ? 2 : 0) | (unstaged() ? 1 : 0);
}

QString GitReviewModel::comparisonName(GitComparison comparison)
{
    if (comparison == GitComparison::Staged)
        return "staged";
    if (comparison == GitComparison::Combined)
        return "combined";
    return "unstaged";
}
int GitReviewModel::comparisonBit(GitComparison comparison)
{
    if (comparison == GitComparison::Staged)
        return 2;
    if (comparison == GitComparison::Combined)
        return 4;
    return 1;
}

bool GitReviewModel::parse(const QJsonObject &object, GitReviewModel &result, QString &error)
{
    result = {};
    result.root = object.value("root").toString();
    result.branch = object.value("branch").toString();
    result.head = object.value("head").toString();
    result.snapshot = object.value("snapshot").toString();
    const auto changes = object.value("changes").toArray();
    const QRegularExpression digest("^[0-9a-f]{64}$");
    if (result.root.isEmpty() || !digest.match(result.snapshot).hasMatch() || changes.size() > maximumFiles) {
        error = "Invalid repository identity or changed-file budget.";
        return false;
    }
    QSet<QString> paths;
    for (const auto &value : changes) {
        const auto change = value.toObject();
        GitReviewFile file{change.value("path").toString(), change.value("original_path").toString(),
            change.value("status").toString(), change.value("revision").toString(), change.value("binary").toBool()};
        if (file.path.isEmpty() || file.path.size() > 4096 || paths.contains(file.path)
            || file.status.size() != 2 || !digest.match(file.revision).hasMatch()) {
            error = "Invalid or duplicate changed-file identity.";
            return false;
        }
        paths.insert(file.path);
        result.files.append(file);
    }
    std::sort(result.files.begin(), result.files.end(), [](const auto &left, const auto &right) { return left.path < right.path; });
    return true;
}

bool GitReviewDiff::parse(const QJsonObject &object, GitReviewDiff &result, QString &error)
{
    result = {};
    result.path = object.value("path").toString();
    result.revision = object.value("revision").toString();
    result.beforeLabel = object.value("before_label").toString();
    result.afterLabel = object.value("after_label").toString();
    result.notice = object.value("notice").toString();
    result.binary = object.value("binary").toBool();
    result.conflict = object.value("conflict").toBool();
    result.limited = object.value("limited").toBool();
    const QString comparison = object.value("comparison").toString();
    if (comparison == "staged")
        result.comparison = GitComparison::Staged;
    else if (comparison == "combined")
        result.comparison = GitComparison::Combined;
    else if (comparison != "unstaged") {
        error = "Unknown comparison source.";
        return false;
    }
    const QString patch = object.value("patch").toString();
    if (patch.size() > 2 * 1024 * 1024) {
        error = "Comparison exceeds the focused-code budget.";
        return false;
    }
    const QRegularExpression header("^@@ -(\\d+)(?:,(\\d+))? \\+(\\d+)(?:,(\\d+))? @@(.*)$");
    QVector<QPair<int, QString>> removed;
    QVector<QPair<int, QString>> added;
    int beforeLine = 0, afterLine = 0, beforeRemaining = 0, afterRemaining = 0, rows = 0;
    auto flush = [&]() {
        if (result.hunks.isEmpty())
            return;
        auto &hunk = result.hunks.last();
        for (int index = 0; index < qMax(removed.size(), added.size()); ++index) {
            GitReviewRow row;
            if (index < removed.size()) {
                row.beforeLine = removed[index].first;
                row.before = removed[index].second;
                row.removed = true;
                ++hunk.deletions;
            }
            if (index < added.size()) {
                row.afterLine = added[index].first;
                row.after = added[index].second;
                row.added = true;
                ++hunk.additions;
            }
            hunk.rows.append(row);
        }
        removed.clear();
        added.clear();
    };
    for (const QString &line : patch.split('\n')) {
        if (line.size() > 16384) {
            result.hunks.clear();
            result.limited = true;
            result.notice = "A line exceeds 16,384 characters; inspect this file in the conventional editor.";
            return true;
        }
        if (line.startsWith("\\ No newline"))
            result.notice = "This comparison includes content without a final newline.";
        const auto match = header.match(line);
        if (match.hasMatch()) {
            flush();
            if (beforeRemaining != 0 || afterRemaining != 0) {
                error = "Incomplete diff hunk; refresh the comparison.";
                return false;
            }
            beforeLine = match.captured(1).toInt();
            afterLine = match.captured(3).toInt();
            beforeRemaining = match.captured(2).isEmpty() ? 1 : match.captured(2).toInt();
            afterRemaining = match.captured(4).isEmpty() ? 1 : match.captured(4).toInt();
            result.hunks.append({line, {}});
        } else if (!result.hunks.isEmpty() && (beforeRemaining > 0 || afterRemaining > 0)) {
            if (line.startsWith(' ')) {
                flush();
                result.hunks.last().rows.append({line.mid(1), line.mid(1), beforeLine++, afterLine++});
                --beforeRemaining;
                --afterRemaining;
            } else if (line.startsWith('-')) {
                removed.append({beforeLine++, line.mid(1)});
                --beforeRemaining;
            } else if (line.startsWith('+')) {
                added.append({afterLine++, line.mid(1)});
                --afterRemaining;
            } else if (!line.startsWith("\\ No newline")) {
                error = "Unsupported or incomplete diff content.";
                return false;
            }
            if (++rows > 20000) {
                result.hunks.clear();
                result.limited = true;
                result.notice = "Comparison exceeds 20,000 display rows; inspect it in the conventional editor.";
                return true;
            }
            if (beforeRemaining < 0 || afterRemaining < 0) {
                error = "Diff line mapping does not match its hunk header.";
                return false;
            }
        }
    }
    flush();
    if (beforeRemaining != 0 || afterRemaining != 0) {
        error = "Truncated diff hunk.";
        return false;
    }
    return true;
}
