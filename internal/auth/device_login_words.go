package auth

import (
	"crypto/rand"
	"fmt"
)

var deviceMatchAdjectives = []string{
	"amber", "brisk", "calm", "cedar", "clear", "cloud", "copper", "crisp",
	"delta", "ember", "fable", "fern", "frost", "gentle", "golden", "harbor",
	"hazel", "hollow", "indigo", "ivy", "jade", "juniper", "kindle", "lagoon",
	"linen", "lunar", "maple", "meadow", "misty", "navy", "nova", "oak",
	"olive", "opal", "orbit", "pepper", "pine", "plum", "prairie", "quiet",
	"raven", "river", "rose", "rustic", "sable", "sage", "scarlet", "silver",
	"smoky", "solstice", "spruce", "stone", "summer", "sunny", "timber", "topaz",
	"velvet", "violet", "willow", "winter", "woodland", "zephyr",
}

var deviceMatchNouns = []string{
	"anchor", "apple", "birch", "brook", "canyon", "castle", "comet", "cove",
	"crest", "dawn", "ember", "field", "fjord", "forest", "glade", "grove",
	"harbor", "hawk", "hill", "island", "lake", "lantern", "meadow", "mesa",
	"moon", "oasis", "ocean", "orchard", "owl", "pine", "planet", "pond",
	"quartz", "rain", "reef", "ridge", "river", "rock", "shadow", "shore",
	"signal", "snow", "spark", "star", "stone", "stream", "summit", "sun",
	"thunder", "trail", "tree", "valley", "wave", "whisper", "wind", "wolf",
}

func randomMatchCode() (string, error) {
	adjective, err := randomWord(deviceMatchAdjectives)
	if err != nil {
		return "", err
	}
	noun, err := randomWord(deviceMatchNouns)
	if err != nil {
		return "", err
	}
	return adjective + " " + noun, nil
}

func randomWord(list []string) (string, error) {
	if len(list) == 0 {
		return "", fmt.Errorf("empty word list")
	}
	buf := make([]byte, 1)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return list[int(buf[0])%len(list)], nil
}
