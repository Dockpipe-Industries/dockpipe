package remote

const MaxLiveLog = 64 << 10

// LiveLog is a bounded preview of a running workflow. The result remains the
// authoritative log after completion.
type LiveLog struct {
	Data      []byte `json:"data,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

type LiveLogUpdate struct {
	ID      string  `json:"id"`
	Session string  `json:"session"`
	Log     LiveLog `json:"log"`
}

type LiveLogView struct {
	ID     string  `json:"id"`
	Status string  `json:"status"`
	Log    LiveLog `json:"log"`
}
