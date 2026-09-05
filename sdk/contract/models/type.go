package models

import "gorm.io/gorm/schema"

// ActiveRecord is what the framework's generic CRUD Actions require of a
// model. It is self-referencing (Generate returns ActiveRecord), which is
// exactly why it must stay a type alias on the go-admin side rather than a
// defined type: a defined type breaks the method set every implementer needs
// to satisfy this interface. See sdk/contract/models's package doc test for
// the counterproof.
type ActiveRecord interface {
	schema.Tabler
	SetCreateBy(createBy int)
	SetUpdateBy(updateBy int)
	Generate() ActiveRecord
	GetId() interface{}
}
