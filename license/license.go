package license

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type Fields struct {
	MaxBookmarks      int    `json:"max_bookmarks"`
	FeatureTier       string `json:"feature_tier"`
	SearchEnabled     bool   `json:"search_enabled"`
	PublicCollections bool   `json:"public_collections"`
	ImportExport      bool   `json:"import_export"`
	ExpiresAt         string `json:"expires_at"`
}

type Checker struct {
	sdkAddr   string
	mu        sync.RWMutex
	fields    Fields
	loaded    bool
	lastError string
}

func NewChecker(sdkAddr string) *Checker {
	return &Checker{
		sdkAddr: sdkAddr,
		fields: Fields{
			MaxBookmarks: 0,
			FeatureTier:  "",
		},
	}
}

func (c *Checker) RefreshLoop(ctx context.Context, interval time.Duration) {
	c.refresh()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refresh()
		}
	}
}

func (c *Checker) refresh() {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(c.sdkAddr + "/api/v1/license/fields")
	if err != nil {
		c.mu.Lock()
		c.lastError = err.Error()
		c.mu.Unlock()
		return
	}
	defer resp.Body.Close()

	var result map[string]struct {
		Name  string          `json:"name"`
		Title string          `json:"title"`
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.mu.Lock()
		c.lastError = "failed to decode license fields: " + err.Error()
		c.mu.Unlock()
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for key, f := range result {
		switch key {
		case "max_bookmarks":
			json.Unmarshal(f.Value, &c.fields.MaxBookmarks)
		case "feature_tier":
			json.Unmarshal(f.Value, &c.fields.FeatureTier)
		case "search_enabled":
			json.Unmarshal(f.Value, &c.fields.SearchEnabled)
		case "public_collections":
			json.Unmarshal(f.Value, &c.fields.PublicCollections)
		case "import_export":
			json.Unmarshal(f.Value, &c.fields.ImportExport)
		case "expires_at":
			json.Unmarshal(f.Value, &c.fields.ExpiresAt)
		}
	}
	c.loaded = true
	c.lastError = ""
	log.Printf("License refreshed: tier=%s, max_bookmarks=%d", c.fields.FeatureTier, c.fields.MaxBookmarks)
}

func (c *Checker) GetFields() Fields {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.fields
}

func (c *Checker) IsLoaded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loaded
}

func (c *Checker) IsExpired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.fields.ExpiresAt == "" {
		return false
	}
	exp, err := time.Parse(time.RFC3339, c.fields.ExpiresAt)
	if err != nil {
		exp, err = time.Parse("2006-01-02", c.fields.ExpiresAt)
		if err != nil {
			return false
		}
	}
	return time.Now().After(exp)
}

func (c *Checker) DaysUntilExpiry() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.fields.ExpiresAt == "" {
		return 999
	}
	exp, err := time.Parse(time.RFC3339, c.fields.ExpiresAt)
	if err != nil {
		exp, err = time.Parse("2006-01-02", c.fields.ExpiresAt)
		if err != nil {
			return 999
		}
	}
	return int(time.Until(exp).Hours() / 24)
}

func (c *Checker) EnforceLimits(currentCount int) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.loaded {
		return nil
	}
	if c.fields.MaxBookmarks > 0 && currentCount >= c.fields.MaxBookmarks {
		return fmt.Errorf("bookmark limit reached: %d/%d (upgrade your plan for more)", currentCount, c.fields.MaxBookmarks)
	}
	return nil
}

func (c *Checker) IsFeatureEnabled(feature string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.loaded {
		return true
	}
	switch feature {
	case "search":
		return c.fields.SearchEnabled
	case "public_collections":
		return c.fields.PublicCollections
	case "import_export":
		return c.fields.ImportExport
	}
	return false
}

func (c *Checker) LastError() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastError
}
