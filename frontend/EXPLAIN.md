# RegiKeep — Backend Configuration & Architecture Guide

This document explains **every configuration, environment variable, edge function, API endpoint, request/response format, and integration point** required to run the RegiKeep backend.

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Environment Variables](#2-environment-variables)
3. [Frontend Configuration](#3-frontend-configuration)
4. [Edge Functions Reference](#4-edge-functions-reference)
   - 4.1 [registry-images](#41-registry-images)
   - 4.2 [audit](#42-audit)
   - 4.3 [keepalive](#43-keepalive)
   - 4.4 [archive](#44-archive)
   - 4.5 [trivy-scan](#45-trivy-scan)
5. [Registry Provider Configuration](#5-registry-provider-configuration)
   - 5.1 [Oracle OCIR](#51-oracle-ocir)
   - 5.2 [AWS ECR](#52-aws-ecr)
   - 5.3 [Docker Hub](#53-docker-hub)
6. [Storage Backend Configuration](#6-storage-backend-configuration)
   - 6.1 [OCI Object Storage](#61-oci-object-storage)
   - 6.2 [AWS S3](#62-aws-s3)
7. [Trivy Integration](#7-trivy-integration)
8. [CORS Configuration](#8-cors-configuration)
9. [Deployment](#9-deployment)
10. [API Authentication](#10-api-authentication)

---

## 1. Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                    React Frontend                        │
│  (src/lib/api.ts — centralized API service layer)        │
└──────────────────────┬──────────────────────────────────┘
                       │ HTTPS (fetch)
                       ▼
┌─────────────────────────────────────────────────────────┐
│              Supabase Edge Functions (Deno)              │
│                                                          │
│  ┌──────────────────┐  ┌──────────────────┐              │
│  │ registry-images   │  │ audit            │              │
│  │ GET  — list       │  │ POST — dry-run   │              │
│  │ POST — pin/unpin  │  │        scan      │              │
│  │       /export     │  │                  │              │
│  └──────────────────┘  └──────────────────┘              │
│                                                          │
│  ┌──────────────────┐  ┌──────────────────┐              │
│  │ keepalive         │  │ archive          │              │
│  │ POST — trigger    │  │ GET  — list      │              │
│  │        refresh    │  │ POST — archive   │              │
│  │                   │  │       /restore   │              │
│  └──────────────────┘  └──────────────────┘              │
│                                                          │
│  ┌──────────────────┐                                    │
│  │ trivy-scan        │                                    │
│  │ POST — vuln scan  │                                    │
│  └──────────────────┘                                    │
└──────────────────────┬──────────────────────────────────┘
                       │
          ┌────────────┼────────────┐
          ▼            ▼            ▼
    ┌──────────┐ ┌──────────┐ ┌──────────┐
    │  OCIR    │ │  ECR     │ │ Docker   │
    │ Registry │ │ Registry │ │   Hub    │
    └──────────┘ └──────────┘ └──────────┘
          │            │
          ▼            ▼
    ┌──────────┐ ┌──────────┐
    │  OCI     │ │  AWS S3  │
    │ Object   │ │ Storage  │
    │ Storage  │ │          │
    └──────────┘ └──────────┘
```

**Runtime:** Deno (via Supabase Edge Functions)  
**Frontend:** React + TypeScript + Vite  
**API Client:** `src/lib/api.ts` — all frontend→backend communication goes through this file.

---

## 2. Environment Variables

### Frontend (Vite — `.env` or `.env.local`)

| Variable | Required | Description | Example |
|---|---|---|---|
| `VITE_SUPABASE_URL` | ✅ Yes | Your Supabase project URL | `https://xyzcompany.supabase.co` |
| `VITE_SUPABASE_ANON_KEY` | ✅ Yes | Supabase anonymous/public API key | `eyJhbGciOiJIUzI1NiIs...` |

These are **publishable keys** — safe to include in client-side code. They are used in `src/lib/api.ts` to construct the base URL and authorization headers:

```
Base URL pattern: ${VITE_SUPABASE_URL}/functions/v1/<function-name>
```

### Backend (Supabase Secrets — set via `supabase secrets set`)

| Secret | Used By | Required | Description |
|---|---|---|---|
| `TRIVY_SERVER_URL` | `trivy-scan` | Optional | URL of your Trivy server instance |
| `OCIR_ENDPOINT` | `registry-images`, `keepalive` | For OCIR | e.g. `fra.ocir.io` |
| `OCIR_TENANCY` | `registry-images`, `keepalive` | For OCIR | OCI tenancy namespace |
| `OCIR_USERNAME` | `registry-images`, `keepalive` | For OCIR | Full OCIR username |
| `OCIR_AUTH_TOKEN` | `registry-images`, `keepalive` | For OCIR | OCIR auth token |
| `OCIR_REGION` | `registry-images`, `keepalive` | For OCIR | e.g. `eu-frankfurt-1` |
| `OCIR_COMPARTMENT_OCID` | `registry-images` | For OCIR | Compartment OCID |
| `AWS_ACCESS_KEY_ID` | `registry-images`, `keepalive`, `archive` | For ECR/S3 | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | `registry-images`, `keepalive`, `archive` | For ECR/S3 | AWS secret key |
| `AWS_REGION` | `registry-images`, `keepalive` | For ECR | e.g. `us-east-1` |
| `ECR_ACCOUNT_ID` | `registry-images` | For ECR | 12-digit AWS account ID |
| `DOCKERHUB_USERNAME` | `registry-images` | For Docker Hub | Docker Hub username |
| `DOCKERHUB_ACCESS_TOKEN` | `registry-images` | For Docker Hub | Docker Hub PAT |
| `DOCKERHUB_NAMESPACE` | `registry-images` | For Docker Hub | Organization namespace |
| `ARCHIVE_S3_BUCKET` | `archive` | For S3 archive | S3 bucket name |
| `ARCHIVE_S3_REGION` | `archive` | For S3 archive | S3 bucket region |
| `ARCHIVE_S3_PREFIX` | `archive` | For S3 archive | Path prefix (e.g. `/archives`) |
| `ARCHIVE_OCI_BUCKET` | `archive` | For OCI archive | OCI OS bucket name |
| `ARCHIVE_OCI_NAMESPACE` | `archive` | For OCI archive | OCI tenancy namespace |
| `ARCHIVE_OCI_REGION` | `archive` | For OCI archive | OCI region |

**How to set secrets:**

```bash
# Set a single secret
supabase secrets set TRIVY_SERVER_URL=http://trivy.internal:4954

# Set multiple secrets
supabase secrets set \
  AWS_ACCESS_KEY_ID=AKIA... \
  AWS_SECRET_ACCESS_KEY=wJalrX... \
  AWS_REGION=us-east-1

# List current secrets
supabase secrets list
```

---

## 3. Frontend Configuration

### Registries (Client-Side)

Registries are currently defined in `src/lib/api.ts` — no backend call needed:

```typescript
export const registries: Registry[] = [
  { id: "ocir-fra", name: "OCIR Frankfurt", region: "eu-frankfurt-1" },
  { id: "ecr-use1", name: "ECR us-east-1", region: "us-east-1" },
  { id: "dockerhub", name: "Docker Hub", region: "global" },
];
```

**To add a new registry:** Add an entry here and ensure your edge functions handle the new registry ID.

### API Client (`src/lib/api.ts`)

All API calls flow through the `request<T>()` helper which:
1. Prepends `${VITE_SUPABASE_URL}/functions/v1` to the path
2. Adds `apikey` and `Authorization: Bearer` headers using the anon key
3. Parses JSON responses and throws on non-2xx status codes

---

## 4. Edge Functions Reference

All edge functions are in `supabase/functions/<name>/index.ts`.  
All functions include CORS preflight handling for browser requests.

---

### 4.1 `registry-images`

**Location:** `supabase/functions/registry-images/index.ts`  
**Purpose:** List, filter, pin/unpin, and export container images across all registries.

#### `GET /registry-images`

List images with optional filters.

**Query Parameters:**

| Param | Type | Default | Description |
|---|---|---|---|
| `registry` | string | `all` | Filter by registry ID (`ocir-fra`, `ecr-use1`, `dockerhub`, or `all`) |
| `status` | string | — | Filter by status (`safe`, `warning`, `critical`, `unpinned`) |
| `search` | string | — | Search in repo name and tag |

**Response:**

```json
{
  "images": [
    {
      "id": "1",
      "repo": "platform/api-gateway",
      "tag": "v2.14.3",
      "digest": "sha256:a1b2c3d4e5f6",
      "region": "eu-frankfurt-1",
      "size": "245 MB",
      "group": "production",
      "pinned": true,
      "expiresIn": 30,
      "lastKeepalive": "2026-03-08T03:42:00Z",
      "status": "safe",
      "registry": "ocir-fra"
    }
  ],
  "total": 1
}
```

**Backend TODO — Replace mock data with actual registry API calls:**

| Registry | API Call |
|---|---|
| OCIR | `GET /20180419/images?compartmentId=<OCID>` |
| ECR | `ecr.describeImages({ repositoryName })` |
| Docker Hub | `GET /v2/repositories/{namespace}/{repo}/tags` |

#### `POST /registry-images` — Pin/Unpin

**Request Body:**

```json
{
  "imageId": "1",
  "action": "pin"       // "pin" or "unpin"
}
```

**Response:**

```json
{
  "success": true,
  "imageId": "1",
  "action": "pin",
  "pinned": true,
  "timestamp": "2026-03-08T04:00:00Z"
}
```

**Backend TODO — Implement per-registry pin logic:**

| Registry | Pin Action |
|---|---|
| OCIR | `PUT /20180419/images/{imageId}` with `{ isPinned: true }` |
| ECR | `putImageTagMutability` / `putLifecyclePolicy` |
| Docker Hub | Tag manipulation via API |

#### `POST /registry-images` — Export

**Request Body:**

```json
{
  "action": "export",
  "imageId": "1",
  "sourceRegistry": "ocir-fra",
  "targetRegistry": "ecr-use1",
  "repo": "platform/api-gateway",
  "tag": "v2.14.3"
}
```

**Backend TODO:** Implement cross-registry image transfer (pull from source, push to target).

---

### 4.2 `audit`

**Location:** `supabase/functions/audit/index.ts`  
**Purpose:** Dry-run retention policy scan — identifies images at risk of garbage collection.

#### `POST /audit`

**Request Body:**

```json
{
  "dryRun": true,                  // Always true for now (safe mode)
  "registryFilter": "ocir-fra"     // Optional — filter by registry, default "all"
}
```

**Response:**

```json
{
  "dryRun": true,
  "timestamp": "2026-03-08T04:00:00Z",
  "summary": {
    "totalScanned": 12,
    "atRisk": 6,
    "critical": 2,
    "warning": 2,
    "unpinned": 2
  },
  "results": [
    {
      "imageId": "4",
      "repo": "ml/inference-engine",
      "tag": "v0.9.4",
      "region": "us-east-1",
      "expiresIn": 1,
      "status": "critical",
      "risk": "critical",
      "recommendation": "Pin immediately or archive to cold storage"
    }
  ]
}
```

**Risk Levels:**

| Risk | Meaning | `expiresIn` |
|---|---|---|
| `critical` | Will be deleted within 1-2 days | ≤ 2 |
| `warning` | Expiring within 7 days | 3-7 |
| `unpinned` | No pin protection, subject to GC anytime | -1 |

**Backend TODO — Replace mock with real retention policy checks:**

| Registry | API Call |
|---|---|
| OCIR | `GET /20180419/retentionPolicies` + compare image ages |
| ECR | `getLifecyclePolicy` + `evaluateLifecyclePolicy` |
| Docker Hub | Check tag age vs configured retention rules |

---

### 4.3 `keepalive`

**Location:** `supabase/functions/keepalive/index.ts`  
**Purpose:** Trigger keepalive actions to reset image retention timers.

#### `POST /keepalive`

**Request Body:**

```json
{
  "imageIds": ["1", "2", "3"],     // Specific images — OR —
  "group": "production",           // All images in a group
  "strategy": "pull"               // "pull" | "retag" | "native"
}
```

You must provide either `imageIds` or `group` (at least one).

**Response:**

```json
{
  "timestamp": "2026-03-08T04:00:00Z",
  "strategy": "pull",
  "processed": 3,
  "results": [
    {
      "imageId": "1",
      "repo": "platform/api-gateway",
      "tag": "v2.14.3",
      "strategy": "pull",
      "success": true,
      "newExpiry": "2026-04-07T04:00:00Z"
    }
  ]
}
```

**Keepalive Strategies Explained:**

| Strategy | How It Works | When to Use |
|---|---|---|
| `pull` | `docker pull <image>` — resets retention timer on most registries | Default, works everywhere |
| `retag` | `docker tag <image> <image>-keepalive-<ts>` then delete temp tag — the retag operation resets the clock | When pull alone doesn't reset retention |
| `native` | Uses registry-specific retention policy APIs to extend the timer directly | OCIR and ECR only, most reliable |

**Backend TODO — Per-strategy implementation:**

| Strategy | OCIR | ECR |
|---|---|---|
| `pull` | `oci artifacts container image pull` | `aws ecr batch-get-image` + `put-image` |
| `retag` | Standard Docker tag manipulation | Standard Docker tag manipulation |
| `native` | `PUT /20180419/images/{imageId}` lifecycle policy | `putLifecyclePolicy` with extended retention |

---

### 4.4 `archive`

**Location:** `supabase/functions/archive/index.ts`  
**Purpose:** Cold storage management — archive images to object storage and restore them.

#### `GET /archive`

List all archived images.

**Response:**

```json
{
  "images": [
    {
      "id": "1",
      "repo": "platform/api-gateway",
      "tag": "v2.12.0",
      "compressedSize": "89 MB",
      "originalSize": "245 MB",
      "archivedAt": "2026-02-15T10:00:00Z",
      "restorable": true,
      "storageBackend": "oci-os"
    }
  ],
  "storage": {
    "totalOriginalMB": 2053,
    "totalCompressedMB": 711,
    "savingsMB": 1342,
    "savingsPercent": 65
  }
}
```

**Note:** The frontend expects the response in this shape:

```json
{
  "archives": [...],           // Array of ArchivedImage
  "total": 4,
  "totalCompressedSize": "711 MB",
  "totalOriginalSize": "2.0 GB"
}
```

You may need to normalize the edge function response to match the frontend's `ArchiveListResponse` type in `src/lib/api.ts`.

#### `POST /archive?action=archive`

Archive images to cold storage.

**Request Body:**

```json
{
  "imageIds": ["4", "7"],          // Specific images — OR —
  "group": "tooling",              // All images in a group
  "targetStorage": "oci-os"        // "oci-os" or "s3"
}
```

**Response:**

```json
{
  "success": true,
  "action": "archive",
  "targetStorage": "oci-os",
  "timestamp": "2026-03-08T04:00:00Z",
  "steps": [
    { "step": "pull", "status": "complete", "duration": "2.3s" },
    { "step": "dedup", "status": "complete", "duration": "0.8s", "layersDeduped": 4 },
    { "step": "compress", "status": "complete", "duration": "5.1s", "ratio": "65%" },
    { "step": "upload", "status": "complete", "duration": "3.2s" },
    { "step": "verify", "status": "complete", "duration": "0.4s", "checksumMatch": true }
  ]
}
```

**Archive Pipeline Steps:**

| Step | Description |
|---|---|
| 1. `pull` | Pull image layers (`docker save` / `skopeo copy`) |
| 2. `dedup` | Deduplicate shared layers across images |
| 3. `compress` | Compress with zstd |
| 4. `upload` | Upload to object storage (OCI OS or S3) |
| 5. `verify` | Checksum verification |
| 6. (optional) | Delete from source registry |

#### `POST /archive?action=restore`

Restore an archived image to a registry.

**Request Body:**

```json
{
  "archiveId": "1",
  "targetRegistry": "ocir-fra"
}
```

**Response:**

```json
{
  "success": true,
  "action": "restore",
  "archiveId": "1",
  "targetRegistry": "ocir-fra",
  "timestamp": "2026-03-08T04:00:00Z",
  "steps": [
    { "step": "download", "status": "complete", "duration": "2.8s" },
    { "step": "decompress", "status": "complete", "duration": "3.5s" },
    { "step": "push_layers", "status": "complete", "duration": "6.2s", "layersPushed": 7 },
    { "step": "push_manifest", "status": "complete", "duration": "0.3s" },
    { "step": "verify", "status": "complete", "duration": "1.1s", "pullable": true }
  ]
}
```

---

### 4.5 `trivy-scan`

**Location:** `supabase/functions/trivy-scan/index.ts`  
**Purpose:** Vulnerability scanning using Trivy.

#### `POST /trivy-scan`

**Request Body:**

```json
{
  "image": "platform/api-gateway:v2.14.3"
}
```

Or (also accepted):

```json
{
  "imageId": "1",
  "repo": "platform/api-gateway",
  "tag": "v2.14.3"
}
```

**Response:**

```json
{
  "scannedAt": "2026-03-08T04:00:00Z",
  "image": "platform/api-gateway:v2.14.3",
  "totalCritical": 2,
  "totalHigh": 5,
  "totalMedium": 12,
  "totalLow": 8,
  "vulnerabilities": [
    {
      "id": "CVE-2026-1234",
      "severity": "CRITICAL",
      "package": "openssl",
      "installedVersion": "1.1.1t",
      "fixedVersion": "1.1.1u",
      "title": "Buffer overflow in X.509 certificate verification"
    }
  ]
}
```

**Severity Levels:** `CRITICAL`, `HIGH`, `MEDIUM`, `LOW`

**Behavior:**
- If `TRIVY_SERVER_URL` secret is set → forwards scan request to your Trivy server at `${TRIVY_SERVER_URL}/v1/scan`
- If `TRIVY_SERVER_URL` is NOT set → returns mock vulnerability data (for development)

---

## 5. Registry Provider Configuration

### 5.1 Oracle OCIR

**Required Secrets:**

```bash
supabase secrets set \
  OCIR_ENDPOINT=fra.ocir.io \
  OCIR_TENANCY=mytenancy \
  OCIR_USERNAME="mytenancy/oracleidentitycloudservice/user@example.com" \
  OCIR_AUTH_TOKEN=<auth-token> \
  OCIR_REGION=eu-frankfurt-1 \
  OCIR_COMPARTMENT_OCID=ocid1.compartment.oc1..aaaaaa...
```

**OCIR API Base:** `https://<OCIR_ENDPOINT>/20180419/`

**Key OCIR Endpoints:**

| Operation | Method | Endpoint |
|---|---|---|
| List images | GET | `/20180419/images?compartmentId={OCID}` |
| Get image | GET | `/20180419/images/{imageId}` |
| Pin image | PUT | `/20180419/images/{imageId}` with `{ isPinned: true }` |
| Retention policy | GET | `/20180419/retentionPolicies` |

### 5.2 AWS ECR

**Required Secrets:**

```bash
supabase secrets set \
  AWS_ACCESS_KEY_ID=AKIA... \
  AWS_SECRET_ACCESS_KEY=wJalrX... \
  AWS_REGION=us-east-1 \
  ECR_ACCOUNT_ID=123456789012
```

**ECR Endpoint:** `${ECR_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com`

**Key ECR Operations (AWS SDK):**

| Operation | SDK Method |
|---|---|
| List images | `ecr.describeImages({ repositoryName })` |
| Pin (immutability) | `ecr.putImageTagMutability({ imageTagMutability: 'IMMUTABLE' })` |
| Lifecycle policy | `ecr.getLifecyclePolicy()` / `putLifecyclePolicy()` |
| Evaluate policy | `ecr.getLifecyclePolicyPreview()` |

### 5.3 Docker Hub

**Required Secrets:**

```bash
supabase secrets set \
  DOCKERHUB_USERNAME=myorg-bot \
  DOCKERHUB_ACCESS_TOKEN=dckr_pat_... \
  DOCKERHUB_NAMESPACE=myorg
```

**Docker Hub API Base:** `https://hub.docker.com/v2/`

**Key Docker Hub Endpoints:**

| Operation | Method | Endpoint |
|---|---|---|
| List repo tags | GET | `/v2/repositories/{namespace}/{repo}/tags` |
| Search (public) | GET | `/v2/search/repositories/?query={q}` |
| Get tag details | GET | `/v2/repositories/{namespace}/{repo}/tags/{tag}` |

**Note:** The Docker Hub public search (`/v2/search/repositories/`) is called directly from the frontend in `DockerHubSearch.tsx` — no auth required for public search.

---

## 6. Storage Backend Configuration

### 6.1 OCI Object Storage

Used for archiving images to Oracle Cloud.

**Required Secrets:**

```bash
supabase secrets set \
  ARCHIVE_OCI_BUCKET=regikeep-archive \
  ARCHIVE_OCI_NAMESPACE=mytenancy \
  ARCHIVE_OCI_REGION=eu-frankfurt-1
```

**Key Commands (OCI CLI equivalent):**

```bash
# List archived images
oci os object list --bucket-name regikeep-archive

# Upload archive
oci os object put --bucket-name regikeep-archive --file archive.tar.zst

# Download for restore
oci os object get --bucket-name regikeep-archive --name archive.tar.zst --file archive.tar.zst
```

### 6.2 AWS S3

Used for archiving images to AWS.

**Required Secrets:**

```bash
supabase secrets set \
  ARCHIVE_S3_BUCKET=regikeep-archive-s3 \
  ARCHIVE_S3_REGION=us-east-1 \
  ARCHIVE_S3_PREFIX=/archives
```

**Key Commands (AWS CLI equivalent):**

```bash
# List archived images
aws s3api list-objects-v2 --bucket regikeep-archive-s3 --prefix /archives

# Upload archive
aws s3 cp archive.tar.zst s3://regikeep-archive-s3/archives/

# Download for restore
aws s3 cp s3://regikeep-archive-s3/archives/archive.tar.zst ./archive.tar.zst
```

---

## 7. Trivy Integration

### Option A: Self-hosted Trivy Server

```bash
# Run Trivy in server mode
docker run -d --name trivy-server \
  -p 4954:4954 \
  aquasec/trivy:latest server --listen 0.0.0.0:4954

# Set the secret
supabase secrets set TRIVY_SERVER_URL=http://trivy-server:4954
```

**Trivy Server API:**

```bash
# Scan an image
POST http://<trivy-server>:4954/v1/scan
Content-Type: application/json

{
  "image": "platform/api-gateway:v2.14.3"
}
```

### Option B: No Trivy Server (Development Mode)

If `TRIVY_SERVER_URL` is not set, the edge function returns mock vulnerability data. This is useful for frontend development without a running Trivy instance.

---

## 8. CORS Configuration

All edge functions include CORS headers for browser access:

```typescript
const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
};
```

The `trivy-scan` function includes additional Supabase client headers:

```typescript
'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type, x-supabase-client-platform, x-supabase-client-platform-version, x-supabase-client-runtime, x-supabase-client-runtime-version',
```

All functions handle `OPTIONS` preflight requests by returning `200` with CORS headers.

**To restrict CORS in production:** Replace `'*'` with your domain:

```typescript
'Access-Control-Allow-Origin': 'https://yourdomain.com'
```

---

## 9. Deployment

### Deploy Edge Functions

```bash
# Deploy all functions
supabase functions deploy registry-images
supabase functions deploy audit
supabase functions deploy keepalive
supabase functions deploy archive
supabase functions deploy trivy-scan

# Or deploy all at once
supabase functions deploy
```

### Verify Deployment

```bash
# Test registry-images
curl -X GET "https://<project>.supabase.co/functions/v1/registry-images" \
  -H "Authorization: Bearer <anon-key>" \
  -H "apikey: <anon-key>"

# Test audit
curl -X POST "https://<project>.supabase.co/functions/v1/audit" \
  -H "Authorization: Bearer <anon-key>" \
  -H "apikey: <anon-key>" \
  -H "Content-Type: application/json" \
  -d '{"dryRun": true}'

# Test keepalive
curl -X POST "https://<project>.supabase.co/functions/v1/keepalive" \
  -H "Authorization: Bearer <anon-key>" \
  -H "apikey: <anon-key>" \
  -H "Content-Type: application/json" \
  -d '{"imageIds": ["1"], "strategy": "pull"}'

# Test archive list
curl -X GET "https://<project>.supabase.co/functions/v1/archive" \
  -H "Authorization: Bearer <anon-key>" \
  -H "apikey: <anon-key>"

# Test trivy scan
curl -X POST "https://<project>.supabase.co/functions/v1/trivy-scan" \
  -H "Authorization: Bearer <anon-key>" \
  -H "apikey: <anon-key>" \
  -H "Content-Type: application/json" \
  -d '{"repo": "platform/api-gateway", "tag": "v2.14.3"}'
```

---

## 10. API Authentication

All API calls from the frontend include:

```
apikey: <VITE_SUPABASE_ANON_KEY>
Authorization: Bearer <VITE_SUPABASE_ANON_KEY>
```

Currently, JWT verification is handled by Supabase's default behavior. To disable JWT verification for specific functions (e.g., for public access or external webhook calls), add to `supabase/config.toml`:

```toml
[functions.registry-images]
verify_jwt = false

[functions.audit]
verify_jwt = false
```

**For production:** Keep JWT verification enabled and use proper Supabase auth tokens from authenticated users rather than the anon key.

---

## Quick Start Checklist

1. ☐ Set `VITE_SUPABASE_URL` and `VITE_SUPABASE_ANON_KEY` in frontend env
2. ☐ Deploy all 5 edge functions
3. ☐ Set registry secrets (OCIR / ECR / Docker Hub — whichever you use)
4. ☐ Set storage secrets (OCI OS / S3 — for archive functionality)
5. ☐ (Optional) Set `TRIVY_SERVER_URL` for real vulnerability scanning
6. ☐ Replace mock data in edge functions with real registry API calls
7. ☐ Test each endpoint with curl commands above
8. ☐ Restrict CORS for production
