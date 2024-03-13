package ifile

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

type (
	WatchJob struct {
		log     *zap.Logger
		status  atomic.Int32
		stopped chan struct{}
		errs    chan error

		scanPath string
		ifile    string
	}

	WatchJobStatus int32

	WatchJobInfo struct {
		ScanPath string  `json:"scanPath"`
		Ifile    string  `json:"ifile"`
		Errors   []error `json:"errors"`
	}
)

const (
	WatchJobStatusWillRun WatchJobStatus = iota
	WatchJobStatusRunning
	WatchJobStatusFailed
	WatchJobStatusStopped
)

const failAfter = 20 // 20 seconds

func (s WatchJobStatus) String() string {
	switch s {
	case WatchJobStatusWillRun:
		return "will run"
	case WatchJobStatusRunning:
		return "running"
	case WatchJobStatusFailed:
		return "failed"
	case WatchJobStatusStopped:
		return "stopped"
	default:
		return "<invalid status>"
	}
}

func NewWatchJob(log *zap.Logger, ScanPath, Ifile string) *WatchJob {
	j := &WatchJob{
		log:      log,
		status:   atomic.Int32{},
		stopped:  make(chan struct{}),
		errs:     make(chan error, 5),
		scanPath: ScanPath,
		ifile:    Ifile,
	}
	j.status.Store(int32(WatchJobStatusWillRun))
	return j
}

func (j *WatchJob) ScanPath() string { return j.scanPath }

func (j *WatchJob) Ifile() string { return j.ifile }

func (j *WatchJob) Status() WatchJobStatus { return WatchJobStatus(j.status.Load()) }

func (j *WatchJob) Errors() <-chan error { return j.errs }

func (j *WatchJob) Info() *WatchJobInfo {
	errs := make([]error, 0, len(j.errs))
	for range len(j.errs) {
		select {
		case err := <-j.errs:
			errs = append(errs, err)
		default:
		}
	}

	return &WatchJobInfo{
		ScanPath: j.scanPath,
		Ifile:    j.ifile,
		Errors:   errs,
	}
}

func (j *WatchJobInfo) String() string {
	e := ""
	for _, err := range j.Errors {
		e += "    " + err.Error() + "\n"
	}
	return fmt.Sprintf("Scan path: %s\nIfile: %s\nErrors: %s", j.ScanPath, j.Ifile, e)
}

func (j *WatchJob) Start() error {
	go func() {
		err := j.walk()
		if err != nil {
			j.logError(err)
			j.fail()
			return
		}

		firstAttempt := time.Now()

	outer:
		for {
			watcher, c, err := watch(j.scanPath)
			if err != nil {
				j.logError(err)
				j.sleepBeforeRetry(1)
				if time.Since(firstAttempt).Seconds() >= failAfter {
					j.fail()
				}
				continue
			}

			j.status.Store(int32(WatchJobStatusRunning))

			for {
				select {
				case <-c:
					err := j.walk()
					if err != nil {
						j.logError(err)
						j.sleepBeforeRetry(1)
						if time.Since(firstAttempt).Seconds() >= failAfter {
							j.fail()
						}
						continue outer
					}
				case <-j.stopped:
					watcher.Close()
					return
				}
			}
		}
	}()
	return nil
}

func (j *WatchJob) logError(err error) {
	if err != nil {
		err = fmt.Errorf("watch: %v", err)
		j.log.Error(err.Error())
		select {
		case j.errs <- err:
		default:
		}
	}
}

func (j *WatchJob) sleepBeforeRetry(seconds time.Duration) {
	time.Sleep(seconds * time.Second)
	j.log.Sugar().Info("retry in %d second(s)", seconds)
}

func (j *WatchJob) fail() { j.status.Store(int32(WatchJobStatusFailed)) }

func (j *WatchJob) walk() error {
	// TODO: Arguments
	i, err := New(j.ifile, Include, true, true)
	if err != nil {
		return err
	}
	defer i.Close()
	return i.Walk(j.scanPath)
}

func (j *WatchJob) Stop() error {
	close(j.stopped)
	j.status.Store(int32(WatchJobStatusStopped))
	return nil
}

func watch(path string) (*fsnotify.Watcher, <-chan string, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, err
	}
	err = watcher.Add(path)
	if err != nil {
		return nil, nil, err
	}

	c := make(chan string, 1)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
					base := filepath.Base(event.Name)
					if base == gitignore || base == csignore {

					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Fprintf(os.Stderr, "Event error: %v\n", err)
			}
		}
	}()
	return watcher, c, nil
}