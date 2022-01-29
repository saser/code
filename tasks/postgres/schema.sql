BEGIN;

-- The main table of tasks.
CREATE TABLE tasks (
    id BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    completed BOOLEAN NOT NULL,
    create_time TIMESTAMP WITH TIME ZONE NOT NULL,
    delete_time TIMESTAMP WITH TIME ZONE, -- If non-null, then task is considered deleted.
    expiry_time TIMESTAMP WITH TIME ZONE,

    PRIMARY KEY (id),
    CONSTRAINT title_not_empty CHECK (title <> ''),
    CONSTRAINT create_time_not_after_delete_time CHECK (delete_time IS NULL OR create_time <= delete_time),
    CONSTRAINT delete_time_iff_expiry_time CHECK ((delete_time IS NULL) = (expiry_time IS NULL)),
    CONSTRAINT delete_time_not_after_expiry_time CHECK (delete_time IS NULL OR delete_time <= expiry_time)
);

-- A table of page tokens. The rows in this table define the set of acceptable
-- values for ListTasksRequest.next_page_token.
CREATE TABLE page_tokens (
    token UUID NOT NULL,
    minimum_id BIGINT NOT NULL REFERENCES tasks (id),
    show_deleted BOOLEAN NOT NULL,

    PRIMARY KEY (token)
);

COMMIT;
