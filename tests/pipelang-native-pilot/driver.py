"""Receipt-backed native pilot; no compiler/backend selection in DockPipe's engine."""
from pathlib import Path
import argparse, hashlib, json, os, sys, subprocess, threading
R=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(R/'tests/containedexec'))
from campaign import Campaign,InputGuard,file_identity,atomic_json,resource_accepted,fingerprint
from verification import StageRunner,source_paths
from job import verify_job
from estate import CampaignBudget


def main():
    ap=argparse.ArgumentParser();ap.add_argument('--prepared',type=Path,required=True);ap.add_argument('--output',type=Path,required=True);ap.add_argument('--cache',type=Path,required=True);ap.add_argument('--go',type=Path,required=True);ap.add_argument('--roots',type=Path,required=True);ap.add_argument('--mode',choices=['fresh','resume'],default='fresh');ap.add_argument('--interrupt-after',type=int,default=0);ap.add_argument('--repetitions',type=int,default=1);a=ap.parse_args()
    verify_job();out=a.output.resolve();out.mkdir(exist_ok=True,mode=0o700);(out/'storage').mkdir(exist_ok=True,mode=0o700)
    roots=[Path(x) for x in json.loads(a.roots.read_text())]
    # The preparation inventory includes the complete task tree and shared cache.
    watch=set()
    # Guard include-search directories during discovery and later compilation.
    for root in [Path('/usr/include'),Path('/usr/lib/gcc'),Path('/usr/lib/qt6'),Path('/usr/lib/x86_64-linux-gnu/cmake')]:
        for directory,dirs,files in os.walk(root):watch.add(Path(directory))
    watch.update([Path('/usr/bin'),Path('/usr/lib/x86_64-linux-gnu')])
    pre=InputGuard([],watch_directories=watch)
    with CampaignBudget(roots,out/'storage',96<<30,population_roots=[out,a.cache]) as budget:
        # Reuse the existing coordinator reclaim policy while hashing content.
        stop=threading.Event()
        def reclaim():
            while not stop.wait(.1):budget.memory.check()
        thread=threading.Thread(target=reclaim,daemon=True);thread.start()
        producer=out/('producer-'+str(os.getpid()));producer.mkdir(mode=0o700)
        source_guard=InputGuard(source_paths())
        def producer_unit(name,cmd):
            prefix=producer/name
            with (producer/(name+'.runner.log')).open('w') as log:
                rc=subprocess.run([sys.executable,'-B',str(R/'tests/containedexec/run.py'),'--output',str(prefix),'--cache',str(a.cache),'--timeout','30','--memory-high-mib','700','--',*cmd],cwd=R,stdout=log,stderr=subprocess.STDOUT).returncode
            assert rc==0 and resource_accepted(json.loads(Path(str(prefix)+'.json').read_text())),name
        producer_unit('build',[str(a.go),'test','-p=1','-c','-o',str(producer/'pilot.test'),'./src/lib/pipelang'])
        producer_unit('export',['env','PIPELANG_CPP_PILOT_OUTPUT='+str(producer/'corpus'),str(producer/'pilot.test'),'-test.run','^TestCPPPilot(Export|CoreRefusals)$','-test.v','-test.timeout=25s'])
        def inventory(root):return {str(p.relative_to(root)):file_identity(p) for p in root.rglob('*') if p.is_file()}
        fresh=inventory(producer/'corpus');assert fresh==inventory(a.prepared),'fresh frontend/Core/oracle/emitter output disagrees with frozen corpus'
        source_guard.check();source_guard.close()
        atomic_json(producer/'acceptance.json',dict(source_identity=source_guard.identity,artifacts=fresh,binary=file_identity(producer/'pilot.test')))
        source_files=[p.resolve() for p in a.prepared.rglob('*') if p.is_file()]
        source_files += [p.resolve() for p in (R/'tests/pipelang-native-pilot').rglob('*') if p.is_file() and '__pycache__' not in p.parts]
        discovery=out/('toolchain-'+str(os.getpid())+'.json')
        with (out/'discovery.log').open('w') as log:
            rc=subprocess.run([sys.executable,'-B',str(R/'tests/containedexec/run.py'),'--output',str(out/('discovery-'+str(os.getpid()))),'--cache',str(a.cache),'--timeout','30','--memory-high-mib','700','--',sys.executable,'-B',str(R/'tests/pipelang-native-pilot/toolchain.py'),str(discovery)],stdout=log,stderr=subprocess.STDOUT).returncode
        assert rc==0 and resource_accepted(json.loads((out/('discovery-'+str(os.getpid())+'.json')).read_text())),'toolchain discovery'
        pre.check()
        tool_files=[Path(p) for p in json.loads(discovery.read_text())['files']]
        guard=InputGuard(source_paths()+source_files+tool_files+[a.go.parent.parent],settings=dict(capability='pipelang.cpp.pilot.v1',repetitions=a.repetitions),watch_directories=watch)
        pre.check();pre.close()
        try:
            with Campaign(out/'campaign',mode=a.mode) as campaign:
                cases=sorted(p.name for p in a.prepared.iterdir() if p.is_dir())
                stages=[c+'/'+kind+'-'+variant for c in cases for kind,variant in [('go','app'),('cpp','app'),('go','check'),('cpp','check'),('assembly','app')]]+[c+'/execute' for c in cases]+['host-negatives']
                runner=StageRunner(campaign,'pilot',dict(digest=guard.check()),stages,a.cache,lambda:dict(digest=guard.check()))
                atomic_json(out/'protocol.json',dict(cases=cases,stages=stages,repetitions=a.repetitions,source_identity=guard.identity))
                rows=[];built={};count=0
                def run(label,action,config):
                    nonlocal count
                    def command(directory):
                        path=directory/'config.json';atomic_json(path,config)
                        return [sys.executable,'-B',str(R/'tests/pipelang-native-pilot/worker.py'),action,str(path),str(directory/'artifacts')]
                    row=runner.run([label],command,artifacts=lambda d:[p for p in (d/'artifacts').rglob('*') if p.is_file()]+[d/'config.json'])
                    assert row['exit']==0,label
                    rows.append(row);count+=1;print(label+': passed'+(' (resumed)' if row.get('resumed') else ''),flush=True)
                    atomic_json(out/'progress.json',rows)
                    budget.check()
                    if a.interrupt_after and count==a.interrupt_after:os._exit(19)
                    return Path(row['directory'])/'artifacts'
                for case in cases:
                    built[case]={}
                    for kind,variant in [('go','app'),('cpp','app'),('go','check'),('cpp','check'),('assembly','app')]:
                        label=kind+'-'+variant
                        directory=run(case+'/'+label,'build',dict(source=str(a.prepared/case),kind=kind,variant=variant,go=str(a.go),go_cache=str(a.cache)))
                        built[case][label]=dict(path=str(directory/'program'),sha256=file_identity(directory/'program')['sha256'],variant=variant)
                for case in cases:run(case+'/execute','execute',dict(binaries=built[case],fixture=str(a.prepared/case/'inputs.bin'),expected=str(a.prepared/case/'expected.json'),repetitions=a.repetitions))
                run('host-negatives','negatives',dict(binaries=built))
                atomic_json(out/'result.json',dict(status='passed',reconciliation=runner.finish(),storage=budget.check(),source_identity=guard.identity,rows=rows))
        finally:guard.close();stop.set();thread.join()

if __name__=='__main__':main()
