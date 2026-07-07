package sections

import (
	"encoding/json"
	"fmt"
	"strings"
)

type CardImageStyle string

const (
	CardImageStyleAuto      CardImageStyle = ""
	CardImageStylePortrait  CardImageStyle = "portrait"
	CardImageStyleLandscape CardImageStyle = "landscape"
)

type cardImageStyleConfig struct {
	CardImageStyle string `json:"card_image_style"`
}

func CardImageStyleFromConfig(config json.RawMessage) CardImageStyle {
	style, _ := ParseCardImageStyle(config)
	return style
}

func ParseCardImageStyle(config json.RawMessage) (CardImageStyle, error) {
	var raw cardImageStyleConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &raw); err != nil {
			return CardImageStyleAuto, err
		}
	}

	switch strings.ToLower(strings.TrimSpace(raw.CardImageStyle)) {
	case "", "auto":
		return CardImageStyleAuto, nil
	case string(CardImageStylePortrait):
		return CardImageStylePortrait, nil
	case string(CardImageStyleLandscape):
		return CardImageStyleLandscape, nil
	default:
		return CardImageStyleAuto, fmt.Errorf("card_image_style must be 'portrait' or 'landscape'")
	}
}
