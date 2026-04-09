package web

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"linkvault/cache"
	"linkvault/license"
	"linkvault/sdk"
	"linkvault/store"
)

var (
	titleRe = regexp.MustCompile(`(?i)<title[^>]*>\s*([^<]+?)\s*</title>`)
	descRe  = regexp.MustCompile(`(?i)<meta[^>]+name\s*=\s*["']description["'][^>]+content\s*=\s*["']([^"']+)["']`)
	descRe2 = regexp.MustCompile(`(?i)<meta[^>]+content\s*=\s*["']([^"']+)["'][^>]+name\s*=\s*["']description["']`)
)

func NewRouter(db *store.Store, c *cache.Cache, lic *license.Checker, sdkClient *sdk.Client) http.Handler {
	mux := http.NewServeMux()

	staticSub, _ := fs.Sub(staticFiles, "static")
	fileServer := http.FileServer(http.FS(staticSub))

	// Serve shared page at /shared/{code}
	mux.HandleFunc("GET /shared/{code}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, staticSub.(fs.FS), "shared.html")
	})

	// --- Bookmarks ---
	mux.HandleFunc("GET /api/bookmarks", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		query := r.URL.Query().Get("q")

		if query != "" && !lic.IsFeatureEnabled("search") {
			writeJSON(w, http.StatusForbidden, map[string]interface{}{
				"error":   "Search requires a Pro or Enterprise license",
				"blocked": true,
			})
			return
		}

		bookmarks, err := db.ListBookmarks(tag, query)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, bookmarks)
	})

	mux.HandleFunc("POST /api/bookmarks", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			URL          string   `json:"url"`
			Tags         []string `json:"tags"`
			CollectionID int      `json:"collection_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
			return
		}

		if err := lic.EnforceLimits(db.CountBookmarks()); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]interface{}{
				"error":   err.Error(),
				"blocked": true,
			})
			return
		}

		if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
			req.URL = "https://" + req.URL
		}

		title, description, faviconURL := fetchPageInfo(req.URL)

		bookmark, err := db.CreateBookmark(req.URL, title, description, faviconURL, req.Tags)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if req.CollectionID > 0 {
			db.AddBookmarkToCollection(req.CollectionID, bookmark.ID)
		}

		c.Delete("bookmarks:*")
		writeJSON(w, http.StatusCreated, bookmark)
	})

	mux.HandleFunc("DELETE /api/bookmarks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
			return
		}
		if err := db.DeleteBookmark(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		c.Delete("bookmarks:*")
		writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
	})

	// --- Tags ---
	mux.HandleFunc("GET /api/tags", func(w http.ResponseWriter, r *http.Request) {
		tags, err := db.ListTags()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, tags)
	})

	// --- Collections ---
	mux.HandleFunc("GET /api/collections", func(w http.ResponseWriter, r *http.Request) {
		collections, err := db.ListCollections()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, collections)
	})

	mux.HandleFunc("POST /api/collections", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
			return
		}
		col, err := db.CreateCollection(req.Name, req.Description)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, col)
	})

	mux.HandleFunc("GET /api/collections/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
			return
		}
		col, err := db.GetCollection(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "collection not found"})
			return
		}
		writeJSON(w, http.StatusOK, col)
	})

	mux.HandleFunc("PUT /api/collections/{id}/share", func(w http.ResponseWriter, r *http.Request) {
		if !lic.IsFeatureEnabled("public_collections") {
			writeJSON(w, http.StatusForbidden, map[string]interface{}{
				"error":   "Public collections require a Pro or Enterprise license",
				"blocked": true,
			})
			return
		}
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
			return
		}
		col, err := db.ToggleCollectionPublic(id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, col)
	})

	mux.HandleFunc("POST /api/collections/{id}/bookmarks", func(w http.ResponseWriter, r *http.Request) {
		colID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
			return
		}
		var req struct {
			BookmarkID int `json:"bookmark_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BookmarkID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bookmark_id is required"})
			return
		}
		if err := db.AddBookmarkToCollection(colID, req.BookmarkID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "added"})
	})

	mux.HandleFunc("DELETE /api/collections/{collectionId}/bookmarks/{bookmarkId}", func(w http.ResponseWriter, r *http.Request) {
		colID, _ := strconv.Atoi(r.PathValue("collectionId"))
		bmID, _ := strconv.Atoi(r.PathValue("bookmarkId"))
		if err := db.RemoveBookmarkFromCollection(colID, bmID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "removed"})
	})

	// --- Shared ---
	mux.HandleFunc("GET /api/shared/{code}", func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		col, err := db.GetCollectionByShareCode(code)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "collection not found or not public"})
			return
		}
		writeJSON(w, http.StatusOK, col)
	})

	// --- Import/Export ---
	mux.HandleFunc("GET /api/bookmarks/export", func(w http.ResponseWriter, r *http.Request) {
		if !lic.IsFeatureEnabled("import_export") {
			writeJSON(w, http.StatusForbidden, map[string]interface{}{
				"error":   "Import/Export requires a Pro or Enterprise license",
				"blocked": true,
			})
			return
		}
		bookmarks, err := db.ListBookmarks("", "")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=linkvault-export.json")
		json.NewEncoder(w).Encode(bookmarks)
	})

	mux.HandleFunc("POST /api/bookmarks/import", func(w http.ResponseWriter, r *http.Request) {
		if !lic.IsFeatureEnabled("import_export") {
			writeJSON(w, http.StatusForbidden, map[string]interface{}{
				"error":   "Import/Export requires a Pro or Enterprise license",
				"blocked": true,
			})
			return
		}
		var bookmarks []struct {
			URL  string   `json:"url"`
			Tags []string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&bookmarks); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		imported := 0
		for _, b := range bookmarks {
			if b.URL == "" {
				continue
			}
			if err := lic.EnforceLimits(db.CountBookmarks()); err != nil {
				break
			}
			title, desc, favicon := fetchPageInfo(b.URL)
			if _, err := db.CreateBookmark(b.URL, title, desc, favicon, b.Tags); err == nil {
				imported++
			}
		}
		writeJSON(w, http.StatusOK, map[string]int{"imported": imported})
	})

	// --- License & SDK ---
	mux.HandleFunc("GET /api/license", func(w http.ResponseWriter, r *http.Request) {
		fields := lic.GetFields()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"fields":  fields,
			"loaded":  lic.IsLoaded(),
			"error":   lic.LastError(),
			"expired": lic.IsExpired(),
			"features": map[string]bool{
				"search":             lic.IsFeatureEnabled("search"),
				"public_collections": lic.IsFeatureEnabled("public_collections"),
				"import_export":      lic.IsFeatureEnabled("import_export"),
			},
			"enforcement": map[string]interface{}{
				"bookmarks_used":  db.CountBookmarks(),
				"bookmarks_limit": fields.MaxBookmarks,
			},
			"days_until_expiry": lic.DaysUntilExpiry(),
		})
	})

	mux.HandleFunc("GET /api/updates", func(w http.ResponseWriter, r *http.Request) {
		info := sdkClient.GetUpdateInfo()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"available": info != nil,
			"update":    info,
		})
	})

	// --- Health ---
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		pgStatus := "connected"
		if err := db.Ping(); err != nil {
			pgStatus = "disconnected"
		}
		redisStatus := "connected"
		if !c.IsConnected() {
			redisStatus = "disconnected"
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":   "ok",
			"postgres": pgStatus,
			"redis":    redisStatus,
			"uptime":   fmt.Sprintf("%.1fh", db.UptimeHours()),
		})
	})

	// --- Support Bundle ---
	mux.HandleFunc("POST /api/support-bundle", func(w http.ResponseWriter, r *http.Request) {
		sdkAddr := strings.TrimSuffix(sdkClient.SDKAddr(), "/")

		if err := createSupportBundleJob(sdkAddr); err != nil {
			log.Printf("Support bundle: failed to create job: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to start support bundle collection"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"message": "Support bundle generation started. It will be uploaded to the Vendor Portal automatically."})
	})

	// Static files (catch-all, must be last)
	mux.Handle("/", fileServer)

	return mux
}

