create table users (
    id uuid primary key,
    auth_uid varchar(255) unique,
    name varchar(255) not null,
    email varchar(255) unique,
    phone varchar(255),
    deleted_at timestamp with time zone,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

create table outlets (
    id uuid primary key,
    name varchar(150) not null,
    latitude numeric(10, 7) not null,
    longitude numeric(10, 7) not null,
    radius_meters integer not null,
    geofence_enabled boolean not null default false,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

create table outlet_memberships (
    id uuid primary key,
    outlet_id uuid not null,
    user_id uuid not null,
    role varchar(20) not null,
    status varchar(20) not null,
    invited_by_user_id uuid,
    removed_at timestamp with time zone,
    removed_by_user_id uuid,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    constraint uk_outlet_membership_outlet_user unique (outlet_id, user_id),
    constraint fk_outlet_membership_outlet foreign key (outlet_id) references outlets(id),
    constraint fk_outlet_membership_user foreign key (user_id) references users(id),
    constraint fk_outlet_membership_invited_by foreign key (invited_by_user_id) references users(id),
    constraint fk_outlet_membership_removed_by foreign key (removed_by_user_id) references users(id)
);

create table refresh_tokens (
    id uuid primary key,
    token_hash varchar(255) not null unique,
    expires_at timestamp with time zone not null,
    revoked boolean not null,
    user_id uuid not null,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    constraint fk_refresh_token_user foreign key (user_id) references users(id)
);

create table attendance_entries (
    id uuid primary key,
    user_id uuid not null,
    outlet_id uuid not null,
    type varchar(20) not null,
    entry_time timestamp with time zone not null,
    latitude numeric(10, 7) not null,
    longitude numeric(10, 7) not null,
    created_by uuid,
    updated_by uuid,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    constraint fk_attendance_entry_user foreign key (user_id) references users(id),
    constraint fk_attendance_entry_outlet foreign key (outlet_id) references outlets(id)
);

create table audit_events (
    id uuid primary key,
    actor_user_id uuid,
    action varchar(100) not null,
    entity_type varchar(100) not null,
    entity_id uuid,
    metadata_json text,
    ip_address varchar(100),
    user_agent varchar(500),
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

create index idx_users_auth_uid on users(auth_uid);
create index idx_users_email on users(email);
create index idx_outlet_memberships_lookup on outlet_memberships(outlet_id, user_id, removed_at);
create index idx_outlet_memberships_user_status on outlet_memberships(user_id, status, removed_at);
create index idx_outlet_memberships_outlet_removed on outlet_memberships(outlet_id, removed_at);
create index idx_refresh_tokens_user_revoked on refresh_tokens(user_id, revoked);
create index idx_refresh_tokens_expires_at on refresh_tokens(expires_at);
create index idx_attendance_entries_outlet_entry_time on attendance_entries(outlet_id, entry_time);
create index idx_attendance_entries_outlet_user_entry_time on attendance_entries(outlet_id, user_id, entry_time);
create index idx_audit_events_actor_created_at on audit_events(actor_user_id, created_at);
create index idx_audit_events_entity_created_at on audit_events(entity_type, entity_id, created_at);
