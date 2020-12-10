package poster

import (
	"bytes"
	"crypto/tls"
	"errors"
	"image"
	"io/ioutil"
	"net/http"
)

// GetImage 从源读取图片，支持网络和本地
func GetImage(src string) (m image.Image, err error) {
	var r *bytes.Reader
	r, err = getResourceReader(src)
	if err != nil {
		return nil, err
	}
	m, _, err = image.Decode(r)
	return
}

// getResourceReader 读取图片 支持本地和网络图片
func getResourceReader(src string) (r *bytes.Reader, err error) {
	if len(src) < 5 {
		return nil, errors.New("图片源错误")
	}

	//跳过证书验证
	c := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	if src[0:4] == "http" {
		resp, err := c.Get(src)
		if err != nil {
			return r, err
		}
		defer resp.Body.Close()
		fileBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return r, err
		}
		r = bytes.NewReader(fileBytes)
	} else {
		fileBytes, err := ioutil.ReadFile(src)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(fileBytes)
	}
	return r, nil
}
