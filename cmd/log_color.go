package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ViBiOh/kmux/pkg/output"
)

const defaultColor = 2

var colorOrder = map[string]uint{
	"red":    0,
	"yellow": 1,
	"white":  defaultColor,
	"green":  3,
}

type Formatter func(a ...interface{}) string

func getColorFromJSON(stream io.Reader, keys ...string) (uint, Formatter) {
	decoder := json.NewDecoder(stream)

	if err := moveDecoderToKey(decoder, keys...); err != nil {
		return defaultColor, nil
	}

	token, err := decoder.Token()
	if err != nil {
		return defaultColor, nil
	}

	switch value := token.(type) {
	case string:
		switch strings.ToLower(value) {
		case "error", "critical", "fatal":
			return 0, output.Red
		case "warn", "warning":
			return 1, output.Yellow
		case "trace", "debug":
			return 3, output.Green
		default:
			return defaultColor, nil
		}

	case float64:
		switch {
		case value >= http.StatusInternalServerError:
			return 0, output.Red
		case value >= http.StatusBadRequest:
			return 1, output.Yellow
		case value >= http.StatusMultipleChoices:
			return 3, output.Green
		default:
			return defaultColor, nil
		}

	default:
		return defaultColor, nil
	}
}

func moveDecoderToKey(decoder *json.Decoder, keys ...string) error {
	var token json.Token
	var nested uint64
	var err error

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
