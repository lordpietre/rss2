"""
Feed Analysis and Categorization Utilities
Provides functions to automatically detect language, suggest country and category
"""

import re
from typing import Dict, Optional, Tuple
import logging
from urllib.parse import urlparse
import requests

logger = logging.getLogger(__name__)

# Language to country mapping (primary countries for each language)
LANGUAGE_COUNTRY_MAP = {
    'es': 'España',
    'en': 'Reino Unido',
    'fr': 'Francia',
    'de': 'Alemania',
    'it': 'Italia',
    'pt': 'Portugal',
    'nl': 'Países Bajos',
    'pl': 'Polonia',
    'ru': 'Rusia',
    'zh': 'China',
    'ja': 'Japón',
    'ko': 'Corea del Sur',
    'ar': 'Arabia Saudita',
    'tr': 'Turquía',
    'ca': 'España',  # Catalan
    'eu': 'España',  # Basque
    'gl': 'España',  # Galician
}

# Domain to country mapping
DOMAIN_COUNTRY_MAP = {
    '.es': 'España',
    '.uk': 'Reino Unido',
    '.co.uk': 'Reino Unido',
    '.fr': 'Francia',
    '.de': 'Alemania',
    '.it': 'Italia',
    '.pt': 'Portugal',
    '.br': 'Brasil',
    '.mx': 'México',
    '.ar': 'Argentina',
    '.cl': 'Chile',
    '.co': 'Colombia',
    '.pe': 'Perú',
    '.ve': 'Venezuela',
    '.us': 'Estados Unidos',
    '.ca': 'Canadá',
    '.au': 'Australia',
    '.nz': 'Nueva Zelanda',
    '.in': 'India',
    '.cn': 'China',
    '.jp': 'Japón',
    '.kr': 'Corea del Sur',
    '.ru': 'Rusia',
    '.nl': 'Países Bajos',
    '.be': 'Bélgica',
    '.ch': 'Suiza',
    '.at': 'Austria',
    '.se': 'Suecia',
    '.no': 'Noruega',
    '.dk': 'Dinamarca',
    '.fi': 'Finlandia',
    '.pl': 'Polonia',
    '.gr': 'Grecia',
    '.tr': 'Turquía',
}

# Category keywords (Spanish)
CATEGORY_KEYWORDS = {
    'Política': [
        'politica', 'gobierno', 'elecciones', 'parlamento', 'congreso',
        'ministerio', 'partido', 'votacion', 'democracia', 'legislativo'
    ],
    'Economía': [
        'economia', 'finanzas', 'bolsa', 'mercado', 'empresa', 'negocio',
        'banco', 'dinero', 'comercio', 'industria', 'pib', 'inflacion'
    ],
    'Tecnología': [
        'tecnologia', 'tech', 'digital', 'software', 'hardware', 'internet',
        'startup', 'innovacion', 'ciencia', 'ai', 'inteligencia artificial'
    ],
    'Deportes': [
        'deportes', 'futbol', 'basket', 'tenis', 'olimpicos', 'liga',
        'champions', 'competicion', 'atleta', 'deporte'
    ],
    'Cultura': [
        'cultura', 'arte', 'museo', 'exposicion', 'literatura', 'libros',
        'teatro', 'musica', 'cine', 'film', 'festival'
    ],
    'Sociedad': [
        'sociedad', 'social', 'comunidad', 'gente', 'vida', 'familia',
        'educacion', 'salud', 'sanidad', 'medicina'
    ],
    'Internacional': [
        'internacional', 'mundo', 'global', 'extranjero', 'exterior',
        'foreign', 'world', 'internacional'
    ],
    'Nacional': [
        'nacional', 'espana', 'pais', 'nacional', 'domestic', 'local'
    ],
}


def detect_country_from_url(url: str) -> Optional[str]:
    """
    Detect country from URL domain
    Returns country name or None
    """
    try:
        parsed = urlparse(url)
        domain = parsed.netloc.lower()
        
        # Check domain extensions
        for tld, country in DOMAIN_COUNTRY_MAP.items():
            if domain.endswith(tld):
                return country
        
        # Check for country names in domain
        domain_parts = domain.split('.')
        for part in domain_parts:
            for tld, country in DOMAIN_COUNTRY_MAP.items():
                if tld.strip('.') == part:
                    return country
        
        return None
        
    except Exception as e:
        logger.error(f"Error detecting country from URL {url}: {e}")
        return None


