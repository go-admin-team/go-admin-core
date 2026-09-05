package dto

import (
	"net/http"

	vd "github.com/bytedance/go-tagexpr/v2/validator"
	"github.com/gin-gonic/gin"

	"github.com/go-admin-team/go-admin-core/v2/sdk/api"
)

// ObjectById binds either a single :id from the URI or, on DELETE, a JSON
// body carrying ids for a batch delete.
type ObjectById struct {
	Id  int   `uri:"id"`
	Ids []int `json:"ids"`
}

func (s *ObjectById) Bind(ctx *gin.Context) error {
	var err error
	log := api.GetRequestLogger(ctx)
	err = ctx.ShouldBindUri(s)
	if err != nil {
		log.Warnf("ShouldBindUri error: %s", err.Error())
		return err
	}
	if ctx.Request.Method == http.MethodDelete {
		err = ctx.ShouldBind(s)
		if err != nil {
			log.Warnf("ShouldBind error: %s", err.Error())
			return err
		}
		if len(s.Ids) > 0 {
			return nil
		}
		if s.Ids == nil {
			s.Ids = make([]int, 0)
		}
		if s.Id != 0 {
			s.Ids = append(s.Ids, s.Id)
		}
	}
	if err = vd.Validate(s); err != nil {
		log.Errorf("Validate error: %s", err.Error())
		return err
	}
	return err
}

// GetId returns the ids this request addresses: the batch from the body when
// there is one, otherwise the single id from the URI.
//
// It must not write back to the receiver. Appending to s.Ids in place made
// GetId non-idempotent - a second call returned the URI id twice - and an
// unconditional append put a 0 into the slice on any route that carries no
// :id, which is a dangerous value to hand to a caller that treats 0 as a
// sentinel.
func (s *ObjectById) GetId() interface{} {
	if len(s.Ids) == 0 {
		return s.Id
	}
	if s.Id == 0 {
		return s.Ids
	}
	ids := make([]int, 0, len(s.Ids)+1)
	ids = append(ids, s.Ids...)
	return append(ids, s.Id)
}

// ObjectGetReq binds a single :id from the URI for a detail lookup.
type ObjectGetReq struct {
	Id int `uri:"id"`
}

func (s *ObjectGetReq) Bind(ctx *gin.Context) error {
	var err error
	log := api.GetRequestLogger(ctx)
	err = ctx.ShouldBindUri(s)
	if err != nil {
		log.Warnf("ShouldBindUri error: %s", err.Error())
		return err
	}
	if err = vd.Validate(s); err != nil {
		log.Errorf("Validate error: %s", err.Error())
		return err
	}
	return err
}

func (s *ObjectGetReq) GetId() interface{} {
	return s.Id
}

// ObjectDeleteReq binds a JSON body of ids for a batch delete.
type ObjectDeleteReq struct {
	Ids []int `json:"ids"`
}

func (s *ObjectDeleteReq) Bind(ctx *gin.Context) error {
	var err error
	log := api.GetRequestLogger(ctx)
	err = ctx.ShouldBind(s)
	if err != nil {
		log.Warnf("ShouldBind error: %s", err.Error())
		return err
	}
	if len(s.Ids) > 0 {
		return nil
	}
	if s.Ids == nil {
		s.Ids = make([]int, 0)
	}

	if err = vd.Validate(s); err != nil {
		log.Errorf("Validate error: %s", err.Error())
		return err
	}
	return err
}

func (s *ObjectDeleteReq) GetId() interface{} {
	return s.Ids
}
