package model

import "time"

type BaseEntity struct {
	ID        int64     `json:"id"`
	Version   int64     `json:"version"`
	Deleted   bool      `json:"deleted"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
