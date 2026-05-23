package partman

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const deleteBatchSize = 10000

var partitionBoundsRE = regexp.MustCompile(`FROM \('([^']+)'\) TO \('([^']+)'\)`)

type Granularity int

const (
	Daily Granularity = iota + 1
	Weekly
)

type Manager struct {
	pool        *pgxpool.Pool
	table       string
	granularity Granularity
	createAhead int
}

func NewManager(pool *pgxpool.Pool, table string, granularity Granularity, createAhead int) *Manager {
	return &Manager{
		pool:        pool,
		table:       table,
		granularity: granularity,
		createAhead: createAhead,
	}
}

func (m *Manager) EnsureFuturePartitions(ctx context.Context) error {
	if m == nil {
		return nil
	}

	start := m.granularity.truncate(time.Now().UTC())
	for i := 0; i <= m.createAhead; i++ {
		lower := m.granularity.addPeriods(start, i)
		upper := m.granularity.next(lower)
		name := m.partitionName(lower)
		if _, err := m.pool.Exec(ctx, fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS public.%s PARTITION OF public.%s FOR VALUES FROM (%s) TO (%s)`,
			quoteIdent(name),
			quoteIdent(m.table),
			quoteLiteralTimestamp(lower),
			quoteLiteralTimestamp(upper),
		)); err != nil {
			return fmt.Errorf("create partition %s: %w", name, err)
		}
	}

	return nil
}

func (m *Manager) DropExpiredPartitions(ctx context.Context, cutoff time.Time) ([]string, error) {
	if m == nil {
		return nil, nil
	}

	type partitionInfo struct {
		name  string
		upper time.Time
	}

	rows, err := m.pool.Query(ctx, `
		SELECT child.relname, pg_get_expr(child.relpartbound, child.oid)
		FROM pg_inherits
		JOIN pg_class parent ON parent.oid = pg_inherits.inhparent
		JOIN pg_class child ON child.oid = pg_inherits.inhrelid
		JOIN pg_namespace ns ON ns.oid = child.relnamespace
		WHERE parent.relname = $1
		  AND ns.nspname = 'public'
	`, m.table)
	if err != nil {
		return nil, fmt.Errorf("query partitions for %s: %w", m.table, err)
	}
	defer rows.Close()

	var partitions []partitionInfo
	for rows.Next() {
		var name string
		var bound string
		if err := rows.Scan(&name, &bound); err != nil {
			return nil, fmt.Errorf("scan partition metadata for %s: %w", m.table, err)
		}
		if strings.Contains(bound, "DEFAULT") {
			continue
		}
		upper, err := parsePartitionUpperBound(bound)
		if err != nil {
			return nil, fmt.Errorf("parse bound for %s: %w", name, err)
		}
		if !upper.After(cutoff.UTC()) {
			partitions = append(partitions, partitionInfo{name: name, upper: upper})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate partitions for %s: %w", m.table, err)
	}

	sort.Slice(partitions, func(i, j int) bool {
		return partitions[i].upper.Before(partitions[j].upper)
	})

	dropped := make([]string, 0, len(partitions))
	for _, partition := range partitions {
		if _, err := m.pool.Exec(ctx, fmt.Sprintf(`DROP TABLE public.%s`, quoteIdent(partition.name))); err != nil {
			return dropped, fmt.Errorf("drop partition %s: %w", partition.name, err)
		}
		dropped = append(dropped, partition.name)
	}

	return dropped, nil
}

func (m *Manager) DeleteExpiredRowsFromDefault(ctx context.Context, cutoff time.Time) (int64, error) {
	if m == nil {
		return 0, nil
	}

	defaultTable := m.defaultPartitionName()
	var exists bool
	if err := m.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+defaultTable).Scan(&exists); err != nil {
		return 0, fmt.Errorf("check default partition for %s: %w", m.table, err)
	}
	if !exists {
		return 0, nil
	}

	totalDeleted := int64(0)
	for {
		tag, err := m.pool.Exec(ctx, fmt.Sprintf(`
			WITH doomed AS (
				SELECT ctid
				FROM public.%s
				WHERE "timestamp" < $1
				LIMIT $2
			)
			DELETE FROM public.%s
			WHERE ctid IN (SELECT ctid FROM doomed)
		`, quoteIdent(defaultTable), quoteIdent(defaultTable)), cutoff.UTC(), deleteBatchSize)
		if err != nil {
			return totalDeleted, fmt.Errorf("delete expired rows from %s: %w", defaultTable, err)
		}
		deleted := tag.RowsAffected()
		totalDeleted += deleted
		if deleted < deleteBatchSize {
			return totalDeleted, nil
		}
	}
}

func (m *Manager) partitionName(lower time.Time) string {
	return fmt.Sprintf("%s_p_%s", m.table, lower.UTC().Format("20060102"))
}

func (m *Manager) defaultPartitionName() string {
	return m.table + "_default"
}

func (g Granularity) truncate(t time.Time) time.Time {
	u := t.UTC()
	day := time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)

	switch g {
	case Daily:
		return day
	case Weekly:
		offset := int(day.Weekday())
		if offset == 0 {
			offset = 7
		}
		return day.AddDate(0, 0, -(offset - 1))
	default:
		return day
	}
}

func (g Granularity) next(t time.Time) time.Time {
	switch g {
	case Weekly:
		return t.UTC().AddDate(0, 0, 7)
	default:
		return t.UTC().AddDate(0, 0, 1)
	}
}

func (g Granularity) addPeriods(t time.Time, periods int) time.Time {
	switch g {
	case Weekly:
		return t.UTC().AddDate(0, 0, 7*periods)
	default:
		return t.UTC().AddDate(0, 0, periods)
	}
}

func parsePartitionUpperBound(bound string) (time.Time, error) {
	matches := partitionBoundsRE.FindStringSubmatch(bound)
	if len(matches) != 3 {
		return time.Time{}, fmt.Errorf("unrecognized partition bound: %q", bound)
	}
	return parseBoundTimestamp(matches[2])
}

func parseBoundTimestamp(value string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05.999999-07",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05-07:00",
	}

	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("parse timestamp %q", value)
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func quoteLiteralTimestamp(t time.Time) string {
	return "'" + t.UTC().Format("2006-01-02 15:04:05-07") + "'"
}
