package models

// Response is the envelope go-admin's handlers wrap every JSON reply in.
type Response struct {
	// Code is the business status code.
	Code int `json:"code" example:"200"`
	// Data is the payload.
	Data interface{} `json:"data"`
	// Msg is a human-readable message.
	Msg       string `json:"msg"`
	RequestId string `json:"requestId"`
}

// Page wraps a list result with its pagination metadata.
type Page struct {
	List      interface{} `json:"list"`
	Count     int         `json:"count"`
	PageIndex int         `json:"pageIndex"`
	PageSize  int         `json:"pageSize"`
}

// ReturnOK marks the response as successful.
func (res *Response) ReturnOK() *Response {
	res.Code = 200
	return res
}

// ReturnError marks the response as failed with the given code.
func (res *Response) ReturnError(code int) *Response {
	res.Code = code
	return res
}
