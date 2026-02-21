package pterodactyl

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Client interacts with the Pterodactyl panel client API.
type Client struct {
	PanelURL string
	APIKey   string
	HTTP     *http.Client
}

// NewClient creates a new Pterodactyl API client.
func NewClient(panelURL, apiKey string) *Client {
	return &Client{
		PanelURL: strings.TrimRight(panelURL, "/"),
		APIKey:   apiKey,
		HTTP:     &http.Client{Timeout: 30 * time.Second},
	}
}

// Backup represents a single backup entry from the Pterodactyl API.
type Backup struct {
	UUID        string    `json:"uuid"`
	Name        string    `json:"name"`
	IsSuccessful bool    `json:"is_successful"`
	IsLocked    bool      `json:"is_locked"`
	Bytes       int64     `json:"bytes"`
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

type backupListResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object     string `json:"object"`
		Attributes Backup `json:"attributes"`
	} `json:"data"`
}

type downloadResponse struct {
	Object     string `json:"object"`
	Attributes struct {
		URL string `json:"url"`
	} `json:"attributes"`
}

func (c *Client) doRequest(method, path string) ([]byte, error) {
	url := c.PanelURL + path

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned status %d for %s: %s", resp.StatusCode, url, string(body))
	}

	return body, nil
}

// ListBackups returns all backups for a given server, sorted by creation time
// (newest first).
func (c *Client) ListBackups(serverID string) ([]Backup, error) {
	body, err := c.doRequest("GET", "/api/client/servers/"+serverID+"/backups")
	if err != nil {
		return nil, err
	}

	var result backupListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding backup list: %w", err)
	}

	var backups []Backup
	for _, d := range result.Data {
		backups = append(backups, d.Attributes)
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

// GetLatestBackup returns the most recent successful backup for a server.
func (c *Client) GetLatestBackup(serverID string) (*Backup, error) {
	backups, err := c.ListBackups(serverID)
	if err != nil {
		return nil, err
	}

	for i := range backups {
		if backups[i].IsSuccessful {
			return &backups[i], nil
		}
	}

	return nil, fmt.Errorf("no successful backup found for server %s", serverID)
}

// GetBackupDownloadURL returns a signed download URL for the given backup.
func (c *Client) GetBackupDownloadURL(serverID, backupUUID string) (string, error) {
	body, err := c.doRequest("GET", "/api/client/servers/"+serverID+"/backups/"+backupUUID+"/download")
	if err != nil {
		return "", err
	}

	var result downloadResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decoding download URL response: %w", err)
	}

	if result.Attributes.URL == "" {
		return "", fmt.Errorf("empty download URL returned for backup %s", backupUUID)
	}

	return result.Attributes.URL, nil
}
