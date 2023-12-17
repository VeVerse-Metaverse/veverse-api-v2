begin;

-- game mode

drop table if exists game_mode;
create table if not exists game_mode
(
    id         uuid not null
        primary key
        references public.entities
            on delete cascade,
    package_id uuid default null -- id of the package that contains this game mode blueprint
                    references mods
                        on delete set null,
    name       text,             -- display name
    path       text              -- game mode blueprint path (relative to the game server files)
);

comment on table game_mode is 'Game mode table (game mode is a game mode blueprint like TDM, CTF, etc).';

create index if not exists game_mode_name_idx
    on game_mode (name);

-- insert example data
with e as (
    insert into entities (id, created_at, entity_type, public)
        values (gen_random_uuid(), now(), 'game-mode', true)
        returning id)
insert
into game_mode (id, name, path)
select e.id, 'Example', 'Game/Blueprints/GameModes/Example/BP_ExampleGameMode'
from e
returning id;

select *
from game_mode;

-- region

drop table if exists region;
create table if not exists region
(
    id    uuid not null
        primary key
        references public.entities
            on delete cascade,
    name  text, -- actual name (e.g. us-west-2)
    title text  -- display name (e.g. US West (Oregon))
);

-- pixel streaming instance

drop table if exists pixel_streaming_instance;
create table if not exists pixel_streaming_instance
(
    id         uuid not null
        primary key
        references public.entities
            on delete cascade,
    release_id uuid not null     -- id of the app release that this instance is running
        references app_v2
            on delete cascade,
    region_id  uuid default null -- id of the region where this instance is running
                    references region
                        on delete set null,
    host       text,             -- instance host for users to connect to
    port       int,              -- instance port for users to connect to
    status     text              -- instance status (online, offline, etc)
);

comment on table pixel_streaming_instance is 'Pixel streaming instance table (pixel streaming instance is a pixel streaming client instance).';

create index if not exists pixel_streaming_instance_release_id_idx
    on pixel_streaming_instance (release_id);

create index if not exists pixel_streaming_instance_region_id_idx
    on pixel_streaming_instance (region_id);

-- server

drop table if exists game_server_v2;
create table if not exists game_server_v2
(
    id             uuid not null
        primary key
        references public.entities
            on delete cascade,
    release_id     uuid              -- release id (to download the server files to the pod)
                        references release_v2
                            on delete set null,
    world_id       uuid default null -- id of the world running on this server
                        references spaces
                            on delete set null,
    game_mode_id   uuid default null -- id of the game mode running on this server
                        references game_mode
                            on delete set null,
    type           text,             -- server type (official, community)
    region_id      uuid default null -- server region, in cluster deployments this is the cluster region (e.g. us-east-1), in community deployments can be any string or empty
                        references region
                            on delete set null,
    host           text,             -- server host, in cluster deployments this is the cluster service host, in community deployments this is the server public IP
    port           int,              -- server port, in cluster deployments this is the cluster service port, in community deployments this is the server public port
    max_players    int,              -- server max players allowed at the server
    status         text,             -- server status, in cluster deployments this is the cluster pod status (created, deploying, online, offline, failed), in community deployments this is the server status (online, offline, etc).
    status_message text default null -- server status message (error message or empty)
);

comment on table game_server_v2 is 'Game server table (game server is a game instance hosting a game world with a game mode).';

create index if not exists game_server_release_id_idx
    on game_server_v2 (release_id);

create index if not exists game_server_world_id_idx
    on game_server_v2 (world_id);

create index if not exists game_server_game_mode_id_idx
    on game_server_v2 (game_mode_id);

create index if not exists game_server_type_idx
    on game_server_v2 (type);

create index if not exists game_server_region_idx
    on game_server_v2 (region_id);

create index if not exists game_server_status_idx
    on game_server_v2 (status);

select *
from release_v2;

-- insert example data
with e as (
    insert into entities (id, created_at, entity_type, public)
        values (gen_random_uuid(), now(), 'game-server-v2', true)
        returning id)
insert
into game_server_v2 (id, release_id, game_mode_id, world_id, type, region_id, host, port,
                     max_players,
                     status, status_message)
select e.id,
       'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid,
       'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid,
       'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid,
       'official',
       'aws-region',
       'test.gs.veverse.com',
       7777,
       100,
       'offline',
       null
from e
returning id;

