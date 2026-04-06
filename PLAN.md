# LinkVault — Bookmark Manager

## Full Bootcamp Plan

A bookmark manager where users save, tag, organize, and share web bookmarks. Built to exercise every Replicated vendor platform feature for the SE bootcamp.

---

## Application Overview

**LinkVault** is a Go-based web application that lets users save URLs as bookmarks, organize them with tags and collections, search their library, and share collections publicly via shareable links.

### Tech Stack

| Component | Choice | Reasoning |
|---|---|---|
| Backend | Go | Single binary, fast, matches team codebase |
| Frontend | Embedded HTML/CSS/JS (served by Go) | No build step, single container |
| Primary DB | PostgreSQL (Helm subchart) | Stores all bookmark data |
| Cache | Redis (Helm subchart) | Search cache, recently viewed, rate limiting |
| Container | Docker (pushed to private registry) | Multi-stage build |
| Orchestration | Helm chart + Replicated SDK subchart | Standard Replicated distribution |

### Database Schema

```sql
CREATE TABLE bookmarks (
    id          SERIAL PRIMARY KEY,
    url         TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    favicon_url TEXT NOT NULL DEFAULT '',
    is_public   BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE tags (
    id   SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE bookmark_tags (
    bookmark_id INT REFERENCES bookmarks(id) ON DELETE CASCADE,
    tag_id      INT REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (bookmark_id, tag_id)
);

CREATE TABLE collections (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_public   BOOLEAN NOT NULL DEFAULT false,
    share_code  TEXT UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE collection_bookmarks (
    collection_id INT REFERENCES collections(id) ON DELETE CASCADE,
    bookmark_id   INT REFERENCES bookmarks(id) ON DELETE CASCADE,
    position      INT NOT NULL DEFAULT 0,
    PRIMARY KEY (collection_id, bookmark_id)
);
```

### API Endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/` | Main UI (embedded HTML) |
| GET | `/api/bookmarks` | List bookmarks (supports `?q=`, `?tag=`, `?collection_id=`) |
| POST | `/api/bookmarks` | Create bookmark (auto-fetches title/description from URL) |
| DELETE | `/api/bookmarks/:id` | Delete bookmark |
| GET | `/api/tags` | List all tags with counts |
| GET | `/api/collections` | List collections |
| POST | `/api/collections` | Create collection |
| GET | `/api/collections/:id` | Get collection with its bookmarks |
| GET | `/shared/:code` | Public collection view (no auth) |
| POST | `/api/bookmarks/import` | Import bookmarks from JSON/CSV |
| GET | `/api/bookmarks/export` | Export bookmarks as JSON |
| GET | `/healthz` | Health check (structured JSON) |
| GET | `/api/license` | License info from SDK |
| GET | `/api/updates` | Update check from SDK |
| POST | `/api/support-bundle` | Trigger support bundle generation + upload |
| GET | `/api/metrics` | Current app metrics |

### UI

Single-page app with these views:
- **Main view**: bookmark list with search bar, tag sidebar, add bookmark form
- **Collection view**: bookmarks grouped in a collection, share button
- **Public share page**: read-only collection view for shared links
- **Settings/admin panel**: license info, update banner, support bundle button, app version

---

## Rubric Mapping — Every Task

### Getting Started (Pre-Tier 0)

| Task | Implementation |
|---|---|
| New vendor portal account | Create fresh account, new team |
| Admin: $1000 CMX credits | Self-serve in admin panel |
| Remove trial expiration | Admin panel |
| Turn on entitlements/features | Enable: custom metrics, license fields, embedded cluster, enterprise portal, scoped RBAC |
| GitHub Collab Repo | Add `superci-replicated` as collab on the linkvault repo |
| CMX as VM/cluster target | Use CMX for all installs |

---

### Tier 0: Build It

#### 0.1 — Custom web app with stateful component

**What to build:**
- Go HTTP server serving embedded static files
- PostgreSQL stores all bookmark data (stateful)
- Auto-fetch page title + description when saving a URL (HTTP GET + parse `<title>` and `<meta name="description">`)
- CRUD for bookmarks, tags, collections

**Acceptance criteria:** Run locally with `docker-compose up` or `go run .` + local PostgreSQL. Show the Dockerfile.

**Dockerfile:**
```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o linkvault .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/linkvault /usr/local/bin/linkvault
EXPOSE 8080
CMD ["linkvault"]
```

#### 0.2 — Helm chart packages and deploys, values.schema.json

**Chart structure:**
```
helm/linkvault/
├── Chart.yaml
├── values.yaml
├── values.schema.json          # REQUIRED by rubric
├── Chart.lock
├── charts/
└── templates/
    ├── _helpers.tpl
    ├── deployment.yaml
    ├── service.yaml
    ├── ingress.yaml
    ├── preflight.yaml
    ├── support-bundle.yaml
    └── NOTES.txt
```

**`values.schema.json`** must validate:
- `image.repository` (required, string)
- `image.tag` (required, string)
- `replicaCount` (integer, minimum 1)
- `service.type` (enum: ClusterIP, NodePort, LoadBalancer)
- `postgresql.enabled` (boolean)
- `redis.enabled` (boolean)

**Acceptance criteria:** `helm lint` returns no errors. App opens in browser after `helm install`.

