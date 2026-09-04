package models

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

// ControlBy holds who created and who last updated a row. Embed it in any
// model whose writes go through the framework's CRUD Actions or hand-written
// Services: actions.Permission's data-scope SQL joins on CreateBy, so a model
// without this embed makes every data-scope rule silently match nothing.
type ControlBy struct {
	CreateBy int `json:"createBy" gorm:"index;comment:creator"`
	UpdateBy int `json:"updateBy" gorm:"index;comment:updater"`
}

// SetCreateBy sets the id of the user who created the row.
func (e *ControlBy) SetCreateBy(createBy int) {
	e.CreateBy = createBy
}

// SetUpdateBy sets the id of the user who last updated the row.
func (e *ControlBy) SetUpdateBy(updateBy int) {
	e.UpdateBy = updateBy
}

// Model is the primary key embed shared by every framework-managed table.
type Model struct {
	Id int `json:"id" gorm:"primaryKey;autoIncrement;comment:primary key"`
}

// ModelTime holds the created/updated/deleted timestamps every framework
// table carries.
type ModelTime struct {
	CreatedAt time.Time `json:"createdAt" gorm:"comment:creation time"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"comment:last updated time"`

	// DeletedAt is milliseconds since the epoch, zero while the row is live,
	// and never null.
	//
	// A nullable marker cannot take part in a unique index. Two live rows are
	// (name, NULL) and (name, NULL), and NULL is not equal to NULL, so the
	// index permits both — it looks like a constraint and enforces nothing.
	// With zero for live rows the pair collides, while two deletions of the
	// same name differ by their timestamps and both remain.
	DeletedAt soft_delete.DeletedAt `json:"-" gorm:"softDelete:milli;index;comment:deletion time"`
}
