-- Migration: Reset all feeds to inactive with 0 attempts
-- Run this to set all feeds as inactive (activo = false) and reset fallbacks to 0

UPDATE feeds SET activo = false, fallos = 0;
