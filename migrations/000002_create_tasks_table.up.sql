 CREATE TABLE IF NOT EXISTS tasks (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        title VARCHAR(255) NOT NULL,
        description TEXT,
        status VARCHAR(50) NOT NULL DEFAULT 'todo',
        priority VARCHAR(50) NOT NULL DEFAULT 'medium',
        created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
    );

    -- Index for querying all tasks belonging to a user
    CREATE INDEX IF NOT EXISTS idx_tasks_user_id ON tasks(user_id);

    -- Composite index for filtering user tasks by status (e.g. user's 'todo' tasks)
    CREATE INDEX IF NOT EXISTS idx_tasks_user_id_status ON tasks(user_id, status);