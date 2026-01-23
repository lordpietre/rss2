from flask import Flask

from config import SECRET_KEY
from utils import safe_html, format_date, country_flag

from routers.home import home_bp
from routers.feeds import feeds_bp
from routers.urls import urls_bp
from routers.noticia import noticia_bp
from routers.backup import backup_bp
from routers.config import config_bp
from routers.favoritos import favoritos_bp
from routers.search import search_bp
from routers.rss import rss_bp
from routers.stats import stats_bp
from routers.pdf import pdf_bp
from routers.notifications import notifications_bp
from routers.auth import auth_bp
from routers.account import account_bp
from routers.parrillas import parrillas_bp


def create_app() -> Flask:
    app = Flask(__name__)
    app.config["SECRET_KEY"] = SECRET_KEY

    app.jinja_env.filters["safe_html"] = safe_html
    app.jinja_env.filters["format_date"] = format_date
    app.jinja_env.filters["country_flag"] = country_flag

    app.register_blueprint(home_bp)
    app.register_blueprint(feeds_bp)
    app.register_blueprint(urls_bp)
    app.register_blueprint(noticia_bp)
    app.register_blueprint(backup_bp)
    app.register_blueprint(config_bp)
    app.register_blueprint(favoritos_bp)
    app.register_blueprint(search_bp)
    app.register_blueprint(rss_bp)
    app.register_blueprint(stats_bp)
    app.register_blueprint(pdf_bp)
    app.register_blueprint(notifications_bp)
    
    from routers.conflicts import conflicts_bp
    from routers.topics import topics_bp
    
    app.register_blueprint(conflicts_bp)
    app.register_blueprint(topics_bp)
    app.register_blueprint(auth_bp)
    app.register_blueprint(account_bp)
    app.register_blueprint(parrillas_bp)


    return app


app = create_app()

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8001, debug=True)

