BEGIN;

-- The main table of tasks.
CREATE TABLE tasks (
    id BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    completed BOOLEAN NOT NULL,
    create_time TIMESTAMP WITH TIME ZONE NOT NULL,
    delete_time TIMESTAMP WITH TIME ZONE, -- If non-null, then task is considered deleted.

    PRIMARY KEY (id),
    CONSTRAINT title_not_empty CHECK (title <> ''),
    CONSTRAINT create_time_before_delete_time CHECK (delete_time IS NULL OR create_time < delete_time)
);

-- A table of page tokens. The rows in this table define the set of acceptable
-- values for ListTasksRequest.next_page_token.
CREATE TABLE page_tokens (
    token UUID NOT NULL,
    minimum_id BIGINT NOT NULL REFERENCES tasks (id),

    PRIMARY KEY (token)
);

COMMIT;
