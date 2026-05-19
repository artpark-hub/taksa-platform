-- Rollback migration 001 (drops topic browser tables; deletes all materialized topics)

DROP TABLE IF EXISTS device_topic_catalog;
DROP TABLE IF EXISTS device_topics;
