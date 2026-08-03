package stats

import (
	"sort"
	"time"
)

// Filter defines the query filter for stats aggregation.
type Filter struct {
	Profile string
	Cmd     string
	Attrs   map[string]string
	Since   *time.Time
	Until   *time.Time
}

// Query aggregates records by cmd × profile × attrs.
func Query(records []*Record, filter Filter) []*AggregatedRecord {
	type key struct {
		Cmd     string
		Profile string
		Attrs   string // sorted attrs string for stable grouping
	}

	agg := make(map[key]*AggregatedRecord)

	for _, r := range records {
		if filter.Profile != "" && r.Profile != filter.Profile {
			continue
		}
		if filter.Cmd != "" && r.Cmd != filter.Cmd {
			continue
		}
		if filter.Since != nil && r.Timestamp.Before(*filter.Since) {
			continue
		}
		if filter.Until != nil && r.Timestamp.After(*filter.Until) {
			continue
		}
		if !matchAttrs(r.Attrs, filter.Attrs) {
			continue
		}

		k := key{
			Cmd:     r.Cmd,
			Profile: r.Profile,
			Attrs:   attrsKey(r.Attrs),
		}

		a, ok := agg[k]
		if !ok {
			a = &AggregatedRecord{
				Cmd:     r.Cmd,
				Profile: r.Profile,
				Attrs:   r.Attrs,
			}
			agg[k] = a
		}

		if r.OK {
			a.OK++
		} else {
			a.Fail++
		}
		a.AvgMs += r.DurationMs
	}

	// Compute averages and sort
	result := make([]*AggregatedRecord, 0, len(agg))
	for _, a := range agg {
		total := a.OK + a.Fail
		if total > 0 {
			a.AvgMs = a.AvgMs / int64(total)
		}
		result = append(result, a)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Cmd != result[j].Cmd {
			return result[i].Cmd < result[j].Cmd
		}
		if result[i].Profile != result[j].Profile {
			return result[i].Profile < result[j].Profile
		}
		return attrsKey(result[i].Attrs) < attrsKey(result[j].Attrs)
	})

	return result
}

// matchAttrs checks if record attrs contain all filter attrs.
func matchAttrs(record, filter map[string]string) bool {
	if len(filter) == 0 {
		return true
	}
	for k, v := range filter {
		if rv, ok := record[k]; !ok || rv != v {
			return false
		}
	}
	return true
}

// attrsKey creates a stable string key from attrs map.
func attrsKey(attrs map[string]string) string {
	if len(attrs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := ""
	for _, k := range keys {
		if result != "" {
			result += ","
		}
		result += k + "=" + attrs[k]
	}
	return result
}
