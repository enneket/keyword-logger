package counter

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewCounter(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.Version != 1 {
		t.Errorf("Version = %d, want 1", c.Version)
	}
	if c.Data == nil || len(c.Data) != 0 {
		t.Error("Data should be empty map")
	}
	if c.StartedAt == "" {
		t.Error("StartedAt should be set")
	}
}

func TestIncrementAndGetStats(t *testing.T) {
	c := New()
	c.Increment("chrome", "Key_A")
	c.Increment("chrome", "Key_A")
	c.Increment("chrome", "Key_B")
	c.Increment("firefox", "Key_C")

	stats := c.GetStats(time.Time{}, time.Time{}, "", "")
	if stats.Total != 4 {
		t.Errorf("Total = %d, want 4", stats.Total)
	}
	if stats.Apps["chrome"].Total != 3 {
		t.Errorf("chrome total = %d, want 3", stats.Apps["chrome"].Total)
	}
	if stats.Apps["firefox"].Total != 1 {
		t.Errorf("firefox total = %d, want 1", stats.Apps["firefox"].Total)
	}
}

func TestGetStatsWithAppFilter(t *testing.T) {
	c := New()
	c.Increment("chrome", "Key_A")
	c.Increment("firefox", "Key_B")
	c.Increment("chrome", "Key_C")

	stats := c.GetStats(time.Time{}, time.Time{}, "chrome", "")
	if stats.Total != 2 {
		t.Errorf("Total = %d, want 2", stats.Total)
	}
	if _, ok := stats.Apps["firefox"]; ok {
		t.Error("firefox should be filtered out")
	}
}

func TestGetStatsWithTimeFilter(t *testing.T) {
	c := New()
	now := time.Now()
	// 插入两个不同分钟 bucket 的数据
	c.Increment("chrome", "Key_A") // 当前分钟

	// 手动插入历史 bucket
	histBucket := now.Add(-2 * time.Hour).Format(BucketFormat)
	if c.Data["chrome"] == nil {
		c.Data["chrome"] = make(map[string]map[string]int64)
	}
	if c.Data["chrome"][histBucket] == nil {
		c.Data["chrome"][histBucket] = make(map[string]int64)
	}
	c.Data["chrome"][histBucket]["Key_B"] = 5

	since := now.Add(-1 * time.Hour)
	stats := c.GetStats(since, time.Time{}, "", "")
	// 当前分钟的数据应该被包含，历史的 2 小时前的不
	if stats.Total != 1 {
		t.Errorf("Total = %d, want 1 (only recent data)", stats.Total)
	}
}

func TestGetStatsGranularityCollapsing(t *testing.T) {
	c := New()

	// 同一小时插入 3 个不同分钟 bucket 的数据
	c.Increment("chrome", "Key_A") // 当前分钟
	time.Sleep(time.Second)
	c.Increment("chrome", "Key_A") // 下一个分钟（同小时）
	time.Sleep(time.Second)
	c.Increment("chrome", "Key_A") // 再下一个分钟（同小时）

	stats := c.GetStats(time.Time{}, time.Time{}, "", "hour")
	if len(stats.Buckets) != 1 {
		t.Errorf("Buckets count = %d, want 1 (collapsed to 1 hour)", len(stats.Buckets))
	}
	if stats.Buckets[0].Count != 3 {
		t.Errorf("Bucket count = %d, want 3", stats.Buckets[0].Count)
	}
}

func TestMerge(t *testing.T) {
	c := New()
	batch := map[string]map[string]int64{
		"chrome": {"Key_A": 5, "Key_B": 3},
		"firefox": {"Key_C": 2},
	}
	c.Merge(batch)

	stats := c.GetStats(time.Time{}, time.Time{}, "", "")
	if stats.Total != 10 {
		t.Errorf("Total = %d, want 10", stats.Total)
	}
	if stats.Apps["chrome"].Keys["Key_A"] != 5 {
		t.Errorf("chrome Key_A = %d, want 5", stats.Apps["chrome"].Keys["Key_A"])
	}
}

func TestMergeAccumulates(t *testing.T) {
	c := New()
	c.Increment("chrome", "Key_A")
	c.Merge(map[string]map[string]int64{"chrome": {"Key_A": 5}})

	stats := c.GetStats(time.Time{}, time.Time{}, "", "")
	if stats.Apps["chrome"].Keys["Key_A"] != 6 {
		t.Errorf("chrome Key_A = %d, want 6", stats.Apps["chrome"].Keys["Key_A"])
	}
}

func TestConcurrentIncrement(t *testing.T) {
	c := New()
	var wg sync.WaitGroup
	goroutines := 10
	keysPerG := 100

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for k := 0; k < keysPerG; k++ {
				c.Increment("app", "Key")
			}
		}(g)
	}
	wg.Wait()

	stats := c.GetStats(time.Time{}, time.Time{}, "", "")
	want := int64(goroutines * keysPerG)
	if stats.Total != want {
		t.Errorf("Total = %d, want %d", stats.Total, want)
	}
}

func TestWaitForUpdate(t *testing.T) {
	c := New()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan string, 1)
	go func() {
		result, _ := c.WaitForUpdate(ctx, "")
		done <- result
	}()

	// 让 goroutine 先阻塞
	time.Sleep(100 * time.Millisecond)
	c.Increment("chrome", "Key_A")

	select {
	case updated := <-done:
		if updated == "" {
			t.Error("WaitForUpdate returned empty UpdatedAt")
		}
	case <-time.After(2 * time.Second):
		t.Error("WaitForUpdate timed out - notification not received")
	}
}

func TestWaitForUpdateContextCancel(t *testing.T) {
	c := New()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := c.WaitForUpdate(ctx, "")
	if err == nil {
		t.Error("WaitForUpdate should return error on context cancel")
	}
}

func TestUpdatedAtSetOnMerge(t *testing.T) {
	c := New()
	beforeMerge := c.UpdatedAt

	time.Sleep(1100 * time.Millisecond) // 确保时间戳不同（秒级精度）
	c.Merge(map[string]map[string]int64{"chrome": {"Key_A": 1}})

	if c.UpdatedAt == beforeMerge {
		t.Error("UpdatedAt should be set after Merge")
	}
}