def detect_country_from_language(language: str) -> Optional[str]:
    """
    Get primary country for a language code
    """
    if not language:
        return None
    
    # Extract first 2 characters
    lang_code = language[:2].lower()
    return LANGUAGE_COUNTRY_MAP.get(lang_code)


def suggest_category_from_text(text: str) -> Tuple[Optional[str], float]:
    """
    Suggest category based on text analysis
    Returns (category_name, confidence_score)
    """
    if not text:
        return None, 0.0
    
    text_lower = text.lower()
    scores = {}
    
    # Count keyword matches for each category
    for category, keywords in CATEGORY_KEYWORDS.items():
        score = 0
        for keyword in keywords:
            # Count occurrences
            count = len(re.findall(r'\b' + re.escape(keyword) + r'\b', text_lower))
            score += count
        
        if score > 0:
            scores[category] = score
    
    if not scores:
        return None, 0.0
    
    # Get category with highest score
    best_category = max(scores.items(), key=lambda x: x[1])
    total_keywords = sum(scores.values())
    confidence = best_category[1] / total_keywords if total_keywords > 0 else 0.0
    
    return best_category[0], confidence


def analyze_feed(feed_metadata: Dict) -> Dict:
    """
    Comprehensive feed analysis to detect country and suggest category
    
    Args:
        feed_metadata: Dictionary with feed metadata from get_feed_metadata()
        
    Returns:
        Dictionary with analysis results:
        {
            'detected_country': 'España',
            'country_source': 'domain' or 'language',
            'suggested_category': 'Política',
            'category_confidence': 0.75,
            'language': 'es',
            'analysis_notes': 'Detected from .es domain'
        }
    """
    analysis = {
        'detected_country': None,
        'country_source': None,
        'suggested_category': None,
        'category_confidence': 0.0,
        'language': None,
        'analysis_notes': []
    }
    
    # Extract data from metadata
    feed_url = feed_metadata.get('url', '')
    feed_title = feed_metadata.get('title', '')
    feed_description = feed_metadata.get('description', '')
    feed_language = feed_metadata.get('language', '')
    
    # Detect language
    if feed_language:
        analysis['language'] = feed_language[:2].lower()
        analysis['analysis_notes'].append(f"Language: {feed_language}")
    
    # Detect country from domain
    country_from_domain = detect_country_from_url(feed_url)
    if country_from_domain:
        analysis['detected_country'] = country_from_domain
        analysis['country_source'] = 'domain'
        analysis['analysis_notes'].append(f"Country from domain: {country_from_domain}")
    
    # If no country from domain, try language
    if not analysis['detected_country'] and analysis['language']:
        country_from_lang = detect_country_from_language(analysis['language'])
        if country_from_lang:
            analysis['detected_country'] = country_from_lang
            analysis['country_source'] = 'language'
            analysis['analysis_notes'].append(f"Country from language: {country_from_lang}")
    
    # Suggest category from title and description
    combined_text = f"{feed_title} {feed_description}"
    category, confidence = suggest_category_from_text(combined_text)
    
    if category:
        analysis['suggested_category'] = category
        analysis['category_confidence'] = confidence
        analysis['analysis_notes'].append(
            f"Suggested category: {category} (confidence: {confidence:.2%})"
        )
    
    # Join notes
    analysis['analysis_notes'] = ' | '.join(analysis['analysis_notes'])
    
    return analysis


def get_country_id_by_name(conn, country_name: str) -> Optional[int]:
    """Get country ID from database by name"""
    if not country_name:
        return None
    
    try:
        with conn.cursor() as cur:
            cur.execute(
                "SELECT id FROM paises WHERE LOWER(nombre) = LOWER(%s)",
                (country_name,)
            )
            result = cur.fetchone()
            return result[0] if result else None
    except Exception as e:
        logger.error(f"Error getting country ID for {country_name}: {e}")
        return None


def get_category_id_by_name(conn, category_name: str) -> Optional[int]:
    """Get category ID from database by name"""
    if not category_name:
        return None
    
    try:
        with conn.cursor() as cur:
            cur.execute(
                "SELECT id FROM categorias WHERE LOWER(nombre) = LOWER(%s)",
                (category_name,)
            )
            result = cur.fetchone()
            return result[0] if result else None
    except Exception as e:
        logger.error(f"Error getting category ID for {category_name}: {e}")
        return None
