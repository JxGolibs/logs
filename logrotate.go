package logs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultSize int64 = 1           // in Mbyte of log file.
	megabyte    int64 = 1024 * 1024 // base Mbyte.
	defaultMaxCount  int=7 //默认最大log文件数量7，-1为不删除老的日志文件
)

// NewLogrotate return logrotate struct with name and max size (Mbyte) of file.
func NewLogrotate(filename string, size int64, maxcount ...int) io.WriteCloser {
	if size < defaultSize {
		size = defaultSize
	}
	l := &Logrotate{
		Filename: filename,
		MaxSize:  size * megabyte,
		MaxCount: defaultMaxCount,
	}
	if len(maxcount) > 0 {
		l.MaxCount = maxcount[0]
	}
	return l
}

// Logrotate is an io.WriteCloser that writes to the specified filename.
//
// Filename is the file to write logs to. Backup log files will be retained
// in the same directory.
//
// MaxSize is the maximum size in megabytes of the log file before it gets
// rotated. It defaults to 10 megabytes.
type Logrotate struct {
	Filename string
	MaxSize  int64
	MaxCount int

	mu   sync.Mutex
	file *os.File
	size int64
}

// Write implements io.Writer, and write data in current file.
func (l *Logrotate) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	writeLen := int64(len(p))

	if writeLen > l.MaxSize {
		return 0, errors.New("write length exceeds maximum file size")
	}

	if l.file == nil {
		err := l.createFile()
		if err != nil {
			return 0, err
		}
	}

	if writeLen+l.size > l.MaxSize {
		err := l.rotateFile()
		if err != nil {
			return 0, err
		}
	}

	n, err = l.file.Write(p)
	l.size += int64(n)

	return n, err
}

// Close implements io.Closer, and closes the current file.
func (l *Logrotate) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.closeFile()
}

// close closes the file if it is open.
func (l *Logrotate) closeFile() error {
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

func (l *Logrotate) createFile() error {
	if _, err := os.Stat(filepath.Dir(l.Filename)); os.IsNotExist(err) {
		err := os.Mkdir(filepath.Dir(l.Filename), 0777)
		if err != nil {
			return err
		}
	}

	f, err := os.OpenFile(l.Filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}

	l.file = f
	l.size = 0
	return nil
}

func (l *Logrotate) rotateFile() error {
	err := l.closeFile()
	if err != nil {
		return err
	}

	err = os.Rename(l.Filename, l.backupName())
	if err != nil {
		return err
	}

	//删除老的日志文件（MaxCount=-1时保留所有日志文件）
	paths, fileName := filepath.Split(l.Filename)
	fileSuffix := path.Ext(fileName)
	filenameOnly := strings.TrimSuffix(fileName, fileSuffix)
 
	files := getFilelist(paths, filenameOnly+"_")
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	fmt.Println("couf:", len(files))
	for _, value := range files {
		fmt.Println(value)
	}

	for l.MaxCount > 0 && len(files) > l.MaxCount {
		i := len(files) - 1
		os.Remove(files[i])
		fmt.Println("删除log：", files[i])

		files = append(files[:i], files[i+1:]...)
	}

	return l.createFile()
}

func (l *Logrotate) backupName() string {
	fileSuffix := path.Ext(l.Filename)
	filenameOnly := strings.TrimSuffix(l.Filename, fileSuffix)

	return filenameOnly + "_" + time.Now().Format("20060102_150405") + fileSuffix
}

func getFilelist(path string, basefilename string) []string {
	fmt.Println("path base:", path, basefilename)
	files := []string{}
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if info == nil || info.IsDir() {
			return nil
		}
		_, fileName := filepath.Split(path)
		fmt.Println(path, strings.Index(fileName, basefilename))
		if strings.Index(fileName, basefilename) == 0 {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("filepath.Walk() error: %v\n", err)
	}
	return files
}
