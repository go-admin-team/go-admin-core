package writer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileNameTimeFormat 文件名称格式
var FileNameTimeFormat = "2006-01-02"

// FileWriter 文件写入结构体
type FileWriter struct {
	path         string
	file         *os.File
	size         int64  //todo 后期加入大文件写入分割
	suffix       string //文件扩展名
	FilenameFunc func(*FileWriter) string
}

// NewFileWriter 实例化FileWriter
func NewFileWriter(path, suffix string) (*FileWriter, error) {
	p := &FileWriter{
		path:   path,
		suffix: suffix,
	}
	filename := p.getFilenameAccordingToTimestamp()
	var err error
	p.file, err = os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_SYNC, 0600)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// Write 写入方法
func (p *FileWriter) Write(data []byte) (n int, err error) {
	if p == nil {
		return 0, errors.New("logFileWriter is nil")
	}
	if p.file == nil {
		return 0, errors.New("file not opened")
	}
	n, e := p.file.Write(data)
	p.size += int64(n)
	//每天一个文件
	filename := p.getFilenameAccordingToTimestamp()
	if p.file.Name() != filename {
		_ = p.file.Close()
		p.file, _ = os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_SYNC, 0600)
		p.size = 0
	}
	return n, e
}

// getFilenameAccordingToTimestamp 通过日期命名log文件
func (p *FileWriter) getFilenameAccordingToTimestamp() string {
	if p.FilenameFunc != nil {
		return p.FilenameFunc(p)
	}
	return filepath.Join(p.path, fmt.Sprintf("%s.%s", time.Now().Format(FileNameTimeFormat), p.suffix))
}