#### 0.3 — 2 open-source Helm subcharts, embedded default + BYO opt-in

**Subchart 1: PostgreSQL (Bitnami)** — stateful component
- Repository: `https://charts.bitnami.com/bitnami`
- Condition: `postgresql.enabled` (default `true`)
- BYO: when `postgresql.enabled=false`, user provides `externalPostgresql.host`, `.port`, `.user`, `.password`, `.database`

**Subchart 2: Redis (Bitnami)** — open-source cache
- Repository: `https://charts.bitnami.com/bitnami`
- Condition: `redis.enabled` (default `true`)
- BYO: when `redis.enabled=false`, user provides `externalRedis.host`, `.port`, `.password`

**values.yaml pattern:**
```yaml
postgresql:
  enabled: true
  auth:
    postgresPassword: "linkvault-default-pw"
    database: "linkvault"
  primary:
    persistence:
      size: 1Gi

externalPostgresql:
  host: ""
  port: "5432"
  user: "postgres"
  password: ""
  database: "linkvault"

redis:
  enabled: true
  auth:
    enabled: false

externalRedis:
  host: ""
  port: "6379"
  password: ""
```

**deployment.yaml** — conditional env vars:
```yaml
env:
  - name: PGHOST
    {{- if .Values.postgresql.enabled }}
    value: "{{ include "linkvault.fullname" . }}-postgresql"
    {{- else }}
    value: {{ .Values.externalPostgresql.host | quote }}
    {{- end }}
  # ... same pattern for PGPORT, PGUSER, PGPASSWORD, PGDATABASE
  - name: REDIS_ADDR
    {{- if .Values.redis.enabled }}
    value: "{{ include "linkvault.fullname" . }}-redis-master:6379"
    {{- else }}
    value: "{{ .Values.externalRedis.host }}:{{ .Values.externalRedis.port }}"
    {{- end }}
```

**Demo:**
1. Install with embedded (default) — show PostgreSQL and Redis pods Running
2. Install with BYO — `--set postgresql.enabled=false --set externalPostgresql.host=my-external-pg...` — show no PG pod, app using external instance

#### 0.4 — Kubernetes best practices

**Probes** (in deployment.yaml):
```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: http
  initialDelaySeconds: 5
  periodSeconds: 30
readinessProbe:
  httpGet:
    path: /healthz
    port: http
  initialDelaySeconds: 3
  periodSeconds: 10
```

**Resource requests/limits** on ALL containers:
```yaml
# linkvault container
resources:
  requests:
    cpu: 50m
    memory: 64Mi
  limits:
    cpu: 200m
    memory: 128Mi
```

**Health endpoint** (`/healthz`):
```json
{
  "status": "ok",
  "postgres": "connected",
  "redis": "connected",
  "version": "1.0.0",
  "uptime_seconds": 3600
}
```

**Data persistence:** Delete the app pod → it restarts → bookmarks are still in PostgreSQL (PVC-backed). Demo by creating bookmarks, deleting the pod, refreshing the page.

#### 0.5 — HTTPS with certificate options

**Three TLS modes in values.yaml:**
```yaml
ingress:
  enabled: false
  tls:
    enabled: false
    # Option 1: cert-manager auto-provisioned
    certManager:
      enabled: false
      issuerName: "letsencrypt-prod"
      issuerKind: "ClusterIssuer"
    # Option 2: manually uploaded certificate
    secretName: ""
    # Option 3: self-signed (optional)
    selfSigned: false
```

**ingress.yaml** — conditional TLS:
```yaml
{{- if .Values.ingress.tls.enabled }}
tls:
  - hosts:
      - {{ .Values.ingress.host }}
    {{- if .Values.ingress.tls.secretName }}
    secretName: {{ .Values.ingress.tls.secretName }}
    {{- else if .Values.ingress.tls.certManager.enabled }}
    secretName: {{ include "linkvault.fullname" . }}-tls
    {{- end }}
{{- end }}
```

**cert-manager annotation** (conditional):
```yaml
{{- if .Values.ingress.tls.certManager.enabled }}
cert-manager.io/{{ .Values.ingress.tls.certManager.issuerKind | lower }}: {{ .Values.ingress.tls.certManager.issuerName }}
{{- end }}
```

#### 0.6 — App waits for database before starting

**Init container** in deployment.yaml:
```yaml
initContainers:
  - name: wait-for-postgres
    image: busybox:1.36
    command: ['sh', '-c']
    args:
      - |
        until nc -z -w2 $PGHOST $PGPORT; do
          echo "Waiting for PostgreSQL at $PGHOST:$PGPORT..."
          sleep 2
        done
        echo "PostgreSQL is ready"
    env:
      - name: PGHOST
        # same conditional logic as main container
      - name: PGPORT
        value: "5432"
```

Also add a wait-for-redis init container if Redis is enabled.

**Acceptance criteria:** No crash-loops on startup. Pods start cleanly even if DB takes 10+ seconds.

#### 0.7 — At least 2 user-facing, demoable features

**Feature 1: Bookmark Management with Auto-Fetch**
- Paste a URL → app fetches the page title, description, and favicon automatically
- Tag bookmarks, filter by tag, search by title/URL/description
- Full CRUD

