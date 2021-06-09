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
	file         *os.File
	size         uint
	FilenameFunc func(*FileWriter) string
	num          uint
	opts         options
}

// NewFileWriter 实例化FileWriter, 支持大文件分割
func NewFileWriter(opts ...Option) (*FileWriter, error) {
	p := &FileWriter{
		opts: setDefault(),
	}
	for _, o := range opts {
		o(&p.opts)
	}
	var filename string
	var err error
	for {
		filename = p.getFilenameAccordingToTimestamp()
		info, err := os.Stat(filename)
		if err != nil {
			if os.IsNotExist(err) {
				if p.size < p.opts.cap && p.num > 0 {
					p.num--
					fmt.Println(p.size, p.opts.cap, p.num)
					filename = p.getFilenameAccordingToTimestamp()
				}
				//文件不存在
				break
			}
			//存在，但是报错了
			return nil, err
		}
		p.size = uint(info.Size())
		p.num++
		if p.opts.cap == 0 {
			break
		}
	}
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
	p.size += uint(n)
	//每天一个文件
	filename := p.getFilenameAccordingToTimestamp()
	if p.file.Name() != filename || p.size > p.opts.cap {
		if p.size > p.opts.cap {
			p.num += 1
			filename = p.getFilenameAccordingToTimestamp()
		} else {
			p.num = 0
		}
		p.size = 0
		_ = p.file.Close()
		p.file, _ = os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_SYNC, 0600)
	}
	return n, e
}

// getFilenameAccordingToTimestamp 通过日期命名log文件
func (p *FileWriter) getFilenameAccordingToTimestamp() string {
	if p.FilenameFunc != nil {
		return p.FilenameFunc(p)
	}
	if p.opts.cap == 0 {
		return filepath.Join(p.opts.path,
			fmt.Sprintf("%s.%s",
				time.Now().Format(FileNameTimeFormat),
				p.opts.suffix))
	}
	return filepath.Join(p.opts.path,
		fmt.Sprintf("%s-[%d].%s",
			time.Now().Format(FileNameTimeFormat),
			p.num,
			p.opts.suffix))
}
