// internal/modio/types.go
package modio

import "time" // For potentially converting timestamps later

// ModioUser represents the "submitted_by" field
type ModioUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	// Add other user fields you need, e.g., AvatarURL, ProfileURL
	// Avatar struct { Original string `json:"original"` } `json:"avatar"` // Example for nested avatar
	ProfileURL string `json:"profile_url"`
}

// ModioLogo represents the "logo" field
type ModioLogo struct {
	Filename      string `json:"filename"`
	Original      string `json:"original"`
	Thumb320x180  string `json:"thumb_320x180"`
	// Add other thumbnail sizes if needed
}

// ModioModfile represents the "modfile" field
type ModioModfile struct {
	ID        int    `json:"id"`
	Filename  string `json:"filename"`
	Version   string `json:"version"`
	Filesize  int64  `json:"filesize"` // Size in bytes
	Download  struct {
		BinaryURL   string `json:"binary_url"`
		DateExpires int64  `json:"date_expires"` // Unix timestamp
	} `json:"download"`
	// Add other modfile fields you need
}

// ModioTag represents an individual tag
type ModioTag struct {
	Name string `json:"name"`
	// Add 'name_localized', 'date_added' if needed
}

// ModioStats represents the "stats" field
type ModioStats struct {
	DownloadsTotal         int    `json:"downloads_total"`
	SubscribersTotal       int    `json:"subscribers_total"`
	RatingsPositive        int    `json:"ratings_positive"`
	RatingsNegative        int    `json:"ratings_negative"`
	RatingsDisplayText   string `json:"ratings_display_text"`
	// Add other stats fields you need
}

// ModioMedia contains links to media, like images
type ModioImage struct {
	Filename     string `json:"filename"`
	Original     string `json:"original"`
	Thumb320x180 string `json:"thumb_320x180"` // Or other relevant thumbs
}

type ModioMedia struct {
    Images  []ModioImage `json:"images"`
    // Youtube []string `json:"youtube"` // Example if you need youtube links
    // Sketchfab []string `json:"sketchfab"` // Example if you need sketchfab links
}

// Mod represents a single mod or map item from Mod.io
// This is the main struct for the items in the "data" array
type Mod struct {
	ID                  int             `json:"id"`
	GameID              int             `json:"game_id"`
	Name                string          `json:"name"`
	NameID              string          `json:"name_id"`
	Summary             string          `json:"summary"`
	Description         string          `json:"description_plaintext"` // Use description_plaintext for cleaner text
	ProfileURL          string          `json:"profile_url"`
	SubmittedBy         ModioUser       `json:"submitted_by"`
	DateAdded           int64           `json:"date_added"`   // Unix timestamp
	DateUpdated         int64           `json:"date_updated"` // Unix timestamp
	DateLive            int64           `json:"date_live"`    // Unix timestamp
	Logo                ModioLogo       `json:"logo"`
	Modfile             ModioModfile    `json:"modfile"` // Assuming one primary modfile, adjust if it can be an array or nullable
	Tags                []ModioTag      `json:"tags"`
	Stats               ModioStats      `json:"stats"`
    Media               ModioMedia      `json:"media"`
	// Add any other top-level fields you need from the JSON example
}

// ModioAPIResponse represents the structure of the paginated response from Mod.io
// (e.g., when you request /v1/games/{gameId}/mods)
type ModioAPIResponse struct {
	Data         []Mod `json:"data"`
	ResultCount  int   `json:"result_count"`  // Number of results in this response
	ResultOffset int   `json:"result_offset"` // Current offset
	ResultLimit  int   `json:"result_limit"`  // Max results per page
	ResultTotal  int   `json:"result_total"`  // Total results available matching filter
}

// CachedAPIResponse is the structure your API will serve for a list of items
type CachedAPIResponse struct {
	Items       []Mod     `json:"items"`
	ItemType    string    `json:"itemType"` // e.g., "maps" or "scripts"
	LastUpdated time.Time `json:"lastUpdated"`
	TotalItems  int       `json:"totalItems"`
}