**Feature 2: Public Collections with Shareable Links**
- Create named collections, add bookmarks to them
- Toggle a collection to "public" → generates a unique share code
- Anyone with the link `/shared/:code` can view the collection (no auth)
- Share button copies the link to clipboard

Both are real features a production app would have. Easy to demo end-to-end.

---

### Tier 1: Automate It

#### 1.1 — Container images built and pushed to private registry in CI

**Private registry:** Create a private registry on GCP Artifact Registry, ECR, or keep Docker Hub private repo.

**CI workflow** (`.github/workflows/release.yml`):
```yaml
- name: Build and push Docker image
  run: |
    docker build -t $REGISTRY/linkvault:${{ github.ref_name }} .
    docker push $REGISTRY/linkvault:${{ github.ref_name }}
```

#### 1.2a — Scoped RBAC policy

```json
{
  "v1": {
    "name": "CI Release Bot",
    "resources": {
      "allowed": [
        "platform/app/*/release/**",
        "platform/app/*/channel/**",
        "platform/app/*/read",
        "kots/app/*/release/**",
        "kots/app/*/channel/**",
        "kots/app/*/read"
      ],
      "denied": ["**/*"]
    }
  }
}
```

Create service account, assign policy, store token as `REPLICATED_API_TOKEN` GitHub secret.

#### 1.2b — PR workflow using `.replicated` file + Replicated GitHub Actions

**New file: `.github/workflows/pr.yml`**
```yaml
name: PR Check
on:
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: azure/setup-helm@v4

      - name: Build Docker image
        run: docker build -t linkvault:pr-${{ github.event.pull_request.number }} .

      - name: Push image (to registry for testing)
        run: |
          docker tag linkvault:pr-${{ github.event.pull_request.number }} $REGISTRY/linkvault:pr-${{ github.event.pull_request.number }}
          docker push $REGISTRY/linkvault:pr-${{ github.event.pull_request.number }}

      - name: Package Helm chart
        run: |
          helm dependency update helm/linkvault/
          helm package helm/linkvault/ -d release/

      - name: Create test release
        uses: replicatedhq/replicated-actions/create-release@v1
        with:
          app-slug: linkvault
          yaml-dir: release/
          promote-channel: Unstable
          version: pr-${{ github.event.pull_request.number }}

      # Optionally: spin up a CMX cluster and run tests against it
```

**`.replicated` file:**
```yaml
appSlug: linkvault
charts:
  - path: ./helm/linkvault
manifests:
  - ./release/*.yaml
```

#### 1.3 — Release workflow: merge to main → Unstable

**`.github/workflows/release.yml`** triggers on merge to main:
```yaml
name: Release
on:
  push:
    branches: [main]

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set version
        id: version
        run: echo "version=$(date +%Y%m%d)-$(git rev-parse --short HEAD)" >> $GITHUB_OUTPUT

      - name: Build and push image
        run: |
          docker build -t $REGISTRY/linkvault:${{ steps.version.outputs.version }} .
          docker push $REGISTRY/linkvault:${{ steps.version.outputs.version }}

      - name: Trivy CVE scan
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: $REGISTRY/linkvault:${{ steps.version.outputs.version }}
          severity: CRITICAL,HIGH
        continue-on-error: true

      - name: Package and release
        run: |
          helm dependency update helm/linkvault/
          helm package helm/linkvault/ -d release/

      - name: Create release and promote to Unstable
        uses: replicatedhq/replicated-actions/create-release@v1
        with:
          app-slug: linkvault
          yaml-dir: release/
          promote-channel: Unstable
          version: ${{ steps.version.outputs.version }}
```

Promotion to Stable is manual or via a separate tag-triggered workflow.

#### 1.4 — Email notifications on Stable promotion

In Vendor Portal > Notifications:
- Create email notification rule
- Trigger: release promoted to Stable channel
- Recipient: your @replicated.com address

---

### Tier 2: Ship It with Helm

#### 2.1 — Replicated SDK deployed as subchart, renamed

In `Chart.yaml`:
```yaml
dependencies:
  - name: replicated
    version: "1.x.x"
    repository: oci://registry.replicated.com/library
    alias: linkvault-sdk
```

Using `alias` renames the subchart so the deployment becomes `<release>-linkvault-sdk`.

Verify: `kubectl get deployment <release>-linkvault-sdk -n <namespace>` shows Running.

#### 2.2 — All container images proxied through custom domain

**Custom domain:** `proxy.linkvault.dev` (or whatever domain you buy) aliases `proxy.replicated.com`.

**Every image must use the proxy domain:**
```yaml
# values.yaml
image:
  repository: proxy.linkvault.dev/proxy/<app-slug>/<registry>/<user>/linkvault

postgresql:
  image:
    registry: proxy.linkvault.dev
    repository: proxy/<app-slug>/registry-1.docker.io/bitnamicharts/postgresql

redis:
  image:
    registry: proxy.linkvault.dev
    repository: proxy/<app-slug>/registry-1.docker.io/bitnamicharts/redis
```

Verify: `kubectl get pods -A -o custom-columns='STATUS:.status.phase,IMAGE:.spec.containers[*].image'` — every image starts with `proxy.linkvault.dev`.

#### 2.4 — Custom metrics visible in Vendor Portal

