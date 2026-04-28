# MySQL Analysis Patterns

SQL analysis patterns for MySQL databases. Use these when the profile's `db` field is `mysql`.

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

Top N tables by size — the starting point for understanding a database.

```sql
SELECT TABLE_NAME,
  TABLE_ROWS,
  ROUND((DATA_LENGTH + INDEX_LENGTH) / 1024 / 1024, 2) AS total_mb,
  ROUND(DATA_LENGTH / 1024 / 1024, 2) AS data_mb,
  ROUND(INDEX_LENGTH / 1024 / 1024, 2) AS index_mb,
  UPDATE_TIME
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = '<database>'
ORDER BY (DATA_LENGTH + INDEX_LENGTH) DESC
LIMIT 20;
```

## Database Total Size

```sql
SELECT ROUND(SUM(DATA_LENGTH + INDEX_LENGTH) / 1024 / 1024, 2) AS total_mb,
  SUM(TABLE_ROWS) AS total_rows
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = '<database>';
```

## Fragmentation Analysis

InnoDB `DATA_FREE` shows allocated-but-unused space. A high ratio of `DATA_FREE` to used space means `OPTIMIZE TABLE` would reclaim significant space and improve scan performance.

```sql
SELECT TABLE_NAME,
  ROUND((DATA_LENGTH + INDEX_LENGTH) / 1024 / 1024, 2) AS used_mb,
  ROUND(DATA_FREE / 1024 / 1024, 2) AS free_mb,
  CASE WHEN (DATA_LENGTH + INDEX_LENGTH) > 0
    THEN ROUND(DATA_FREE / (DATA_LENGTH + INDEX_LENGTH + DATA_FREE) * 100, 2)
    ELSE 0 END AS frag_pct
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = '<database>'
  AND ENGINE = 'InnoDB'
  AND DATA_FREE > 1048576  -- only tables with >1MB free space
ORDER BY DATA_FREE DESC
LIMIT 20;
```

Interpretation: `frag_pct > 30%` is worth investigating; `frag_pct > 50%` is a strong signal for `OPTIMIZE TABLE`.

## Missing Index Detection

Tables with high row count but few indexes are likely candidates for slow queries on common filter/join columns.

```sql
SELECT t.TABLE_NAME, t.TABLE_ROWS,
  COUNT(DISTINCT i.INDEX_NAME) AS index_count
FROM information_schema.TABLES t
LEFT JOIN information_schema.STATISTICS i
  ON t.TABLE_SCHEMA = i.TABLE_SCHEMA AND t.TABLE_NAME = i.TABLE_NAME
WHERE t.TABLE_SCHEMA = '<database>'
GROUP BY t.TABLE_NAME, t.TABLE_ROWS
HAVING index_count <= 1 AND t.TABLE_ROWS > 10000
ORDER BY t.TABLE_ROWS DESC;
```

## Stale Table Detection

Tables with no recent update activity might be abandoned. Tables with `UPDATE_TIME IS NULL` may use bulk ETL or MyISAM — flag them for user confirmation rather than assuming they're dead.

```sql
SELECT TABLE_NAME, TABLE_ROWS,
  ROUND((DATA_LENGTH + INDEX_LENGTH) / 1024 / 1024, 2) AS total_mb,
  UPDATE_TIME
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = '<database>'
  AND (UPDATE_TIME IS NULL OR UPDATE_TIME < DATE_SUB(NOW(), INTERVAL 180 DAY))
  AND (DATA_LENGTH + INDEX_LENGTH) > 1048576  -- only tables >1MB
ORDER BY (DATA_LENGTH + INDEX_LENGTH) DESC;
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
WHERE <timestamp_column> >= DATE_SUB(NOW(), INTERVAL 14 DAY)
GROUP BY DATE(<timestamp_column>)
ORDER BY dt DESC;
```

## Server Status

Key MySQL status variables for deep performance diagnostics:

```sql
SHOW GLOBAL STATUS WHERE Variable_name IN (
  'Innodb_buffer_pool_size', 'Innodb_buffer_pool_pages_free',
  'Slow_queries', 'Threads_connected', 'Max_used_connections',
  'Uptime', 'Queries', 'Innodb_row_lock_waits'
);
```
