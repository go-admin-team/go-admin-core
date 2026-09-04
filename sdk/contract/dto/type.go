package dto

import (
	"github.com/gin-gonic/gin"

	"github.com/go-admin-team/go-admin-core/v2/sdk/contract/models"
)

// Index is what a list-request DTO implements: bind itself from the
// request, report its own pagination, and hand back the struct search tags
// are resolved against.
type Index interface {
	Generate() Index
	Bind(ctx *gin.Context) error
	GetPageIndex() int
	GetPageSize() int
	GetNeedSearch() interface{}
}

// Control is what a create/update-request DTO implements: bind itself, and
// produce the model row it maps onto.
type Control interface {
	Generate() Control
	Bind(ctx *gin.Context) error
	GenerateM() (models.ActiveRecord, error)
	GetId() interface{}
}
