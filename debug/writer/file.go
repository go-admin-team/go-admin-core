package writer

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// fileNameTimeFormat 文件名称格式
const fileNameTimeFormat = "2006-01-02"

// FileWriter 文件写入结构体
type FileWriter struct {
	file         *os.File
	FilenameFunc func(*FileWriter) string
	num          uint
	opts         Options
	input        chan []byte
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
		_, err := os.Stat(filename)
		if err != nil {
			if os.IsNotExist(err) {
				if p.num > 0 {
					p.num--
					filename = p.getFilenameAccordingToTimestamp()
				}
				//文件不存在
				break
			}
			//存在，但是报错了
			return nil, err
		}
		p.num++
		if p.opts.cap == 0 {
			break
		}
	}
	p.file, err = os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_SYNC, 0600)
	if err != nil {
		return nil, err
	}
	p.input = make(chan []byte, 100)
	go p.write()
	return p, nil
}

func (p *FileWriter) write() {
	for {
		select {
		case d := <-p.input:
			_, err := p.file.Write(d)
			if err != nil {
				log.Printf("write file failed, %s\n", err.Error())
			}
			p.checkFile()
		}
	}
}

func (p *FileWriter) checkFile() {
	info, _ := p.file.Stat()
	if strings.Index(p.file.Name(), time.Now().Format(fileNameTimeFormat)) < 0 ||
		(p.opts.cap > 0 && uint(info.Size()) > p.opts.cap) {
		//生成新文件
		if uint(info.Size()) > p.opts.cap {
			p.num++
		} else {
			p.num = 0
		}
		filename := p.getFilenameAccordingToTimestamp()
		_ = p.file.Close()
		p.file, _ = os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_SYNC, 0600)
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
	n = len(data)
	go func() {
		p.input <- data
	}()
	return n, nil
}

// getFilenameAccordingToTimestamp 通过日期命名log文件
func (p *FileWriter) getFilenameAccordingToTimestamp() string {
	if p.FilenameFunc != nil {
		return p.FilenameFunc(p)
	}
	if p.opts.cap == 0 {
		return filepath.Join(p.opts.path,
			fmt.Sprintf("%s.%s",
				time.Now().Format(fileNameTimeFormat),
				p.opts.suffix))
	}
	return filepath.Join(p.opts.path,
		fmt.Sprintf("%s-[%d].%s",
			time.Now().Format(fileNameTimeFormat),
			p.num,
			p.opts.suffix))
}
