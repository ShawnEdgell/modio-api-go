// internal/modio/client.go
package modio

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ShawnEdgell/modio-api-go/internal/config" // Adjust to your module path
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
		return nil, fmt.Errorf("Mod.io API key is not configured")
	}
	return &Client{
		httpClient: &http.Client{Timeout: requestTimeout},
		apiKey:     cfg.ModioAPIKey,
		gameID:     cfg.ModioGameID,
		apiDomain:  cfg.ModioAPIDomain,
	}, nil
}

func (c *Client) fetchPage(itemTypeTag string, limit, offset int) (*ModioAPIResponse, error) {
	apiBaseURL := fmt.Sprintf("https://%s/v1/games/%s/mods", c.apiDomain, c.gameID)

	// Prepare query parameters for logging (WITHOUT the API key)
	loggingParams := url.Values{}
	if itemTypeTag != "" {
		loggingParams.Add("tags-in", itemTypeTag)
	}
	loggingParams.Add("_sort", defaultSort)
	loggingParams.Add("_limit", strconv.Itoa(limit))
	loggingParams.Add("_offset", strconv.Itoa(offset))

	slog.Debug("Preparing to fetch from Mod.io",
		"base_url", apiBaseURL,
		"params_for_log", loggingParams.Encode(), // API key is NOT included here
	)

	// Prepare actual query parameters for the request (WITH the API key)
	actualParams := url.Values{}
	actualParams.Add("api_key", c.apiKey) // API key added here for the actual request
	if itemTypeTag != "" {
		actualParams.Add("tags-in", itemTypeTag)
	}
	actualParams.Add("_sort", defaultSort)
	actualParams.Add("_limit", strconv.Itoa(limit))
	actualParams.Add("_offset", strconv.Itoa(offset))

	reqURL := apiBaseURL + "?" + actualParams.Encode() // This is the full URL used for the request

	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to make GET request to Mod.io target [%s?%s]: %w", apiBaseURL, loggingParams.Encode(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// You might want to read a small part of the body here for error context if Mod.io provides it
		// Be careful not to read too much if errors can be large.
		return nil, fmt.Errorf("Mod.io API request failed for target [%s?%s] with status: %s", apiBaseURL, loggingParams.Encode(), resp.Status)
	}

	var apiResponse ModioAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode Mod.io JSON response from target [%s?%s]: %w", apiBaseURL, loggingParams.Encode(), err)
	}

	return &apiResponse, nil
}

// FetchAllItems function remains the same as before
func (c *Client) FetchAllItems(itemTypeTag string, maxPagesToFetch int) ([]Mod, error) {
	var allItems []Mod
	var currentOffset int = 0

	slog.Info("Starting to fetch all items", "type", itemTypeTag, "max_pages", maxPagesToFetch)

	for page := 0; page < maxPagesToFetch; page++ {
		currentOffset = page * apiPageSize
		slog.Info("Fetching page for Mod.io items",
			"type", itemTypeTag,
			"page", page+1,
			"offset", currentOffset,
			"limit", apiPageSize,
		)

		response, err := c.fetchPage(itemTypeTag, apiPageSize, currentOffset)
		if err != nil {
			slog.Error("Failed to fetch a page from Mod.io", "type", itemTypeTag, "page", page+1, "error", err)
			return nil, fmt.Errorf("failed to fetch page %d for %s: %w", page+1, itemTypeTag, err)
		}

		if response != nil && len(response.Data) > 0 {
			allItems = append(allItems, response.Data...)
		}

		if response == nil || len(response.Data) < apiPageSize {
			slog.Info("Fetched last page for items", "type", itemTypeTag, "items_on_this_page", len(response.Data))
			break
		}

		if page < maxPagesToFetch-1 {
			slog.Debug("Sleeping between Mod.io paged requests", "duration", requestDelay)
			time.Sleep(requestDelay)
		}
	}

	slog.Info("Finished fetching all items", "type", itemTypeTag, "total_items_fetched", len(allItems))
	return allItems, nil
}