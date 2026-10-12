package infrastructure

import (
	"os"

	"dockpipe/src/lib/infrastructure/operationrecord"
)

const (
	OperationStatusStart    = operationrecord.OperationStatusStart
	OperationStatusProgress = operationrecord.OperationStatusProgress
	OperationStatusDone     = operationrecord.OperationStatusDone
	OperationStatusFail     = operationrecord.OperationStatusFail
)

type OperationResult = operationrecord.OperationResult

type OperationOptions = operationrecord.OperationOptions

func RunOperation(stderr *os.File, unit, spinnerMessage string, ids map[string]string, fn func() error) error {
	return operationrecord.RunOperation(stderr, unit, spinnerMessage, ids, fn)
}

func RunOperationWithOptions(stderr *os.File, unit, spinnerMessage string, ids map[string]string, options OperationOptions, fn func() error) error {
	return operationrecord.RunOperationWithOptions(stderr, unit, spinnerMessage, ids, options, fn)
}

func LogOperationResult(stderr *os.File, result OperationResult) {
	operationrecord.LogOperationResult(stderr, result)
}

func RunOperationWithResult(stderr *os.File, unit, spinnerMessage string, ids map[string]string, fn func() error) (OperationResult, error) {
	return operationrecord.RunOperationWithResult(stderr, unit, spinnerMessage, ids, fn)
}

func RunOperationWithResultOptions(stderr *os.File, unit, spinnerMessage string, ids map[string]string, options OperationOptions, fn func() error) (OperationResult, error) {
	return operationrecord.RunOperationWithResultOptions(stderr, unit, spinnerMessage, ids, options, fn)
}

func OperationEventFields(result OperationResult) map[string]string {
	return operationrecord.OperationEventFields(result)
}
