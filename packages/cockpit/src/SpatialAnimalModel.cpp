#include "SpatialAnimalModel.h"
#include <QRegularExpression>
#include <cmath>

namespace {
void body(SpatialMesh &mesh, const QVector3D &center, const QVector3D &size)
{
    const int first = mesh.vertices.size();
    mesh.vertices += {center + QVector3D(-size.x(), 0, 0), center + QVector3D(size.x(), 0, 0),
        center + QVector3D(0, -size.y(), 0), center + QVector3D(0, size.y(), 0),
        center + QVector3D(0, 0, -size.z()), center + QVector3D(0, 0, size.z())};
    for (const auto &face : QVector<QVector<int>>{{0, 3, 4}, {4, 3, 1}, {1, 3, 5}, {5, 3, 0},
             {4, 2, 0}, {1, 2, 4}, {5, 2, 1}, {0, 2, 5}})
        mesh.faces.append({first + face[0], first + face[1], first + face[2]});
}

void limb(SpatialMesh &mesh, QVector3D root, QVector3D joint, QVector3D tip)
{
    // Bent tapered ribbons are deliberately cheap: a quad and triangle per limb.
    // Both sides are rendered by the existing mesh renderer.
    const int first = mesh.vertices.size();
    const QVector3D width(0, 6, 0);
    mesh.vertices += {root + width, root - width, joint + width, joint - width, tip};
    mesh.faces += {{first, first + 1, first + 3, first + 2}, {first + 2, first + 3, first + 4}};
}
}

bool SpatialAnimalModel::validForm(const QString &form)
{
    return QStringList{"spider", "crab", "insect", "butterfly", "worm", "fish", "bird", "quadruped", "cephalopod"}.contains(form);
}

void SpatialAnimalModel::interpret(SpatialWebItem &item, const QString &context)
{
    if (item.kind != "link" && item.kind != "animal" && item.kind != "image")
        return;
    static const QRegularExpression subject("\\b(animal|animals|wildlife|habitat|invertebrates|mammals|birds|fauna|aquarium)\\b",
        QRegularExpression::CaseInsensitiveOption);
    if (!subject.match(context).hasMatch())
        return;
    const QString name = item.title.toLower();
    const QVector<QPair<QString, QString>> forms = {
        {"spider", "\\b(spider|tarantula|black widow)\\b"},
        {"crab", "\\bcrab\\b"},
        {"butterfly", "\\b(butterfly|moth)\\b"},
        {"insect", "\\b(cicada|beetle|ant|bee|grasshopper|dragonfly|mantis)\\b"},
        {"worm", "\\b(earthworm|worm)\\b"},
        {"cephalopod", "\\b(octopus|squid|cuttlefish)\\b"},
        {"fish", "\\b(shark|fish|salmon|tuna|clownfish|seahorse)\\b"},
        {"bird", "\\b(eagle|owl|penguin|parrot|flamingo|bird)\\b"},
        {"quadruped", "\\b(lion|tiger|wolf|bear|elephant|giraffe|fox|zebra|panda|rhinoceros)\\b"}};
    for (const auto &form : forms) {
        if (QRegularExpression(form.second).match(name).hasMatch()) {
            item.animalForm = form.first;
            item.kind = "animal";
            return;
        }
    }
    // Explicit animal article links can request a conservative neural body-plan
    // suggestion. Broad category/navigation labels remain destinations.
    if (item.url.path().contains("/animals/") && item.url.path().contains("/facts/"))
        item.kind = "animal";
}

SpatialMesh SpatialAnimalModel::assemble(const QString &form)
{
    SpatialMesh mesh;
    mesh.color = QColor("#d9ac66");
    if (!validForm(form))
        return mesh;
    if (form == "worm") {
        mesh.color = QColor("#b78075");
        for (int segment = 0; segment < 5; ++segment)
            body(mesh, {float(segment * 32 - 64), float(10 + segment % 2 * 6), float(segment % 2 * 12)}, {22, 16, 18});
        return mesh;
    }
    const bool crab = form == "crab";
    const bool swimmer = form == "fish" || form == "cephalopod";
    body(mesh, {0, 35, 12}, {crab ? 60.0f : 35.0f, 30, swimmer ? 65.0f : 48.0f});
    if (form != "cephalopod")
        body(mesh, {0, 40, -48}, {22, 23, 25});
    int pairs = 0;
    if (form == "spider" || crab || form == "cephalopod")
        pairs = 4;
    else if (form == "insect")
        pairs = 3;
    else if (form == "quadruped")
        pairs = 2;
    for (int side : {-1, 1}) {
        for (int pair = 0; pair < pairs; ++pair) {
            const float z = -30 + pair * 24;
            const float reach = form == "quadruped" ? 35 : 100;
            const float spread = pair - (pairs - 1) / 2.0f;
            limb(mesh, {float(side * 22), 30, z}, {side * reach * (0.65f + pair * 0.07f), 35.0f + pair * 8, z + spread * 25},
                {side * reach * (0.75f + pair * 0.1f), -15, z + spread * 45});
        }
        if (form == "butterfly" || form == "bird" || form == "fish")
            limb(mesh, {float(side * 18), 35, -10}, {float(side * 110), 80, 5}, {float(side * 85), 15, 65});
        if (crab)
            limb(mesh, {float(side * 45), 35, -15}, {float(side * 85), 50, -75}, {float(side * 60), 70, -110});
    }
    return mesh;
}
