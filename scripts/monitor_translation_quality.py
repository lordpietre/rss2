#!/usr/bin/env python3
"""
Monitor translation quality in real-time.
Shows statistics about translation quality and detects issues.
"""
import os
import sys
import time
import psycopg2
from datetime import datetime, timedelta
from dotenv import load_dotenv

load_dotenv()

DB_CONFIG = {
    "host": os.environ.get("DB_HOST", "localhost"),
    "port": int(os.environ.get("DB_PORT", 5432)),
    "dbname": os.environ.get("DB_NAME", "rss"),
    "user": os.environ.get("DB_USER", "rss"),
    "password": os.environ.get("DB_PASS", ""),
}

def get_stats(conn, hours=24):
    """Get translation statistics for the last N hours."""
    with conn.cursor() as cur:
        # Total translations in period
        cur.execute("""
            SELECT 
                COUNT(*) as total,
                COUNT(CASE WHEN status='done' THEN 1 END) as done,
                COUNT(CASE WHEN status='pending' THEN 1 END) as pending,
                COUNT(CASE WHEN status='processing' THEN 1 END) as processing,
                COUNT(CASE WHEN status='error' THEN 1 END) as errors
            FROM traducciones
            WHERE created_at > NOW() - INTERVAL '%s hours'
        """, (hours,))
        
        stats = cur.fetchone()
        
        # Check for repetitive patterns in recent translations
        cur.execute("""
            SELECT COUNT(*) 
            FROM traducciones 
            WHERE status='done' 
              AND created_at > NOW() - INTERVAL '%s hours'
              AND (
                resumen_trad LIKE '%%la línea de la línea%%' 
                OR resumen_trad LIKE '%%de la la %%'
                OR resumen_trad LIKE '%%de Internet de Internet%%'
              )
        """, (hours,))
        
        repetitive = cur.fetchone()[0]
        
        # Get error messages
        cur.execute("""
            SELECT error, COUNT(*) as count
            FROM traducciones
            WHERE status='error' 
              AND created_at > NOW() - INTERVAL '%s hours'
            GROUP BY error
            ORDER BY count DESC
            LIMIT 5
        """, (hours,))
        
        errors = cur.fetchall()
        
        return {
            'total': stats[0],
            'done': stats[1],
            'pending': stats[2],
            'processing': stats[3],
            'errors': stats[4],
            'repetitive': repetitive,
            'error_details': errors
        }

def print_stats(stats, hours):
    """Pretty print statistics."""
    print(f"\n{'='*60}")
    print(f"📊 Translation Quality Report - Last {hours}h")
    print(f"{'='*60}")
    print(f"Total Translations: {stats['total']}")
    print(f"  ✅ Done:        {stats['done']:>6} ({stats['done']/max(stats['total'],1)*100:>5.1f}%)")
    print(f"  ⏳ Pending:     {stats['pending']:>6} ({stats['pending']/max(stats['total'],1)*100:>5.1f}%)")
    print(f"  🔄 Processing:  {stats['processing']:>6} ({stats['processing']/max(stats['total'],1)*100:>5.1f}%)")
    print(f"  ❌ Errors:      {stats['errors']:>6} ({stats['errors']/max(stats['total'],1)*100:>5.1f}%)")
    print(f"\n🔍 Quality Issues:")
    print(f"  ⚠️  Repetitive:  {stats['repetitive']:>6} ({stats['repetitive']/max(stats['done'],1)*100:>5.1f}% of done)")
    
    if stats['error_details']:
        print(f"\n📋 Top Error Messages:")
        for error, count in stats['error_details']:
            error_short = (error[:50] + '...') if error and len(error) > 50 else (error or 'Unknown')
            print(f"  • {error_short}: {count}")
    
    # Quality score
    if stats['done'] > 0:
        quality_score = (1 - stats['repetitive'] / stats['done']) * 100
        quality_emoji = "🟢" if quality_score > 95 else "🟡" if quality_score > 90 else "🔴"
        print(f"\n{quality_emoji} Quality Score: {quality_score:.1f}%")
    
    print(f"{'='*60}\n")

def main():
    import argparse
    parser = argparse.ArgumentParser(description='Monitor translation quality')
    parser.add_argument('--hours', type=int, default=24, help='Hours to look back (default: 24)')
    parser.add_argument('--watch', action='store_true', help='Continuous monitoring mode')
    parser.add_argument('--interval', type=int, default=60, help='Update interval in seconds (default: 60)')
    
    args = parser.parse_args()
    
    conn = psycopg2.connect(**DB_CONFIG)
    
    try:
        if args.watch:
            print("🔄 Starting continuous monitoring (Ctrl+C to stop)...")
            while True:
                stats = get_stats(conn, args.hours)
                print(f"\033[2J\033[H")  # Clear screen
                print(f"Last updated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
                print_stats(stats, args.hours)
                time.sleep(args.interval)
        else:
            stats = get_stats(conn, args.hours)
            print_stats(stats, args.hours)
    except KeyboardInterrupt:
        print("\n\n👋 Monitoring stopped")
    finally:
        conn.close()

if __name__ == "__main__":
    main()
