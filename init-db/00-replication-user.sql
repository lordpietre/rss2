-- Create replication user for streaming replication
-- This user will be used by the replica to connect to the primary

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'replicator') THEN
        CREATE ROLE replicator WITH REPLICATION LOGIN PASSWORD 'replica_password';
    END IF;
END
$$;

-- Grant necessary permissions
GRANT CONNECT ON DATABASE rss TO replicator;
