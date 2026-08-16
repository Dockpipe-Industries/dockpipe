//go:build linux || windows

package sqliteevidence

import (
	"context"
	"testing"
)

func runPublicationCohortCycles(
	t *testing.T,
	ctx context.Context,
	writer, reader *publicationChild,
	databasePath string,
	initialRow aggregateRow,
) (aggregateRow, int, int, int, int) {
	t.Helper()

	oldRow := initialRow
	oldReads := 0
	busyResults := 0
	newReads := 0
	protectedJournals := 0

	for cycle := 1; cycle <= publicationCohortCycles; cycle++ {
		if err := ctx.Err(); err != nil {
			t.Fatalf("publication cohort deadline before cycle %d: %v", cycle, err)
		}

		oldResponse, err := reader.exchange(publicationCommand{Cycle: cycle, Operation: "read_old"})
		if err != nil {
			t.Fatalf("cycle %d old-reader protocol: %v", cycle, err)
		}
		requirePublicationResponse(t, oldResponse, cycle, "read_old", "row", oldRow)
		oldReads++

		newRow := publicationRow(oldRow.Revision + 1)
		stageResponse, err := writer.exchange(publicationCommand{Cycle: cycle, Operation: "stage"})
		if err != nil {
			t.Fatalf("cycle %d writer-stage protocol: %v", cycle, err)
		}
		requirePublicationResponse(t, stageResponse, cycle, "stage", "staged", newRow)

		busyResponse, err := reader.exchange(publicationCommand{Cycle: cycle, Operation: "expect_busy"})
		if err != nil {
			t.Fatalf("cycle %d live-owner reader protocol: %v", cycle, err)
		}
		if busyResponse.Cycle != cycle || busyResponse.Operation != "expect_busy" || busyResponse.Status != "busy_or_locked" || (busyResponse.SQLiteCode != 5 && busyResponse.SQLiteCode != 6) {
			t.Fatalf("cycle %d live-owner response mismatch: %+v", cycle, busyResponse)
		}
		busyResults++

		if _, err := requirePublicationJournal(databasePath); err != nil {
			t.Fatalf("cycle %d live journal: %v", cycle, err)
		}
		protectedJournals++

		releaseResponse, err := writer.exchange(publicationCommand{Cycle: cycle, Operation: "commit"})
		if err != nil {
			t.Fatalf("cycle %d writer-commit protocol: %v", cycle, err)
		}
		requirePublicationResponse(t, releaseResponse, cycle, "commit", "released", newRow)

		newResponse, err := reader.exchange(publicationCommand{Cycle: cycle, Operation: "read_new"})
		if err != nil {
			t.Fatalf("cycle %d new-reader protocol: %v", cycle, err)
		}
		requirePublicationResponse(t, newResponse, cycle, "read_new", "row", newRow)
		newReads++
		oldRow = newRow
	}

	return oldRow, oldReads, busyResults, newReads, protectedJournals
}
