package ifile

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
	pathspec "github.com/shibumi/go-pathspec"
)

type (
	Ifile struct {
		typ      IncludeOrIgnore
		existing map[string]interface{}
		buf      bytes.Buffer
		bufMu    sync.Mutex
		end      []byte

		filePath string
		file     *os.File
		flock    *flock.Flock
		once     sync.Once
	}

	entry struct {
		path  string
		isDir bool
	}
	entries []*entry

	ignorefile struct {
		p   *pathspec.PathSpec
		dir string
	}

	IncludeOrIgnore int
)

const (
	Include = iota
	Ignore
)

const (
	generatedBy    = "# Generated by kopyaship. DO NOT TOUCH"
	beginIndicator = "# I_BEGIN"
	endIndicator   = "# I_END"
)

func New(filePath string, typ IncludeOrIgnore, appendToExisting, shell bool) (ifile *Ifile, err error) {
	ifile = &Ifile{
		typ:      typ,
		filePath: filePath,
	}

	flags := os.O_CREATE | os.O_WRONLY
	if !appendToExisting {
		flags |= os.O_TRUNC
	}
	ifile.file, err = os.OpenFile(ifile.filePath, flags, 0660)
	if err != nil {
		return nil, err
	}

	ifile.flock = flock.New(ifile.filePath)
	if shell {
		go func() {
			time.Sleep(5 * time.Second)
			if !ifile.flock.Locked() {
				fmt.Printf("Waiting to lock file `%s`. Another process holds lock to the file.", ifile.filePath)
			}
		}()
	}
	err = ifile.flock.Lock()
	if err != nil {
		return nil, err
	}

	if appendToExisting {
		ifile.existing, err = ifile.seekToEnd()
		if err != nil {
			return nil, err
		}
	}
	return
}

func (i *Ifile) seekToEnd() (existing map[string]interface{}, err error) {
	content, err := os.ReadFile(i.filePath)
	if err != nil {
		return nil, err
	}
	content = bytes.ReplaceAll(content, []byte{'\r', '\n'}, []byte{'\n'})
	splitted := bytes.Split(content, []byte{'\n'})
	existing = make(map[string]interface{}, len(splitted)-2)

	begin := -1
	end := -1
	c := 0

	for _, line := range splitted {
		if bytes.Equal(line, []byte(endIndicator)) {
			end = c
			break
		} else if bytes.Equal(line, []byte(beginIndicator)) {
			begin = c
		} else if begin != -1 && end == -1 {
			if len(line) != 0 {
				existing[string(line)] = nil
			}
		}

		c += len(line) + 1
	}

	if begin == -1 && end != -1 {
		return nil, fmt.Errorf("ifile begin indicator ('%s') not found", beginIndicator)
	} else if end == -1 && begin != -1 {
		return nil, fmt.Errorf("ifile end indicator ('%s') not found", endIndicator)
	}

	// To apply potential newline change
	err = os.WriteFile(i.filePath, content, 0660)
	if err != nil {
		return nil, err
	}

	if begin == -1 && end == -1 {
		_, err := i.file.Seek(0, io.SeekEnd)
		if err != nil {
			return nil, err
		}
	} else {
		i.end = content[end:]
		_, err = i.file.Seek(int64(end), io.SeekStart)
		if err != nil {
			return nil, err
		}
	}
	return
}

func (i *Ifile) Close() (err error) {
	var (
		err1 error
		err2 error
	)
	i.once.Do(func() {
		_, err1 = i.file.Write(i.buf.Bytes())
		if len(i.end) != 0 {
			_, err2 = i.file.Write(i.end)
		} else {
			_, err2 = i.file.WriteString("\n")
		}
		i.flock.Unlock()
		i.file.Close()
	})

	// Prioritize err1
	if err1 != nil {
		err = err1
	} else if err2 != nil {
		err = err2
	}
	return
}

func (e *entry) Len() int {
	// + 1 is for newline; + 10 is for potential '\[' and '\]'
	return len(e.path) + 1 + 10
}

func (e *entry) String() string {
	s := e.path + "\n"
	s = strings.ReplaceAll(s, "[", "\\[")
	s = strings.ReplaceAll(s, "]", "\\]")
	return s
}
