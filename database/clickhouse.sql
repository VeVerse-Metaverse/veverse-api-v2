CREATE DATABASE IF NOT EXISTS "dev";

SET allow_experimental_object_type = 1;

DROP TABLE IF EXISTS "dev"."request";

CREATE TABLE IF NOT EXISTS "dev"."request"
(
    id        UUID     DEFAULT generateUUIDv4(),
    ipv4      IPv4,                   -- IP address of the client
    ipv6      IPv6,                   -- IP address of the client
    userid    UUID,
    method LowCardinality(String),
    path      String,
    headers Map(String, String),
    body      String,
    status    UInt16,
    timestamp DateTime DEFAULT now(), -- timestamp of the event
    source LowCardinality(String),    -- source of the event (e.g. "APIv2")

    INDEX idx_userid userid TYPE minmax GRANULARITY 1024
) ENGINE = MergeTree() ORDER BY (source, timestamp, userid, id);


CREATE DATABASE IF NOT EXISTS "test";

SET allow_experimental_object_type = 1;

DROP TABLE IF EXISTS "test"."request";

CREATE TABLE IF NOT EXISTS "test"."request"
(
    id        UUID     DEFAULT generateUUIDv4(),
    ipv4      IPv4,                   -- IP address of the client
    ipv6      IPv6,                   -- IP address of the client
    userid    UUID,
    method LowCardinality(String),
    path      String,
    headers Map(String, String),
    body      String,
    status    UInt16,
    timestamp DateTime DEFAULT now(), -- timestamp of the event
    source LowCardinality(String),    -- source of the event (e.g. "APIv2")

    INDEX idx_userid userid TYPE minmax GRANULARITY 1024
) ENGINE = MergeTree() ORDER BY (source, timestamp, userid, id);


DROP TABLE IF EXISTS "dev"."events";
CREATE TABLE IF NOT EXISTS "dev"."events"
(
    id                UUID     DEFAULT generateUUIDv4(), -- unique id of the event
    appId             UUID,                              -- app id
    contextEntityId   UUID,                              -- context entity id (e.g. world id, package id, app id)
    contextEntityType String,                            -- context entity type (e.g. "world", "package", "app")
    userId            UUID,                              -- user id
    platform LowCardinality(String),                     -- platform (e.g. "Win64", "MacOS", "Linux", "IOS", "Android", "Web")
    deployment LowCardinality(String),                   -- deployment ("Client", "Server")
    configuration LowCardinality(String),                -- configuration of the event (e.g. "Development", "Test", "Shipping")
    event             String,                            -- event name
    timestamp         DateTime DEFAULT now(),            -- timestamp of the event
    payload           String,                            -- JSON payload

    INDEX idx_appId appId TYPE minmax GRANULARITY 1024,
    INDEX idx_contextEntityId contextEntityId TYPE minmax GRANULARITY 1024,
    INDEX idx_contextEntityType contextEntityType TYPE minmax GRANULARITY 1024,
    INDEX idx_userId userId TYPE minmax GRANULARITY 1024,
    INDEX idx_platform platform TYPE minmax GRANULARITY 1024,
    INDEX idx_deployment deployment TYPE minmax GRANULARITY 1024,
    INDEX idx_configuration configuration TYPE minmax GRANULARITY 1024,
    INDEX idx_event event TYPE minmax GRANULARITY 1024

) ENGINE = MergeTree() ORDER BY (timestamp, contextEntityType, contextEntityId, userId);


DROP TABLE IF EXISTS "test"."events";
CREATE TABLE IF NOT EXISTS "test"."events"
(
    id                UUID     DEFAULT generateUUIDv4(), -- unique id of the event
    appId             UUID,                              -- app id
    contextEntityId   UUID,                              -- context entity id (e.g. world id, package id, app id)
    contextEntityType String,                            -- context entity type (e.g. "world", "package", "app")
    userId            UUID,                              -- user id
    platform LowCardinality(String),                     -- platform (e.g. "Win64", "MacOS", "Linux", "IOS", "Android", "Web")
    deployment LowCardinality(String),                   -- deployment ("Client", "Server")
    configuration LowCardinality(String),                -- configuration of the event (e.g. "Development", "Test", "Shipping")
    event             String,                            -- event name
    timestamp         DateTime DEFAULT now(),            -- timestamp of the event
    payload           String,                            -- JSON payload

    INDEX idx_appId appId TYPE minmax GRANULARITY 1024,
    INDEX idx_contextEntityId contextEntityId TYPE minmax GRANULARITY 1024,
    INDEX idx_contextEntityType contextEntityType TYPE minmax GRANULARITY 1024,
    INDEX idx_userId userId TYPE minmax GRANULARITY 1024,
    INDEX idx_platform platform TYPE minmax GRANULARITY 1024,
    INDEX idx_deployment deployment TYPE minmax GRANULARITY 1024,
    INDEX idx_configuration configuration TYPE minmax GRANULARITY 1024,
    INDEX idx_event event TYPE minmax GRANULARITY 1024

) ENGINE = MergeTree() ORDER BY (timestamp, contextEntityType, contextEntityId, userId);

SELECT id, contextEntityType, platform, deployment, configuration, event, payload FROM "test".events;