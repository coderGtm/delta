-- Seed data for k6 load/rate-limit tests.
-- Idempotent: safe to re-run against the same database.
-- Assumes the delta-postgres container is running (docker compose up).

insert into users (id, auth_uid, name, email, phone, deleted_at, created_at, updated_at) values
    ('11111111-1111-1111-1111-111111111111', 'loadtest-owner-uid',  'Load Owner',    'owner@loadtest.local',    '+911111111111', null, now(), now()),
    ('22222222-2222-2222-2222-222222222222', 'loadtest-employee-1', 'Load Employee 1', 'employee1@loadtest.local', '+912222222222', null, now(), now()),
    ('33333333-3333-3333-3333-333333333333', 'loadtest-employee-2', 'Load Employee 2', 'employee2@loadtest.local', '+913333333333', null, now(), now()),
    ('44444444-4444-4444-4444-444444444444', 'loadtest-employee-3', 'Load Employee 3', 'employee3@loadtest.local', '+914444444444', null, now(), now()),
    ('55555555-5555-5555-5555-555555555555', 'loadtest-employee-4', 'Load Employee 4', 'employee4@loadtest.local', '+915555555555', null, now(), now())
on conflict (id) do nothing;

insert into outlets (id, name, latitude, longitude, radius_meters, geofence_enabled, created_at, updated_at) values
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'Load Test Outlet', 12.9715987, 77.5945627, 500, false, now(), now())
on conflict (id) do nothing;

insert into outlet_memberships (id, outlet_id, user_id, role, status, display_name, invited_by_user_id, removed_at, removed_by_user_id, created_at, updated_at) values
    ('c1111111-1111-1111-1111-111111111111', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '11111111-1111-1111-1111-111111111111', 'OWNER',    'ACCEPTED', 'Load Owner',    null, null, null, now(), now()),
    ('c2222222-2222-2222-2222-222222222222', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '22222222-2222-2222-2222-222222222222', 'EMPLOYEE', 'ACCEPTED', 'Load Employee 1', null, null, null, now(), now()),
    ('c3333333-3333-3333-3333-333333333333', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '33333333-3333-3333-3333-333333333333', 'EMPLOYEE', 'ACCEPTED', 'Load Employee 2', null, null, null, now(), now()),
    ('c4444444-4444-4444-4444-444444444444', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '44444444-4444-4444-4444-444444444444', 'EMPLOYEE', 'ACCEPTED', 'Load Employee 3', null, null, null, now(), now()),
    ('c5555555-5555-5555-5555-555555555555', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '55555555-5555-5555-5555-555555555555', 'EMPLOYEE', 'ACCEPTED', 'Load Employee 4', null, null, null, now(), now())
on conflict (outlet_id, user_id) do nothing;

-- A few weeks of paired clock-in/clock-out entries per employee so list
-- endpoints and salary reports query a non-empty table.
insert into attendance_entries (id, user_id, outlet_id, type, entry_time, latitude, longitude, created_by, updated_by, created_at, updated_at)
select
    gen_random_uuid(),
    u.user_id::uuid,
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
    t.type,
    now() - make_interval(days => d.days, hours => case when t.type = 'CLOCK_IN' then 3 else 8 end),
    12.9715987,
    77.5945627,
    u.user_id::uuid,
    u.user_id::uuid,
    now(),
    now()
from (values
    ('22222222-2222-2222-2222-222222222222'),
    ('33333333-3333-3333-3333-333333333333'),
    ('44444444-4444-4444-4444-444444444444'),
    ('55555555-5555-5555-5555-555555555555')
) as u(user_id)
cross join (values ('CLOCK_IN'), ('CLOCK_OUT')) as t(type)
cross join generate_series(0, 19) as d(days);
