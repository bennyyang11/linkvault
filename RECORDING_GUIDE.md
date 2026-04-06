# LinkVault — Video Recording Guide

Each tier requires a recorded video demonstrating the acceptance criteria. This guide tells you exactly what to show on screen for each task.

---

## Tier 0: Build It

### 0.1 — Custom web app with stateful component
**Show on video:**
- [ ] App running locally (`go run .` or `docker-compose up` with PostgreSQL)
- [ ] Open the app in a browser — show the bookmark manager UI working
- [ ] Show the custom `Dockerfile` in your editor
- [ ] Show that PostgreSQL stores the data (create a bookmark, show it persists)

### 0.2 — Helm chart packages and deploys
**Show on video:**
- [ ] Run `helm lint helm/linkvault/` — no errors
- [ ] Open `values.schema.json` in your editor — show it validates image, service, etc.
- [ ] `helm install` the app on a cluster
- [ ] Open the app in a browser from the cluster

### 0.3 — 2 open-source subcharts with BYO
**Show on video:**
- [ ] Open `Chart.yaml` — show `postgresql` and `redis` dependencies with `condition:` field
- [ ] **Embedded mode (default):** `kubectl get pods` — show PostgreSQL and Redis pods Running
- [ ] **BYO mode:** Install with `--set postgresql.enabled=false --set externalPostgresql.host=...` — show NO PostgreSQL pod, app is using the external instance
- [ ] Toggle back: set `postgresql.enabled=true` — show the stateful component pod Running again

### 0.4 — Kubernetes best practices
**Show on video:**
- [ ] Open `deployment.yaml` — show `livenessProbe` and `readinessProbe` defined
- [ ] Open `deployment.yaml` — show `resources.requests` and `resources.limits` on ALL containers
- [ ] `curl /healthz` (or port-forward) — show structured JSON response: `{"status":"ok","postgres":"connected","redis":"connected"}`
- [ ] Create some bookmarks in the app
- [ ] `kubectl delete pod <linkvault-pod>` — pod restarts
- [ ] Refresh the app — bookmarks are still there (data persisted in PostgreSQL PVC)

### 0.5 — HTTPS with certificate options
**Show on video:**
- [ ] Open `ingress.yaml` — show the 3 TLS options (cert-manager, manual secret, self-signed)
- [ ] Open `values.yaml` — show `ingress.tls` section with the 3 options
- [ ] Open the app at `https://<your-domain>` in a browser
- [ ] Click the lock icon — show a valid TLS certificate

### 0.6 — App waits for database before starting
**Show on video:**
- [ ] Open `deployment.yaml` — show the `initContainers` section (wait-for-postgres, wait-for-redis)
- [ ] Fresh install: `kubectl get pods -w` — show the init containers completing before the main container starts
- [ ] No crash-loops visible

### 0.7 — 2 user-facing demoable features
**Show on video:**
- [ ] **Feature 1 — Bookmark Management with Auto-Fetch:** Paste a URL → show title/description auto-populated → add tags → filter by tag → delete a bookmark
- [ ] **Feature 2 — Public Collections with Shareable Links:** Create a collection → add bookmarks → toggle "Share" → copy the public link → open it in an incognito window → show the read-only public view

---

## Tier 1: Automate It

### 1.1 — Container images built and pushed to private registry in CI
**Show on video:**
- [ ] Show the GitHub Actions run log — highlight the Docker build and push steps
- [ ] Go to Docker Hub → show the `bennyyang11/linkvault` image with tags (private repo)

### 1.2a — Scoped RBAC policy
**Show on video:**
- [ ] Vendor Portal → Team → RBAC → show the "CI Release Bot" policy with `denied: ["**/*"]` baseline
- [ ] Show the Service Account assigned to the policy

### 1.2b — PR workflow with .replicated file
**Show on video:**
- [ ] Open `.replicated` file — show `appSlug`, `charts`, `manifests` config
- [ ] Open a test PR → show the PR Check workflow running
- [ ] Show the passing Actions run (build + lint)

### 1.3 — Release workflow promotes to Unstable
**Show on video:**
- [ ] Push a tag (`git tag v1.x.0 && git push origin v1.x.0`)
- [ ] Show the Release workflow running and completing
- [ ] Vendor Portal → Releases → show the new release on the Unstable channel

### 1.4 — Email notifications on Stable promotion
**Show on video:**
- [ ] Vendor Portal → Notifications → show the email notification rule (trigger: promoted to Stable)
- [ ] Promote a release to Stable
- [ ] Show the email arriving at your @replicated.com address

