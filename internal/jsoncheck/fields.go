package jsoncheck

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Run after schema validation; all request objects currently map to structs.
// EqualFold mirrors encoding/json's case-insensitive struct-field aliases.
func UniqueStructFields(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	switch token {
	case json.Delim('{'):
		var keys []string
		for d.More() {
			token, err := d.Token()
			if err != nil {
				return err
			}
			key := token.(string)
			for _, previous := range keys {
				if strings.EqualFold(key, previous) {
					return fmt.Errorf("duplicate JSON field")
				}
			}
			keys = append(keys, key)
			if err := UniqueStructFields(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
	case json.Delim('['):
		for d.More() {
			if err := UniqueStructFields(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
	}
	return err
}