func fetchPageInfo(rawURL string) (title, description, faviconURL string) {
	title = rawURL

	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "LinkVault/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return
	}
	page := string(body)

	if m := titleRe.FindStringSubmatch(page); len(m) > 1 {
		title = html.UnescapeString(strings.TrimSpace(m[1]))
	}

	if m := descRe.FindStringSubmatch(page); len(m) > 1 {
		description = html.UnescapeString(strings.TrimSpace(m[1]))
	} else if m := descRe2.FindStringSubmatch(page); len(m) > 1 {
		description = html.UnescapeString(strings.TrimSpace(m[1]))
	}

	// Build favicon URL from the base domain
	if idx := strings.Index(rawURL, "://"); idx > 0 {
		rest := rawURL[idx+3:]
		if slashIdx := strings.Index(rest, "/"); slashIdx > 0 {
			faviconURL = rawURL[:idx+3+slashIdx] + "/favicon.ico"
		} else {
			faviconURL = rawURL + "/favicon.ico"
		}
	}

	return
}

func createSupportBundleJob(sdkAddr string) error {
	token, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return fmt.Errorf("read SA token: %w", err)
	}
	ns, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return fmt.Errorf("read namespace: %w", err)
	}
	namespace := strings.TrimSpace(string(ns))

	jobName := fmt.Sprintf("support-bundle-%d", time.Now().Unix())
	ttl := 300
	backoff := int32(0)

	jobJSON := fmt.Sprintf(`{
  "apiVersion": "batch/v1",
  "kind": "Job",
  "metadata": {
    "name": %q,
    "namespace": %q
  },
  "spec": {
    "backoffLimit": %d,
    "ttlSecondsAfterFinished": %d,
    "template": {
      "spec": {
        "serviceAccountName": "linkvault-support-bundle",
        "restartPolicy": "Never",
        "volumes": [{"name": "bundle", "emptyDir": {}}],
        "initContainers": [{
          "name": "collect",
          "image": "replicated/troubleshoot:latest",
          "command": ["support-bundle"],
          "args": ["--load-cluster-specs", "--interactive=false", "--debug", "-o", "/share/bundle"],
          "volumeMounts": [{"name": "bundle", "mountPath": "/share"}]
        }],
        "containers": [{
          "name": "upload",
          "image": "curlimages/curl:latest",
          "command": ["curl", "--fail", "--silent", "--show-error", "-X", "POST", "-H", "Content-Type: application/gzip", "--data-binary", "@/share/bundle.tar.gz", "%s/api/v1/supportbundle"],
          "volumeMounts": [{"name": "bundle", "mountPath": "/share"}]
        }]
      }
    }
  }
}`, jobName, namespace, backoff, ttl, sdkAddr)

	caCert, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		return fmt.Errorf("read CA cert: %w", err)
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caCert)

	k8sClient := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: certPool},
		},
	}

	apiURL := fmt.Sprintf("https://kubernetes.default.svc/apis/batch/v1/namespaces/%s/jobs", namespace)
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(jobJSON))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+string(token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := k8sClient.Do(req)
	if err != nil {
		return fmt.Errorf("k8s API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("k8s API returned %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("Support bundle Job %q created in namespace %q", jobName, namespace)
	return nil
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
