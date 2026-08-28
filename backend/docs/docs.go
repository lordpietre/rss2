// Package docs provides Swagger documentation for the RSS2 API
//
// @title           RSS2 API
// @version         1.0
// @description     RSS2 News Aggregator API - A comprehensive news aggregation and analysis platform
// @termsOfService  http://swagger.io/terms/
//
// @contact.name   RSS2 Team
// @contact.url    http://github.com/rss2
// @contact.email  support@rss2.example.com
//
// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT
//
// @host      localhost:8080
// @BasePath  /api
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
//
// @tag.name auth
// @tag.description Authentication endpoints
//
// @tag.name news
// @tag.description News management endpoints
//
// @tag.name feeds
// @tag.description Feed management endpoints
//
// @tag.name search
// @tag.description Search and discovery endpoints
//
// @tag.name entities
// @tag.description Entity extraction and analysis
//
// @tag.name admin
// @tag.description Admin management endpoints
//
// @tag.name stats
// @tag.description Statistics and analytics
package docs