package persist

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"keyword-logger/internal/counter"
)

const createTable = `
CREATE TABLE IF NOT EXISTS key_events (
	app    TEXT NOT NULL,
	bucket TEXT NOT NULL,
	key    TEXT NOT NULL,
	count  INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (app, bucket, key)
);
CREATE INDEX IF NOT EXISTS idx_bucket ON key_events(bucket);
`

const upsertStmt = `
INSERT INTO key_events (app, bucket, key, count) VALUES (?, ?, ?, ?)
ON CONFLICT(app, bucket, key) DO UPDATE SET count = count + excluded.count;
`

type Saver struct {
	path    string
	c       *counter.Counter
	stopCh  chan struct{}
}

func New(path string, _ time.Duration, c *counter.Counter) *Saver {
	return &Saver{
		path:    path,
		c:       c,
		stopCh: make(chan struct{}),
	}
}

func (s *Saver) Init() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	db, err := s.open()
	if err != nil {
		return err
	}
	return db.Close()
}

func (s *Saver) open() (*sql.DB, error) {
	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(createTable); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// SaveBatch persists a batch of key counts (from Recorder's pending buffer).
// This is called from Recorder.flush() after the batch has been merged into
// the Counter, ensuring each batch is written exactly once.
func (s *Saver) SaveBatch(batch map[string]map[string]int64) {
	if len(batch) == 0 {
		return
	}

	db, err := s.open()
	if err != nil {
		log.Printf("persist: open failed: %v", err)
		return
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		log.Printf("persist: begin failed: %v", err)
		return
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(upsertStmt)
	if err != nil {
		log.Printf("persist: prepare failed: %v", err)
		return
	}
	defer stmt.Close()

	for app, keys := range batch {
		bucket := s.c.CurrentBucket()
		for key, count := range keys {
			if _, err := stmt.Exec(app, bucket, key, count); err != nil {
				log.Printf("persist: exec failed: %v", err)
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("persist: commit failed: %v", err)
	}
}

func (s *Saver) Stop() {
	close(s.stopCh)
}

func LoadFromFile(path string, c *counter.Counter) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query("SELECT app, bucket, key, count FROM key_events")
	if err != nil {
		return err
	}
	defer rows.Close()

	data := make(map[string]map[string]map[string]int64)
	for rows.Next() {
		var app, bucket, key string
		var count int64
		if err := rows.Scan(&app, &bucket, &key, &count); err != nil {
			return err
		}
		if data[app] == nil {
			data[app] = make(map[string]map[string]int64)
		}
		if data[app][bucket] == nil {
			data[app][bucket] = make(map[string]int64)
		}
		data[app][bucket][key] = count
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// 保留现有的 Version（不覆盖 New() 设置的初始值）
	snap := counter.Snapshot{Data: data}
	snap.Version = c.Version
	c.Restore(snap)
	return nil
}
