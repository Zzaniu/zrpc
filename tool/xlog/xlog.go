// TODO 并发有问题: 1. 写入没有加锁   2. 切割文件没有加锁

package xlog

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

type LogLevel int16

const (
	UNKNOW LogLevel = iota
	DEBUG
	INFO
	WARN
	ERROR
	FATAL
)

var XLog *Xlog

type Xlog struct {
	Level   LogLevel
	fileObj *os.File
	utils   *Utils
}

func NewXlog(logLevel string) *Xlog {
	u := &Utils{}
	lx := &Xlog{
		Level: u.getLevel(logLevel),
	}
	if lx.Level > DEBUG {
		lx.fileObj = u.getFileObj()
	}
	return lx
}

func (lx *Xlog) wlog(curr_f *os.File, msg string) {
	nows := time.Now().Format("2006-01-02")
	f := lx.utils.reOpenFile(curr_f, nows)
	if f != nil {
		curr_f = f
	}
	fmt.Fprintln(curr_f, msg)
}

func (lx *Xlog) logf(logl LogLevel, format string, args ...interface{}) {
	if logl >= lx.Level {
		msg := lx.utils.getLogfMsg(logl, format, args...)

		if lx.Level > DEBUG {
			lx.wlog(lx.fileObj, msg)
		} else {
			fmt.Printf(msg)
		}
		// 是否需要退出
		if logl == FATAL {
			panic(msg)
		}
	}
}

func (lx *Xlog) log(logl LogLevel, args ...interface{}) {
	if logl >= lx.Level {
		msg := lx.utils.getLogMsg(logl, args...)

		if lx.Level > DEBUG {
			lx.wlog(lx.fileObj, msg)
		} else {
			fmt.Println(msg)
		}
		// 是否需要退出
		if logl == FATAL {
			panic(msg)
		}
	}
}

func (lx *Xlog) Close() {
	defer func() {
		recover()
	}()
	if lx.Level > DEBUG {
		lx.fileObj.Close()
	}
}

// ---------------------------------------------------------------log fn

func (lx *Xlog) Debugf(format string, args ...interface{}) {
	lx.logf(DEBUG, format, args...)
}

func (lx *Xlog) Infof(format string, args ...interface{}) {
	lx.logf(INFO, format, args...)
}

func (lx *Xlog) Warnf(format string, args ...interface{}) {
	lx.logf(WARN, format, args...)
}

func (lx *Xlog) Errorf(format string, args ...interface{}) {
	lx.logf(ERROR, format, args...)
}

func (lx *Xlog) Fatalf(format string, args ...interface{}) {
	lx.logf(FATAL, format, args...)
}

func (lx *Xlog) Debug(args ...interface{}) {
	lx.log(DEBUG, args...)
}

func (lx *Xlog) Info(args ...interface{}) {
	lx.log(INFO, args...)
}

func (lx *Xlog) Warn(args ...interface{}) {
	lx.log(WARN, args...)
}

func (lx *Xlog) Error(args ...interface{}) {
	lx.log(ERROR, args...)
}

func (lx *Xlog) Fatal(args ...interface{}) {
	lx.log(FATAL, args...)
}

// -------------------------------------- utils func()

type Utils struct {
}

func (u *Utils) getLevel(levels string) LogLevel {
	lowLevals := strings.ToLower(levels)
	m := map[string]LogLevel{
		"debug": 1,
		"info":  2,
		"warn":  3,
		"error": 4,
		"fatal": 5,
	}
	ll, ok := m[lowLevals]
	if ok {
		return ll
	}
	msg := fmt.Sprintf("NewXlog()中参数的日志级别: %v不存在, 只能在debug, info, warn, error, fatal中选择一个!", levels)
	panic(msg)
}

func (u *Utils) getLevelByIdx(idx LogLevel) string {
	m := map[LogLevel]string{
		1: "debug",
		2: "info",
		3: "warn",
		4: "error",
		5: "fatal",
	}
	return m[idx]
}

func (u *Utils) createPath(path string) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(path, os.ModePerm)
		}
	}
}

func (u *Utils) getFileObj() (f *os.File) {
	_, mfile, _, ok := runtime.Caller(2)
	if ok {
		farr := strings.Split(mfile, "/")
		farr = farr[:len(farr)-1]
		logFileName := time.Now().Format("2006-01-02")

		farr = append(farr, "logs")
		u.createPath(strings.Join(farr, "/"))
		farr = append(farr, logFileName+".log")

		fullPath := strings.Join(farr, "/")
		fObj, err := os.OpenFile(fullPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
		if err != nil {
			panic(err)
		}
		f = fObj
	}
	return
}

func (u *Utils) reOpenFile(f *os.File, name string) (fo *os.File) {
	farr := strings.Split(f.Name(), "/")
	name = name + ".log"
	if farr[len(farr)-1] != name {
		f.Close()
		farr[len(farr)-1] = name
		fullPath := strings.Join(farr, "/")
		fObj, err := os.OpenFile(fullPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
		if err != nil {
			// panic(err)
			fmt.Println("重新打开新的日志文件错误!")
		}
		fo = fObj
	}
	return
}

func (u *Utils) getWhere(skip int) string {
	pc, file, line, ok := runtime.Caller(skip)
	if ok {
		// fn = strings.Split(fn, ".")[1]
		fn := runtime.FuncForPC(pc).Name()
		fns := strings.Split(fn, ".")
		fn = fns[len(fns)-1]

		return fmt.Sprintf("%v %v():%v", file, fn, line)
	}
	return ""
}

func (u *Utils) getLogfMsg(logl LogLevel, format string, args ...interface{}) string {
	msg := fmt.Sprintf(format, args...)
	levels := u.getLevelByIdx(logl)
	msg = fmt.Sprintf("[%v][%v][%v] %v", time.Now().Format("20060102 15:04:05"), levels, u.getWhere(4), msg)
	return msg
}

func (u *Utils) getLogMsg(logl LogLevel, args ...interface{}) string {
	msg := fmt.Sprintf("%v", args)
	r := []rune(msg)
	msg = string(r[1 : len(r)-1])
	levels := u.getLevelByIdx(logl)
	msg = fmt.Sprintf("[%v][%v][%v] %v", time.Now().Format("20060102 15:04:05"), levels, u.getWhere(4), msg)
	return msg
}

func init() {
	XLog = NewXlog("info")
}
