package log

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/fatih/color"
)

var colorNames = map[string]*color.Color{
	"red":    output.Red,
	"yellow": output.Yellow,
	"green":  output.Green,
}

var colorRanks = map[*color.Color]uint{
	output.Red:    0,
	output.Yellow: 1,
	output.White:  2,
	output.Green:  3,
}

func ColorFromName(name string) *color.Color {
	found, ok := colorNames[name]
	if ok {
		return found
	}

	return nil
}

func ColorIsGreater(first, second *color.Color) bool {
	if first == nil {
		return true
	}

	if second == nil {
		return false
	}

	return colorRanks[first] > colorRanks[second]
}

func ColorOfJSON(content string, keys ...string) *color.Color {
	if !strings.HasPrefix(content, "{") || len(keys) == 0 {
		return output.White
	}

	decoder := json.NewDecoder(strings.NewReader(content))

	if err := moveDecoderToKey(decoder, keys...); err != nil {
		return output.White
	}

	token, err := decoder.Token()
	if err != nil {
		return output.White
	}

	switch value := token.(type) {
	case string:
		switch strings.ToLower(value) {
		case "error", "critical", "fatal":
			return output.Red
		case "warn", "warning":
			return output.Yellow
		case "trace", "debug":
			return output.Green
		default:
			return output.White
		}

	case float64:
		switch {
		case value >= http.StatusInternalServerError:
			return output.Red
		case value >= http.StatusBadRequest:
			return output.Yellow
		case value >= http.StatusMultipleChoices:
			return output.Green
		default:
			return output.White
		}

	default:
		return output.White
	}
}

func moveDecoderToKey(decoder *json.Decoder, keys ...string) error {
	var (
		token  json.Token
		nested uint64
		err    error
	)

	for {
		token, err = decoder.Token()
		if err != nil {
			return fmt.Errorf("decode token: %w", err)
		}

		tokenStr := fmt.Sprintf("%s", token)

		if nested == 1 {
			for _, key := range keys {
				if strings.EqualFold(tokenStr, key) {
					return nil
				}
			}
		}

		switch tokenStr {
		case "{":
			nested++
		case "}":
			nested--
		}
	}
}
