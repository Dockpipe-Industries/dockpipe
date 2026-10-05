package process

import "os/exec"

// RunNested is for a helper inside an already owned command tree. On Unix it
// must preserve the inherited group: creating another group would let children
// survive cancellation of the outer resolver. Its caller must finish promptly
// after return so the owner can clean up all remaining descendants.
func RunNested(command *exec.Cmd) error {
	if inheritedTree() {
		return command.Run()
	}
	return Run(command)
}
