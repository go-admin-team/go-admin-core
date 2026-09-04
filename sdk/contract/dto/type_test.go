package dto

import (
	"github.com/gin-gonic/gin"

	"github.com/go-admin-team/go-admin-core/v2/sdk/contract/models"
)

// probeIndex and probeControl are compile-time checks that Index/Control are
// still satisfiable by the same method shapes app code has always written.
type probeIndex struct{ Pagination }

func (p *probeIndex) Generate() Index             { return p }
func (p *probeIndex) Bind(ctx *gin.Context) error { return nil }
func (p *probeIndex) GetNeedSearch() interface{}  { return p }

var _ Index = &probeIndex{}

type probeControl struct{ ObjectById }

func (p *probeControl) Generate() Control                       { return p }
func (p *probeControl) GenerateM() (models.ActiveRecord, error) { return nil, nil }

var _ Control = &probeControl{}
