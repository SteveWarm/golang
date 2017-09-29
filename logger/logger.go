package logger

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
	"time"
)

const (
	_VER string = "1.0.0"
)

type LEVEL int32

var logLevel LEVEL = 1
var maxFileSize int64
var maxFileCount int32
var dailyRolling bool = true
var consoleAppender bool = true
var logObj *_FILE

const DATEFORMAT = "2006-01-02"

type UNIT int64

const (
	_       = iota
	KB UNIT = 1 << (iota * 10)
	MB
	GB
	TB
)

const (
	ALL LEVEL = iota
	DEBUG
	INFO
	WARN
	ERROR
	FATAL
	OFF
)

type _FILE struct {
	dir      string
	filename string
	_suffix  int
	isCover  bool
	_date    *time.Time
	mu       *sync.RWMutex
	logfile  *os.File
	lg       *log.Logger
}

func SetConsole(isConsole bool) {
	consoleAppender = isConsole
}

func SetLevel(_level LEVEL) {
	logLevel = _level
}

func GetLevel() LEVEL {
	return logLevel
}

func SetRollingDaily(fileDir, fileName string) {
	dailyRolling = true
	t, _ := time.Parse(DATEFORMAT, time.Now().Format(DATEFORMAT))
	logObj = &_FILE{dir: fileDir, filename: fileName, _date: &t, isCover: false, mu: new(sync.RWMutex)}
	logObj.mu.Lock()
	defer logObj.mu.Unlock()

	if !logObj.isMustRename() {
		logObj.logfile, _ = os.OpenFile(fileDir+"/"+fileName, os.O_RDWR|os.O_APPEND|os.O_CREATE, os.FileMode(0644))
		logObj.lg = log.New(logObj.logfile, "\n", log.Ldate|log.Ltime|log.Lshortfile)
	} else {
		logObj.rename()
	}
}

func console(s ...interface{}) {
	if consoleAppender {
		_, file, line, _ := runtime.Caller(2)
		short := file
		for i := len(file) - 1; i > 0; i-- {
			if file[i] == '/' {
				short = file[i+1:]
				break
			}
		}
		file = short
		log.Println(file+":"+strconv.Itoa(line), s)
	}
}

func catchError() {
	if err := recover(); err != nil {
		heap := debug.Stack()
		if len(heap) > 10240 {
			heap = heap[0:10240]
		}
		log.Println("err", err, "stack:\n"+string(heap))
	}
}

func All(v ...interface{}) {
	if logLevel <= ALL {
		if dailyRolling {
			fileCheck()
		}
		defer catchError()

		if logObj != nil && logObj.mu != nil && logObj.lg != nil {
			logObj.mu.RLock()
			defer logObj.mu.RUnlock()
			logObj.lg.Output(2, fmt.Sprintln("all", v))
		}
		console("all", v)
	}
}

func Debug(v ...interface{}) {
	if logLevel <= DEBUG {
		if dailyRolling {
			fileCheck()
		}
		defer catchError()

		if logObj != nil && logObj.mu != nil && logObj.lg != nil {
			logObj.mu.RLock()
			defer logObj.mu.RUnlock()
			logObj.lg.Output(2, fmt.Sprintln("debug", v))
		}
		console("debug", v)
	}
}

func Info(v ...interface{}) {
	if logLevel <= INFO {
		if dailyRolling {
			fileCheck()
		}
		defer catchError()

		if logObj != nil && logObj.mu != nil && logObj.lg != nil {
			logObj.mu.RLock()
			defer logObj.mu.RUnlock()
			logObj.lg.Output(2, fmt.Sprintln("info", v))
		}
		console("info", v)
	}
}
func Warn(v ...interface{}) {
	if logLevel <= WARN {
		if dailyRolling {
			fileCheck()
		}
		defer catchError()
		if logObj != nil && logObj.mu != nil && logObj.lg != nil {
			logObj.mu.RLock()
			defer logObj.mu.RUnlock()
			logObj.lg.Output(2, fmt.Sprintln("warn", v))
		}
		console("warn", v)
	}
}
func Error(v ...interface{}) {
	if logLevel <= ERROR {
		if dailyRolling {
			fileCheck()
		}
		defer catchError()

		if logObj != nil && logObj.mu != nil && logObj.lg != nil {
			logObj.mu.RLock()
			defer logObj.mu.RUnlock()
			logObj.lg.Output(2, fmt.Sprintln("error", v))
		}
		console("error", v)
	}
}
func Fatal(v ...interface{}) {
	if logLevel <= FATAL {
		if dailyRolling {
			fileCheck()
		}
		defer catchError()

		if logObj != nil && logObj.mu != nil && logObj.lg != nil {
			logObj.mu.RLock()
			defer logObj.mu.RUnlock()
			logObj.lg.Output(2, fmt.Sprintln("fatal", v))
		}
		console("fatal", v)
	}
}

func (f *_FILE) isMustRename() bool {
	if dailyRolling {
		t, _ := time.Parse(DATEFORMAT, time.Now().Format(DATEFORMAT))
		if t.After(*f._date) {
			return true
		}
	} else {
		if maxFileCount > 1 {
			if fileSize(f.dir+"/"+f.filename) >= maxFileSize {
				return true
			}
		}
	}
	return false
}

func (f *_FILE) rename() {
	if dailyRolling {
		fn := f.dir + "/" + f.filename + "." + f._date.Format(DATEFORMAT)
		if !isExist(fn) && f.isMustRename() {
			if f.logfile != nil {
				f.logfile.Close()
			}
			err := os.Rename(f.dir+"/"+f.filename, fn)
			if err != nil {
				f.lg.Println("rename err", err.Error())
			}
			t, _ := time.Parse(DATEFORMAT, time.Now().Format(DATEFORMAT))
			f._date = &t
			f.logfile, _ = os.Create(f.dir + "/" + f.filename)
			f.lg = log.New(logObj.logfile, "\n", log.Ldate|log.Ltime|log.Lshortfile)
		}
	} else {
		f.coverNextOne()
	}
}

func (f *_FILE) nextSuffix() int {
	return int(f._suffix%int(maxFileCount) + 1)
}

func (f *_FILE) coverNextOne() {
	f._suffix = f.nextSuffix()
	if f.logfile != nil {
		f.logfile.Close()
	}
	if isExist(f.dir + "/" + f.filename + "." + strconv.Itoa(int(f._suffix))) {
		os.Remove(f.dir + "/" + f.filename + "." + strconv.Itoa(int(f._suffix)))
	}
	os.Rename(f.dir+"/"+f.filename, f.dir+"/"+f.filename+"."+strconv.Itoa(int(f._suffix)))
	f.logfile, _ = os.Create(f.dir + "/" + f.filename)
	f.lg = log.New(logObj.logfile, "\n", log.Ldate|log.Ltime|log.Lshortfile)
}

func fileSize(file string) int64 {
	f, e := os.Stat(file)
	if e != nil {
		fmt.Println(e.Error())
		return 0
	}
	return f.Size()
}

func isExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

func fileMonitor() {
	timer := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-timer.C:
			fileCheck()
		}
	}
}

func fileCheck() {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()
	if logObj != nil && logObj.lg != nil && logObj.isMustRename() {
		logObj.mu.Lock()
		defer logObj.mu.Unlock()
		logObj.rename()
	}
}
