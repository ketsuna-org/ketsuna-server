package hooks

import "github.com/pocketbase/pocketbase/core"

// util helpers for cleaner code

func getString(r *core.Record, key string) string {
	return r.GetString(key)
}

func getInt(r *core.Record, key string) int {
	return r.GetInt(key)
}

func getFloat(r *core.Record, key string) float64 {
	return r.GetFloat(key)
}

// ensure we can access these from other files in same package
