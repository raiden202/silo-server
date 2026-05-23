package nfo

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

// nfoUniqueID represents a <uniqueid> element inside an NFO XML file.
type nfoUniqueID struct {
	Type    string `xml:"type,attr"`
	Default string `xml:"default,attr"`
	Value   string `xml:",chardata"`
}

// nfoMovie represents the <movie> root element.
type nfoMovie struct {
	XMLName   xml.Name      `xml:"movie"`
	Title     string        `xml:"title"`
	Year      int           `xml:"year"`
	Plot      string        `xml:"plot"`
	UniqueIDs []nfoUniqueID `xml:"uniqueid"`
}

// nfoTVShow represents the <tvshow> root element.
type nfoTVShow struct {
	XMLName   xml.Name      `xml:"tvshow"`
	Title     string        `xml:"title"`
	Year      int           `xml:"year"`
	Plot      string        `xml:"plot"`
	UniqueIDs []nfoUniqueID `xml:"uniqueid"`
}

// nfoEpisode represents the <episodedetails> root element.
type nfoEpisode struct {
	XMLName   xml.Name      `xml:"episodedetails"`
	Title     string        `xml:"title"`
	Season    int           `xml:"season"`
	Episode   int           `xml:"episode"`
	Plot      string        `xml:"plot"`
	UniqueIDs []nfoUniqueID `xml:"uniqueid"`
}

// parsedNFO holds extracted data from an NFO file.
type parsedNFO struct {
	Title    string
	Year     int
	Overview string
	TmdbID   string
	ImdbID   string
	TvdbID   string
	Type     string // movie, series, episode
	Season   int
	Episode  int
}

// parseNFOData parses NFO XML data and returns structured results.
func parseNFOData(data []byte) (*parsedNFO, error) {
	// Strip UTF-8 BOM if present.
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	rootTag, err := detectRootTag(data)
	if err != nil {
		return nil, fmt.Errorf("nfo: %w", err)
	}

	switch rootTag {
	case "movie":
		return parseMovieNFO(data)
	case "tvshow":
		return parseTVShowNFO(data)
	case "episodedetails":
		return parseEpisodeNFO(data)
	default:
		return nil, fmt.Errorf("nfo: unsupported root element <%s>", rootTag)
	}
}

func detectRootTag(data []byte) (string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := decoder.Token()
		if err != nil {
			return "", fmt.Errorf("cannot detect root element: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se.Name.Local, nil
		}
	}
}

func parseMovieNFO(data []byte) (*parsedNFO, error) {
	var m nfoMovie
	if err := xmlUnmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("nfo: failed to parse movie: %w", err)
	}
	p := &parsedNFO{
		Title:    strings.TrimSpace(m.Title),
		Year:     m.Year,
		Overview: strings.TrimSpace(m.Plot),
		Type:     "movie",
	}
	applyUniqueIDs(p, m.UniqueIDs)
	return p, nil
}

func parseTVShowNFO(data []byte) (*parsedNFO, error) {
	var s nfoTVShow
	if err := xmlUnmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("nfo: failed to parse tvshow: %w", err)
	}
	p := &parsedNFO{
		Title:    strings.TrimSpace(s.Title),
		Year:     s.Year,
		Overview: strings.TrimSpace(s.Plot),
		Type:     "series",
	}
	applyUniqueIDs(p, s.UniqueIDs)
	return p, nil
}

func parseEpisodeNFO(data []byte) (*parsedNFO, error) {
	var e nfoEpisode
	if err := xmlUnmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("nfo: failed to parse episodedetails: %w", err)
	}
	p := &parsedNFO{
		Title:    strings.TrimSpace(e.Title),
		Overview: strings.TrimSpace(e.Plot),
		Type:     "episode",
		Season:   e.Season,
		Episode:  e.Episode,
	}
	applyUniqueIDs(p, e.UniqueIDs)
	return p, nil
}

func xmlUnmarshal(data []byte, v any) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	return decoder.Decode(v)
}

func applyUniqueIDs(p *parsedNFO, ids []nfoUniqueID) {
	for _, uid := range ids {
		val := strings.TrimSpace(uid.Value)
		switch strings.ToLower(uid.Type) {
		case "imdb":
			p.ImdbID = val
		case "tmdb":
			p.TmdbID = val
		case "tvdb":
			p.TvdbID = val
		}
	}
}
