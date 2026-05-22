# Topic Browser API — Bruno testing

Tests for materialized UNS topic APIs (`ListDeviceTopics`, `ListTopicNodes`, `GetDeviceTopic`, `GetDeviceTopicCatalogStatus`).

## Northbound vs southbound auth

| Surface | Proto | Caller | Auth |
|---------|-------|--------|------|
| **Northbound** | `api/devicemgmt/v1/devicemgmt.proto` | Management console (via Oathkeeper) | `Authorization: Bearer <JWT>` — `tenant_id` in claims |
| **Southbound** | `api/umh-core/v2/instance.proto` | Edge / DCD (umh-core) | `Cookie: token=<JWT>` after `POST /api/v2/instance/login` |

Topic **read** APIs (`ListDeviceTopics`, `ListTopicNodes`, `GetDeviceTopic`, `GetDeviceTopicCatalogStatus`) are **northbound** — use **Bearer**.

Topic **ingestion** happens on **southbound** push (`Instance Push` with status + `core.topicBrowser`).

## Prerequisites

1. Device Management running (`base_url`, default `http://localhost:8000`)
2. Database migrated: `psql "$DATABASE_URL" -f db/migrations/001_device_topics.up.sql` (or full `db/schema.postgres.sql`)
3. **Northbound:** set `console_bearer_token` — JWT with `tenant_id` (from Oathkeeper / console; not the device login cookie)
4. **Southbound (push path only):** `device_id`, `token_hash` from RegisterDevice; `jwt_token` from Instance-Login

## Load topic data

### Fixture push (default for this folder)

1. `Instance-Login.bru` — device cookie JWT (southbound)
2. `00-Push-TopicBrowserStatus.bru` — push `core.topicBrowser` using `fixtures/push-status-topic-browser.json`
3. `01`–`06` — topic APIs with **Bearer** (northbound)

Assertions in `02`–`05` expect fixture topic names (`Enterprise`, `Site`, `Temperature`, etc.).

### Live edge (integration)

Skip `00` if the device already syncs topics from umh-core (e.g. generic-generate bridge). Run `01`–`06` only; relax or skip tests that assert fixture-specific segment names.

## Test order

| Seq | File | Purpose |
|-----|------|---------|
| 00 | `00-Push-TopicBrowserStatus.bru` | Populate DB via instance push |
| 01 | `01-GetCatalogStatus.bru` | Catalog sync metadata |
| 02 | `02-ListTopicNodes-Root.bru` | Tree root segments |
| 03 | `03-ListTopicNodes-Child.bru` | Children under Enterprise |
| 04 | `04-ListDeviceTopics.bru` | Flat list + counts |
| 05 | `05-ListDeviceTopics-PathPrefix.bru` | Topics under prefix |
| 06 | `06-GetDeviceTopic.bru` | Single topic detail |
| 07 | `07-EnsureDeviceStatusSubscription.bru` | Queue edge subscribe (explicit) |

## Auth (topic APIs = northbound)

```
Authorization: Bearer {{console_bearer_token}}
```

Set `console_bearer_token` in Bruno environment to a console/Oathkeeper JWT that includes `tenant_id` for the tenant owning the device.

Do **not** use the device login cookie for `01`–`06` — that token is for southbound instance routes only (though the server middleware accepts cookie as a fallback for local dev).