---

## Tier 2: Ship It with Helm

### 2.1 — SDK subchart renamed for branding
**Show on video:**
- [ ] `kubectl get deployment linkvault-linkvault-sdk -n <namespace>` — shows Running
- [ ] Show the `alias: linkvault-sdk` in `Chart.yaml`

### 2.2 — All images proxied through custom domain
**Show on video:**
- [ ] Run: `kubectl get pods -A -o custom-columns='STATUS:.status.phase,IMAGE:.spec.containers[*].image'`
- [ ] Show EVERY app image starts with `proxy.link-vaults.com` and all are Running
- [ ] Subchart images (PostgreSQL, Redis) also use the proxy domain

### 2.4 — Custom metrics visible in Vendor Portal
**Show on video:**
- [ ] Create a few bookmarks, collections, run some searches
- [ ] Vendor Portal → Instances → Instance Details → Custom Metrics
- [ ] Show at least one meaningful metric (e.g., `total_bookmarks`, `collections_count`)

### 2.5 — License entitlement gates a feature via SDK
**Show on video:**
- [ ] Install with `search_enabled=false` on the license
- [ ] Show the search bar is disabled/locked in the app — "Search requires Pro plan"
- [ ] Vendor Portal → Customer → update `search_enabled=true` (no redeploy)
- [ ] Refresh the app — search now works
- [ ] Show the app code queries the SDK directly (`license/license.go` → `http.Get(sdkAddr + "/api/v1/license/fields")`)

### 2.6a — Update available banner
**Show on video:**
- [ ] Deploy version 1.x.0
- [ ] Promote a newer version to the same channel
- [ ] Wait ~5 minutes (or restart the app)
- [ ] Show the blue update banner appearing in the app: "Update available: v1.x.0"

### 2.6b — License validity enforced via SDK
**Show on video:**
- [ ] Show the app running normally with a valid license
- [ ] Vendor Portal → Customer → set expiration to a past date
- [ ] Wait for license refresh (or restart app)
- [ ] Show the app displaying an expired overlay / blocking access
- [ ] Reset expiration to the future → app returns to normal

### 2.7 — Optional ingress
**Show on video:**
- [ ] Show `values.yaml` — `ingress.enabled: false` (default)
- [ ] Show it works with `--set ingress.enabled=true` — traffic routes to the app

### 2.8 — Service type is configurable
**Show on video:**
- [ ] Show `values.yaml` — `service.type: ClusterIP`
- [ ] Show it works with `--set service.type=NodePort`

### 2.9 — Instance is live, named, tagged
**Show on video:**
- [ ] Vendor Portal → Instances → show the instance is live and reporting
- [ ] Name it (e.g., "Acme Corp - Production")
- [ ] Add tags (e.g., "bootcamp", "pro-tier")
- [ ] Show it reporting healthy with custom metrics

### 2.10 — Services show healthy in instance reporting
**Show on video:**
- [ ] Vendor Portal → Instance Details → show all services (linkvault, postgresql, redis, SDK) reporting as healthy

---

## Tier 3: Support It

### 3.1 — Preflight checks (5 required)
**Show on video — run twice:**

**Run 1 — Failing scenarios (show some/all failing):**
- [ ] (1) External DB connectivity: configure BYO with unreachable host → fails with actionable message
- [ ] (2) External endpoint connectivity: show it fails when unreachable
- [ ] (3) Cluster resources: show CPU/memory check
- [ ] (4) K8s version: show it would fail below 1.25.0
- [ ] (5) Distribution: run on docker-desktop → fails naming the distro and linking to supported options

**Run 2 — All passing:**
- [ ] Run preflights on a supported cluster with sufficient resources → all green

### 3.2 — Log collection covers all components
**Show on video:**
- [ ] Open `support-bundle.yaml` — show separate logs collectors for: app, PostgreSQL, Redis, SDK
- [ ] Each has `maxLines` or `maxAge` limits
- [ ] Run the support bundle
- [ ] Open the output — show each component's log directory is present and non-empty

### 3.3 — Health endpoint with http collector + textAnalyze
**Show on video:**
- [ ] Open `support-bundle.yaml` — show the `http` collector calling `/healthz` via in-cluster service DNS
- [ ] Show the `textAnalyze` analyzer parsing for `"status":"ok"`
- [ ] **Passing:** Run bundle with app healthy → analyzer shows pass
- [ ] **Failing:** Scale app to 0 replicas → run bundle → analyzer shows fail

