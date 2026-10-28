package remote

import (
	"encoding/json"
	"net/http"
)

const operatorResponseHeader = "X-Dockpipe-Operator-Signature"

func (broker *Broker) serveOperator(w http.ResponseWriter, request *http.Request, body []byte, admission operatorAdmission, authorizationError error) {
	if broker.security == nil || admission.nonce == "" || authorizationError != nil {
		http.Error(w, "OS-authenticated remote access is locked or unavailable", http.StatusLocked)
		return
	}
	if err := broker.security.CheckAdmission(admission); err != nil {
		http.Error(w, "invalid, locked or expired remote authority", http.StatusLocked)
		return
	}
	value, operationError := broker.operator(request.URL.Path, body)
	status := http.StatusOK
	payload, err := json.Marshal(value)
	if operationError != nil {
		status = http.StatusConflict
		payload = []byte("remote operation rejected\n")
	} else if err != nil {
		status = http.StatusInternalServerError
		payload = []byte("remote response encoding failed\n")
	}
	signature, err := broker.state.Identity.Sign(machineMessage("operator-response", admission.nonce, request.URL.Path, status, payload))
	if err != nil {
		http.Error(w, "remote response signing failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set(operatorResponseHeader, signature)
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}
