package hostdependencies

// Adapters supplies invocation-local host operations. All callbacks are required;
// the caller owns composition and shared package-tool discovery.
type Adapters struct {
	LookupCommand      func(string) (string, error)
	LookupPowerShell   func(string) (string, error)
	RunInstaller       func(string) error
	PackageToolsDir    func(string) string
	BundledCommandPath func(string, string) (string, error)
	TerminalAvailable  func() bool
}
