"""Bounded pilot worker, invoked only inside the repository's contained runner."""
from pathlib import Path
import argparse, hashlib, json, os, re, shutil, subprocess, time, sys, fcntl, contextlib
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/"containedexec"))
from campaign import InputGuard


def sha(path):
    h=hashlib.sha256()
    with Path(path).open('rb') as f:
        for b in iter(lambda:f.read(1<<20),b''):h.update(b)
    return h.hexdigest()


@contextlib.contextmanager
def sealed(binary, expected):
    """Admit current bytes and execute one sealed, private Linux descriptor."""
    binary=Path(binary);guard=InputGuard([binary],watch_directories=[binary.parent])
    fd=os.memfd_create('pipelang-cpp-pilot',os.MFD_CLOEXEC|os.MFD_ALLOW_SEALING)
    try:
        h=hashlib.sha256()
        with binary.open('rb') as stream:
            for block in iter(lambda:stream.read(1<<20),b''):
                h.update(block);remaining=memoryview(block)
                while remaining:
                    n=os.write(fd,remaining)
                    if n<=0:raise RuntimeError('short executable write')
                    remaining=remaining[n:]
        assert h.hexdigest()==expected,'current executable bytes disagree'
        guard.check();os.fchmod(fd,0o500)
        seals=fcntl.F_SEAL_WRITE|fcntl.F_SEAL_GROW|fcntl.F_SEAL_SHRINK|fcntl.F_SEAL_SEAL
        fcntl.fcntl(fd,fcntl.F_ADD_SEALS,seals)
        assert fcntl.fcntl(fd,fcntl.F_GET_SEALS)&seals==seals
        yield '/proc/self/fd/'+str(fd),fd
        guard.check()
    finally:os.close(fd);guard.close()


