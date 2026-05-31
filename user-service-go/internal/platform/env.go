package platform

import "os"

// envHasKey returns true when the env variable is present (even if empty).
// Used to distinguish "operator left it default" from "operator opted out".
func envHasKey(key string) bool {
	_, ok := os.LookupEnv(key)
	return ok
}