### 3.4 — Status analyzers for all workload types
**Show on video:**
- [ ] Open `support-bundle.yaml` — show `deploymentStatus` (app, redis, SDK) and `statefulsetStatus` (PostgreSQL)
- [ ] Failure messages name the component and describe operational impact
- [ ] **Demo failure:** `kubectl scale deployment linkvault --replicas=0` → run bundle → show analyzer surfaces it with actionable message

### 3.5 — textAnalyze catches a known app failure pattern
**Show on video:**
- [ ] Open `support-bundle.yaml` — show the `textAnalyze` with regex `pq: (connection refused|no such host|...)` searching app log files
- [ ] Failure message includes remediation step and documentation link
- [ ] **Demo:** Misconfigure the DB password → show the error in logs → run bundle → analyzer fires on the pattern

### 3.6 — Storage class and node readiness
**Show on video:**
- [ ] Open `support-bundle.yaml` — show `storageClass` analyzer (fails when no default)
- [ ] Show `nodeResources` analyzer (fails when any node not Ready)
- [ ] Both have clear, actionable failure messages

### 3.7 — Support bundle from app UI → uploaded to Vendor Portal
**Show on video:**
- [ ] Open the app → Admin panel → click "Support Bundle" button
- [ ] Show it triggering generation
- [ ] Vendor Portal → Instance Details → show the bundle appearing
- [ ] Open the bundle → walk through collected data and analyzer results

---

## Tier 4: Ship It on a VM (Embedded Cluster v3)

