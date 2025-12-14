package mvc

import (
	"gorm.io/gorm"
)

type Page struct {
	PageNum int         `json:"pageNum"`
	Size    int         `json:"size"`
	Sort    interface{} `json:"sort"`
}

func Paginate(page *Page) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		pageNum := page.PageNum
		size := page.Size

		if pageNum == 0 {
			pageNum = 1
		}

		if size <= 0 {
			size = 10
		}

		offset := (pageNum - 1) * size
		return db.Offset(offset).Limit(size)
	}
}

func PaginateEs(page Page) (int, int) {
	pageNum := page.PageNum
	size := page.Size

	if pageNum == 0 {
		pageNum = 1
	}

	offset := (pageNum - 1) * size

	return offset, size
}

func (page *Page) Paginate() (int, int) {
	pageNum := page.PageNum
	size := page.Size

	if pageNum == 0 {
		pageNum = 1
	}

	offset := (pageNum - 1) * size

	return offset, size
}
