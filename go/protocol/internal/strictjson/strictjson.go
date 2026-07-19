package strictjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// DecodeObject rejects duplicate keys at every nesting level and requires one
// top-level JSON object.
func DecodeObject(input []byte) (map[string]json.RawMessage, error) {
	if err := Validate(input); err != nil {
		return nil, err
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(input, &result); err != nil || result == nil {
		return nil, errors.New("payload must contain one JSON object")
	}
	return result, nil
}

// Validate rejects duplicate object keys and trailing JSON values.
func Validate(input []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	if err := readValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("payload must contain one JSON value")
	}
	return nil
}

func readValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return errors.New("payload must contain valid JSON")
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		keys := make(map[string]struct{})
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return errors.New("payload must contain valid JSON")
			}
			key, ok := token.(string)
			if !ok {
				return errors.New("object key must be a string")
			}
			if _, exists := keys[key]; exists {
				return fmt.Errorf("duplicate object key %q", key)
			}
			keys[key] = struct{}{}
			if err := readValue(decoder); err != nil {
				return err
			}
		}
		return consumeDelimiter(decoder, '}')
	case '[':
		for decoder.More() {
			if err := readValue(decoder); err != nil {
				return err
			}
		}
		return consumeDelimiter(decoder, ']')
	default:
		return errors.New("payload must contain valid JSON")
	}
}

func consumeDelimiter(decoder *json.Decoder, expected json.Delim) error {
	token, err := decoder.Token()
	if err != nil || token != expected {
		return errors.New("payload must contain valid JSON")
	}
	return nil
}
