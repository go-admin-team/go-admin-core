package writer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileWriter 文件写入结构体
type FileWriter struct {
	path   string
	file   *os.File
	size   int64 //todo 后期加入大文件写入分割
	suffix string
}

// NewFileWriter 实例化FileWriter
func NewFileWriter(path, suffix string) *FileWriter {
	return &FileWriter{
		path:   path,
		suffix: suffix,
	}
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
	filename := filepath.Join(p.path, fmt.Sprintf("%s.%s", time.Now().Format("2006-01-02"), p.suffix))
	if p.file.Name() != filename {
		_ = p.file.Close()
		p.file, _ = os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_SYNC, 0600)
		p.size = 0
	}
	return n, e
}
