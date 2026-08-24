package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"example.com/legal-browser-uploads/internal/infrai"
	"example.com/legal-browser-uploads/internal/intake"
)

const defaultBucketName = "legal-matter-assets"

type service struct {
	storage    *infrai.Client
	bucketName string
}

type uploadResponse struct {
	UploadURL   string `json:"upload_url"`
	Method      string `json:"method"`
	Key         string `json:"key"`
	ContentType string `json:"content_type"`
	MaxBytes    int64  `json:"max_bytes"`
	ExpiresAt   string `json:"expires_at"`
}

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := infrai.New(apiKey)
	bucketName := os.Getenv("INFRAI_STORAGE_BUCKET")
	if bucketName == "" {
		bucketName = defaultBucketName
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := client.CreateBucket(ctx, bucketName); err != nil && !isBucketExists(err) {
		log.Fatalf("prepare asset bucket: %v", err)
	}

	svc := &service{storage: client, bucketName: bucketName}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /upload-intents", svc.createUploadIntent)
	mux.Handle("GET /", http.FileServer(http.Dir("web")))
	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("legal upload service listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func (s *service) createUploadIntent(w http.ResponseWriter, r *http.Request) {
	var input intake.UploadRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	decision, err := intake.DecideUpload(input)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}

	result, err := s.storage.PresignPut(r.Context(), s.bucketName, decision.Key, infrai.PresignInput{
		Op:             "put",
		ExpiresSeconds: decision.ExpiresSeconds,
		ContentType:    decision.ContentType,
		MaxBytes:       decision.MaxBytes,
		IdempotencyKey: fmt.Sprintf("upload-%s-%s", input.MatterID, input.AssetID),
	})
	if err != nil {
		var apiErr *infrai.APIError
		if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
			writeJSON(w, apiErr.Status, map[string]string{"error": apiErr.Message})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "storage request could not be completed"})
		return
	}

	writeJSON(w, http.StatusCreated, uploadResponse{
		UploadURL: result.URL, Method: http.MethodPut, Key: decision.Key,
		ContentType: decision.ContentType, MaxBytes: decision.MaxBytes,
		ExpiresAt: time.Now().UTC().Add(time.Duration(decision.ExpiresSeconds) * time.Second).Format(time.RFC3339),
	})
}

func isBucketExists(err error) bool {
	var apiErr *infrai.APIError
	return errors.As(err, &apiErr) && apiErr.Code == "STORAGE_BUCKET_EXISTS"
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
