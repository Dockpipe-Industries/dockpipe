package dependency

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

const ChangeSchema = "dockpipe.dependency.change/v1"

// ChangeRequest binds a local operator's exact inputs. Provider parameters are
// opaque to Core; only a selected trusted resolver interprets them.
type ChangeRequest struct {
	Schema          string            `json:"schema"`
	Resolver        string            `json:"resolver"`
	ProviderVersion string            `json:"provider_version"`
	Machine         string            `json:"machine"`
	Parameters      map[string]string `json:"parameters"`
}

func (request ChangeRequest) Validate() error {
	if request.Schema != ChangeSchema || !ValidLifecycleIdentifier(request.Resolver) ||
		(request.ProviderVersion == "" || !boundedLifecycleText(request.ProviderVersion, 128)) || request.Machine != "local" ||
		len(request.Parameters) == 0 || len(request.Parameters) > 16 {
		return errors.New("dependency changes require a versioned resolver, local machine and 1..16 parameters")
	}
	for key, value := range request.Parameters {
		if !ValidLifecycleIdentifier(key) || value == "" || !boundedLifecycleText(value, 4096) {
			return errors.New("invalid dependency change parameter")
		}
	}
	return nil
}

// ApprovalID binds the operation as well as all inputs. It is explicit operator
// confirmation, not authentication, a remote grant or a provider trust signature.
func (request ChangeRequest) ApprovalID(operation string) string {
	data, _ := json.Marshal(struct {
		Operation string        `json:"operation"`
		Request   ChangeRequest `json:"request"`
	}{operation, request})
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

type ChangeResult struct {
	Schema  string `json:"schema"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (result ChangeResult) Validate() error {
	if result.Schema != ChangeSchema || !boundedLifecycleText(result.Message, 4096) || result.Message == "" {
		return errors.New("invalid dependency change result")
	}
	switch result.Status {
	case "active-healthy", "activation-recovered", "ready", "installed-activation-pending", "recovered-activation-pending", "recovery-required", "manual-required":
		return nil
	default:
		return errors.New("unknown dependency change outcome")
	}
}
