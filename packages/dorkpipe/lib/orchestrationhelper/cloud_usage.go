package orchestrationhelper

import (
	"errors"
	"fmt"
	"io"
	"os"

	"dorkpipe.orchestrator/orchestrationhelper/internal/cloudusage"
)

func runUsageCommand(args []string, stdout io.Writer) error {
	var path, provider, field string
	switch args[0] {
	case "usage-number":
		if len(args) != 3 {
			return errors.New("usage: orchestrate-helper usage-number <cloud-usage.json> <key>")
		}
		path, field = args[1], args[2]
	case "provider-usage-number":
		if len(args) != 4 {
			return errors.New("usage: orchestrate-helper provider-usage-number <cloud-usage.json> <provider> <field>")
		}
		path, provider, field = args[1], args[2], args[3]
	}
	value, err := readUsageNumber(path, provider, field)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, value)
	return err
}

// Budget decisions must not treat missing or damaged accounting as zero usage.
func readUsageNumber(path, provider, field string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read cloud usage: %w", err)
	}
	ledger, err := cloudusage.Parse(data)
	if err != nil {
		return 0, err
	}
	if provider != "" {
		return ledger.ProviderNumber(provider, field)
	}
	return ledger.Number(field)
}
