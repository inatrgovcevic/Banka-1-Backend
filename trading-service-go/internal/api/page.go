package api

// Spring Data Page<> wrapper. The Java order-service returns paginated endpoints
// (e.g. GET /actuaries/agents) as a Spring PageImpl, which Jackson serializes
// with the fields below. trading-service application.properties does not
// customize spring.data.web serialization, so this is the default shape. Field
// order is irrelevant — the paritycheck normalizer sorts map keys before diffing.
//
// NOTE: the exact PageImpl JSON is a known migration gotcha; confirm against the
// live Java service during the parity sweep and adjust if the running Spring
// version differs.

// Sort mirrors the nested "sort" object. With no sort requested it is
// empty/unsorted.
type Sort struct {
	Empty    bool `json:"empty"`
	Sorted   bool `json:"sorted"`
	Unsorted bool `json:"unsorted"`
}

// Pageable mirrors the nested "pageable" object of a paged response.
type Pageable struct {
	PageNumber int   `json:"pageNumber"`
	PageSize   int   `json:"pageSize"`
	Sort       Sort  `json:"sort"`
	Offset     int64 `json:"offset"`
	Paged      bool  `json:"paged"`
	Unpaged    bool  `json:"unpaged"`
}

// Page is the generic Spring PageImpl envelope.
type Page[T any] struct {
	Content          []T      `json:"content"`
	Pageable         Pageable `json:"pageable"`
	Last             bool     `json:"last"`
	TotalElements    int64    `json:"totalElements"`
	TotalPages       int      `json:"totalPages"`
	First            bool     `json:"first"`
	Size             int      `json:"size"`
	Number           int      `json:"number"`
	Sort             Sort     `json:"sort"`
	NumberOfElements int      `json:"numberOfElements"`
	Empty            bool     `json:"empty"`
}

func unsortedSort() Sort {
	// Spring renders an absent sort as empty=true, sorted=false, unsorted=true.
	return Sort{Empty: true, Sorted: false, Unsorted: true}
}

// NewPage builds a Spring-style page. content is the already-sliced current page;
// total is the element count across all pages. Mirrors PageImpl(slice, PageRequest.of(page,size), total).
func NewPage[T any](content []T, page, size int, total int64) Page[T] {
	if content == nil {
		content = []T{}
	}
	totalPages := 0
	if size > 0 {
		totalPages = int((total + int64(size) - 1) / int64(size))
	}
	return Page[T]{
		Content: content,
		Pageable: Pageable{
			PageNumber: page,
			PageSize:   size,
			Sort:       unsortedSort(),
			Offset:     int64(page) * int64(size),
			Paged:      true,
			Unpaged:    false,
		},
		Last:             page+1 >= totalPages,
		TotalElements:    total,
		TotalPages:       totalPages,
		First:            page == 0,
		Size:             size,
		Number:           page,
		Sort:             unsortedSort(),
		NumberOfElements: len(content),
		Empty:            len(content) == 0,
	}
}
