BEGIN;

CREATE TABLE tasks (
    uuid UUID NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    completed BOOLEAN NOT NULL,

    PRIMARY KEY (uuid),
    CONSTRAINT title_not_empty CHECK (title <> '')
);

COMMIT;
