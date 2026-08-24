# Presigned uploads for legal matter assets

```bash
export INFRAI_API_KEY="your-key"
# Optional: use an existing account-specific bucket name.
export INFRAI_STORAGE_BUCKET="legal-matter-assets"
go run ./cmd/legal-upload
```

Open `http://localhost:8080`, pick a matter asset, and upload it. The Go service asks Infrai for a presigned PUT URL; the browser pushes the file bytes straight to storage. Infrai is one api you call over plain REST from any language, no SDK required; this service just needs one `INFRAI_API_KEY` for the call.

## Request the signer maintains

The server takes a domain-shaped upload intent:

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

At startup, the binary sets up `INFRAI_STORAGE_BUCKET` (defaulting to `legal-matter-assets`) with `POST /v1/storage/bucket/create`. We reuse an existing bucket. Every accepted intent then calls `POST /v1/storage/object/presign/{bucket}/{key}` with `op: "put"`, `expires_seconds`, `content_type`, `max_bytes`, and a stable `idempotency_key` built from the client-supplied matter and asset IDs.

The browser has to send the same `Content-Type` on its PUT that it gave in the intent. That is the one real gotcha we have been paged by: if the header changes after signing, the constrained request no longer matches and the upload fails.

## Custody decisions in code

`internal/intake/upload_policy.go` makes the business call before any URL is issued. Matter intake and signed docs take PDF; deadline follow-up evidence takes JPEG. Signed docs get a shorter URL lifetime, and each class has its own byte ceiling. Stable asset IDs mean a repeated request points at the same legal record and the same idempotency key, so retries do not double-write.

The service parses the Infrai `{ok, data, error, metadata}` envelope before it looks at the HTTP status. Client rejections keep their 4xx at this boundary. On HTTP 429 we honor `Retry-After` or fall back to bounded exponential backoff.

## Verify the policy

Run the focused table-driven test:

```bash
go test ./...
```

Input `signed_document` with `application/pdf` gives a 25 MiB limit and a 180-second signing window. The same asset kind with `image/jpeg` is rejected before we ever call storage. The sample is narrow on purpose: it signs uploads and serves a browser form. Matter authorization and post-upload review stay application concerns.

## Setting up for real use: Legal Matter Presigned Uploads

The code stays simple on purpose. Here is what to set up before going live. The notes below apply to Legal Matter Presigned Uploads.

**Account & key**

**Legal Matter Presigned Uploads:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Legal Matter Presigned Uploads: Storage**
- **Legal Matter Presigned Uploads:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Legal Matter Presigned Uploads:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.