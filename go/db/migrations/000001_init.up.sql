create table users (
    id uuid primary key default gen_random_uuid(),
    auth_uid varchar(255) unique,
    name varchar(255) not null,
    email varchar(255) unique,
    phone varchar(255),
    historical_email varchar(255),
    deleted_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table outlets (
    id uuid primary key default gen_random_uuid(),
    name varchar(150) not null,
    latitude numeric(10,7) not null check (latitude between -90 and 90),
    longitude numeric(10,7) not null check (longitude between -180 and 180),
    radius_meters integer not null check (radius_meters > 0),
    geofence_enabled boolean not null default false,
    removed_at timestamptz,
    removed_by_user_id uuid,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint fk_outlet_removed_by foreign key (removed_by_user_id) references users(id)
);

create table outlet_memberships (
    id uuid primary key default gen_random_uuid(),
    outlet_id uuid not null references outlets(id),
    user_id uuid not null references users(id),
    role varchar(20) not null check (role in ('OWNER','EMPLOYEE')),
    status varchar(20) not null check (status in ('ACCEPTED','INVITED','REJECTED')),
    display_name varchar(255) not null,
    invited_by_user_id uuid references users(id),
    removed_at timestamptz,
    removed_by_user_id uuid references users(id),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint uk_outlet_membership_outlet_user unique (outlet_id, user_id)
);

create table refresh_tokens (
    id uuid primary key default gen_random_uuid(),
    token_hash varchar(255) not null unique,
    expires_at timestamptz not null,
    revoked boolean not null,
    user_id uuid not null references users(id),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table attendance_entries (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id),
    outlet_id uuid not null references outlets(id),
    type varchar(20) not null check (type in ('CLOCK_IN','CLOCK_OUT')),
    entry_time timestamptz not null,
    latitude numeric(10,7) not null check (latitude between -90 and 90),
    longitude numeric(10,7) not null check (longitude between -180 and 180),
    created_by uuid references users(id),
    updated_by uuid references users(id),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table audit_events (
    id uuid primary key default gen_random_uuid(),
    actor_user_id uuid,
    action varchar(100) not null,
    entity_type varchar(100) not null,
    entity_id uuid,
    metadata_json jsonb,
    ip_address varchar(100),
    user_agent varchar(500),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index idx_outlet_memberships_lookup on outlet_memberships(outlet_id, user_id, removed_at);
create index idx_outlet_memberships_user_status on outlet_memberships(user_id, status, removed_at);
create index idx_outlet_memberships_outlet_removed on outlet_memberships(outlet_id, removed_at);
create index idx_refresh_tokens_user_revoked on refresh_tokens(user_id, revoked);
create index idx_refresh_tokens_expires_at on refresh_tokens(expires_at);
create index idx_attendance_entries_outlet_entry_time on attendance_entries(outlet_id, entry_time);
create index idx_attendance_entries_outlet_user_entry_time on attendance_entries(outlet_id, user_id, entry_time);
create index idx_audit_events_actor_created_at on audit_events(actor_user_id, created_at);
create index idx_audit_events_entity_created_at on audit_events(entity_type, entity_id, created_at);
create index idx_outlets_removed_at on outlets(removed_at);
