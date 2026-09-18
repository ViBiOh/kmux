package log

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/fatih/color"
)

var colorNames = map[string]*color.Color{
	"red":    output.Red,
	"yellow": output.Yellow,
	"white":  output.White,
	"green":  output.Green,
}

var colorRanks = map[*color.Color]uint{
	output.Red:    0,
	output.Yellow: 1,
	output.White:  2,
	output.Green:  3,
}

var errKeyNotFound = errors.New("key not found")

func ColorNames() []string {
	names := make([]string, 0, len(colorNames))

	for name := range colorNames {
		names = append(names, name)
	}

	slices.SortFunc(names, func(first, second string) int {
		return int(colorRanks[colorNames[first]]) - int(colorRanks[colorNames[second]])
	})

	return names
}

func ColorFromName(name string) *color.Color {
	found, ok := colorNames[name]
	if ok {
		return found
	}

	return nil
}

func colorIsGreater(first, second *color.Color) bool {
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
	var depth uint64

	expectKey := true

	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return errKeyNotFound
			}

			return fmt.Errorf("decode token: %w", err)
		}

		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{', '[':
				depth++
			case '}', ']':
				if depth > 0 {
					depth--
				}
			}

			expectKey = depth == 1
			continue
		}

		if depth != 1 {
			continue
		}

		if !expectKey {
			expectKey = true

			continue
		}

		expectKey = false

		name, ok := token.(string)
		if !ok {
			continue
		}

		for _, key := range keys {
			if strings.EqualFold(name, key) {
				return nil
			}
		}
	}
}
