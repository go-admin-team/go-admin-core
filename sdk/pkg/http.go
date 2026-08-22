package pkg

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// Get 发送GET请求
// url：         请求地址
// response：    请求返回的内容
func Get(url string) (string, error) {

	client := &http.Client{}
	// The header was set before this check, so a url NewRequest rejects gave
	// the caller a nil dereference instead of the error it returns.
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// A body that fails halfway through used to come back as a short string
	// with no error, which reads exactly like a successful empty response.
	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

// Post 发送POST请求
// url：         请求地址
// data：        POST请求提交的数据
// contentType： 请求体格式，如：application/json
// content：     请求放回的内容
func Post(url string, data interface{}, contentType string) ([]byte, error) {

	// 超时时间：5秒
	client := &http.Client{Timeout: 5 * time.Second}
	// A value that will not marshal used to be posted as an empty body.
	jsonStr, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	resp, err := client.Post(url, contentType, bytes.NewBuffer(jsonStr))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return result, nil

}
