package store_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/dansimau/hal/store"
	_ "github.com/glebarez/go-sqlite" // registers the "sqlite" database/sql driver
	"gorm.io/gorm"
	"gotest.tools/v3/assert"
)

// holdWriteLock opens a second, independent connection to the database at path
// and takes the write lock, returning a function that releases it.
func holdWriteLock(t *testing.T, path string) func() {
	t.Helper()

	other, err := sql.Open("sqlite", path)
	assert.NilError(t, err)

	conn, err := other.Conn(context.Background())
	assert.NilError(t, err)

	_, err = conn.ExecContext(context.Background(), "BEGIN IMMEDIATE")
	assert.NilError(t, err)

	return func() {
		_, err := conn.ExecContext(context.Background(), "COMMIT")
		assert.NilError(t, err)
		assert.NilError(t, conn.Close())
		assert.NilError(t, other.Close())
	}
}

// TestBatchIsRetriedWhileDatabaseLocked verifies that writes are not lost when
// another connection holds the write lock for longer than busy_timeout: the
// batch is retried once the lock is released rather than each write failing.
func TestBatchIsRetriedWhileDatabaseLocked(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/locked.db"
	db, err := store.Open(path)
	assert.NilError(t, err)
	defer db.Close()

	release := holdWriteLock(t, path)

	for i := range 3 {
		db.EnqueueWrite(func(tx *gorm.DB) error {
			return tx.Create(&store.Log{Timestamp: time.Now(), Level: "INFO", LogText: string(rune('a' + i))}).Error
		})
	}

	// Hold the lock past busy_timeout (5s) so the first commit attempt fails.
	time.Sleep(6 * time.Second)
	release()

	db.WaitForWrites()

	var count int64
	assert.NilError(t, db.Model(&store.Log{}).Count(&count).Error)
	assert.Equal(t, count, int64(3))
}

// TestDeleteInChunks verifies that DeleteInChunks removes exactly the matching
// rows across multiple chunks and reports the total.
func TestDeleteInChunks(t *testing.T) {
	t.Parallel()

	db := openTestStore(t)

	rollups := make([]store.MetricRollup, 2500)
	for i := range rollups {
		rollups[i] = store.MetricRollup{MetricType: store.MetricTypeTickProcessingTime, BucketStart: int64(i * 60)}
	}
	assert.NilError(t, db.CreateInBatches(rollups, 500).Error)

	deleted, err := store.DeleteInChunks(db.DB, &store.MetricRollup{}, "bucket_start < ?", 2100*60)
	assert.NilError(t, err)
	assert.Equal(t, deleted, int64(2100))

	var remaining int64
	assert.NilError(t, db.Model(&store.MetricRollup{}).Count(&remaining).Error)
	assert.Equal(t, remaining, int64(400))
}

// TestPruneQueriesUseIndexes verifies that the time-range queries used by
// pruning and rollups can use an index instead of scanning the whole table.
func TestPruneQueriesUseIndexes(t *testing.T) {
	t.Parallel()

	db := openTestStore(t)

	for _, tc := range []struct{ query, index string }{
		{"SELECT MAX(bucket_start) FROM metric_rollups", "idx_rollup_bucket_start"},
		{"SELECT id FROM metric_rollups WHERE bucket_start < 1", "idx_rollup_bucket_start"},
		{"SELECT id FROM metrics WHERE timestamp < '2026-01-01'", "idx_metric_timestamp"},
	} {
		var plan []struct{ Detail string }
		assert.NilError(t, db.Raw("EXPLAIN QUERY PLAN "+tc.query).Scan(&plan).Error)
		assert.Assert(t, len(plan) > 0)
		assert.Assert(t, strings.Contains(plan[0].Detail, tc.index), "%s: plan %q", tc.query, plan[0].Detail)
	}
}

// TestOpenSwitchesFullAutoVacuumToIncremental verifies that an existing
// database created with auto_vacuum=FULL is switched to INCREMENTAL on open.
func TestOpenSwitchesFullAutoVacuumToIncremental(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/full.db"

	legacy, err := sql.Open("sqlite", path+"?_pragma=auto_vacuum(FULL)")
	assert.NilError(t, err)
	_, err = legacy.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY)")
	assert.NilError(t, err)
	assert.NilError(t, legacy.Close())

	db, err := store.Open(path)
	assert.NilError(t, err)
	defer db.Close()

	var autoVacuum int
	assert.NilError(t, db.Raw("PRAGMA auto_vacuum").Scan(&autoVacuum).Error)
	assert.Equal(t, autoVacuum, 2) // INCREMENTAL
	assert.NilError(t, store.IncrementalVacuum(db.DB, 10))
}

// TestIncrementalVacuumReclaimsPages verifies that IncrementalVacuum returns
// exactly the requested number of free pages to the filesystem (not just one,
// which is all a plain Exec of the pragma frees), and stops when none are left.
func TestIncrementalVacuumReclaimsPages(t *testing.T) {
	t.Parallel()

	db, err := store.Open(t.TempDir() + "/vacuum.db")
	assert.NilError(t, err)
	defer db.Close()

	for _, stmt := range []string{
		"CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)",
		"WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < 2000) " +
			"INSERT INTO t (v) SELECT '" + strings.Repeat("x", 1000) + "' FROM n",
		"DELETE FROM t",
	} {
		assert.NilError(t, db.Exec(stmt).Error)
	}

	freePages := func() int {
		var n int
		assert.NilError(t, db.Raw("PRAGMA freelist_count").Scan(&n).Error)
		return n
	}

	before := freePages()
	assert.Assert(t, before >= 100, "expected the delete to free pages, got %d", before)

	assert.NilError(t, store.IncrementalVacuum(db.DB, 50))
	assert.Equal(t, freePages(), before-50)

	assert.NilError(t, store.IncrementalVacuum(db.DB, before))
	assert.Equal(t, freePages(), 0)
}
