package models

import "time"

type Order struct {
	ID                 int64
	UserID             int64
	UserEmail          string
	Type               string
	Description        string
	PlanDurationMonths int
	Status             string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
