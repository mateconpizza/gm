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
