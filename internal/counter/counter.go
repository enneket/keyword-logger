package counter

import (
	"sort"
	"sync"
	"time"
)

const BucketFormat = "2006-01-02T15:04"

type Granularity string

const (
	GranularityHour  Granularity = "hour"
	GranularityDay   Granularity = "day"
	GranularityWeek  Granularity = "week"
	GranularityMonth Granularity = "month"
	GranularityYear  Granularity = "year"
)

func IsValidGranularity(g string) bool {
	switch Granularity(g) {
	case GranularityHour, GranularityDay, GranularityWeek, GranularityMonth, GranularityYear:
		return true
	}
	return false
}

func defaultRange(g Granularity) (time.Time, time.Time) {
	until := time.Now()
	var since time.Time
	switch g {
	case GranularityHour:
		since = until.Add(-24 * time.Hour)
	case GranularityDay:
		since = until.AddDate(0, 0, -30)
	case GranularityWeek:
		since = until.AddDate(0, 0, -7*12)
	case GranularityMonth:
		since = until.AddDate(0, -12, 0)
	case GranularityYear:
		since = until.AddDate(-5, 0, 0)
	}
	return since, until
}

// bucketKey collapses a minute-bucket time into the start of the
// coarser bucket for the given granularity. Week buckets are Monday-based.
func bucketKey(t time.Time, g Granularity) string {
	switch g {
	case GranularityHour:
		return t.Format("2006-01-02T15")
	case GranularityDay:
		return t.Format("2006-01-02")
	case GranularityWeek:
		offset := (int(t.Weekday()) + 6) % 7 // 0=Mon .. 6=Sun
		return t.AddDate(0, 0, -offset).Format("2006-01-02")
	case GranularityMonth:
		return t.Format("2006-01")
	case GranularityYear:
		return t.Format("2006")
	}
	return t.Format(BucketFormat)
}

type Counter struct {
	mu        sync.RWMutex
	Version   int                               `json:"version"`
	StartedAt string                            `json:"started_at"`
	UpdatedAt string                            `json:"updated_at"`
	Data      map[string]map[string]map[string]int64 `json:"data"`
}

func New() *Counter {
	return &Counter{
		Version:   1,
		StartedAt: time.Now().Format(time.RFC3339),
		Data:      make(map[string]map[string]map[string]int64),
	}
}

func (c *Counter) Increment(app, key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := time.Now()
	bucket := t.Format(BucketFormat)
	if c.Data[app] == nil {
		c.Data[app] = make(map[string]map[string]int64)
	}
	if c.Data[app][bucket] == nil {
		c.Data[app][bucket] = make(map[string]int64)
	}
	c.Data[app][bucket][key]++
	c.UpdatedAt = time.Now().Format(time.RFC3339)
}

type AppStats struct {
	Total int64            `json:"total"`
	Keys  map[string]int64 `json:"keys"`
}

type BucketPoint struct {
	Start string `json:"start"`
	Count int64  `json:"count"`
}

type StatsOutput struct {
	Version     int                 `json:"version"`
	StartedAt   string              `json:"started_at"`
	UpdatedAt   string              `json:"updated_at"`
	Total       int64               `json:"total"`
	Apps        map[string]AppStats `json:"apps"`
	Granularity string              `json:"granularity,omitempty"`
	Buckets     []BucketPoint       `json:"buckets,omitempty"`
}