select *
from game_server_v2;

-- server player

drop table if exists game_server_player_v2;
create table if not exists game_server_player_v2
(
    server_id  uuid not null
        references game_server_v2
            on delete cascade,
    user_id    uuid not null
        references public.entities
            on delete cascade,
    created_at timestamp, -- player connection time
    updated_at timestamp, -- player last update time (heartbeat or status change)
    status     text       -- player status (connected, disconnected)
);

comment on table game_server_player_v2 is 'Game server player table (game server player is a player connected to a game server).';

create index if not exists game_server_player_server_id_idx
    on game_server_player_v2 (server_id);

create index if not exists game_server_player_user_id_idx
    on game_server_player_v2 (user_id);

create index if not exists game_server_player_status_idx
    on game_server_player_v2 (status);

create index if not exists game_server_player_updated_at_idx
    on game_server_player_v2 (updated_at);

select *
from game_server_v2;

select *
from users;

-- insert example data
insert into game_server_player_v2 (server_id, user_id, created_at, updated_at, status)
values ('XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid, 'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid, now(), now(),
        'disconnected');

select *
from game_server_player_v2;

-- lobby

drop table if exists game_lobby;
create table if not exists game_lobby
(
    id          uuid not null
        primary key
        references public.entities
            on delete cascade,
    created_at  timestamp,
    updated_at  timestamp, -- lobby last update time (status change)
    server_id   uuid       -- id of the server the lobby is waiting to join (server is created when the lobby is ready)
        references game_server_v2
            on delete cascade,
    max_players int,       -- max players allowed in the lobby
    status      text       -- lobby status (waiting - waiting for players to join, ready - ready to join the server, failed - failed to join the server, closed - lobby closed)
);

comment on table game_lobby is 'Lobby table (lobby is a group of players waiting to join a game server). Note: Uses properties table to store lobby custom properties. Uses accesibles table to track ownership.';

-- insert example data
with e as (
    insert into entities (id, created_at, entity_type, public)
        values (gen_random_uuid(), now(), 'game-lobby', true)
        returning id)
insert
into game_lobby (id, created_at, updated_at, max_players, status)
select e.id,
       now(),
       now(),
       4,
       'closed'
from e
returning id;

select *
from game_lobby;

-- lobby player

drop table if exists game_lobby_player;
create table if not exists game_lobby_player
(
    lobby_id   uuid not null -- lobby id
        references game_lobby
            on delete cascade,
    user_id    uuid not null -- player who joined the lobby
        references public.entities
            on delete cascade,
    created_at timestamp,    -- player connection time
    updated_at timestamp,    -- player last update time ( status change)
    status     text          -- player status (joined - player joined the lobby, ready - player is ready to join the server, failed - player failed to join the server, left - player left the lobby)
);

comment on table game_lobby_player is 'Lobby player table (lobby player is a player waiting to join a game server).';

create index if not exists game_lobby_player_lobby_id_idx
    on game_lobby_player (lobby_id);

create index if not exists game_lobby_player_user_id_idx
    on game_lobby_player (user_id);

create index if not exists game_lobby_player_status_idx
    on game_lobby_player (status);

-- insert example data
insert into game_lobby_player (lobby_id, user_id, created_at, updated_at, status)
values ('XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid, 'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid, now(), now(),
        'left');

-- cloud save

drop table if exists game_cloud_save;
create table if not exists game_cloud_save
(
    id         uuid not null
        primary key
        references public.entities
            on delete cascade,
    created_at timestamp,
    updated_at timestamp,
    app_id     uuid not null, -- id of the app that created the save
    name       text not null  -- save name
);

comment on table game_cloud_save is 'Cloud save table (a save game stored in the cloud). Note: Uses accesibles table to track ownership. Uses files table to store the save file in the cloud.';

create index if not exists game_cloud_save_app_id_idx
    on game_cloud_save (app_id);

create index if not exists game_cloud_save_name_idx
    on game_cloud_save (name);

-- insert example data
select *
from app_v2;

with e as (
    insert into entities (id, created_at, entity_type, public)
        values (gen_random_uuid(), now(), 'game-cloud-save', true)
        returning id)
insert
into game_cloud_save (id, created_at, updated_at, app_id, name)
select e.id,
       now(),
       now(),
       'XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX'::uuid,
       'example'
from e
returning id;

select *
from game_cloud_save;

commit;
