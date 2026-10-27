package application

import "dockpipe/src/lib/application/internal/workspacegitcmd"

func dispatchCommand(argv []string) (bool, error) {
	switch argv[0] {
	case "init":
		return true, cmdInit(argv[1:])
	case "action":
		return true, cmdAction(argv[1:])
	case "pre":
		return true, cmdPre(argv[1:])
	case "template":
		return true, cmdTemplate(argv[1:])
	case "windows":
		return true, cmdWindows(argv[1:])
	case "workflow":
		return true, cmdWorkflow(argv[1:])
	case "catalog":
		return true, cmdCatalog(argv[1:])
	case "pipelang":
		return true, cmdPipeLang(argv[1:])
	case "doctor":
		return true, cmdDoctor(argv[1:])
	case "test":
		return true, cmdTest(argv[1:])
	case "runs":
		return true, cmdRuns(argv[1:])
	case "result":
		return true, cmdResult(argv[1:])
	case "session":
		return true, cmdSession(argv[1:])
	case "git":
		return true, workspacegitcmd.Run(argv[1:])
	case "remote":
		return true, cmdRemote(argv[1:])
	case "dependency":
		return true, cmdDependency(argv[1:])
	case "install":
		return true, cmdInstall(argv[1:])
	case "package":
		return true, cmdPackage(argv[1:])
	case "build":
		return true, cmdBuild(argv[1:])
	case "clean":
		return true, cmdClean(argv[1:])
	case "rebuild":
		return true, cmdRebuild(argv[1:])
	case "compile":
		return true, cmdPackage(append([]string{"compile"}, argv[1:]...))
	case "clone":
		return true, cmdClone(argv[1:])
	case "release":
		return true, cmdRelease(argv[1:])
	case "core":
		return true, cmdCore(argv[1:])
	case "terraform":
		return true, cmdTerraform(argv[1:])
	case "sdk":
		return true, cmdSDK(argv[1:])
	case "get":
		return true, cmdGet(argv[1:])
	case "scope":
		return true, cmdScope(argv[1:])
	case "__state":
		return true, cmdInternalState(argv[1:])
	default:
		return false, nil
	}
}
