CREATE TABLE contacts (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    first_name VARCHAR(100) NOT NULL,
    last_name  VARCHAR(100) NOT NULL,
    email      VARCHAR(100) NOT NULL
);

INSERT INTO contacts (first_name, last_name, email)
       VALUES ('Mary', 'Sativa', 'mary@example.com'),
              ('Jane', 'Indica', 'jane@example.com');