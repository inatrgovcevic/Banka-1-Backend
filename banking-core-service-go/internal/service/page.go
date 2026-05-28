package service

type Page[T any] struct {
	Content          []T  `json:"content"`
	TotalElements    int  `json:"totalElements"`
	TotalPages       int  `json:"totalPages"`
	Size             int  `json:"size"`
	Number           int  `json:"number"`
	NumberOfElements int  `json:"numberOfElements"`
	First            bool `json:"first"`
	Last             bool `json:"last"`
	Empty            bool `json:"empty"`
}

func NewPage[T any](content []T, page, size, total int) Page[T] {
	if size <= 0 {
		size = 10
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + size - 1) / size
	}
	return Page[T]{
		Content:          content,
		TotalElements:    total,
		TotalPages:       totalPages,
		Size:             size,
		Number:           page,
		NumberOfElements: len(content),
		First:            page == 0,
		Last:             totalPages == 0 || page >= totalPages-1,
		Empty:            len(content) == 0,
	}
}
