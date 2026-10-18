#include "GitReviewProgress.h"
#include <QCryptographicHash>
#include <QJsonDocument>
#include <QSettings>

namespace {
QString hash(const QString &value)
{
    return QString::fromLatin1(QCryptographicHash::hash(value.toUtf8(), QCryptographicHash::Sha256).toHex());
}
}
QString GitReviewProgress::key(const GitReviewFile &file) const
{
    return hash(file.path);
}
void GitReviewProgress::load(const QString &root)
{
    m_rootKey = hash(root);
    const QByteArray bytes = QSettings().value("cockpit/gitReview/" + m_rootKey).toByteArray();
    m_data = bytes.size() <= 64 * 1024 * 1024 ? QJsonDocument::fromJson(bytes).object() : QJsonObject{};
}
void GitReviewProgress::save()
{
    if (m_rootKey.isEmpty())
        return;
    QSettings settings;
    settings.beginGroup("cockpit/gitReview");
    // At most sixteen repository sessions, each at most 2048 compact records.
    QStringList order = settings.value("recent").toStringList();
    order.removeAll(m_rootKey);
    order.prepend(m_rootKey);
    while (order.size() > 16)
        settings.remove(order.takeLast());
    settings.setValue("recent", order);
    settings.setValue(m_rootKey, QJsonDocument(m_data).toJson(QJsonDocument::Compact));
}
bool GitReviewProgress::reviewed(const GitReviewFile &file) const
{
    const auto record = m_data.value(key(file)).toObject();
    return !file.conflict() && record.value("revision").toString() == file.revision
        && (record.value("mask").toInt() & file.requiredReviews()) == file.requiredReviews();
}
bool GitReviewProgress::stale(const GitReviewFile &file) const
{
    const auto record = m_data.value(key(file)).toObject();
    return record.value("mask").toInt() != 0 && record.value("revision").toString() != file.revision;
}
void GitReviewProgress::visit(const GitReviewFile &file, int hunk, GitComparison comparison)
{
    m_data["lastPath"] = file.path;
    m_data["lastHunk"] = qMax(0, hunk);
    m_data["lastComparison"] = int(comparison);
    save();
}
bool GitReviewProgress::mark(const GitReviewFile &file, GitComparison comparison, int hunk)
{
    if (!m_data.contains(key(file)) && m_data.size() >= GitReviewModel::maximumFiles + 3)
        return false;
    auto record = m_data.value(key(file)).toObject();
    int mask = record.value("revision").toString() == file.revision ? record.value("mask").toInt() : 0;
    mask |= GitReviewModel::comparisonBit(comparison);
    // A combined comparison is sufficient only when no independent index
    // version can be hidden by a mixed staged/unstaged working tree.
    if (comparison == GitComparison::Combined && !(file.staged() && file.unstaged()))
        mask |= file.requiredReviews();
    record["revision"] = file.revision;
    record["mask"] = mask;
    m_data[key(file)] = record;
    visit(file, hunk, comparison);
    return true;
}
bool GitReviewProgress::setNote(const GitReviewFile &file, const QString &note)
{
    if (!m_data.contains(key(file)) && m_data.size() >= GitReviewModel::maximumFiles + 3)
        return false;
    auto record = m_data.value(key(file)).toObject();
    record["note"] = note.left(4096);
    record["noteRevision"] = file.revision;
    m_data[key(file)] = record;
    save();
    return true;
}
QString GitReviewProgress::note(const GitReviewFile &file) const
{
    const auto record = m_data.value(key(file)).toObject();
    QString note = record.value("note").toString();
    if (!note.isEmpty() && record.value("noteRevision").toString() != file.revision)
        note.prepend("[Note from an older content revision]\n");
    return note;
}
