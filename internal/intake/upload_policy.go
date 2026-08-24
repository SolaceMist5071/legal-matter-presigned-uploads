package intake

import (
	"errors"
	"fmt"
	"regexp"
)

type AssetKind string

const (
	MatterIntake     AssetKind = "matter_intake"
	SignedDocument   AssetKind = "signed_document"
	DeadlineEvidence AssetKind = "deadline_followup"
)

type UploadRequest struct {
	MatterID    string    `json:"matter_id"`
	AssetID     string    `json:"asset_id"`
	Kind        AssetKind `json:"kind"`
	ContentType string    `json:"content_type"`
}

type UploadDecision struct {
	Key            string
	ContentType    string
	MaxBytes       int64
	ExpiresSeconds int
}

var safeID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{2,63}$`)

func DecideUpload(req UploadRequest) (UploadDecision, error) {
	if !safeID.MatchString(req.MatterID) || !safeID.MatchString(req.AssetID) {
		return UploadDecision{}, errors.New("matter_id and asset_id must be 3-64 URL-safe characters")
	}

	var allowed string
	var maxBytes int64
	var expiry int
	switch req.Kind {
	case MatterIntake:
		allowed, maxBytes, expiry = "application/pdf", 10<<20, 300
	case SignedDocument:
		allowed, maxBytes, expiry = "application/pdf", 25<<20, 180
	case DeadlineEvidence:
		allowed, maxBytes, expiry = "image/jpeg", 8<<20, 300
	default:
		return UploadDecision{}, errors.New("unknown asset kind")
	}
	if req.ContentType != allowed {
		return UploadDecision{}, fmt.Errorf("%s requires content_type %s", req.Kind, allowed)
	}

	return UploadDecision{
		Key:            fmt.Sprintf("matters/%s/%s/%s", req.MatterID, req.Kind, req.AssetID),
		ContentType:    allowed,
		MaxBytes:       maxBytes,
		ExpiresSeconds: expiry,
	}, nil
}
