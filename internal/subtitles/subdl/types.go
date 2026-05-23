// internal/subtitles/subdl/types.go
package subdl

type searchResponse struct {
	Status    bool         `json:"status"`
	Results   []resultItem `json:"results"`
	Subtitles []subtitle   `json:"subtitles"`
}

type resultItem struct {
	SdID   int    `json:"sd_id"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	IMDbID string `json:"imdb_id"`
	Year   int    `json:"year"`
}

type subtitle struct {
	ReleaseName     string `json:"release_name"`
	Name            string `json:"name"`
	Lang            string `json:"lang"`
	Author          string `json:"author"`
	URL             string `json:"url"`
	SubtitlePage    string `json:"subtitlePage"`
	Season          int    `json:"season"`
	Episode         int    `json:"episode"`
	Language        string `json:"language"`
	HearingImpaired bool   `json:"hi"`
	DownloadCount   int    `json:"download_count"`
	FullURL         string `json:"full_url"`
}
