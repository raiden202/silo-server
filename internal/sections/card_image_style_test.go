package sections

import (
	"encoding/json"
	"testing"
)

func TestParseCardImageStyle(t *testing.T) {
	tests := []struct {
		name    string
		config  json.RawMessage
		want    CardImageStyle
		wantErr bool
	}{
		{name: "missing defaults to auto", config: json.RawMessage(`{}`), want: CardImageStyleAuto},
		{name: "empty defaults to auto", config: nil, want: CardImageStyleAuto},
		{name: "auto is accepted", config: json.RawMessage(`{"card_image_style":"auto"}`), want: CardImageStyleAuto},
		{name: "portrait", config: json.RawMessage(`{"card_image_style":"portrait"}`), want: CardImageStylePortrait},
		{name: "landscape", config: json.RawMessage(`{"card_image_style":"landscape"}`), want: CardImageStyleLandscape},
		{name: "trims and lowercases", config: json.RawMessage(`{"card_image_style":" Landscape "}`), want: CardImageStyleLandscape},
		{name: "rejects unknown", config: json.RawMessage(`{"card_image_style":"square"}`), wantErr: true},
		{name: "rejects invalid json", config: json.RawMessage(`{`), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCardImageStyle(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Fatal("ParseCardImageStyle() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseCardImageStyle() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ParseCardImageStyle() = %q, want %q", got, tt.want)
			}
		})
	}
}
