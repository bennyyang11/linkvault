package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"sync"
	"time"
)

type Metrics interface {
	CountBookmarks() int
	BookmarksAddedToday() int
	CountCollections() int
	CountTags() int
	StorageUsedMB() float64
	SearchesToday() int64
}

type UpdateInfo struct {
	VersionLabel string `json:"versionLabel"`
	CreatedAt    string `json:"createdAt"`
	IsRequired   bool   `json:"isRequired"`
}

type Client struct {
	sdkAddr    string
	mu         sync.RWMutex
	updateInfo *UpdateInfo
}

func NewClient(sdkAddr string) *Client {
	return &Client{sdkAddr: sdkAddr}
}

func (c *Client) SDKAddr() string { return c.sdkAddr }

func (c *Client) ReportLoop(ctx context.Context, m Metrics, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.reportMetrics(m)
		}
	}
}

func (c *Client) reportMetrics(m Metrics) {
	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"total_bookmarks":      m.CountBookmarks(),
			"bookmarks_added_today": m.BookmarksAddedToday(),
			"collections_count":    m.CountCollections(),
			"tags_count":           m.CountTags(),
			"storage_used_mb":      math.Round(m.StorageUsedMB()*10) / 10,
			"searches_today":       m.SearchesToday(),
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(c.sdkAddr+"/api/v1/app/custom-metrics", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("Failed to report metrics: %v", err)
		return
	}
	resp.Body.Close()
}

func (c *Client) UpdateCheckLoop(ctx context.Context, interval time.Duration) {
	c.checkUpdates()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.checkUpdates()
		}
	}
}

func (c *Client) checkUpdates() {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(c.sdkAddr + "/api/v1/app/updates")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var updates []UpdateInfo
	if err := json.NewDecoder(resp.Body).Decode(&updates); err != nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if len(updates) > 0 {
		c.updateInfo = &updates[len(updates)-1]
	} else {
		c.updateInfo = nil
	}
}

func (c *Client) GetUpdateInfo() *UpdateInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.updateInfo
}
