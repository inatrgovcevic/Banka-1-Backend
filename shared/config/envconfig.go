// Package config loads configuration into struct fields using kelseyhightower/envconfig.
// The prefix scopes env var names so e.g. INTERBANK_PORT maps to a Port field.
package config

import "github.com/kelseyhightower/envconfig"

// Load populates target using env vars prefixed with prefix. Target must be a non-nil pointer
// to a struct with `envconfig:"..."` tags. Required, default, and split_words options are
// inherited from kelseyhightower/envconfig.
func Load(prefix string, target any) error {
	return envconfig.Process(prefix, target)
}
