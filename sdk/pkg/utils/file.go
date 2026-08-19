package utils

import (
	"errors"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/shamsher31/goimgext"
)

// GetSize 获取文件大小
func GetSize(f multipart.File) (int, error) {
	content, err := ioutil.ReadAll(f)

	return len(content), err
}

// GetExt 获取文件后缀
func GetExt(fileName string) string {
	return path.Ext(fileName)
}

// CheckExist 检查文件是否存在
func CheckExist(src string) bool {
	_, err := os.Stat(src)

	return os.IsNotExist(err)
}

// CheckPermission 检查文件权限
func CheckPermission(src string) bool {
	_, err := os.Stat(src)

	return os.IsPermission(err)
}

// IsNotExistMkDir 检查文件夹是否存在
// 如果不存在则新建文件夹
func IsNotExistMkDir(src string) error {
	if exist := !CheckExist(src); exist == false {
		if err := MkDir(src); err != nil {
			return err
		}
	}

	return nil
}

// MkDir 新建文件夹
func MkDir(src string) error {
	err := os.MkdirAll(src, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

// Open 打开文件
//
// The caller owns the file and closes it. Closing it here — which a deferred
// call did — handed back a file that was already closed.
func Open(name string, flag int, perm os.FileMode) (*os.File, error) {
	f, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}

	return f, nil
}

// GetImgType 获取Img文件类型
func GetImgType(p string) (string, error) {
	file, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer file.Close()

	buff := make([]byte, 512)

	// DetectContentType is given what was actually read: a file shorter than
	// the buffer used to be padded with zeroes and detected as those.
	n, err := file.Read(buff)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	filetype := http.DetectContentType(buff[:n])

	ext := imgext.Get()

	for i := 0; i < len(ext); i++ {
		if strings.Contains(ext[i], filetype[6:]) {
			return filetype, nil
		}
	}

	return "", errors.New("Invalid image type")
}

// GetType 获取文件类型
func GetType(p string) (string, error) {
	file, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer file.Close()

	buff := make([]byte, 512)

	n, err := file.Read(buff)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	filetype := http.DetectContentType(buff[:n])

	//ext := GetExt(p)
	//var list = strings.Split(filetype, "/")
	//filetype = list[0] + "/" + ext
	return filetype, nil
}