**Metrics reported to SDK every 60s:**
```go
metrics := map[string]interface{}{
    "total_bookmarks":      store.CountBookmarks(),
    "bookmarks_added_today": store.BookmarksAddedToday(),
    "collections_count":    store.CountCollections(),
    "searches_today":       store.SearchesToday(),
    "tags_count":           store.CountTags(),
    "storage_used_mb":      store.StorageUsedMB(),
}
```

All from real activity — creating bookmarks, searching, making collections.

#### 2.5 — License entitlement gates a real feature via SDK

**License fields (create in Vendor Portal):**

| Field Name | Type | Default | Purpose |
|---|---|---|---|
| `max_bookmarks` | Integer | 100 | Bookmark limit |
| `feature_tier` | String | "free" | free/pro/enterprise |
| `search_enabled` | Boolean | false | Search feature access |
| `public_collections` | Boolean | false | Public sharing |
| `import_export` | Boolean | false | Bulk import/export |

**App code queries SDK directly:**
```go
resp, _ := http.Get("http://replicated:3000/api/v1/license/fields")
// Parse fields, enforce limits
if bookmarkCount >= fields.MaxBookmarks {
    return errors.New("bookmark limit reached")
}
if !fields.SearchEnabled {
    // Return 403 on search API endpoint
}
```

**Demo:** Install with `search_enabled=false` → search bar is disabled/hidden. Update license to enable it → search works. No redeploy needed.

#### 2.6a — Update available banner

Check SDK every 5 minutes:
```go
resp, _ := http.Get("http://replicated:3000/api/v1/app/updates")
```
Show banner at top of page: "Update available: v1.2.0"

#### 2.6b — License validity enforced via SDK

App checks license expiry via SDK. If expired:
- Show warning banner 30 days before
- Block access with overlay when expired
- Must actively check, not just passive display

#### 2.7 — Optional ingress, off by default

```yaml
ingress:
  enabled: false  # off by default
```

When enabled, routes traffic to the app. Already covered in 0.5.

#### 2.8 — Service type configurable

```yaml
service:
  type: ClusterIP  # ClusterIP, NodePort, or LoadBalancer
  port: 8080
  nodePort: ""     # only used when type=NodePort
```

#### 2.9 — Instance is live, named, tagged

- Deploy a live instance on CMX cluster
- Name it in Vendor Portal
- Add tags (e.g., "bootcamp", "demo", "stable")
- Instance reports healthy, shows custom metrics

#### 2.10 — Services show healthy in instance reporting

Ensure the Replicated SDK can see all pods as healthy. The SDK auto-detects Helm-managed resources and reports their status. Verify in Vendor Portal > Instances > Instance Details.

---

### Tier 3: Support It

#### 3.1 — Preflight checks (5 required)

**All 5 checks in `templates/preflight.yaml`:**

```yaml
apiVersion: troubleshoot.sh/v1beta2
kind: Preflight
metadata:
  name: {{ include "linkvault.fullname" . }}
spec:
  collectors:
    - clusterResources: {}
    {{- if and (not .Values.postgresql.enabled) .Values.externalPostgresql.host }}
    - exec:
        name: external-db-check
        selector:
          - app.kubernetes.io/name={{ include "linkvault.name" . }}
        command: ["sh", "-c"]
        args: ["nc -z -w5 {{ .Values.externalPostgresql.host }} {{ .Values.externalPostgresql.port }} && echo reachable || echo unreachable"]
    {{- end }}
  analyzers:
    # CHECK 1: External database connectivity (conditional)
    {{- if and (not .Values.postgresql.enabled) .Values.externalPostgresql.host }}
    - textAnalyze:
        checkName: External Database Connectivity
        fileName: external-db-check/external-db-check.log
        outcomes:
          - fail:
              when: "unreachable"
              message: |
                Cannot reach external PostgreSQL at {{ .Values.externalPostgresql.host }}:{{ .Values.externalPostgresql.port }}.
                Verify the host is correct, the port is open, and any firewalls allow traffic from this cluster.
          - pass:
              when: "reachable"
              message: External PostgreSQL is reachable.
    {{- end }}

    # CHECK 2: Required external endpoint (e.g., DNS resolution)
    # We check that the Replicated SDK endpoint is reachable
    - textAnalyze:
        checkName: Replicated API Connectivity
        # (use a collector that checks connectivity to replicated.app or the proxy domain)

    # CHECK 3: Cluster resource check (CPU and memory)
    - nodeResources:
        checkName: Cluster CPU
        outcomes:
          - fail:
              when: "sum(cpuAllocatable) < 2"
              message: |
                The cluster has less than 2 CPU cores allocatable. LinkVault requires at least 2 cores.
                Add more nodes or increase node size.
          - pass:
              when: "sum(cpuAllocatable) >= 2"
              message: Cluster has sufficient CPU.
    - nodeResources:
        checkName: Cluster Memory
        outcomes:
          - fail:
              when: "sum(memoryAllocatable) < 2Gi"
              message: |
                The cluster has less than 2Gi allocatable memory. LinkVault requires at least 2Gi.
                Add more nodes or increase node size.
          - pass:
              when: "sum(memoryAllocatable) >= 2Gi"
              message: Cluster has sufficient memory.

    # CHECK 4: Kubernetes version
    - clusterVersion:
        outcomes:
          - fail:
              when: "< 1.25.0"
              message: |
                Kubernetes version is below 1.25.0. LinkVault requires 1.25.0 or later.
                Upgrade your cluster to a supported version.
          - pass:
              when: ">= 1.25.0"
              message: Kubernetes version is compatible.

    # CHECK 5: Distribution check (unsupported distros)
    - distribution:
        outcomes:
          - fail:
              when: "== docker-desktop"
              message: |
                Docker Desktop is not a supported Kubernetes distribution for LinkVault.
                See https://docs.linkvault.dev/supported-platforms for supported options.
          - fail:
              when: "== microk8s"
              message: |
                MicroK8s is not a supported Kubernetes distribution for LinkVault.
                See https://docs.linkvault.dev/supported-platforms for supported options.
          - pass:
              message: Kubernetes distribution is supported.
```

