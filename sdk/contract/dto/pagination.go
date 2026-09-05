package dto

// Pagination is the request-side embed a list DTO carries for page index and
// page size, with defaults applied by the getters rather than by zero
// values, so an unset field means "the default page", not "page 0".
type Pagination struct {
	PageIndex int `form:"pageIndex"`
	PageSize  int `form:"pageSize"`
}

func (m *Pagination) GetPageIndex() int {
	if m.PageIndex <= 0 {
		m.PageIndex = 1
	}
	return m.PageIndex
}

func (m *Pagination) GetPageSize() int {
	if m.PageSize <= 0 {
		m.PageSize = 10
	}
	return m.PageSize
}
