"""
Feed Discovery Utility
Provides functions to automatically discover RSS/Atom feeds from URLs.
"""

import feedfinder2
import feedparser
import requests
from typing import List, Dict, Optional
import logging

logger = logging.getLogger(__name__)


def discover_feeds(url: str, timeout: int = 15) -> List[Dict[str, str]]:
    """
    Discover RSS/Atom feeds from a given URL, including link text context.
    
    Args:
        url: The URL to search for feeds
        timeout: Request timeout in seconds
        
    Returns:
        List of dictionaries with feed information:
        [
            {
                'url': 'feed_url',
                'title': 'Feed Title',
                'context_label': 'Text on the link that pointed here',
                'type': 'rss' or 'atom',
                'valid': True/False
            }
        ]
    """
    discovered_feeds = []
    feed_context_map = {}
    
    try:
        # 1. Fetch content yourself to parse context (link names)
        logger.info(f"Fetching content from: {url}")
        headers = {'User-Agent': 'Mozilla/5.0 (compatible; RSS2-Feed-Discovery/1.0; +http://localhost)'}
        response = requests.get(url, timeout=timeout, headers=headers)
        
        if response.status_code == 200:
            from bs4 import BeautifulSoup
            from urllib.parse import urljoin
            
            soup = BeautifulSoup(response.content, 'html.parser')
            
            # Find <link> tags in head
            for link in soup.find_all('link', rel='alternate'):
                if link.get('type') in ['application/rss+xml', 'application/atom+xml']:
                    href = link.get('href')
                    title = link.get('title')
                    if href:
                        abs_url = urljoin(url, href)
                        if title:
                            feed_context_map[abs_url] = title
                            
            # Find <a> tags that might be feeds
            for a in soup.find_all('a', href=True):
                href = a['href']
                # Simple heuristic for potential RSS links
                if any(x in href.lower() for x in ['.rss', '.xml', 'feed', 'rss']):
                    abs_url = urljoin(url, href)
                    text = a.get_text(strip=True)
                    title = a.get('title')
                    
                    # Prefer text, then title
                    context = text if text else title
                    if context:
                        # Don't overwrite if we already have a nice title from <link> 
                        # actually, <a> text is often more descriptive on index pages ("Politics", "Sports")
                        # while <link> title is usually "Site Name RSS"
                        # Let's keep the shortest/cleanest or just overwrite. 
                        # For now, let's keep the existing logic or overwrite if not present.
                        if abs_url not in feed_context_map:
                            feed_context_map[abs_url] = context

        # 2. Use feedfinder2 for robust discovery
        logger.info(f"Discovering feeds from: {url}")
        feed_urls = feedfinder2.find_feeds(url)
        
        if not feed_urls:
            # Fallback: maybe feedfinder missed some that we found manually?
            # feedfinder matches strict rules. Let's add ours if valid URLs.
            for mapped_url in feed_context_map.keys():
                if mapped_url not in feed_urls:
                    feed_urls.append(mapped_url)
        
        if not feed_urls:
            logger.warning(f"No feeds found for URL: {url}")
            return []
        
        logger.info(f"Found {len(feed_urls)} potential feeds")
        
        # 3. Validate and merge context
        for feed_url in feed_urls:
            feed_info = validate_feed(feed_url, timeout=timeout)
            if feed_info:
                # Add the discovered context label
                # fuzzy match if exact match fails?
                context = feed_context_map.get(feed_url)
                if not context:
                    # Try trailing slash variations
                    context = feed_context_map.get(feed_url.rstrip('/'))
                
                feed_info['context_label'] = context or ""
                discovered_feeds.append(feed_info)
        
        # Sort by validity (valid feeds first)
        discovered_feeds.sort(key=lambda x: x['valid'], reverse=True)
        
        return discovered_feeds
        
    except Exception as e:
        logger.error(f"Error discovering feeds from {url}: {e}")
        return []


