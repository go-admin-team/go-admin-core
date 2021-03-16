package response

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	log "github.com/go-admin-team/go-admin-core/logger"
	"github.com/go-admin-team/go-admin-core/sdk/pkg"
)

// 失败数据处理
func Error(c *gin.Context, code int32, err error, msg string) {
	msgID := pkg.GenerateMsgIDFromContext(c)
	var res response
	if err != nil {
		res.Msg = err.Error()
	}
	if msg != "" {
		res.Msg = msg
	}
	res.RequestId = pkg.GenerateMsgIDFromContext(c)
	res.Code = code
	var jsonStr []byte
	jsonStr, err = json.Marshal(res)
	if err != nil {
		log.Debugf("MsgID[%s] ShouldBind error: %#v", msgID, err.Error())
	}
	c.Set("result", string(jsonStr))
	c.AbortWithStatusJSON(http.StatusOK, res)
}

// 通常成功数据处理
func OK(c *gin.Context, data interface{}, msg string) {
	msgID := pkg.GenerateMsgIDFromContext(c)
	var res response
	res.Data = data
	if msg != "" {
		res.Msg = msg
	}
	res.RequestId = pkg.GenerateMsgIDFromContext(c)
	res.Code = 200
	var jsonStr []byte
	jsonStr, err := json.Marshal(res)
	if err != nil {
		log.Debugf("MsgID[%s] ShouldBind error: %#v", msgID, err.Error())
	}
	c.Set("result", string(jsonStr))
	c.AbortWithStatusJSON(http.StatusOK, res)
}

// 分页数据处理
func PageOK(c *gin.Context, result interface{}, count int, pageIndex int, pageSize int, msg string) {
	var res page
	res.List = result
	res.Count = count
	res.PageIndex = pageIndex
	res.PageSize = pageSize
	OK(c, res, msg)
}

// 兼容函数
func Custum(c *gin.Context, data gin.H) {
	msgID := pkg.GenerateMsgIDFromContext(c)
	Return := data
	var jsonStr []byte
	jsonStr, err := json.Marshal(Return)
	if err != nil {
		log.Debugf("MsgID[%s] ShouldBind error: %#v", msgID, err.Error())
	}
	c.Set("result", string(jsonStr))
	c.AbortWithStatusJSON(http.StatusOK, Return)
}