**Demo:** Run preflights twice — once failing (e.g., on docker-desktop or with unreachable external DB), once passing.

#### 3.2 — Log collection for all components

```yaml
collectors:
  - clusterResources: {}
  - logs:
      name: linkvault-app-logs
      selector:
        - app.kubernetes.io/name={{ include "linkvault.name" . }}
      limits:
        maxLines: 10000
  - logs:
      name: postgresql-logs
      selector:
        - app.kubernetes.io/name=postgresql
      limits:
        maxLines: 5000
  - logs:
      name: redis-logs
      selector:
        - app.kubernetes.io/name=redis
      limits:
        maxLines: 5000
  - logs:
      name: replicated-sdk-logs
      selector:
        - app.kubernetes.io/name=replicated
      limits:
        maxLines: 5000
```

Each component has its own collector with limits. Demo: run bundle, show each directory is non-empty.

#### 3.3 — Health endpoint with `http` collector + textAnalyze

```yaml
collectors:
  - http:
      collectorName: health-check
      get:
        url: http://{{ include "linkvault.fullname" . }}.{{ .Release.Namespace }}.svc.cluster.local:{{ .Values.service.port }}/healthz

analyzers:
  - textAnalyze:
      checkName: Application Health
      fileName: health-check.json
      outcomes:
        - pass:
            when: '"status":"ok"'
            message: LinkVault is healthy.
        - fail:
            message: LinkVault health check failed. The application may not be running or may not be able to connect to its database.
```

Uses `http` collector (not exec+wget). Calls in-cluster service DNS. textAnalyze parses the response.

#### 3.4 — Status analyzers for all workload types

```yaml
analyzers:
  - deploymentStatus:
      name: {{ include "linkvault.fullname" . }}
      outcomes:
        - fail:
            when: "< 1"
            message: |
              LinkVault application has no ready replicas.
              Users cannot access the bookmark manager. Check pod logs for errors.
        - pass:
            when: ">= 1"
            message: LinkVault application is running.

  - statefulsetStatus:
      name: {{ include "linkvault.fullname" . }}-postgresql
      outcomes:
        - fail:
            when: "< 1"
            message: |
              PostgreSQL database has no ready replicas.
              All bookmark data is inaccessible. The application cannot function without the database.
        - pass:
            when: ">= 1"
            message: PostgreSQL is running.

  - deploymentStatus:
      name: {{ include "linkvault.fullname" . }}-redis-master
      outcomes:
        - fail:
            when: "< 1"
            message: |
              Redis cache has no ready replicas.
              Search and caching are degraded. The application will still function but with reduced performance.
        - pass:
            when: ">= 1"
            message: Redis is running.

  - deploymentStatus:
      name: {{ include "linkvault.fullname" . }}-linkvault-sdk
      outcomes:
        - fail:
            when: "< 1"
            message: |
              Replicated SDK has no ready replicas.
              License validation and custom metrics reporting are unavailable. The application may not enforce license limits.
        - pass:
            when: ">= 1"
            message: Replicated SDK is running.
```

Note: PostgreSQL uses `statefulsetStatus` (Bitnami deploys as StatefulSet). Redis master and SDK use `deploymentStatus`.

#### 3.5 — textAnalyze catches a known app failure pattern

```yaml
collectors:
  - logs:
      name: linkvault-app-logs
      selector:
        - app.kubernetes.io/name={{ include "linkvault.name" . }}
      limits:
        maxLines: 10000

analyzers:
  - textAnalyze:
      checkName: Database Connection Failures
      fileName: linkvault-app-logs/*.log
      regex: 'pq: (?:connection refused|no such host|password authentication failed|the database system is starting up)'
      outcomes:
        - fail:
            when: "true"
            message: |
              LinkVault logs show PostgreSQL connection errors.
              Check that PostgreSQL is running and that the connection credentials are correct.
              If using an external database, verify the host, port, and firewall rules.
              See: https://docs.linkvault.dev/troubleshooting/database
        - pass:
            when: "false"
            message: No database connection errors found in logs.
```

Pattern is specific to the app's failure modes (lib/pq error messages). Includes remediation step and doc link.

#### 3.6 — Storage class and node readiness

