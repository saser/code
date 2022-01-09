BEGIN;

CREATE TABLE tasks (
    id INT NOT NULL,
    title TEXT NOT NULL,

    PRIMARY KEY (id),
    CONSTRAINT title_not_empty CHECK (title <> '')
);

COMMIT;
