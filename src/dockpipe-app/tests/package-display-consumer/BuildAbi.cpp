#include <QSysInfo>
#include <QString>
#include <cstdio>

int main()
{
    std::puts(QSysInfo::buildAbi().toUtf8().constData());
    return 0;
}