def build(config, out):
    source=Path(config['source']);variant=config['variant'];kind=config['kind']
    shutil.copyfile(source/'inputs.bin',out/'inputs.bin')
    shutil.copyfile(source/'expected.json',out/'expected.json')
    if kind=='go':
        shutil.copyfile(source/('traced.go' if variant=='check' else 'generated.go'),out/'impl.go')
        shutil.copyfile(source/'main.go',out/'main.go')
        cmd=[config['go'],'build','-p=1','-o',str(out/'program'),str(out/'impl.go'),str(out/'main.go')]
    else:
        shutil.copyfile(source/('traced.hpp' if variant=='check' else 'generated.hpp'),out/'active.hpp')
        shutil.copyfile(source/'main.cpp',out/'main.cpp')
        cmd=['/usr/bin/g++','-std=c++17','-O2','-g','-march=x86-64','-mtune=generic',str(out/'main.cpp'),'-o',str(out/'program')]
    env=dict(os.environ,GOENV='off',GOTOOLCHAIN='local',GOPROXY='off',GOSUMDB='off',GOCACHE=config['go_cache'])
    started=time.monotonic()
    if kind=='assembly':
        asm=cmd[:-2]+['-S','-o',str(out/'program.s')]
        subprocess.run(asm,env=env,check=True,timeout=25)
        subprocess.run(['/usr/bin/g++','-c',str(out/'program.s'),'-o',str(out/'program.o')],env=env,check=True,timeout=25)
        subprocess.run(['/usr/bin/g++',str(out/'program.o'),'-o',str(out/'program')],env=env,check=True,timeout=25)
    else:subprocess.run(cmd,env=env,check=True,timeout=25)
    elapsed=time.monotonic()-started
    sections=subprocess.check_output(['/usr/bin/readelf','-S',str(out/'program')],text=True)
    assert '.debug_info' in sections,'debug sections missing'
    dep=subprocess.run(['/usr/bin/ldd',str(out/'program')],text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
    closure={}
    for line in dep.stdout.splitlines():
        for path in re.findall(r'(/[^\s()]+)',line):
            p=Path(path).resolve()
            if p.is_file():closure[str(p)]=dict(bytes=p.stat().st_size,sha256=sha(p))
    (out/'build.json').write_text(json.dumps(dict(command=cmd,elapsed_s=elapsed,executable_bytes=(out/'program').stat().st_size,executable_sha256=sha(out/'program'),runtime_closure=closure,debug_info=True),indent=2))


def execute(config,out):
    expected=json.loads(Path(config['expected']).read_text());rows=[]
    repetitions=config.get('repetitions',1)
    for iteration in range(repetitions):
        variants=list(config['binaries'])
        if iteration%2:variants.reverse()
        for label in variants:
            artifact=config['binaries'][label];binary=Path(artifact['path'])
            assert sha(binary)==artifact['sha256'],'executable drift before fresh execution'
            fixture=out/(str(iteration)+'-'+label+'.bin');shutil.copyfile(config['fixture'],fixture)
            stdout=out/(str(iteration)+'-'+label+'.stdout');usage=out/(str(iteration)+'-'+label+'.usage')
            start=time.monotonic()
            with stdout.open('wb') as f, sealed(binary,artifact['sha256']) as (executable,fd):
                result=subprocess.run(['/usr/bin/time','-f','{"user_s":%U,"system_s":%S,"rss_kib":%M}','-o',str(usage),executable,str(fixture)],stdout=f,stderr=subprocess.PIPE,timeout=5,pass_fds=(fd,))
            elapsed=time.monotonic()-start
            assert result.returncode==0,(label,result.stderr.decode(errors='replace')[-500:])
            observed=stdout.read_text().splitlines()
            wanted=expected if artifact['variant']=='check' else [x.split('|')[0]+'|' for x in expected]
            assert observed==wanted,(label,'independent Value/Trace disagreement',observed[:2],wanted[:2])
            assert sha(binary)==artifact['sha256'],'executable drift after fresh execution'
            rows.append(dict(iteration=iteration,label=label,elapsed_s=elapsed,usage=json.loads(usage.read_text()),stdout_sha256=sha(stdout),fixture_sha256=sha(fixture),vectors=len(observed),sealed=True))
    (out/'execution.json').write_text(json.dumps(rows,indent=2))


def qt(config,out):
    binary=Path(config['binary']);expected=sha(binary)
    with sealed(binary,expected) as (executable,fd):
        result=subprocess.run([executable,str(out)],env=dict(os.environ,DISPLAY=config['display'],QT_QPA_PLATFORM='xcb'),pass_fds=(fd,),timeout=5)
    assert result.returncode==0,'Qt event/ownership proof'


def negatives(config,out):
    """Sixteen malformed host fixtures, independently enumerated here."""
    import struct
    u32=lambda n:struct.pack('<I',n)
    malformed=[('C1',b''),('C1',u32(1001)),('C1',u32(1)+b'\xff'),('C1',u32(1)+b'\0\2\0'),('C1',u32(1)+b'\0\0\2'),('C1',u32(0)+b'extra'),('A1',u32(1)+b'\1'+u32((1<<20)+1)),('A2',u32(1)+b'\0'+u32(4097))]
    for raw in [b'\x80',b'\xc0\xaf',b'\xed\xa0\x80',b'\xf4\x90\x80\x80',b'\xe2\x82',b'\xff',b'\xe0\x80\x80',b'\xf0\x80\x80\x80']:
        # A1 Create: an invalid UTF-8 string followed by a canonical Bool.
        malformed.append(('A1',u32(1)+b'\1'+u32(len(raw))+raw+b'\0'))
    rows=[]
    for i,(case,data) in enumerate(malformed):
        fixture=out/(str(i)+'.bin');fixture.write_bytes(data)
        for label,artifact in config['binaries'][case].items():
            with sealed(artifact['path'],artifact['sha256']) as (executable,fd):
                p=subprocess.run([executable,str(fixture)],stdout=subprocess.PIPE,stderr=subprocess.PIPE,pass_fds=(fd,),timeout=5)
            assert p.returncode==3,(i,label,p.returncode)
            rows.append(dict(case=i,program=case,backend=label,exit=p.returncode))
    (out/'negatives.json').write_text(json.dumps(rows,indent=2))


def main():
    parser=argparse.ArgumentParser();parser.add_argument('action',choices=['build','execute','negatives','qt']);parser.add_argument('config',type=Path);parser.add_argument('output',type=Path);args=parser.parse_args()
    # The outer run.py enforces the complete process tree; never use this as a standalone launcher.
    if not os.environ.get('CONTAINED_JOB_SLICE'):raise RuntimeError('enclosing contained job required')
    args.output.mkdir(exist_ok=True)
    globals()[args.action](json.loads(args.config.read_text()),args.output)

if __name__=='__main__':main()
