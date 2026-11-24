// Package nativeinvocation supplies attribution asserted by a native entrypoint.
// It does not authenticate a remote caller, issue grants, or evaluate policy.
package nativeinvocation

import (
	"os"
	"strconv"

	"dockpipe/src/lib/domain/invocation"
)

var processIdentity = invocation.Identity{
	Issuer:  "dockpipe.native-process",
	Subject: strconv.Itoa(os.Getpid()),
}

// Local is used only by trusted local composition. A remote adapter must supply
// its verified identity and exact remote machine target instead of calling Local.
func Local() (invocation.Target, invocation.Identity) {
	return invocation.Target{Kind: invocation.Local, ID: "this-computer"}, processIdentity
}