### 4.1 — EC install on bare VM
**Show on video:**
- [ ] Start with a fresh CMX VM (show it's bare)
- [ ] Download and run the embedded cluster installer
- [ ] `sudo k0s kubectl get pods -A` — all pods Running
- [ ] Open the app in a browser

### 4.2 — In-place upgrade without data loss
**Show on video:**
- [ ] Install release 1 → create bookmarks/collections in the app
- [ ] Promote release 2
- [ ] Trigger upgrade via the installer
- [ ] Show the data is still present after upgrade
- [ ] `sudo k0s kubectl get pods -A` — all pods Running

### 4.3 — Air-gapped install
**Show on video:**
- [ ] Build an air gap bundle from the release
- [ ] Transfer it to a VM with no internet
- [ ] Install using only the bundle
- [ ] `sudo k0s kubectl get pods -A` — all pods Running
- [ ] Open the app in a browser

### 4.6 — App icon and name
**Show on video:**
- [ ] Screenshot of the installer showing the correct LinkVault icon and "LinkVault" app name

### 4.7 — License entitlement gates a configurable feature (EC path)
**Show on video:**
- [ ] License has `search_enabled=false` → config screen hides/locks the Search feature toggle
- [ ] App: search feature is unavailable
- [ ] Update license to `search_enabled=true`
- [ ] Config screen: Search toggle is now visible and configurable
- [ ] Enable it, show it working in the app
- [ ] Show `LicenseFieldValue` template in `helmchart.yaml`

---

## Tier 5: Config Screen (at least 3 meaningful capabilities)

### 5.0 — External DB toggle with conditional fields
**Show on video:**
- [ ] **Embedded install:** Config screen shows "Embedded PostgreSQL" selected → host/port/credential fields are hidden
- [ ] `sudo k0s kubectl get pods -A` — PostgreSQL pod is Running
- [ ] **External install:** Config screen shows "External PostgreSQL" → connection fields appear (host, port, credentials)
- [ ] No PostgreSQL pod → app is using the external instance

### 5.1 — Configurable app features (2+ required)
**Show on video:**
- [ ] **Feature 1 (Public Collections):** Enable via config screen → share button works in app → disable → share button gone
- [ ] **Feature 2 (Full-Text Search):** Enable via config screen → search works → disable → search disabled

### 5.2 — Generated default survives upgrade
**Show on video:**
- [ ] First install → app connects to embedded PostgreSQL (auto-generated password)
- [ ] Perform an upgrade (promote new release, apply)
- [ ] App still connects to the database — no reconfiguration needed
- [ ] Show the `embedded_db_password` config item uses `RandomString 32` with `hidden: true`

### 5.3 — Input validation
**Show on video:**
- [ ] Config screen → PostgreSQL Port field → enter "abc" → show validation error: "Port must be a number..."
- [ ] Enter "5432" → validation passes
- [ ] Import File Size field → enter "10" → validation error → enter "10MB" → passes

### 5.4 — Help text on all config items
**Show on video:**
- [ ] Scroll through the config screen
- [ ] Show `help_text` on every single item
- [ ] Each describes what the field does and what valid values look like (not just restating the label)

---

## Tier 6: Deliver It (Enterprise Portal v2)

### 6.1 — EP branding & identity
**Show on video:**
- [ ] Screenshot of Enterprise Portal showing: custom logo, favicon, title "LinkVault", primary/secondary colors

### 6.2 — Custom email sender
**Show on video:**
- [ ] Trigger an invitation email from EP
- [ ] Show the email arriving from `notifications@link-vaults.com` (your domain, not Replicated)
- [ ] Show SPF/DKIM/Return-Path DNS records are configured

### 6.3 — EP Security Center
**Show on video:**
- [ ] Log in as customer → Security Center
- [ ] Show the CVE list for the application image
- [ ] If CVEs exist: explain them, show using securebuild base image to reduce count

### 6.4 — EP custom setup / instructions
**Show on video:**
- [ ] Show the GitHub docs repo integrated via the Replicated GitHub app
- [ ] Show customized left nav and main content in EP relevant to LinkVault

### 6.5 — Helm chart reference in EP docs
**Show on video:**
- [ ] Show `toc.yaml` with the generated Helm chart reference section
- [ ] Show the rendered chart reference in EP
- [ ] Point out at least 1 field that is intentionally not documented

### 6.6 — Terraform modules in EP docs
**Show on video:**
- [ ] Show the (fake) Terraform module included in EP docs
- [ ] Show it's enabled/disabled by a custom license field

### 6.8 — EP self-serve sign-up
**Show on video:**
- [ ] Share the sign-up URL
- [ ] Complete the sign-up flow as a new user
- [ ] Vendor Portal → Customers → show the new customer record

### 6.9 — End-to-end install via EP (both paths)
**Show on video:**
- [ ] Invite customer to EP
- [ ] **Helm path:** As customer, follow EP instructions → running app
- [ ] **EC path:** As customer, follow EP instructions → running app

### 6.10 — Upgrade without downtime
**Show on video:**
- [ ] **Helm:** `helm upgrade` → app stays accessible during upgrade
- [ ] **EC:** Upgrade via Admin Console → app stays accessible during upgrade

---

## Tier 7: Operationalize It

### 7.1 — Notifications
**Show on video:**
- [ ] Show email notifications triggered by account activity
- [ ] Show webhook notifications triggered by account activity

### 7.2 — Security posture
**Show on video:**
- [ ] Run Trivy on the image → show the CVE list
- [ ] Explain each CVE: which are from base OS vs app deps
- [ ] Describe how you could reduce them (base image update, dependency update, securebuild)

### 7.3 — Sign images
**Show on video:**
- [ ] Sign the Docker image with cosign
- [ ] Verify the signature

### 7.4 — Network policy for airgap (0 outbound requests)
**Show on video:**
- [ ] Install LinkVault with CMX network policy option enabled
- [ ] Exercise all functionality: create bookmarks, search, collections, share
- [ ] Download the network policy report
- [ ] Show the report confirms 0 outbound requests

---

## Current Status

| Tier | Status | What's Done | What's Left |
|---|---|---|---|
| **0** | ✅ Complete | App, Dockerfile, Helm chart, subcharts, BYO, probes, init containers, TLS options, values.schema.json | — |
| **1** | ✅ Complete | CI builds/pushes to Docker Hub, RBAC policy, PR workflow, release workflow, Dependabot | Email notification for Stable promotion |
| **2** | 🔄 In Progress | SDK subchart aliased, custom domain (proxy.link-vaults.com), license/SDK code written | Deploy to CMX, verify metrics, verify image proxy, test license gating live |
| **3** | ✅ Specs Written | Preflight (5 checks), support bundle (http collector, textAnalyze, all analyzers) | Test on live cluster, demo failures |
| **4** | 📝 Config Written | EC config, application.yaml, helmchart.yaml | EC install test, upgrade test, airgap test |
| **5** | ✅ Config Written | config.yaml with DB toggle, features, validation, help text, generated password | Test on live EC install |
| **6** | Not Started | — | EP branding, email domain, security center, docs, terraform, self-serve, E2E |
| **7** | Not Started | — | Notifications, CVE analysis, image signing, network policy |
