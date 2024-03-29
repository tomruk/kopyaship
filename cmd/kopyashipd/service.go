package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/kardianos/service"
	"github.com/kirsle/configdir"
	"github.com/labstack/echo/v4"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/tomruk/kopyaship/config"
	_config "github.com/tomruk/kopyaship/config"
	"github.com/tomruk/kopyaship/ifile"
	"github.com/tomruk/kopyaship/utils"
	"go.uber.org/zap"
)

type svice struct {
	service service.Service

	watchJobs []*ifile.WatchJob
	jobsMu    sync.Mutex

	e *echo.Echo
	s *http.Server

	cacheDir string
	config   *_config.Config
	v        *viper.Viper
	lock     *flock.Flock
	log      *zap.Logger

	once    sync.Once
	errChan <-chan error

	exitHandlers   []func()
	exitHandlersMu sync.Mutex
}

func (v *svice) Start(s service.Service) (err error) {
	v.once.Do(func() {
		errChan := make(chan error, 1)
		v.errChan = errChan
		v.service = s

		pflag.StringP("config", "c", "", "Configuration file")
		pflag.Parse()

		var systemWide bool
		systemWide, err = v.initConfig()
		if err != nil {
			return
		}
		err = v.initCache(systemWide)
		if err != nil {
			return
		}
		v.config.PlaceEnvironmentVariables()
		err = v.config.CheckDaemon()
		if err != nil {
			return
		}
		err = v.initLock()
		if err != nil {
			return
		}
		v.log, err = v.newLogger(false)
		if err != nil {
			return
		}

		if v.config.Daemon.API.Enabled {
			var listen func() error
			v.e, v.s, listen, err = v.newAPIServer()
			if err != nil {
				return
			}

			go func() {
				err := listen()
				if err != nil {
					errChan <- fmt.Errorf("api: %v", err)
					err := s.Stop()
					if err != nil {
						v.log.Error(err.Error())
					}
				}
			}()
		}

		var jobs []*ifile.WatchJob
		jobs, err = v.initWatchJobsFromConfig()
		if err != nil {
			return
		}
		for _, j := range jobs {
			go func(j *ifile.WatchJob) {
				err := j.Run()
				if err != nil {
					errChan <- fmt.Errorf("api: %v", err)
					err := s.Stop()
					if err != nil {
						v.log.Error(err.Error())
					}
				}
			}(j)
		}
	})
	return
}

func (v *svice) initConfig() (systemWide bool, err error) {
	configFile, _ := pflag.CommandLine.GetString("config")
	v.config, v.v, systemWide, err = config.Read(configFile)
	if err != nil {
		return
	}
	err = os.Chdir(filepath.Dir(v.v.ConfigFileUsed()))
	if err != nil {
		return
	}
	return
}

func (v *svice) initCache(systemWide bool) error {
	v.cacheDir = os.Getenv("KOPYASHIP_CACHE")
	if v.cacheDir == "" {
		if systemWide {
			if !utils.RunningOnWindows {
				v.cacheDir = "/var/cache/kopyaship"
			} else {
				v.cacheDir = filepath.Join(os.Getenv("PROGRAMDATA"), "kopyaship", "cache")
			}
		} else {
			v.cacheDir = filepath.Join(configdir.LocalCache(), "kopyaship")
		}
		os.Setenv("KOPYASHIP_CACHE", v.cacheDir)
	}
	if _, err := os.Stat(v.cacheDir); os.IsNotExist(err) {
		err = os.MkdirAll(v.cacheDir, 0755)
		if err != nil {
			return fmt.Errorf("could not create the cache directory: %v", err)
		}
	}
	return nil
}

func (v *svice) initLock() error {
	user, err := user.Current()
	if err != nil {
		return err
	}
	username := user.Username
	username = strings.ReplaceAll(username, "\\", "_")
	lockFile := filepath.Join(v.cacheDir, "kopyashipd_"+username+".lock")
	v.lock = flock.New(lockFile)

	go func() {
		time.Sleep(2 * time.Second)
		if !v.lock.Locked() {
			fmt.Printf("The lockfile %s is being used by another instance. Waiting.", lockFile)
		}
	}()
	err = v.lock.Lock()
	if err != nil {
		return err
	}
	return nil
}

func (v *svice) Stop(s service.Service) (err error) {
	select {
	case err = <-v.errChan:
	default:
	}
	if v.lock != nil {
		v.lock.Unlock()
	}
	if v.config != nil && v.e != nil {
		if v.config.Daemon.API.Listen == "ipc" {
			socketPath := filepath.Join(v.cacheDir, "api.socket")
			defer os.Remove(socketPath)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		v.e.Shutdown(ctx)
		cancel()
	}

	v.jobsMu.Lock()
	defer v.jobsMu.Unlock()
	for _, job := range v.watchJobs {
		jobErr := job.Shutdown()
		if err == nil && jobErr != nil {
			err = jobErr
		}
	}

	v.exitHandlersMu.Lock()
	defer v.exitHandlersMu.Unlock()
	for _, f := range v.exitHandlers {
		f()
	}
	return
}

func (v *svice) addExitHandler(f func()) {
	v.exitHandlersMu.Lock()
	v.exitHandlers = append(v.exitHandlers, f)
	v.exitHandlersMu.Unlock()
}
