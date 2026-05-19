# Database migrations (device-management)

SQL migrations for **incremental** schema changes. Full greenfield installs can use `db/schema.postgres.sql` instead (includes the same objects).

## Apply

```bash
export DATABASE_URL="postgres://user:password@localhost:5432/taksa_platform_dm?sslmode=disable"

psql "$DATABASE_URL" -f db/migrations/001_device_topics.up.sql
```

## Rollback

```bash
psql "$DATABASE_URL" -f db/migrations/001_device_topics.down.sql
```

Migrations are idempotent (`CREATE TABLE IF NOT EXISTS`, etc.) and safe to re-run.

## Local API testing (topics)

Populate topic rows via a real device status push (preferred) or the Bruno southbound push in `bruno/05-TopicBrowser/00-Push-TopicBrowserStatus.bru`. See `bruno/05-TopicBrowser/README-TopicBrowser-API-Testing.md`.

## Migration list

| File | Description |
|------|-------------|
| `001_device_topics.up.sql` | `device_topics`, `device_topic_catalog`, indexes |
