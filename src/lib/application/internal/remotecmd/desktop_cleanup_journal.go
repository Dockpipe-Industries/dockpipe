package remotecmd

import (
	"context"
	"errors"
	"os"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

// The desktop worker lock owns one pending proof for this worker. The record
// contains only the exact final report and a non-advertising catalog.
type desktopCleanupJournal string

func (journal desktopCleanupJournal) save(exchange contract.DesktopExchange) error {
	if len(exchange.Reports) != 1 || exchange.Reports[0].State != "closed" {
		return nil
	}

	exchange.Catalog.View = false
	exchange.Catalog.Input = false
	exchange.Catalog.AdministratorInput = false
	exchange.Catalog.Displays = nil
	if !validDesktopCleanupJournal(exchange) {
		return errors.New("cannot retain an invalid native desktop close report")
	}

	return remoteio.WritePrivate(string(journal), exchange)
}

func validDesktopCleanupJournal(exchange contract.DesktopExchange) bool {
	if exchange.Schema != contract.DesktopVersion || exchange.Catalog.Validate() != nil ||
		exchange.Catalog.View || exchange.Catalog.Input || exchange.Catalog.AdministratorInput ||
		len(exchange.Reports) != 1 {
		return false
	}

	report := exchange.Reports[0]
	return report.State == "closed" && len(report.Signals) == 0 && report.Reference.Validate() == nil &&
		report.Reference.Target == exchange.Catalog.Target && report.Reference.Epoch == exchange.Epoch &&
		report.Ack <= contract.MaxDesktopSignals
}

func (journal desktopCleanupJournal) clear() error {
	err := os.Remove(string(journal))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (journal desktopCleanupJournal) recover(ctx context.Context, client desktopWorkerExchangeClient,
	catalog contract.DesktopCatalog) error {
	var saved contract.DesktopExchange
	if err := remoteio.ReadPrivate(string(journal), &saved); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !validDesktopCleanupJournal(saved) || saved.Catalog.Target.Node != catalog.Target.Node ||
		saved.Catalog.Target.Fingerprint != catalog.Target.Fingerprint || saved.Catalog.Provider != catalog.Provider {
		return errors.New("saved native desktop close report does not match the paired worker and provider")
	}

	// An empty epoch asks for the current broker lifetime without publishing
	// readiness or replaying any session. A previous lifetime has no record to close.
	probe := contract.DesktopExchange{Schema: contract.DesktopVersion, Catalog: saved.Catalog}
	current, err := client.ExchangeDesktop(ctx, probe)
	if err != nil {
		return err
	}
	if current.Epoch != saved.Epoch {
		return journal.clear()
	}

	response, err := client.ExchangeDesktop(ctx, saved)
	if err != nil {
		return err
	}
	if !desktopCleanupAcknowledged(response, saved.Reports[0]) {
		return errors.New("saved native desktop close report has no exact broker acknowledgment")
	}

	return journal.clear()
}
