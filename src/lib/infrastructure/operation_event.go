package infrastructure

import "dockpipe/src/lib/infrastructure/operationrecord"

const (
	EnvDockpipeEventLog   = operationrecord.EnvDockpipeEventLog
	EnvDockpipeEventIndex = operationrecord.EnvDockpipeEventIndex

	OperationEventSchemaV1 = operationrecord.OperationEventSchemaV1
	OperationEventKind     = operationrecord.OperationEventKind

	OperationEventIndexSchemaV1 = operationrecord.OperationEventIndexSchemaV1
)

type OperationEvent = operationrecord.OperationEvent

type OperationEventIndex = operationrecord.OperationEventIndex

type OperationEventUnitIndex = operationrecord.OperationEventUnitIndex

func OperationEventFromResult(result OperationResult) OperationEvent {
	return operationrecord.OperationEventFromResult(result)
}

func AppendOperationEvent(path string, event OperationEvent) error {
	return operationrecord.AppendOperationEvent(path, event)
}

func ReadOperationEvents(path string) ([]OperationEvent, error) {
	return operationrecord.ReadOperationEvents(path)
}

func BuildOperationEventIndex(path string) (OperationEventIndex, error) {
	return operationrecord.BuildOperationEventIndex(path)
}

func WriteOperationEventIndex(path string, index OperationEventIndex) error {
	return operationrecord.WriteOperationEventIndex(path, index)
}
