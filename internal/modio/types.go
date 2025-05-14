package modio

import "time"

type ModioUser struct {
	ID         int    `json:"id"`
	Username   string `json:"username"`
	ProfileURL string `json:"profile_url"`
}

type ModioLogo struct {
	Filename     string `json:"filename"`
	Original     string `json:"original"`
	Thumb320x180 string `json:"thumb_320x180"`
}

type ModioModfile struct {
	ID       int    `json:"id"`
	Filename string `json:"filename"`
	Version  string `json:"version"`
	Filesize int64  `json:"filesize"`
	Download struct {
		BinaryURL   string `json:"binary_url"`
		DateExpires int64  `json:"date_expires"`
	} `json:"download"`
}

type ModioTag struct {
	Name string `json:"name"`
}

type ModioStats struct {
	DownloadsTotal     int    `json:"downloads_total"`
	SubscribersTotal   int    `json:"subscribers_total"`
	RatingsPositive    int    `json:"ratings_positive"`
	RatingsNegative    int    `json:"ratings_negative"`
	RatingsDisplayText string `json:"ratings_display_text"`
}

type ModioImage struct {
	Filename     string `json:"filename"`
	Original     string `json:"original"`
	Thumb320x180 string `json:"thumb_320x180"`
}

type ModioMedia struct {
	Images []ModioImage `json:"images"`
}

type Mod struct {
	ID          int          `json:"id"`
	GameID      int          `json:"game_id"`
	Name        string       `json:"name"`
	NameID      string       `json:"name_id"`
	Summary     string       `json:"summary"`
	Description string       `json:"description_plaintext"`
	ProfileURL  string       `json:"profile_url"`
	SubmittedBy ModioUser    `json:"submitted_by"`
	DateAdded   int64        `json:"date_added"`
	DateUpdated int64        `json:"date_updated"`
	DateLive    int64        `json:"date_live"`
	Logo        ModioLogo    `json:"logo"`
	Modfile     ModioModfile `json:"modfile"`
	Tags        []ModioTag   `json:"tags"`
	Stats       ModioStats   `json:"stats"`
	Media       ModioMedia   `json:"media"`
}

type ModioAPIResponse struct {
	Data         []Mod `json:"data"`
	ResultCount  int   `json:"result_count"`
	ResultOffset int   `json:"result_offset"`
	ResultLimit  int   `json:"result_limit"`
	ResultTotal  int   `json:"result_total"`
}

type ModioEvent struct {
	ID        int    `json:"id"`
	ModID     int    `json:"mod_id"`
	UserID    int    `json:"user_id"`
	DateAdded int64  `json:"date_added"`
	EventType string `json:"event_type"`
}

type ModioEventsAPIResponse struct {
	Data         []ModioEvent `json:"data"`
	ResultCount  int          `json:"result_count"`
	ResultOffset int          `json:"result_offset"`
	ResultLimit  int          `json:"result_limit"`
	ResultTotal  int          `json:"result_total"`
}

type CachedAPIResponse struct {
	Items       []Mod     `json:"items"`
	ItemType    string    `json:"itemType"`
	LastUpdated time.Time `json:"lastUpdated"`
	TotalItems  int       `json:"totalItems"`
}

const (
	MapTag       = "Map"
	ScriptModTag = "Script"
)