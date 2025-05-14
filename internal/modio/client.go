package modio

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ShawnEdgell/modio-api-go/internal/config"
)

const (
	apiPageSize    = 100
	defaultSort    = "-date_updated"
	requestTimeout = 20 * time.Second
	requestDelay   = 500 * time.Millisecond
)

type Client struct {
	httpClient *http.Client
	apiKey     string
	gameID     string
	apiDomain  string
}

func NewClient(cfg *config.AppConfig) (*Client, error) {
	if cfg.ModioAPIKey == "" {
		return nil, fmt.Errorf("mod.io API key is not configured")
	}
	return &Client{
		httpClient: &http.Client{Timeout: requestTimeout},
		apiKey:     cfg.ModioAPIKey,
		gameID:     cfg.ModioGameID,
		apiDomain:  cfg.ModioAPIDomain,
	}, nil
}

func (c *Client) fetchGenericPaginatedData(ctx context.Context, path string, queryParams url.Values, responsePayload interface{}) error {
	actualParams := url.Values{}
	for k, v := range queryParams { // Copy to avoid modifying caller's params map
		actualParams[k] = v
	}
	actualParams.Add("api_key", c.apiKey)

	u := url.URL{
		Scheme:   "https",
		Host:     c.apiDomain,
		Path:     path,
		RawQuery: actualParams.Encode(),
	}

	loggingParams := url.Values{}
	for k, v := range actualParams {
		if k != "api_key" {
			loggingParams[k] = v
		}
	}
	slog.Debug("Preparing to fetch from Mod.io", "url_path", u.Path, "params_for_log", loggingParams.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request for %s: %w", u.Path, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make GET request to %s: %w", u.Path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mod.io API request to %s failed with status %s", u.Path, resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(responsePayload); err != nil {
		return fmt.Errorf("failed to decode JSON response from %s: %w", u.Path, err)
	}
	return nil
}

func (c *Client) FetchAllItems(ctx context.Context, itemTypeTag string, maxPagesToFetch int) ([]Mod, error) {
	var allItems []Mod
	path := fmt.Sprintf("/v1/games/%s/mods", c.gameID)
	slog.Info("Starting to fetch all items from Mod.io", "type_tag", itemTypeTag, "max_pages_limit", maxPagesToFetch, "path", path)

	for page := 0; page < maxPagesToFetch; page++ {
		currentOffset := page * apiPageSize
		queryParams := url.Values{}
		if itemTypeTag != "" {
			queryParams.Add("tags-in", itemTypeTag)
		}
		queryParams.Add("_sort", defaultSort)
		queryParams.Add("_limit", strconv.Itoa(apiPageSize))
		queryParams.Add("_offset", strconv.Itoa(currentOffset))

		slog.Debug("Fetching page for Mod.io items", "type_tag", itemTypeTag, "page_number", page+1)

		var apiResponse ModioAPIResponse
		err := c.fetchGenericPaginatedData(ctx, path, queryParams, &apiResponse)
		if err != nil {
			slog.Error("Failed to fetch a page from Mod.io", "type_tag", itemTypeTag, "page_number", page+1, "error", err)
			return nil, fmt.Errorf("failed to fetch page %d for type %s: %w", page+1, itemTypeTag, err)
		}

		if len(apiResponse.Data) > 0 {
			allItems = append(allItems, apiResponse.Data...)
		}

		if len(apiResponse.Data) < apiPageSize || apiResponse.ResultCount < apiPageSize {
			slog.Info("Fetched last page for items or API limit reached", "type_tag", itemTypeTag, "items_on_this_page", len(apiResponse.Data), "api_result_count", apiResponse.ResultCount)
			break
		}

		if page < maxPagesToFetch-1 {
			slog.Debug("Sleeping between Mod.io paged requests", "duration", requestDelay)
			select {
			case <-time.After(requestDelay):
			case <-ctx.Done():
				return allItems, ctx.Err()
			}
		}
	}
	slog.Info("Finished fetching all items from Mod.io", "type_tag", itemTypeTag, "total_items_fetched", len(allItems))
	return allItems, nil
}

func (c *Client) CheckForNewerMods(ctx context.Context, itemTypeTag string, sinceTimestamp int64) (bool, error) {
	slog.Debug("Checking for newer mods via /mods endpoint", "type_tag", itemTypeTag, "since_timestamp", sinceTimestamp)
	path := fmt.Sprintf("/v1/games/%s/mods", c.gameID)
	queryParams := url.Values{}
	if itemTypeTag != "" {
		queryParams.Add("tags-in", itemTypeTag)
	}
	queryParams.Add("date_updated-min", strconv.FormatInt(sinceTimestamp+1, 10))
	queryParams.Add("_sort", "date_updated")
	queryParams.Add("_limit", "1")
	queryParams.Add("_offset", "0")

	var modsResponse ModioAPIResponse
	err := c.fetchGenericPaginatedData(ctx, path, queryParams, &modsResponse)
	if err != nil {
		return false, fmt.Errorf("failed to check for newer mods (type: %s, since: %d): %w", itemTypeTag, sinceTimestamp, err)
	}

	if len(modsResponse.Data) > 0 {
		slog.Info("Newer mods found via /mods endpoint", "type_tag", itemTypeTag, "count", len(modsResponse.Data))
		return true, nil
	}
	slog.Debug("No newer mods found via /mods endpoint", "type_tag", itemTypeTag)
	return false, nil
}

func (c *Client) FetchModEvents(ctx context.Context, sinceTimestamp int64, offset int, limit int) (*ModioEventsAPIResponse, error) {
	path := fmt.Sprintf("/v1/games/%s/mods/events", c.gameID)
	queryParams := url.Values{}
	if sinceTimestamp > 0 {
		queryParams.Add("date_added-min", strconv.FormatInt(sinceTimestamp+1, 10))
	}
	queryParams.Add("_sort", "date_added") // Process events chronologically
	queryParams.Add("_limit", strconv.Itoa(limit))
	queryParams.Add("_offset", strconv.Itoa(offset))

	slog.Info("Fetching mod events from Mod.io", "since_timestamp", sinceTimestamp, "offset", offset, "limit", limit)

	var eventsResponse ModioEventsAPIResponse
	err := c.fetchGenericPaginatedData(ctx, path, queryParams, &eventsResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch mod events: %w", err)
	}
	return &eventsResponse, nil
}

func (c *Client) GetModDetails(ctx context.Context, modID int) (*Mod, error) {
	path := fmt.Sprintf("/v1/games/%s/mods/%d", c.gameID, modID)
	actualParams := url.Values{} // Only api_key needed here
	actualParams.Add("api_key", c.apiKey)

	u := url.URL{
		Scheme:   "https",
		Host:     c.apiDomain,
		Path:     path,
		RawQuery: actualParams.Encode(),
	}

	slog.Info("Fetching mod details from Mod.io", "mod_id", modID)

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for mod details (id: %d): %w", modID, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make GET request for mod details (id: %d): %w", modID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		slog.Warn("Mod not found on Mod.io during GetModDetails", "mod_id", modID)
		return nil, nil // Return nil, nil to indicate not found explicitly
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mod.io API request for mod details (id: %d) failed with status %s", modID, resp.Status)
	}

	var mod Mod
	if err := json.NewDecoder(resp.Body).Decode(&mod); err != nil {
		return nil, fmt.Errorf("failed to decode JSON response for mod details (id: %d): %w", modID, err)
	}
	return &mod, nil
}