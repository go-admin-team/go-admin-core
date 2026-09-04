package dto

import (
	"gorm.io/gorm"

	"github.com/go-admin-team/go-admin-core/v2/tools/search"
)

// GeneralDelDto binds either a single :id or a JSON body of ids for a batch
// delete, matching what ObjectById does for the request shapes that predate
// it.
//
// Deprecated: prefer ObjectById (single-or-batch) or ObjectDeleteReq
// (batch only). GeneralDelDto binds through both a `uri:"id"` and a
// `json:"id"` tag with `validate:"required"`, and relies on the caller's own
// binder plus that tag to reject a missing id; the newer types bind through
// `uri` alone and implement their own Bind method. GeneralDelDto is not
// being removed - the host has callers depending on this exact shape - but
// new code should not add to them.
type GeneralDelDto struct {
	Id  int   `uri:"id" json:"id" validate:"required"`
	Ids []int `json:"ids"`
}

func (g GeneralDelDto) GetIds() []int {
	ids := make([]int, 0)
	// Id used to be appended again inside an else branch below: passing only
	// Id produced [5 5], and the same row got deleted twice.
	if g.Id > 0 {
		ids = append(ids, g.Id)
	}
	for _, id := range g.Ids {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		// The convention for "nothing selected": delete everything.
		ids = append(ids, 0)
	}
	return ids
}

// GeneralGetDto binds a single :id for a detail lookup.
//
// Deprecated: prefer ObjectGetReq, which binds the same :id through `uri`
// alone and implements its own Bind method instead of relying on the
// caller's own binder plus a `validate:"required"` tag.
type GeneralGetDto struct {
	Id int `uri:"id" json:"id" validate:"required"`
}

// MakeCondition turns q's `search` struct tags into a GORM scope.
//
// The dialect it resolves tags against comes from db.Dialector.Name() inside
// the returned closure, not from a package-level variable: the closure
// already holds the *gorm.DB for this call, and db.Dialector.Name() reports
// exactly the dialect that db is bound to ("mysql", "postgres", "sqlite"),
// which is correct even when a multi-tenant host has more than one database
// open with different drivers. A host-level driver variable can only ever
// hold one of them.
func MakeCondition(q interface{}) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		condition := &search.GormCondition{
			GormPublic: search.GormPublic{},
			Join:       make([]*search.GormJoin, 0),
		}
		search.ResolveSearchQuery(db.Dialector.Name(), q, condition)
		for _, join := range condition.Join {
			if join == nil {
				continue
			}
			db = db.Joins(join.JoinOn)
			for k, v := range join.Where {
				db = db.Where(k, v...)
			}
			for k, v := range join.Or {
				db = db.Or(k, v...)
			}
			for _, o := range join.Order {
				db = db.Order(o)
			}
		}
		for k, v := range condition.Where {
			db = db.Where(k, v...)
		}
		for k, v := range condition.Or {
			db = db.Or(k, v...)
		}
		for _, o := range condition.Order {
			db = db.Order(o)
		}
		return db
	}
}

// Paginate is a GORM scope applying offset/limit for the given page.
func Paginate(pageSize, pageIndex int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		offset := (pageIndex - 1) * pageSize
		if offset < 0 {
			offset = 0
		}
		return db.Offset(offset).Limit(pageSize)
	}
}
