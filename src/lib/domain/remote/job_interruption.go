package remote

import "errors"

const JobInterruptionInspectPath = "/v1/job-interruption-inspect"
const JobInterruptionAcknowledgePath = "/v1/job-interruption-acknowledge"
const InactiveGuestWithoutResult = "inactive_guest_without_worker_result"

type JobInterruptionRequest struct {
	ID          string `json:"id"`
	Node        string `json:"node"`
	ReviewToken string `json:"review_token,omitempty"`
}

func (request JobInterruptionRequest) Validate(acknowledge bool) error {
	if !ValidID(request.ID) || !ValidID(request.Node) {
		return errors.New("job interruption requires an exact --id and --node")
	}
	if acknowledge && !featureDigest(request.ReviewToken) {
		return errors.New("job interruption acknowledgement requires the reviewed --review-token")
	}
	if !acknowledge && request.ReviewToken != "" {
		return errors.New("job interruption inspection does not accept a review token")
	}
	return nil
}

type JobInterruptionReview struct {
	ID           string `json:"id"`
	Node         string `json:"node"`
	Status       string `json:"status"`
	ResultHash   string `json:"result_hash"`
	Evidence     string `json:"evidence,omitempty"`
	ReviewToken  string `json:"review_token"`
	Acknowledged bool   `json:"acknowledged"`
}

func (review JobInterruptionReview) Validate(request JobInterruptionRequest, acknowledge bool) error {
	hasResult := review.Evidence == "" && featureDigest(review.ResultHash)
	missingGuestResult := review.Evidence == InactiveGuestWithoutResult && review.ResultHash == ""
	if review.ID != request.ID || review.Node != request.Node || review.Status != "unknown" ||
		(!hasResult && !missingGuestResult) || !featureDigest(review.ReviewToken) {
		return errors.New("job interruption review changed identity or outcome")
	}
	if acknowledge && (!review.Acknowledged || review.ReviewToken != request.ReviewToken) {
		return errors.New("the broker did not acknowledge the reviewed interruption")
	}
	return nil
}
