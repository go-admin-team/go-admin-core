package response

import (
	pbErrors "github.com/yahao333/go-admin-core/errors"
)

type Response struct {
	pbErrors.Error
	// 数据集
}

type response struct {
	Response
	Data interface{} `json:"data"`
}

type Page struct {
	Count     int `json:"count"`
	PageIndex int `json:"pageIndex"`
	PageSize  int `json:"pageSize"`
}

type page struct {
	Page
	List interface{} `json:"list"`
}
