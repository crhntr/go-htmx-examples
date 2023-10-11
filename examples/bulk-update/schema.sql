CREATE TABLE colors (
    id     INTEGER      PRIMARY KEY AUTOINCREMENT,
    name   VARCHAR(100) NOT NULL,
    active BOOLEAN      NOT NULL
);

INSERT INTO colors (name, active)
       VALUES ('Blue',   TRUE),
              ('Green',  FALSE),
              ('Yellow', FALSE),
              ('Pink',   TRUE),
              ('Red',    FALSE);