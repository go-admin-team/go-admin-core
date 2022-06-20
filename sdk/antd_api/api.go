package antd_apis

import (
	"errors"
	"fmt"
	vd "github.com/bytedance/go-tagexpr/v2/validator"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-admin-team/go-admin-core/sdk/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-admin-team/go-admin-core/logger"
	"github.com/go-admin-team/go-admin-core/sdk/api"
	"github.com/go-admin-team/go-admin-core/sdk/pkg"
	"github.com/go-admin-team/go-admin-core/sdk/pkg/response/antd"
	"gorm.io/gorm"
)

type Api struct {
	Context *gin.Context
	Logger  *logger.Helper
	Orm     *gorm.DB
	Errors  error
}

// GetLogger 获取上下文提供的日志
func (e Api) GetLogger() *logger.Helper {
	return api.GetRequestLogger(e.Context)
}

// GetOrm 获取Orm DB
func (e *Api) GetOrm(c *gin.Context) (*gorm.DB, error) {
	db, err := pkg.GetOrm(c)
	if err != nil {
		e.Error(http.StatusInternalServerError, "数据库连接获取失败", "9")
		return nil, err
	}
	return db, nil
}

// Error 通常错误数据处理
// showType error display type： 0 silent; 1 message.warn; 2 message.error; 4 notification; 9 page
func (e *Api) Error(errCode int, errMsg string, showType string) {
	if showType == "" {
		showType = "2"
	}
	antd.Error(e.Context, strconv.Itoa(errCode), errMsg, showType)
}

// OK 通常成功数据处理
func (e *Api) OK(data interface{}) {
	antd.OK(e.Context, data)
}

// PageOK 分页数据处理
func (e *Api) PageOK(result interface{}, total int, current int, pageSize int) {
	antd.PageOK(e.Context, result, total, current, pageSize)
}

func (e *Api) ListOK(result interface{}, total int, current int, pageSize int) {
	antd.ListOK(e.Context, result, total, current, pageSize)
}

// Custom 兼容函数
func (e *Api) Custom(data gin.H) {
	antd.Custum(e.Context, data)
}

// MakeContext 设置http上下文
func (e *Api) MakeContext(c *gin.Context) *Api {
	e.Context = c
	e.Logger = api.GetRequestLogger(c)
	return e
}

// Bind 参数校验
func (e *Api) Bind(d interface{}, bindings ...binding.Binding) *Api {
	var err error
	if len(bindings) == 0 {
		bindings = constructor.GetBindingForGin(d)
	}
	for i := range bindings {
		if bindings[i] == nil {
			err = e.Context.ShouldBindUri(d)
		} else {
			err = e.Context.ShouldBindWith(d, bindings[i])
		}
		if err != nil && err.Error() == "EOF" {
			e.Logger.Warn("request body is not present anymore. ")
			err = nil
			continue
		}
		if err != nil {
			e.AddError(err)
			break
		}
	}
	if err1 := vd.Validate(d); err1 != nil {
		e.AddError(err1)
	}
	return e
}

// MakeOrm 设置Orm DB
func (e *Api) MakeOrm() *Api {
	var err error
	if e.Logger == nil {
		err = errors.New("at MakeOrm logger is nil")
		e.AddError(err)
		return e
	}
	db, err := pkg.GetOrm(e.Context)
	if err != nil {
		e.Logger.Error(http.StatusInternalServerError, err, "数据库连接获取失败")
		e.AddError(err)
	}
	e.Orm = db
	return e
}

func (e *Api) MakeService(c *service.Service) *Api {
	c.Log = e.Logger
	c.Orm = e.Orm
	return e
}

func (e *Api) AddError(err error) {
	if e.Errors == nil {
		e.Errors = err
	} else if err != nil {
		e.Logger.Error(err)
		e.Errors = fmt.Errorf("%v; %w", e.Error, err)
	}
}

func (e Api) Translate(form, to interface{}) {
	pkg.Translate(form, to)
}
