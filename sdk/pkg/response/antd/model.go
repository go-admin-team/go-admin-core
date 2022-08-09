package antd

import (
	"fmt"

	resp "github.com/go-admin-team/go-admin-core/sdk/pkg/response"
)

const (
	Silent       = "0"
	MessageWarn  = "1"
	MessageError = "2"
	Notification = "4"
	Page         = "9"
)

type Response struct {
	Success      bool   `json:"success,omitempty"`      // if request is success
	ErrorCode    string `json:"errorCode,omitempty"`    // code for errorType
	ErrorMessage string `json:"errorMessage,omitempty"` // message display to user
	ShowType     string `json:"showType,omitempty"`     // error display type： 0 silent; 1 message.warn; 2 message.error; 4 notification; 9 page
	TraceId      string `json:"traceId,omitempty"`      // Convenient for back-end Troubleshooting: unique request ID
	Host         string `json:"host,omitempty"`         // onvenient for backend Troubleshooting: host of current access server
	Status       string `json:"status,omitempty"`
}
type response struct {
	Response
	Data interface{} `json:"data,omitempty"` // response data
}

type Pages struct {
	Response
	Data     interface{} `json:"data,omitempty"` // response data
	Total    int         `json:"total,omitempty"`
	Current  int         `json:"current,omitempty"`
	PageSize int         `json:"pageSize,omitempty"`
}

type pages struct {
	Pages
	Data interface{} `json:"data,omitempty"`
}

type lists struct {
	Response
	ListData ListData `json:"data,omitempty"` // response data
}

type ListData struct {
	List     interface{} `json:"list,omitempty"` // response data
	Total    int         `json:"total,omitempty"`
	Current  int         `json:"current,omitempty"`
	PageSize int         `json:"pageSize,omitempty"`
}

func (e *response) SetCode(code int32) {
	switch code {
	case 200, 0:
	default:
		e.ErrorCode = fmt.Sprintf("C%d", code)
	}
}

func (e *response) SetTraceID(id string) {
	e.TraceId = id
}

func (e *response) SetMsg(msg string) {
	e.ErrorMessage = msg
}

func (e *response) SetData(data interface{}) {
	e.Data = data
}

func (e *response) SetSuccess(success bool) {
	e.Success = success
}

func (e response) Clone() resp.Responses {
	return &e
}
