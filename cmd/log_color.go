package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ViBiOh/kmux/pkg/output"
)

type Formatter func(a ...interface{}) string

func getColorFromJSON(stream io.Reader, keys ...string) Formatter {
	decoder := json.NewDecoder(stream)

	if err := moveDecoderToKey(decoder, keys...); err != nil {
		return nil
	}

	token, err := decoder.Token()
	if err != nil {
		return nil
	}

	switch value := token.(type) {
	case string:
		switch strings.ToLower(value) {
		case "warn", "warning":
			return output.Yellow
		case "error", "critical", "fatal":
			return output.Red
		case "trace", "debug":
			return output.Green
		default:
			return nil
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
			return nil
		}
	default:
		return nil
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

		if nested == 1 {
			tokenStr := fmt.Sprintf("%s", token)
			var found bool

			for _, key := range keys {
				if strings.EqualFold(tokenStr, key) {
					found = true
					break
				}
			}

			if found {
				break
			}
		}

		if strToken := fmt.Sprintf("%s", token); strToken == "{" {
			nested++
		} else if strToken == "}" {
			nested--
		}
	}

	return nil
}
