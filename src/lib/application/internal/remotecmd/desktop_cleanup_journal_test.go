package remotecmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

type desktopCleanupExchangeFunc func(context.Context, contract.DesktopExchange) (contract.DesktopExchangeResult, error)

func (exchange desktopCleanupExchangeFunc) ExchangeDesktop(ctx context.Context, request contract.DesktopExchange) (contract.DesktopExchangeResult, error) {
	return exchange(ctx, request)
}

func desktopCleanupTestJournal(t *testing.T) desktopCleanupJournal {
	t.Helper()

	root := filepath.Join(t.TempDir(), "remote-state")
	if err := remoteio.PrivateDirectory(root); err != nil {
		t.Fatal(err)
	}

	return desktopCleanupJournal(filepath.Join(root, "close.json"))
}

func TestDesktopCleanupJournalSurvivesLostAcknowledgmentAndBridgeRestart(t *testing.T) {
	intent := desktopTestIntent()
	catalog := contract.DesktopCatalog{
		Schema: contract.DesktopVersion, Target: intent.Target, Provider: intent.Provider, View: true, Input: true,
		Displays: []contract.DesktopDisplay{{Source: "external-display", Width: 3840, Height: 2160}},
	}
	report := contract.DesktopReport{Reference: intent.Reference(), State: "closed", Ack: 1}
	previous := contract.DesktopExchange{
		Schema: contract.DesktopVersion, Epoch: intent.Epoch, Catalog: catalog, Reports: []contract.DesktopReport{report},
	}
	journal := desktopCleanupTestJournal(t)
	if err := journal.save(previous); err != nil {
		t.Fatal(err)
	}

	lost := errors.New("cleanup response lost after publication")
	attempts := 0
	client := desktopCleanupExchangeFunc(func(_ context.Context, request contract.DesktopExchange) (contract.DesktopExchangeResult, error) {
		response := contract.DesktopExchangeResult{Epoch: intent.Epoch}
		if request.Catalog.View || request.Catalog.Input || request.Catalog.AdministratorInput ||
			len(request.Catalog.Displays) != 0 || request.Catalog.Target != intent.Target {
			t.Fatalf("recovery changed the original target or advertised access: %+v", request.Catalog)
		}
		if request.Epoch == "" && len(request.Reports) == 0 {
			return response, nil
		}
		if request.Epoch != intent.Epoch || len(request.Reports) != 1 || request.Reports[0].Reference != report.Reference ||
			request.Reports[0].State != report.State || request.Reports[0].Ack != report.Ack || len(request.Reports[0].Signals) != 0 {
			t.Fatalf("recovery changed the exact native close report: %+v", request)
		}

		attempts++
		if attempts == 1 {
			return response, lost
		}
		response.Forget = []contract.DesktopReference{intent.Reference()}
		return response, nil
	})

	if err := journal.recover(context.Background(), client, catalog); !errors.Is(err, lost) {
		t.Fatalf("recovery did not return the lost acknowledgment: %v", err)
	}
	restarted := catalog
	restarted.Target.Generation = strings.Repeat("f", 64)
	if err := journal.recover(context.Background(), client, restarted); err != nil {
		t.Fatalf("a fresh bridge could not recover the saved report: %v", err)
	}
	if _, err := os.Stat(string(journal)); !errors.Is(err, os.ErrNotExist) || attempts != 2 {
		t.Fatalf("cleanup proof was not retired after the acknowledgment: attempts=%d, err=%v", attempts, err)
	}
}
