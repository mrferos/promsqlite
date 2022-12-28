create table samples
(
    name       TEXT    not null,
    dimensions TEXT    not null,
    value      INTEGER,
    timestamp  INTEGER not null
);

create index samples_name_timestamp_index
    on samples (name asc, timestamp desc);

