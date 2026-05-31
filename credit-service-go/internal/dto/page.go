package dto

type PageResponse[T any] struct {
	Content       []T `json:"content"`
	Page          int `json:"page"`
	Size          int `json:"size"`
	TotalElements int `json:"totalElements"`
}
