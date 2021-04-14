package antd

 const (
	Silent       = 0
	MessageWarn  = 1
	MessageError = 2
	Notification = 4
	Page         = 9
)

type Response struct {
	Success      bool   `json:"success"`      // if request is success
	ErrorCode    string `json:"errorCode"`    // code for errorType
	ErrorMessage string `json:"errorMessage"` // message display to user
	ShowType     string `json:"showType"`     // error display type： 0 silent; 1 message.warn; 2 message.error; 4 notification; 9 page
	TraceId      string `json:"traceId"`      // Convenient for back-end Troubleshooting: unique request ID
	Host         string `json:"host"`         // onvenient for backend Troubleshooting: host of current access server
}
type response struct {
	Response
	Data interface{} `json:"data"` // response data
}

type Pages struct {
	Total    int `json:"total"`
	Current  int `json:"current"`
	PageSize int `json:"pageSize"`
}

type pages struct {
	Pages
	List interface{} `json:"list"`
}