func (c *Counter) GetStats(since, until time.Time, appFilter, granularity string) StatsOutput {
	c.mu.RLock()
	defer c.mu.RUnlock()

	g := Granularity(granularity)
	wantBuckets := IsValidGranularity(granularity)

	if wantBuckets && since.IsZero() && until.IsZero() {
		since, until = defaultRange(g)
	}

	out := StatsOutput{
		Version:   c.Version,
		StartedAt: c.StartedAt,
		Apps:      make(map[string]AppStats),
	}
	if wantBuckets {
		out.Granularity = granularity
		out.Buckets = []BucketPoint{}
	}

	for appName, buckets := range c.Data {
		if appFilter != "" && appName != appFilter {
			continue
		}
		var appTotal int64
		keys := make(map[string]int64)
		for bucketStr, keyCounts := range buckets {
			bucketTime, err := time.ParseInLocation(BucketFormat, bucketStr, time.Local)
			if err != nil {
				continue
			}
			if !since.IsZero() && bucketTime.Before(since) {
				continue
			}
			if !until.IsZero() && bucketTime.After(until) {
				continue
			}
			var bucketCount int64
			for k, v := range keyCounts {
				keys[k] += v
				appTotal += v
				bucketCount += v
			}
			if wantBuckets {
				out.Buckets = append(out.Buckets, BucketPoint{
					Start: bucketKey(bucketTime, g),
					Count: bucketCount,
				})
			}
		}
		if appTotal > 0 {
			out.Apps[appName] = AppStats{Total: appTotal, Keys: keys}
			out.Total += appTotal
		}
	}

	if wantBuckets {
		// collapse same-key buckets (multiple minute buckets falling in the
		// same hour/day/week/month/year) and sort by start time.
		collapsed := collapseBuckets(out.Buckets)
		sort.Slice(collapsed, func(i, j int) bool {
			return collapsed[i].Start < collapsed[j].Start
		})
		out.Buckets = collapsed
	}

	out.UpdatedAt = c.UpdatedAt

	return out
}

func collapseBuckets(in []BucketPoint) []BucketPoint {
	if len(in) == 0 {
		return in
	}
	m := make(map[string]int64, len(in))
	order := make([]string, 0, len(in))
	for _, b := range in {
		if _, ok := m[b.Start]; !ok {
			order = append(order, b.Start)
		}
		m[b.Start] += b.Count
	}
	out := make([]BucketPoint, 0, len(order))
	for _, s := range order {
		out = append(out, BucketPoint{Start: s, Count: m[s]})
	}
	return out
}

type Snapshot struct {
	Version   int                                    `json:"version"`
	StartedAt string                                 `json:"started_at"`
	UpdatedAt string                                 `json:"updated_at"`
	Data      map[string]map[string]map[string]int64 `json:"data"`
}

func (c *Counter) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	s := Snapshot{
		Version:   c.Version,
		StartedAt: c.StartedAt,
		UpdatedAt: c.UpdatedAt,
	}

	if len(c.Data) == 0 {
		s.Data = make(map[string]map[string]map[string]int64)
		return s
	}

	s.Data = make(map[string]map[string]map[string]int64)
	for app, buckets := range c.Data {
		bc := make(map[string]map[string]int64)
		for bucket, keys := range buckets {
			kc := make(map[string]int64, len(keys))
			for k, v := range keys {
				kc[k] = v
			}
			bc[bucket] = kc
		}
		s.Data[app] = bc
	}

	return s
}

func (c *Counter) Restore(s Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Version = s.Version
	c.StartedAt = s.StartedAt
	c.UpdatedAt = s.UpdatedAt

	c.Data = make(map[string]map[string]map[string]int64)
	for app, buckets := range s.Data {
		bc := make(map[string]map[string]int64)
		for bucket, keys := range buckets {
			kc := make(map[string]int64, len(keys))
			for k, v := range keys {
				kc[k] = v
			}
			bc[bucket] = kc
		}
		c.Data[app] = bc
	}
}

func (c *Counter) ActiveApps() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	apps := make([]string, 0, len(c.Data))
	for app := range c.Data {
		apps = append(apps, app)
	}
	sort.Strings(apps)
	return apps
}

func (c *Counter) Total() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var total int64
	for _, buckets := range c.Data {
		for _, keyCounts := range buckets {
			for _, v := range keyCounts {
				total += v
			}
		}
	}
	return total
}
