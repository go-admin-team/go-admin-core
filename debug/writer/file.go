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

// timeFormat 时间格式
// 用于文件名称格式
const timeFormat = "2006-01-02"

const MB = 1024 * 1024

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
		filename = p.getFilename()
		_, err := os.Stat(filename)
		if err != nil {
			if os.IsNotExist(err) {
				if p.num > 0 {
					p.num--
					filename = p.getFilename()
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
	if strings.Index(p.file.Name(), time.Now().Format(timeFormat)) < 0 ||
		(p.opts.cap > 0 && uint(info.Size()) > p.opts.cap*MB) {
		//生成新文件
		if uint(info.Size()) > p.opts.cap*MB {
			p.num++
		} else {
			p.num = 0
		}
		filename := p.getFilename()
		_ = p.file.Close()
		p.file, _ = os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_SYNC, 0600)

		dir := filepath.Dir(p.file.Name())
		files, err := os.ReadDir(dir)
		if err != nil {
			fmt.Println("Error reading log directory:", err)
			return
		}

		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), p.opts.suffix) {
				filePath := filepath.Join(dir, file.Name())
				fileInfo, err := file.Info()
				if err != nil {
					fmt.Println("Error get Info file:", err)
					break
				}
				fileModTime := fileInfo.ModTime()
				daysSinceMod := uint(time.Since(fileModTime).Hours() / 24)
				if daysSinceMod > p.opts.daysToKeep {
					err := os.Remove(filePath)
					if err != nil {
						fmt.Println("Error deleting file:", err)
					} else {
						fmt.Println("Deleted file:", filePath)
					}
				}
			}
		}
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

// getFilename 获取log文件名
// 目前为：以日期格式命名，eg：2006-01-02.log or 2006-01-02.log
func (p *FileWriter) getFilename() string {
	if p.FilenameFunc != nil {
		return p.FilenameFunc(p)
	}
	if p.opts.cap == 0 {
		return filepath.Join(p.opts.path,
			fmt.Sprintf("%s.%s",
				time.Now().Format(timeFormat),
				p.opts.suffix))
	}
	return filepath.Join(p.opts.path,
		fmt.Sprintf("%s-[%d].%s",
			time.Now().Format(timeFormat),
			p.num,
			p.opts.suffix))
}
