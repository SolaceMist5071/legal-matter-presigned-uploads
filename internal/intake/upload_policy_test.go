package intake

import "testing"

func TestDecideUpload(t *testing.T) {
	tests := []struct {
		name      string
		request   UploadRequest
		wantBytes int64
		wantTTL   int
		wantError bool
	}{
		{"intake PDF", UploadRequest{"matter_123", "asset_001", MatterIntake, "application/pdf"}, 10 << 20, 300, false},
		{"signed document has shorter custody window", UploadRequest{"matter_123", "asset_002", SignedDocument, "application/pdf"}, 25 << 20, 180, false},
		{"deadline evidence is JPEG", UploadRequest{"matter_123", "asset_003", DeadlineEvidence, "image/jpeg"}, 8 << 20, 300, false},
		{"reject mismatched signed document MIME", UploadRequest{"matter_123", "asset_004", SignedDocument, "image/jpeg"}, 0, 0, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := DecideUpload(test.request)
			if (err != nil) != test.wantError {
				t.Fatalf("DecideUpload() error = %v, wantError %v", err, test.wantError)
			}
			if err == nil && (got.MaxBytes != test.wantBytes || got.ExpiresSeconds != test.wantTTL) {
				t.Fatalf("DecideUpload() = bytes %d, ttl %d", got.MaxBytes, got.ExpiresSeconds)
			}
		})
	}
}