```yaml
analyzers:
  - storageClass:
      checkName: Default Storage Class
      outcomes:
        - fail:
            when: "= 0"
            message: |
              No default storage class found in the cluster.
              LinkVault requires a default storage class for PostgreSQL data persistence.
              Create a storage class and set it as default, or specify one explicitly.
        - pass:
            when: ">= 1"
            message: Default storage class is available.

  - nodeResources:
      checkName: Node Readiness
      outcomes:
        - fail:
            when: "count() == 0"
            message: No nodes found in the cluster.
        - fail:
            when: "min(ready) == false"
            message: |
              One or more cluster nodes are not Ready.
              This may cause pod scheduling failures. Check node status with kubectl get nodes.
        - pass:
            when: "min(ready) == true"
            message: All nodes are Ready.
```

#### 3.7 — Support bundle from app UI → upload to Vendor Portal

**App-side implementation:**

Add a "Generate Support Bundle" button in the settings/admin panel of the UI.

When clicked, the app calls the SDK endpoint to trigger bundle generation and upload:
```go
// POST /api/support-bundle handler
func handleSupportBundle(w http.ResponseWriter, r *http.Request) {
    // Call the SDK to generate and upload the support bundle
    resp, err := http.Post(
        sdkAddr+"/api/v1/app/support-bundle/generate",
        "application/json",
        nil,
    )
    // Return status to the UI
}
```

The SDK handles the actual collection and upload. The bundle appears on the Instance Details page in the Vendor Portal. Demo: click button, show bundle in VP, walk through data and analyzer results.

---

### Tier 4: Ship It on a VM (Embedded Cluster v3)

#### 4.1 — EC install on bare VM

**`release/embedded-cluster-config.yaml`:**
```yaml
apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec:
  version: ""  # Use EC v3 — check latest stable
  domains:
    replicatedAppDomain: get.linkvault.dev
    proxyRegistryDomain: proxy.linkvault.dev
  extensions:
    helm:
      repositories:
        - name: ingress-nginx
          url: https://kubernetes.github.io/ingress-nginx
      charts:
        - name: ingress-nginx
          chartname: ingress-nginx/ingress-nginx
          namespace: ingress-nginx
          version: "4.12.0"
          values: |
            controller:
              service:
                type: NodePort
                nodePorts:
                  http: "80"
                  https: "443"
```

Demo: Fresh CMX VM → download installer → run → `sudo k0s kubectl get pods -A` shows all Running → open app in browser.

#### 4.2 — In-place upgrade without data loss

1. Install release v1.0.0
2. Create some bookmarks, a collection, add tags
3. Promote release v1.1.0
4. Trigger upgrade via Admin Console
5. Show bookmarks still present, all pods Running

#### 4.3 — Air-gapped install

1. Build air gap bundle from release: `replicated release download --air-gap`
2. Transfer bundle to a VM with no internet
3. Install using only the bundle (includes embedded cluster + all images)
4. Show all pods Running, app accessible

#### 4.6 — App icon and name

Set in the KOTS Application manifest (`release/application.yaml`):
```yaml
apiVersion: kots.io/v1beta1
kind: Application
metadata:
  name: linkvault
spec:
  title: LinkVault
  icon: https://your-icon-url/linkvault-icon.png
  statusInformers:
    - deployment/linkvault
```

#### 4.7 — License entitlement gates feature via KOTS LicenseFieldValue

In `release/linkvault-helmchart.yaml`:
```yaml
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: linkvault
spec:
  chart:
    name: linkvault
    chartVersion: 1.0.0
  values:
    features:
      searchEnabled: repl{{ LicenseFieldValue "search_enabled" }}
      publicCollections: repl{{ LicenseFieldValue "public_collections" }}
```

With entitlement disabled: feature's config screen item is hidden/locked. Update license to enable: config item appears, feature works.

---

### Config Screen (5.x)

**New file: `release/config.yaml`**

```yaml
apiVersion: kots.io/v1beta1
kind: Config
metadata:
  name: linkvault-config
spec:
  groups:
    - name: database
      title: Database Configuration
      items:
        - name: db_type
          title: Database
          type: select_one
          default: embedded
          items:
            - name: embedded
              title: Embedded PostgreSQL
            - name: external
              title: External PostgreSQL
          help_text: Choose embedded to deploy PostgreSQL within the cluster, or external to use your own PostgreSQL instance.

        - name: external_db_host
          title: PostgreSQL Host
          type: text
          when: '{{repl ConfigOptionEquals "db_type" "external"}}'
          required: true
          help_text: Hostname or IP address of your external PostgreSQL server. Example: db.example.com

        - name: external_db_port
          title: PostgreSQL Port
          type: text
          default: "5432"
          when: '{{repl ConfigOptionEquals "db_type" "external"}}'
          validation:
            regex:
              pattern: "^[0-9]{1,5}$"
              message: Port must be a number between 1 and 65535.
          help_text: TCP port your PostgreSQL server listens on. Default is 5432.

        - name: external_db_password
          title: PostgreSQL Password
          type: password
          when: '{{repl ConfigOptionEquals "db_type" "external"}}'
          required: true
          help_text: Password for authenticating to your external PostgreSQL server.

        - name: embedded_db_password
          title: Embedded Database Password
          type: password
          hidden: true
          value: '{{repl RandomString 32}}'
          help_text: Auto-generated password for the embedded PostgreSQL instance.

    - name: cache
      title: Cache Configuration
      items:
        - name: cache_type
          title: Redis Cache
          type: select_one
          default: embedded
          items:
            - name: embedded
              title: Embedded Redis
            - name: external
              title: External Redis
          help_text: Choose embedded to deploy Redis within the cluster, or external to use your own Redis instance.

        - name: external_redis_host
          title: Redis Host
          type: text
          when: '{{repl ConfigOptionEquals "cache_type" "external"}}'
          required: true
          help_text: Hostname or IP of your external Redis server.

    - name: features
      title: Application Features
      items:
        - name: enable_public_collections
          title: Public Collections
          type: bool
          default: "1"
          help_text: Allow users to create publicly shareable bookmark collections. Disable to keep all collections private.
          when: repl{{ LicenseFieldValue "public_collections" }}

        - name: max_upload_size
          title: Import File Size Limit
          type: text
          default: "10MB"
          validation:
            regex:
              pattern: "^[0-9]+(MB|KB|GB)$"
              message: "Must be a number followed by KB, MB, or GB. Example: 10MB"
          help_text: Maximum file size for bookmark import files. Accepts KB, MB, or GB suffixes. Example values - 5MB, 100KB, 1GB.

        - name: enable_search
          title: Full-Text Search
          type: bool
          default: "0"
          help_text: Enable full-text search across all bookmarks. Requires Redis for search indexing.
          when: repl{{ LicenseFieldValue "search_enabled" }}
```

