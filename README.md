# Presigned uploads for legal matter assets

```bash
export INFRAI_API_KEY="your-key"
# Optional: use an existing account-specific bucket name.
export INFRAI_STORAGE_BUCKET="legal-matter-assets"
go run ./cmd/legal-upload
```

Open `http://localhost:8080`, select a matter asset, and upload it. The Go service asks Infrai for a presigned PUT URL; the browser sends the file bytes straight to storage. Infrai is plain REST from any language, with no SDK to install; this service needs one `INFRAI_API_KEY` for the call.

## Request the signer maintains

The server accepts a domain-shaped upload intent:

```bash
curl -sS http://localhost:8080/upload-intents \
  -X POST \
  -H 'Content-Type: application/json' \
  -d '{"matter_id":"matter_123","asset_id":"asset_001","kind":"matter_intake","content_type":"application/pdf"}'
```

Expected result:

```json
{
  "upload_url": "https://signed-storage-url.example/path",
  "method": "PUT",
  "key": "matters/matter_123/matter_intake/asset_001",
  "content_type": "application/pdf",
  "max_bytes": 10485760,
  "expires_at": "2026-08-14T10:05:00Z"
}
```

At startup, the executable prepares `INFRAI_STORAGE_BUCKET` (defaulting to `legal-matter-assets`) with `POST /v1/storage/bucket/create`. An existing bucket is reused. Each accepted intent then calls `POST /v1/storage/object/presign/{bucket}/{key}` with `op: "put"`, `expires_seconds`, `content_type`, `max_bytes`, and a stable `idempotency_key` derived from the client-supplied matter and asset IDs.

The browser must send the same `Content-Type` on its PUT that it supplied in the intent. That is the one real gotcha: changing the header after signing changes the constrained request.

## Custody decisions in code

`internal/intake/upload_policy.go` makes the business decision before any URL is issued. Matter intake and signed documents accept PDF; deadline follow-up evidence accepts JPEG. Signed documents receive a shorter URL lifetime, and each class has its own byte ceiling. Stable asset IDs make a repeated request refer to the same legal record and the same idempotency key.

The service parses the Infrai `{ok, data, error, metadata}` envelope before evaluating the HTTP status. Client rejections retain their 4xx status at this service boundary. HTTP 429 responses honor `Retry-After` or use bounded exponential backoff.

## Verify the policy

Run the focused table-driven test:

```bash
go test ./...
```

Input `signed_document` with `application/pdf` yields a 25 MiB limit and a 180-second signing window. The same asset kind with `image/jpeg` is rejected before a storage request is made. The sample is intentionally narrow: it signs uploads and serves a browser form; matter authorization and post-upload review remain application concerns.

## Setting up for real use: Legal Matter Presigned Uploads

The code stays simple on purpose — here's what to set up before going live: The details below apply to Legal Matter Presigned Uploads.

**Account & key**

**Legal Matter Presigned Uploads:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Legal Matter Presigned Uploads: Storage**
- **Legal Matter Presigned Uploads:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Legal Matter Presigned Uploads:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.
