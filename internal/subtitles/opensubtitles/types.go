// internal/subtitles/opensubtitles/types.go
package opensubtitles

type searchResponse struct {
	TotalPages int          `json:"total_pages"`
	TotalCount int          `json:"total_count"`
	Page       int          `json:"page"`
	Data       []searchData `json:"data"`
}

type searchData struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"`
	Attributes searchAttributes `json:"attributes"`
}

type searchAttributes struct {
	SubtitleID        string       `json:"subtitle_id"`
	Language          string       `json:"language"`
	DownloadCount     int          `json:"download_count"`
	HearingImpaired   bool         `json:"hearing_impaired"`
	HD                bool         `json:"hd"`
	FPS               float64      `json:"fps"`
	Ratings           float64      `json:"ratings"`
	FromTrusted       bool         `json:"from_trusted"`
	AITranslated      bool         `json:"ai_translated"`
	MachineTranslated bool         `json:"machine_translated"`
	UploadDate        string       `json:"upload_date"`
	Release           string       `json:"release"`
	Files             []searchFile `json:"files"`
}

type searchFile struct {
	FileID   int    `json:"file_id"`
	FileName string `json:"file_name"`
}

type downloadRequest struct {
	FileID int `json:"file_id"`
}

type downloadResponse struct {
	Link      string `json:"link"`
	FileName  string `json:"file_name"`
	Requests  int    `json:"requests"`
	Remaining int    `json:"remaining"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token  string `json:"token"`
	Status int    `json:"status"`
}
