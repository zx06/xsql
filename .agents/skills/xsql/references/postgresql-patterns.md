# PostgreSQL Analysis Patterns

SQL analysis patterns for PostgreSQL databases. Use these when the profile's `db` field is `pg`.

Run `xsql query "<SQL>" -p <profile> -f json` to execute any query below.

## Table of Contents

1. [Database Overview](#database-overview)
2. [Database Total Size](#database-total-size)
3. [Fragmentation Analysis](#fragmentation-analysis)
4. [Missing Index Detection](#missing-index-detection)
5. [Stale Table Detection](#stale-table-detection)
6. [Column Distribution Analysis](#column-distribution-analysis)
7. [Growth Trend Analysis](#growth-trend-analysis)
8. [Server Status](#server-status)

## Database Overview

Top N tables by size. Use raw bytes rather than `pg_size_pretty` — it may fail in restricted environments.

```sql
SELECT relname AS table_name,
  reltuples::bigint AS row_estimate,
  pg_total_relation_size(relid) AS total_bytes,
  pg_relation_size(relid) AS data_bytes,
  pg_indexes_size(relid) AS index_bytes
FROM pg_stat_user_tables
ORDER BY pg_total_relation_size(relid) DESC
LIMIT 20;
```

## Database Total Size

```sql
SELECT pg_database_size(current_database()) AS total_bytes;
```

For a breakdown across all user tables:

```sql
SELECT SUM(pg_total_relation_size(relid)) AS total_bytes
FROM pg_stat_user_tables;
```

## Fragmentation Analysis

PostgreSQL uses MVCC and can accumulate dead tuples. A high ratio of dead tuples to live tuples means autovacuum isn't keeping up, and `VACUUM FULL` would reclaim significant space.

```sql
SELECT relname AS table_name,
  n_live_tup,
  n_dead_tup,
  CASE WHEN n_live_tup > 0
    THEN ROUND(n_dead_tup * 100.0 / n_live_tup, 2)
    ELSE 0 END AS dead_pct,
  last_vacuum,
  last_autovacuum,
  last_analyze,
  last_autoanalyze
FROM pg_stat_user_tables
WHERE n_dead_tup > 1000
ORDER BY n_dead_tup DESC
LIMIT 20;
```

Interpretation: `dead_pct > 10%` suggests autovacuum might not be keeping up; `dead_pct > 30%` is a strong signal for manual `VACUUM` or `VACUUM FULL`.

## Missing Index Detection

Tables where sequential scans far exceed index scans on large tables suggest missing indexes for common query patterns.

```sql
SELECT schemaname, relname AS table_name,
  reltuples::bigint AS row_estimate,
  COALESCE(idx_scan, 0) AS index_scans,
  COALESCE(seq_scan, 0) AS seq_scans,
  CASE WHEN (seq_scan + COALESCE(idx_scan, 0)) > 0
    THEN ROUND(seq_scan * 100.0 / (seq_scan + COALESCE(idx_scan, 0)), 2)
    ELSE 0 END AS seq_pct
FROM pg_stat_user_tables
WHERE reltuples > 10000
  AND seq_scan > COALESCE(idx_scan, 0) * 2
ORDER BY seq_scan DESC
LIMIT 20;
```

A high `seq_pct` on a large table indicates missing indexes for common query patterns.

For a count of indexes per table:

```sql
SELECT schemaname, relname AS table_name,
  reltuples::bigint AS row_estimate,
  COUNT(i.indexname) AS index_count
FROM pg_stat_user_tables t
LEFT JOIN pg_indexes i ON t.relname = i.tablename AND t.schemaname = i.schemaname
WHERE t.reltuples > 10000
GROUP BY schemaname, relname, reltuples
HAVING COUNT(i.indexname) <= 1
ORDER BY reltuples DESC;
```

## Stale Table Detection

PostgreSQL doesn't track last update time directly, but you can infer staleness from scan and maintenance activity. A table with few scans might simply be used rarely but critically — flag for user confirmation rather than assuming it's abandoned.

```sql
SELECT schemaname, relname AS table_name,
  reltuples::bigint AS row_estimate,
  pg_total_relation_size(relid) AS total_bytes,
  last_vacuum,
  last_autovacuum,
  last_analyze,
  last_autoanalyze,
  COALESCE(seq_scan, 0) + COALESCE(idx_scan, 0) AS total_scans
FROM pg_stat_user_tables
WHERE pg_total_relation_size(relid) > 1048576  -- only tables >1MB
  AND COALESCE(seq_scan, 0) + COALESCE(idx_scan, 0) < 10
ORDER BY pg_total_relation_size(relid) DESC;
```

## Column Distribution Analysis

When the user asks about data distribution or skew in a column:

```sql
SELECT <column>, COUNT(*) AS cnt,
  ROUND(COUNT(*) * 100.0 / (SELECT COUNT(*) FROM <table>), 1) AS pct
FROM <table>
GROUP BY <column>
ORDER BY cnt DESC
LIMIT 20;
```

## Growth Trend Analysis

For tables that seem large or are growing, check recent write volume to understand the growth rate:

```sql
SELECT DATE(<timestamp_column>) AS dt, COUNT(*) AS rows_written
FROM <table>
WHERE <timestamp_column> >= CURRENT_DATE - INTERVAL '14 days'
GROUP BY DATE(<timestamp_column>)
ORDER BY dt DESC;
```

## Server Status

Key PostgreSQL settings and activity for performance diagnostics:

```sql
-- Key configuration
SELECT name, setting, unit, short_desc
FROM pg_settings
WHERE name IN (
  'shared_buffers', 'work_mem', 'effective_cache_size',
  'max_connections', 'maintenance_work_mem',
  'checkpoint_completion_target', 'wal_buffers',
  'default_statistics_target'
);

-- Current activity
SELECT state, COUNT(*) AS count,
  MAX(now() - query_start) AS longest_running
FROM pg_stat_activity
WHERE state != 'idle'
GROUP BY state
ORDER BY count DESC;

-- Connection stats
SELECT max_conn, used, res_for_super,
  (max_conn - used - res_for_super) AS available
FROM (
  SELECT setting::int AS max_conn,
    (SELECT COUNT(*) FROM pg_stat_activity) AS used,
    3 AS res_for_super
) sub;
```
