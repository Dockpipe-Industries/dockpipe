#!/usr/bin/env python3
import argparse,hashlib,json,os,subprocess
from pathlib import Path
parser=argparse.ArgumentParser(description='Capture exact Linux/GCC reconstruction inputs for the bounded shared-runtime experiment')
parser.add_argument('--root', type=Path, required=True)
parser.add_argument('--go', type=Path, required=True)
parser.add_argument('--output', type=Path, required=True)
args=parser.parse_args()
r=args.root.resolve(); go=args.go.resolve()
if args.output.exists(): raise RuntimeError('fresh settings path required')
def digest(p): return hashlib.file_digest(p.open('rb'),'sha256').hexdigest() if hasattr(hashlib,'file_digest') else hashlib.sha256(p.read_bytes()).hexdigest()
paths=[]
for part in ['bin','pkg/tool','src','go.env','VERSION']:
 p=go.parent.parent/part
 paths.extend(f for f in (p.rglob('*') if p.is_dir() else [p]) if f.is_file() and not f.name.endswith('_test.go'))
gccdir=Path(subprocess.check_output(['gcc','-print-libgcc-file-name'],text=True).strip()).resolve().parent
for part in [gccdir,'/usr/include']:
 paths.extend(p.resolve() for p in Path(part).rglob('*') if p.is_file())
paths += [Path(p).resolve() for p in ['/usr/bin/gcc','/usr/bin/ld','/usr/bin/as','/lib/x86_64-linux-gnu/libc.so.6','/lib64/ld-linux-x86-64.so.2','/lib/x86_64-linux-gnu/libm.so.6','/lib/x86_64-linux-gnu/libz.so.1','/lib/x86_64-linux-gnu/libzstd.so.1','/lib/x86_64-linux-gnu/libsframe.so.0'] if Path(p).exists()]
for name in ['crt1.o','Scrt1.o','crti.o','crtn.o','libc.so','libc_nonshared.a']:
 paths.append(Path(subprocess.check_output(['gcc','-print-file-name='+name],text=True).strip()).resolve())
env={k:os.getenv(k,'') for k in ['GOEXPERIMENT','CGO_CFLAGS','CGO_CPPFLAGS','CGO_CXXFLAGS','CGO_LDFLAGS','GODEBUG','GOFIPS140','GO_EXTLINK_ENABLED','GO_LDSO']}
env.update(GOENV='off',GO111MODULE='off',GOPATH=str(r/'gopath'),GOBIN=str(r/'gopath/bin'),GOFLAGS='',GOTOOLCHAIN='local',GOPROXY='off',GOSUMDB='off',GOWORK='off',GOMAXPROCS='4',GOCACHE=str(r/'cache'),CC='/usr/bin/gcc',CXX='/usr/bin/g++',GOOS='linux',GOARCH='amd64',GOAMD64='v1',CGO_ENABLED='1')
value=dict(go=str(go),environment=env,toolchain={str(p):digest(p) for p in sorted(set(paths))},shared_libraries={str(p):digest(p) for p in sorted((r/'std-pkg').glob('*.so'))})
args.output.write_text(json.dumps(value,sort_keys=True,indent=2)+'\n')
print('toolchain files',len(value['toolchain']),'exports',len(list((r/'exports').iterdir())))
