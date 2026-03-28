package models

import "time"

type EntityAlias struct {
	ID            int       `json:"id"`
	CanonicalName string    `json:"canonical_name"`
	Alias         string    `json:"alias"`
	Tipo          string    `json:"tipo"`
	CreatedAt     time.Time `json:"created_at"`
}

type EntityAliasRequest struct {
	CanonicalName string   `json:"canonical_name" binding:"required"`
	Aliases       []string `json:"aliases" binding:"required,min=1"`
	Tipo          string   `json:"tipo" binding:"required,oneof=persona organizacion lugar tema"`
}
