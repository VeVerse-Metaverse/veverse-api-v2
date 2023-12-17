-- Add a new LauncherV2 with specified name (create an entity and launcher_v2).
BEGIN;
WITH e AS (
    INSERT INTO entities (id, created_at, updated_at, entity_type, public, views)
        VALUES (gen_random_uuid(), now(), null, 'launcher-v2', true, null)
        RETURNING id)
INSERT
INTO launcher_v2 (id, name)
SELECT e.id, 'LE7EL'
FROM e;
ROLLBACK;
COMMIT;

-- Add a new AppV2 for the LauncherV2.
BEGIN;
WITH e AS (
    INSERT INTO entities (id, created_at, updated_at, entity_type, public, views)
        VALUES (gen_random_uuid(), now(), null, 'app-v2', true, null)
        RETURNING id)
INSERT
INTO app_v2 (id, name, description, external, sdk_id)
SELECT e.id,
       'LE7EL',
       'LE7EL Play is a social platform for Web3 Gaming, where Players can explore and interact with Games, Metaverses and their Communities',
       false,
       null
FROM e;
ROLLBACK;
COMMIT;

-- Get LauncherV2 ID: {XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX}
SELECT id
FROM launcher_v2
WHERE name = 'LE7EL';

-- Get AppV2 ID: {XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX}
SELECT id
FROM app_v2
WHERE name = 'LE7EL';

-- Add the AppV2 to the LauncherV2.
BEGIN;
INSERT INTO launcher_apps_v2 (launcher_id, app_id)
VALUES ('XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid, 'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid);
ROLLBACK;
COMMIT;

-- Add a new ReleaseV2 for the LauncherV2.
BEGIN;
WITH e AS (
    INSERT INTO entities (id, created_at, updated_at, entity_type, public, views)
        VALUES (gen_random_uuid(), now(), null, 'release-v2', true, null)
        RETURNING id)
INSERT
INTO release_v2 (id, entity_id, version, code_version, content_version, archive, name, description)
SELECT e.id,
       'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid,
       '1.0.0',
       '1.0.0',
       '1.0.0',
       true,
       'LE7EL',
       'LE7EL Play is a social platform for Web3 Gaming, where Players can explore and interact with Games, Metaverses and their Communities'
FROM e;
ROLLBACK;
COMMIT;

-- Get ReleaseV2 ID: {XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX}
SELECT id
FROM release_v2
WHERE entity_id = 'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid;

-- Add a archive file to the ReleaseV2.
BEGIN;
INSERT INTO files (id, entity_id, type, url, mime, size, uploaded_by, width, height, updated_at, hash)
VALUES (gen_random_uuid(), 'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid, 'release-archive',
        'https://xxxx.s3-xxxx.amazonaws.com/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX',
        'application/zip', 0, default, 0, 0,
        default, default);
ROLLBACK;
COMMIT;

-- Add a launcher card image file to the AppV2.
BEGIN;
INSERT INTO files (id, entity_id, type, url, mime, size, uploaded_by, width, height, updated_at, hash)
VALUES (gen_random_uuid(), 'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid, 'launcher-card-image',
        'https://xxxx.s3-xxxx.amazonaws.com/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX',
        'image/png', 0, default, 0, 0,
        default, default);
ROLLBACK;
COMMIT;

-- Add a launcher background image file to the AppV2.
BEGIN;
INSERT INTO files (id, entity_id, type, url, mime, size, uploaded_by, width, height, updated_at, hash)
VALUES (gen_random_uuid(), 'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid, 'launcher-background-image',
        'https://xxxx.s3-xxxx.amazonaws.com/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX',
        'image/png', 0, default, 0, 0,
        default, default);
ROLLBACK;
COMMIT;

UPDATE files
SET url  = 'https://XXXXXXXXXXXX.amazonaws.com/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/launcher-app-background-image.jpg',
    mime = 'image/jpeg',
    type = 'launcher-app-background-image',
    size = 143324
WHERE id = 'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid;

UPDATE files
SET url  = 'https://XXXXXXXXXXXX.amazonaws.com/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/launcher-app-card-image.jpg',
    mime = 'image/jpeg',
    type = 'launcher-app-card-image',
    size = 31245
WHERE id = 'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid;

-- Select release files.
SELECT *
FROM files
WHERE entity_id = 'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid;

-- Add a new ReleaseV2 for the AppV2.
BEGIN;
WITH e AS (
    INSERT INTO entities (id, created_at, updated_at, entity_type, public, views)
        VALUES (gen_random_uuid(), now(), null, 'release-v2', true, null)
        RETURNING id)
INSERT
INTO release_v2 (id, entity_id, version, code_version, content_version, archive, name, description)
SELECT e.id,
       'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid,
       '1.0.0',
       '1.0.0',
       '1.0.0',
       true,
       'LE7EL',
       'LE7EL Play is a social platform for Web3 Gaming, where Players can explore and interact with Games, Metaverses and their Communities'
FROM e;
ROLLBACK;
COMMIT;

-- Add a archive file to the AppV2 ReleaseV2.
BEGIN;
INSERT INTO files (id, entity_id, type, url, mime, size, uploaded_by, width, height, updated_at, hash)
VALUES (gen_random_uuid(), 'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid, 'release-archive',
        'https://XXXXXXXXXXXX.amazonaws.com/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/2048f037-3658-474b-99e9-5c959665d8fe',
        'application/zip', 0, default, 0, 0,
        default, default)
RETURNING id;

UPDATE files
SET url = 'https://XXXXXXXXXXXX.amazonaws.com/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'
WHERE id = 'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid;
ROLLBACK;
COMMIT;

SELECT * FROM files WHERE id = 'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX';
