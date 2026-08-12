alter table outlets add column removed_at timestamp with time zone;
alter table outlets add column removed_by_user_id uuid;

alter table outlets add constraint fk_outlet_removed_by foreign key (removed_by_user_id) references users(id);

create index idx_outlets_removed_at on outlets(removed_at);
