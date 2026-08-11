alter table outlet_memberships add column display_name varchar(255);

update outlet_memberships om
set display_name = (select u.name from users u where u.id = om.user_id)
where om.display_name is null;

alter table outlet_memberships alter column display_name set not null;