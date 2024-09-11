package repo

// TODO:
// [ ] add `favorite`
// [ ] add `last used`

var tableMainSchema = `
CREATE TABLE IF NOT EXISTS %s (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    url         TEXT    NOT NULL UNIQUE,
    title       TEXT    DEFAULT "",
    tags        TEXT    DEFAULT ",",
    desc        TEXT    DEFAULT "",
    created_at  TIMESTAMP
);`

// deleted Schema
// lastid - maybeeee when restoring deleted record, insert in the original id,
// bad idea? KISS?
/* var tableMainSchema = `
CREATE TABLE IF NOT EXISTS %s (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    url         TEXT    NOT NULL UNIQUE,
    title       TEXT    DEFAULT "",
    tags        TEXT    DEFAULT "",
    desc        TEXT    DEFAULT "",
    favorite    INTEGER DEFAULT 0,
    last_used   TIMESTAMP,
    lastid      INTEGER,
    created_at  TIMESTAMP
);` */
