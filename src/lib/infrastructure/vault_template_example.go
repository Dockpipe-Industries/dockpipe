package infrastructure

import (
	"os"
	"path/filepath"
)

const vaultTemplateExample = `# Example vault references for an operator-supplied 1Password item.
# Copy to .env.vault.template and configure secrets.vault_template and vault: op.
# Keep resolved values and private vault templates out of source control.
EXAMPLE_API_TOKEN=op://YourVault/YourItem/token
`

// WriteVaultTemplateExampleIfMissing creates a generic reference-only example.
// An existing operator-authored file is always preserved.
func WriteVaultTemplateExampleIfMissing(projectDir string) error {
	destination := filepath.Join(projectDir, ".env.vault.template.example")
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(vaultTemplateExample)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
