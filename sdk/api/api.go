package api

import (
	"errors"
	"fmt"
	"io"

	"net/http"

	vd "github.com/bytedance/go-tagexpr/v2/validator"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-admin-team/go-admin-core/v2/logger"
	"github.com/go-admin-team/go-admin-core/v2/response"
	"github.com/go-admin-team/go-admin-core/v2/sdk"
	"github.com/go-admin-team/go-admin-core/v2/sdk/pkg"
	"github.com/go-admin-team/go-admin-core/v2/sdk/service"
	"github.com/go-admin-team/go-admin-core/v2/storage"
	"github.com/go-admin-team/go-admin-core/v2/tools/language"
	"gorm.io/gorm"
)

var DefaultLanguage = "zh-CN"

type Api struct {
	Context *gin.Context
	Logger  *logger.Helper
	Orm     *gorm.DB
	Errors  error
	Cache   storage.AdapterCache
}

func (e *Api) AddError(err error) {
	if e.Errors == nil {
		e.Errors = err
	} else if err != nil {
		e.Logger.Error(err)
		e.Errors = fmt.Errorf("%v; %w", e.Errors, err)
	}
}

// MakeContext 设置http上下文
func (e *Api) MakeContext(c *gin.Context) *Api {
	e.Context = c
	e.Logger = GetRequestLogger(c)
	// 清除上下文中的错误
	e.Errors = nil
	return e
}

// GetLogger 获取上下文提供的日志
func (e *Api) GetLogger() *logger.Helper {
	return GetRequestLogger(e.Context)
}

// validate mirrors gin's internal binding.validate. binding.Validator is a
// public, assignable variable, and setting it to nil is the documented way to
// disable validation globally, so it must not be dereferenced unchecked.
func validate(obj interface{}) error {
	if binding.Validator == nil {
		return nil
	}
	return binding.Validator.ValidateStruct(obj)
}

// Bind 参数校验
func (e *Api) Bind(d interface{}, bindings ...binding.Binding) *Api {
	var err error
	// gin validates the binding tags at the end of every binding stage, so one
	// completed stage means validation has already run.
	var validated bool
	if len(bindings) == 0 {
		bindings = constructor.GetBindingForGin(d)
	}
	for i := range bindings {
		if bindings[i] == nil {
			err = e.Context.ShouldBindUri(d)
		} else {
			err = e.Context.ShouldBindWith(d, bindings[i])
		}
		if err != nil && errors.Is(err, io.EOF) {
			// A request may legitimately carry no body while the struct still
			// binds from the uri or the query string.
			e.Logger.Warn("request body is not present anymore. ")
			err = nil
			continue
		}
		if err != nil {
			e.AddError(err)
			break
		}
		validated = true
	}

	// Every stage was skipped, so the binding tags were never checked and
	// required would be bypassed entirely (issue #81).
	if !validated && e.Errors == nil {
		if err = validate(d); err != nil {
			e.AddError(err)
		}
	}
	//vd.SetErrorFactory(func(failPath, msg string) error {
	//	return fmt.Errorf(`"validation failed: %s %s"`, failPath, msg)
	//})
	if err1 := vd.Validate(d); err1 != nil {
		e.AddError(err1)
	}
	return e
}

// GetOrm 获取Orm DB
func (e *Api) GetOrm() (*gorm.DB, error) {
	db, err := pkg.GetOrm(e.Context)
	if err != nil {
		e.Logger.Error(http.StatusInternalServerError, err, "数据库连接获取失败")
		return nil, err
	}
	return db, nil
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
	c.Cache = sdk.Runtime.GetCacheAdapter()
	e.Cache = c.Cache
	return e
}

// Error 通常错误数据处理
func (e *Api) Error(code int, err error, msg string) {
	response.Error(e.Context, code, err, msg)
}

// OK 通常成功数据处理
func (e *Api) OK(data interface{}, msg string) {
	response.OK(e.Context, data, msg)
}

// PageOK 分页数据处理
func (e *Api) PageOK(result interface{}, count int, pageIndex int, pageSize int, msg string) {
	response.PageOK(e.Context, result, count, pageIndex, pageSize, msg)
}

// Custom 兼容函数
func (e *Api) Custom(data gin.H) {
	response.Custum(e.Context, data)
}

func (e Api) Translate(form, to interface{}) {
	pkg.Translate(form, to)
}

// getAcceptLanguage 获取当前语言
func (e Api) getAcceptLanguage() string {
	languages := language.ParseAcceptLanguage(e.Context.GetHeader("Accept-Language"), nil)
	if len(languages) == 0 {
		return DefaultLanguage
	}
	return languages[0]
}
