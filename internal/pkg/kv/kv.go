// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package kv

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// KV represents a key/value pair
type KV struct {
	// Key of the pair
	Key string

	// Value of the pair
	Value string
}

// KeyExists checks whether a key exists
func KeyExists(kvs []KV, key string) bool {
	for _, kv := range kvs {
		if kv.Key == key {
			return true
		}
	}
	return false
}

// SetValue sets the value of a given key
func SetValue(kvs []KV, key string, value string) error {
	for _, kv := range kvs {
		if kv.Key == key {
			kv.Value = value
			return nil
		}
	}

	return fmt.Errorf("unable to find key %s", key)
}

// GetValue returns the value of a given key from a slice of key/value pairs
func GetValue(kvs []KV, key string) string {
	for _, kv := range kvs {
		if kv.Key == key {
			return kv.Value
		}
	}

	return ""
}

// LoadKeyValueConfig loads all the key/value pairs from a configure file with a compatible syntax
func LoadKeyValueConfig(filepath string) ([]KV, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s: %s", filepath, err)
	}
	defer f.Close()

	var data []KV

	lineReader := bufio.NewScanner(f)
	for lineReader.Scan() {
		line := lineReader.Text()

		// We skip empty lines
		if line == "" {
			continue
		}

		// We skip comments
		re1 := regexp.MustCompile(`\s*#`)
		commentMatch := re1.FindStringSubmatch(line)
		if len(commentMatch) != 0 {
			continue
		}

		// For valid lines, we separate the key from the value
		words := strings.Split(line, "=")
		if len(words) != 2 {
			return nil, fmt.Errorf("invalid entry format: %s", line)
		}

		var newKV KV
		newKV.Key = words[0]
		newKV.Value = words[1]
		newKV.Key = strings.Trim(newKV.Key, " \t")
		newKV.Value = strings.Trim(newKV.Value, " \t")

		data = append(data, newKV)
	}

	return data, nil
}

// ToStringSlice converts a slice of key/value pairs into a slice of strings
func ToStringSlice(kvs []KV) []string {
	var newSlice []string

	for _, kv := range kvs {
		newSlice = append(newSlice, kv.Key+" = "+kv.Value)
	}

	return newSlice
}