def validate_feed(feed_url: str, timeout: int = 10) -> Optional[Dict[str, str]]:
    """
    Validate and extract information from a feed URL.
    
    Args:
        feed_url: The feed URL to validate
        timeout: Request timeout in seconds
        
    Returns:
        Dictionary with feed information if valid, None otherwise
    """
    try:
        # Try to parse the feed
        response = requests.get(
            feed_url,
            timeout=timeout,
            headers={'User-Agent': 'RSS2-Feed-Discovery/1.0'}
        )
        
        if response.status_code != 200:
            logger.warning(f"Feed returned status {response.status_code}: {feed_url}")
            return {
                'url': feed_url,
                'title': 'Unknown Feed',
                'type': 'unknown',
                'valid': False,
                'error': f'HTTP {response.status_code}'
            }
        
        # Parse the feed
        feed = feedparser.parse(response.content)
        
        if feed.bozo and not feed.entries:
            # Feed has errors and no entries
            logger.warning(f"Invalid feed (no entries): {feed_url}")
            return {
                'url': feed_url,
                'title': 'Invalid Feed',
                'type': 'unknown',
                'valid': False,
                'error': 'No entries found'
            }
        
        # Extract feed information
        feed_type = feed.get('version', 'unknown')
        if feed_type.startswith('rss'):
            feed_format = 'rss'
        elif feed_type.startswith('atom'):
            feed_format = 'atom'
        else:
            feed_format = 'unknown'
        
        # Get feed title
        title = feed.feed.get('title', 'Untitled Feed')
        
        # Get feed description
        description = feed.feed.get('description', '') or feed.feed.get('subtitle', '')
        
        return {
            'url': feed_url,
            'title': title,
            'description': description,
            'type': feed_format,
            'version': feed_type,
            'valid': True,
            'entry_count': len(feed.entries)
        }
        
    except requests.Timeout:
        logger.error(f"Timeout validating feed: {feed_url}")
        return {
            'url': feed_url,
            'title': 'Timeout',
            'type': 'unknown',
            'valid': False,
            'error': 'Request timeout'
        }
    except Exception as e:
        logger.error(f"Error validating feed {feed_url}: {e}")
        return {
            'url': feed_url,
            'title': 'Error',
            'type': 'unknown',
            'valid': False,
            'error': str(e)
        }


def get_feed_metadata(feed_url: str, timeout: int = 10) -> Optional[Dict[str, any]]:
    """
    Get detailed metadata from a feed URL.
    
    Args:
        feed_url: The feed URL
        timeout: Request timeout in seconds
        
    Returns:
        Dictionary with detailed feed metadata
    """
    try:
        response = requests.get(
            feed_url,
            timeout=timeout,
            headers={'User-Agent': 'RSS2-Feed-Discovery/1.0'}
        )
        
        if response.status_code != 200:
            return None
        
        feed = feedparser.parse(response.content)
        
        if feed.bozo and not feed.entries:
            return None
        
        # Extract comprehensive metadata
        metadata = {
            'title': feed.feed.get('title', ''),
            'description': feed.feed.get('description', '') or feed.feed.get('subtitle', ''),
            'link': feed.feed.get('link', ''),
            'language': feed.feed.get('language', ''),
            'updated': feed.feed.get('updated', ''),
            'image_url': '',
            'entry_count': len(feed.entries),
            'entries': []
        }
        
        # Extract image if available
        if hasattr(feed.feed, 'image'):
            metadata['image_url'] = feed.feed.image.get('href', '')
        
        # Get first 5 entries as preview
        for entry in feed.entries[:5]:
            metadata['entries'].append({
                'title': entry.get('title', ''),
                'link': entry.get('link', ''),
                'published': entry.get('published', '')
            })
        
        return metadata
        
    except Exception as e:
        logger.error(f"Error getting feed metadata for {feed_url}: {e}")
        return None
