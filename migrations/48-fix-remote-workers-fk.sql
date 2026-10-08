-- Migration: Fix remote_workers foreign key constraint
-- Allow deleting workers that have associated translations by setting worker_id to NULL
-- This fixes the "Failed to delete worker" error when trying to remove disabled workers

-- Drop the existing foreign key constraint if it exists
ALTER TABLE traducciones DROP CONSTRAINT IF EXISTS traducciones_worker_id_fkey;

-- Re-add the column with ON DELETE SET NULL to allow worker deletion
-- This ensures that when a worker is deleted, the worker_id in traducciones is set to NULL
-- instead of preventing the deletion
ALTER TABLE traducciones 
ADD CONSTRAINT traducciones_worker_id_fkey 
FOREIGN KEY (worker_id) REFERENCES remote_workers(id) ON DELETE SET NULL;

-- Verify the constraint was created correctly
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'traducciones_worker_id_fkey'
    ) THEN
        RAISE NOTICE 'Warning: Foreign key constraint may not have been created properly';
    ELSE
        RAISE NOTICE 'Foreign key constraint traducciones_worker_id_fkey created successfully with ON DELETE SET NULL';
    END IF;
END $$;

-- Show current constraint info
SELECT 
    tc.constraint_name,
    tc.table_name,
    kcu.column_name,
    rc.delete_rule
FROM information_schema.table_constraints AS tc
JOIN information_schema.key_column_usage AS kcu
    ON tc.constraint_name = kcu.constraint_name
JOIN information_schema.referential_constraints AS rc
    ON tc.constraint_name = rc.constraint_name
WHERE tc.table_name = 'traducciones' 
AND tc.constraint_type = 'FOREIGN KEY'
AND kcu.column_name = 'worker_id';
