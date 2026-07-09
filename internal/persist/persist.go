package persist

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"keyword-logger/internal/counter"
)

type Saver struct {
	path     string
	interval time.Duration
	c        *counter.Counter
	stopCh   chan struct{}
}

func New(path string, interval time.Duration, c *counter.Counter) *Saver {
	return &Saver{
		path:     path,
		interval: interval,
		c:        c,
		stopCh:   make(chan struct{}),
	}
}

func (s *Saver) Start() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := s.save(); err != nil {
				os.Stderr.WriteString("persist: " + err.Error() + "\n")
			}
		case <-s.stopCh:
			s.save()
			return
		}
	}
}

func (s *Saver) Stop() {
	close(s.stopCh)
}

func (s *Saver) save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	snap := s.c.Snapshot()
	tmp, err := os.CreateTemp(dir, "stats-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(snap); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}

	if err := os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

func LoadFromFile(path string, c *counter.Counter) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var snap counter.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}

	c.Restore(snap)
	return nil
}