#### 5.0 — External DB toggle with conditional fields

Covered by the `database` and `cache` config groups above. Selecting "external" reveals host/port/password fields. Selecting "embedded" hides them.

Wire through `release/linkvault-helmchart.yaml`:
```yaml
values:
  postgresql:
    enabled: repl{{ ConfigOptionEquals "db_type" "embedded" }}
  externalPostgresql:
    host: repl{{ ConfigOption "external_db_host" }}
    port: repl{{ ConfigOption "external_db_port" }}
    password: repl{{ ConfigOption "external_db_password" }}
  redis:
    enabled: repl{{ ConfigOptionEquals "cache_type" "embedded" }}
  externalRedis:
    host: repl{{ ConfigOption "external_redis_host" }}
```

#### 5.1 — Configurable app features (2+)

1. **Public Collections toggle** — enable/disable in config → wired to Helm → app shows/hides share button
2. **Full-Text Search toggle** — enable/disable in config → wired to Helm → app shows/hides search bar

Both non-trivial, user-visible features.

#### 5.2 — Generated default survives upgrade

The `embedded_db_password` field uses `{{repl RandomString 32}}` with `hidden: true`. On first install, KOTS generates a random 32-char password. On upgrade, KOTS preserves the existing value (doesn't regenerate). The app keeps its DB connection.

Demo: install → upgrade → app still connects to DB without reconfiguring.

#### 5.3 — Input validation with regex

Two regex-validated fields:
- `external_db_port`: pattern `^[0-9]{1,5}$` — blocks "abc" or empty
- `max_upload_size`: pattern `^[0-9]+(MB|KB|GB)$` — blocks "10" or "tenMB"

Demo: enter invalid value → config screen shows error message → enter valid value → proceeds.

#### 5.4 — Help text on all config items

Every item in the config has `help_text` that describes what the field does and what valid values look like. Not just restating the label.

---

### Tier 5: Deliver It (Enterprise Portal v2)

#### 6.1 — EP branding

Upload in Vendor Portal > Enterprise Portal settings:
- Custom logo (LinkVault logo)
- Favicon (bookmark icon)
- Title: "LinkVault"
- Primary/secondary colors

#### 6.2 — Custom email sender

Buy a domain (e.g., `linkvault.dev`). Set up:
- SPF TXT record
- DKIM CNAME record (from Replicated)
- Return-Path CNAME record
- Sender: `notifications@linkvault.dev`

Trigger invite email → arrives from your domain, not Replicated.

#### 6.3 — EP Security Center

Log in as customer → Security Center → view CVEs. If the app image has CVEs:
- Use Replicated's language-level securebuild base image
- Rebuild with `FROM` securebuild image
- Reduce CVE count

#### 6.4 — EP custom docs with GitHub app

1. Create a GitHub repo for docs (e.g., `linkvault-docs`)
2. Install the Replicated GitHub app into your vendor team
3. Customize left nav and main content in EP to match LinkVault install/operating instructions

#### 6.5 — Helm chart reference in EP docs

In `toc.yaml` for the docs repo, add a generated Helm chart reference section. At least 1 field should be intentionally undocumented in the reference.

#### 6.6 — Terraform modules in EP docs

Create a simple (can be fake) Terraform module for LinkVault. Include it in the EP docs. Enable/disable display via a custom license field.

#### 6.8 — EP self-serve sign-up

Enable self-serve in EP settings. Share the sign-up URL. Complete sign-up as a new user. Show the customer record in Vendor Portal > Customers.

#### 6.9 — End-to-end install via EP (both paths)

1. Invite customer to EP
2. As customer, follow Helm install instructions → running app
3. As customer, follow EC install instructions → running app

Tests that EP instructions are accurate and complete.

#### 6.10 — Upgrade without downtime

Test upgrade instructions work for both Helm (`helm upgrade`) and EC (Admin Console upgrade). App stays accessible during upgrade.

---

### Tier 6: Operationalize It

#### 7.1 — Notifications

Set up in Vendor Portal:
- Email notifications: instance online/offline, license expiring
- Webhook notifications: release promoted, instance status change
- Demo by triggering each

#### 7.2 — Security posture

- Run Trivy on the image, review CVEs
- Explain which are from base OS vs app deps
- Describe remediation plan for each
- Know the CVSS scores and exploitability

#### 7.3 — Sign images

Use cosign to sign the Docker image:
```bash
cosign sign --key cosign.key $REGISTRY/linkvault:v1.0.0
```
Verify:
```bash
cosign verify --key cosign.pub $REGISTRY/linkvault:v1.0.0
```

#### 7.4 — Network policy for airgap

Use CMX network policy option:
1. Install LinkVault in airgap mode
2. Enable network policy monitoring
3. Exercise all app functionality (create bookmarks, search, etc.)
4. Download network policy report
5. Report shows 0 outbound requests

---

## Project Structure

```
linkvault/
├── main.go
├── store/
│   ├── postgres.go          # DB operations (bookmarks, tags, collections)
│   └── migrations.go        # Auto-run CREATE TABLE on startup
├── cache/
│   └── redis.go             # Redis client (search cache, rate limiting)
├── license/
│   └── license.go           # SDK license field reads + enforcement
├── sdk/
│   └── sdk.go               # Custom metrics reporting, update checks
├── web/
│   ├── handler.go           # HTTP handlers
│   ├── embed.go             # Go embed directive
│   └── static/
│       ├── index.html        # Main UI
│       ├── shared.html       # Public collection view
│       ├── app.js            # Frontend logic
│       └── style.css         # Styling
├── Dockerfile
├── go.mod
├── go.sum
├── .dockerignore
├── .gitignore
├── .replicated
├── .github/
│   ├── workflows/
│   │   ├── pr.yml            # PR check workflow
│   │   └── release.yml       # Merge-to-main release workflow
│   └── dependabot.yml
├── helm/
│   └── linkvault/
│       ├── Chart.yaml
│       ├── values.yaml
│       ├── values.schema.json
│       └── templates/
│           ├── _helpers.tpl
│           ├── deployment.yaml
│           ├── service.yaml
│           ├── ingress.yaml
│           ├── preflight.yaml
│           ├── support-bundle.yaml
│           └── NOTES.txt
└── release/
    ├── linkvault-helmchart.yaml
    ├── application.yaml
    ├── config.yaml
    └── embedded-cluster-config.yaml
```

---

## Custom Domains to Set Up

| Service | Subdomain | Aliases |
|---|---|---|
| Proxy Registry | `proxy.linkvault.dev` | `proxy.replicated.com` |
| Download Portal | `get.linkvault.dev` | `replicated.app` |
| Image Registry | `registry.linkvault.dev` | `registry.replicated.com` |

---

## License Fields Summary

| Field | Type | Default | Gated Feature |
|---|---|---|---|
| `max_bookmarks` | Integer | 100 | Bookmark creation limit |
| `feature_tier` | String | "free" | Feature tier (free/pro/enterprise) |
| `search_enabled` | Boolean | false | Full-text search |
| `public_collections` | Boolean | false | Public shareable collections |
| `import_export` | Boolean | false | Bulk import/export |

---

## Build Order

### Phase 1: Application (Days 1-2)
1. Go module init, database schema, store package
2. Web handlers + embedded HTML/CSS/JS UI
3. Bookmark CRUD with auto-fetch
4. Tags and collections
5. Public share page
6. Search (when enabled)
7. Dockerfile + local testing

### Phase 2: Helm Chart (Day 3)
8. Chart.yaml with PostgreSQL + Redis + Replicated dependencies
9. values.yaml with all config
10. values.schema.json
11. Templates: deployment (with init containers + BYO conditionals), service, ingress (with TLS)
12. Preflight spec (5 checks)
13. Support bundle spec (all collectors + analyzers)
14. helm lint + helm template verification

### Phase 3: Replicated Setup (Day 4)
15. Vendor Portal: app, license fields, channels
16. Custom domains + DNS
17. SDK integration: custom metrics, license reads, update checks
18. License enforcement in app code
19. Support bundle UI button
20. RBAC policy + service account

### Phase 4: CI/CD (Day 5)
21. Private registry setup
22. PR workflow
23. Release workflow (merge to main → Unstable)
24. Image scanning (Trivy)
25. Dependabot config

### Phase 5: KOTS / EC (Days 6-7)
26. release/ manifests: helmchart.yaml, config.yaml, application.yaml, EC config
27. Config screen wiring
28. LicenseFieldValue template functions
29. EC install test on CMX VM
30. Upgrade test (data persistence)
31. Air gap build + install

### Phase 6: Enterprise Portal (Day 8)
32. Branding + email domain
33. GitHub app + custom docs
34. Helm chart reference + terraform modules in docs
35. Self-serve sign-up
36. E2E install via EP (both paths)

### Phase 7: Operationalize (Day 9)
37. Notifications (email + webhook)
38. Image signing
39. CVE analysis + security posture
40. Network policy test for airgap

### Phase 8: Polish + Demo Prep (Day 10)
41. Verify all rubric items
42. Prepare demo scripts for each tier
43. Clean up, tag final release
