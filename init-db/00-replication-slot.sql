-- Enable replication access from the replica container
-- This file is sourced after the database is initialized

-- Add pg_hba.conf entry for replication
-- Note: This needs to be done via ALTER SYSTEM or pg_hba.conf file

DO $$
BEGIN
    -- Create replication slot to prevent WAL removal
    IF NOT EXISTS (SELECT 1 FROM pg_replication_slots WHERE slot_name = 'replica_slot') THEN
        PERFORM pg_create_physical_replication_slot('replica_slot');
    END IF;
END
$$;
