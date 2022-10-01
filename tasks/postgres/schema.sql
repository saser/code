BEGIN;

-- The main table of tasks.
CREATE TABLE tasks (
    id BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL,
    parent BIGINT, -- A null parent means it has no parent.
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    complete_time TIMESTAMP WITH TIME ZONE, -- If non-null, then task is completed.
    create_time TIMESTAMP WITH TIME ZONE NOT NULL,
    update_time TIMESTAMP WITH TIME ZONE, -- If non-null, then task has been updated at least once.
    delete_time TIMESTAMP WITH TIME ZONE, -- If non-null, then task is considered deleted.
    expire_time TIMESTAMP WITH TIME ZONE,

    PRIMARY KEY (id),
    CONSTRAINT parent_references_id FOREIGN KEY (parent) REFERENCES tasks (id),
    CONSTRAINT title_not_empty CHECK (title <> ''),
    CONSTRAINT create_time_not_after_complete_time CHECK (complete_time IS NULL OR create_time <= complete_time),
    CONSTRAINT create_time_not_after_delete_time CHECK (delete_time IS NULL OR create_time <= delete_time),
    CONSTRAINT create_time_not_after_update_time CHECK (update_time IS NULL OR create_time <= update_time),
    CONSTRAINT delete_time_iff_expire_time CHECK ((delete_time IS NULL) = (expire_time IS NULL)),
    CONSTRAINT delete_time_not_after_expire_time CHECK (delete_time IS NULL OR delete_time <= expire_time)
);

-- The main table of projects.
CREATE TABLE projects (
    id BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    archive_time TIMESTAMP WITH TIME ZONE, -- If non-null, then project is archived.
    create_time TIMESTAMP WITH TIME ZONE NOT NULL,
    update_time TIMESTAMP WITH TIME ZONE, -- If non-null, then project has been updated at least once.
    delete_time TIMESTAMP WITH TIME ZONE, -- If non-null, then project is considered deleted.
    expire_time TIMESTAMP WITH TIME ZONE,

    PRIMARY KEY (id),
    CONSTRAINT title_not_empty CHECK (title <> ''),
    CONSTRAINT create_time_not_after_archive_time CHECK (archive_time IS NULL OR create_time <= archive_time),
    CONSTRAINT create_time_not_after_delete_time CHECK (delete_time IS NULL OR create_time <= delete_time),
    CONSTRAINT create_time_not_after_update_time CHECK (update_time IS NULL OR create_time <= update_time),
    CONSTRAINT delete_time_iff_expire_time CHECK ((delete_time IS NULL) = (expire_time IS NULL)),
    CONSTRAINT delete_time_not_after_expire_time CHECK (delete_time IS NULL OR delete_time <= expire_time)
);

-- The main table of labels.
CREATE TABLE labels (
    id BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL,
    label TEXT NOT NULL,
    create_time TIMESTAMP WITH TIME ZONE NOT NULL,
    update_time TIMESTAMP WITH TIME ZONE, -- If non-null, then project has been updated at least once.

    PRIMARY KEY (id),
    CONSTRAINT label_not_empty CHECK (label <> ''),
    CONSTRAINT label_is_unique UNIQUE (label),
    CONSTRAINT label_contains_valid_characters CHECK (label ~* '^[a-zA-Z0-9\-\_\:\@]+$'),
    CONSTRAINT create_time_not_after_update_time CHECK (update_time IS NULL OR create_time <= update_time)
);

-- A table of page tokens. The rows in this table define the set of acceptable
-- values for ListTasksRequest.next_page_token.
CREATE TABLE task_page_tokens (
    token UUID NOT NULL,
    minimum_id BIGINT NOT NULL REFERENCES tasks (id),
    show_deleted BOOLEAN NOT NULL,

    PRIMARY KEY (token)
);

