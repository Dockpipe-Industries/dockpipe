"""Discover the native toolchain closure with installed compilers; no downloads."""
from pathlib import Path
import json,os,re,shlex,subprocess,sys
out=Path(sys.argv[1]);files=set()
def add(p):
 p=Path(p).resolve()
 if p.is_file():files.add(p)
def command(args,**kwargs):return subprocess.check_output(args,text=True,timeout=20,**kwargs).strip()
qt=Path(command(['/usr/lib/qt6/bin/qmake6','-query','QT_INSTALL_HEADERS']))
probe='#include <fstream>\n#include <iostream>\n#include <cstring>\n#include <limits>\n#include <vector>\n#include <stdexcept>\n#include <QApplication>\n#include <QLabel>\n#include <QLineEdit>\n#include <QPushButton>\n#include <QVBoxLayout>\n#include <QPointer>\n#include <QTimer>\n#include <QFile>\n#include <QJsonDocument>\n#include <QJsonObject>\n#include <QPixmap>\n#include <QThread>\n'
cmd=['/usr/bin/g++','-std=c++17','-fPIC','-M','-x','c++','-','-I'+str(qt),*['-I'+str(qt/n) for n in ['QtCore','QtGui','QtWidgets']]]
deps=command(cmd,input=probe).replace('\\\n',' ')
for p in shlex.split(deps.split(':',1)[1]):add(p)
programs=['/usr/bin/g++','/usr/bin/as','/usr/bin/ld','/usr/bin/make','/usr/bin/cmake','/usr/bin/time','/usr/bin/readelf','/usr/bin/ldd','/usr/lib/qt6/libexec/moc','/usr/lib/qt6/bin/qmake6']
for name in ['cc1plus','collect2']:programs.append(command(['/usr/bin/g++','-print-prog-name='+name]))
for name in ['crt1.o','Scrt1.o','crti.o','crtn.o','crtbegin.o','crtbeginS.o','crtend.o','crtendS.o','libstdc++.so','libgcc.a','libgcc_s.so','libm.so','libc.so']:
 add(command(['/usr/bin/g++','-print-file-name='+name]))
programs += ['/usr/lib/x86_64-linux-gnu/libQt6Core.so','/usr/lib/x86_64-linux-gnu/libQt6Gui.so','/usr/lib/x86_64-linux-gnu/libQt6Widgets.so','/usr/lib/x86_64-linux-gnu/qt6/plugins/platforms/libqxcb.so']
for path in programs:
 add(path)
 p=subprocess.run(['/usr/bin/ldd',path],text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=20)
 for item in re.findall(r'(/[^\s()]+)',p.stdout):add(item)
for root in [Path('/usr/share/cmake-3.22/Modules'),*Path('/usr/lib/x86_64-linux-gnu/cmake').glob('Qt6*')]:
 for p in root.rglob('*'):
  if p.is_file():add(p)
# Native startup linker scripts name these transitive system inputs explicitly.
for n in ['libc.so.6','libc_nonshared.a','libm.so.6','libmvec.so.1','libmvec_nonshared.a','libpthread_nonshared.a']:
 add(Path('/usr/lib/x86_64-linux-gnu')/n)
out.write_text(json.dumps(dict(files=sorted(map(str,files)),gxx=command(['/usr/bin/g++','--version']).splitlines()[0],qt=command(['/usr/lib/qt6/bin/qmake6','-query','QT_VERSION']),probe=probe,command=cmd),indent=2))
print('native dependency files:',len(files))
