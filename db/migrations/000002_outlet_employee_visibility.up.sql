alter table outlets
    add column show_recent_entries_to_employees boolean not null default false,
    add column show_total_time_today_to_employees boolean not null default false;