-- A table of page tokens. The rows in this table define the set of acceptable
-- values for ListProjectsRequest.next_page_token.
CREATE TABLE project_page_tokens (
    token UUID NOT NULL,
    minimum_id BIGINT NOT NULL REFERENCES projects (id),
    show_deleted BOOLEAN NOT NULL,

    PRIMARY KEY (token)
);

-- In many cases we want to run queries over existing tasks only. This view is a
-- convenient way to do that, compared to having to remember to filter out
-- deleted tasks yourself.
CREATE VIEW existing_tasks AS (
    SELECT *
    FROM tasks
    WHERE delete_time IS NULL
);

-- In many cases we want to run queries over existing projects only. This view
-- is a convenient way to do that, compared to having to remember to filter out
-- deleted projects yourself.
CREATE VIEW existing_projects AS (
    SELECT *
    FROM projects
    WHERE delete_time IS NULL
);

-- This view contains a mapping from parent task ID to child task ID. Tasks
-- without any children are not included as parents in this view.
CREATE VIEW tasks_children AS (
    WITH RECURSIVE children(parent, child) AS (
            SELECT
                NULL::BIGINT AS parent,
                id AS child
            FROM
                tasks AS t
            WHERE
                t.parent IS NULL
        UNION ALL
            SELECT
                t.parent AS parent,
                t.id AS child
            FROM
                tasks AS t,
                children AS d
            WHERE
                t.parent = d.child
    )
    SELECT
        parent,
        child
    FROM
        children
    WHERE
        parent IS NOT NULL
);

-- This view is like tasks_children except only tasks that haven't been deleted
-- are considered.
CREATE VIEW existing_tasks_children AS (
    WITH RECURSIVE children(parent, child) AS (
            SELECT
                NULL::BIGINT AS parent,
                id AS child
            FROM
                existing_tasks AS t
            WHERE
                t.parent IS NULL
        UNION ALL
            SELECT
                t.parent AS parent,
                t.id AS child
            FROM
                existing_tasks AS t,
                children AS d
            WHERE
                t.parent = d.child
    )
    SELECT
        parent,
        child
    FROM
        children
    WHERE
        parent IS NOT NULL
);

-- This view contains for each task all descendants -- direct or transitive --
-- for that task. Tasks with no descendants (i.e., leaf tasks) are not included
-- in this view.
CREATE VIEW tasks_descendants AS (
    WITH RECURSIVE descendants(task, descendant) AS (
            SELECT
                tc.parent AS task,
                tc.child AS descendant
            FROM
                tasks_children AS tc
        UNION ALL
            SELECT
                d.task AS task,
                tc.child AS descendant
            FROM
                tasks_children AS tc,
                descendants AS d
            WHERE
                tc.parent = d.descendant
    )
    SELECT
        task,
        descendant
    FROM
        descendants
    WHERE
        task IS NOT NULL
);

-- This view is like tasks_descendants except only tasks that haven't been
-- deleted are considered.
CREATE VIEW existing_tasks_descendants AS (
    WITH RECURSIVE descendants(task, descendant) AS (
            SELECT
                tc.parent AS task,
                tc.child AS descendant
            FROM
                existing_tasks_children AS tc
        UNION ALL
            SELECT
                d.task AS task,
                tc.child AS descendant
            FROM
                existing_tasks_children AS tc,
                descendants AS d
            WHERE
                tc.parent = d.descendant
    )
    SELECT
        task,
        descendant
    FROM
        descendants
    WHERE
        task IS NOT NULL
);

-- This view contains for each task all ancestors -- direct or transitive -- for
-- that task. Tasks with no ancestors (i.e., root tasks) are not included in
-- this view.
CREATE VIEW tasks_ancestors AS (
    SELECT
        descendant AS task,
        task AS ancestor
    FROM
        tasks_descendants
);

-- This view is like tasks_ancestors except only tasks that haven't been
-- deleted are considered.
CREATE VIEW existing_tasks_ancestors AS (
    SELECT
        descendant AS task,
        task AS ancestor
    FROM
        existing_tasks_descendants
);

COMMIT;
