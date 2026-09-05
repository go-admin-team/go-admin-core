package dto

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OrderDest is a GORM scope that orders by column sort, descending when desc
// is true and ascending otherwise.
func OrderDest(sort string, desc bool) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Order(clause.OrderByColumn{Column: clause.Column{Name: sort}, Desc: desc})
	}
}
