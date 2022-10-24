/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/12 14:26
Desc   : 后续不再维护, 建议是直接使用 zap/logrus 即可

    ......................我佛慈悲......................

                           _oo0oo_
                          o8888888o
                          88" . "88
                          (| -_- |)
                          0\  =  /0
                        ___/`---'\___
                      .' \\|     |// '.
                     / \\|||  :  |||// \
                    / _||||| -卍-|||||- \
                   |   | \\\  -  /// |   |
                   | \_|  ''\---/''  |_/ |
                   \  .-\__  '-'  ___/-. /
                 ___'. .'  /--.--\  `. .'___
              ."" '<  `.___\_<|>_/___.' >' "".
             | | :  `- \`.;`\ _ /`;.`/ - ` : | |
             \  \ `_.   \_ __\ /__ _/   .-` /  /
         =====`-.____`.___ \_____/___.-`___.-'=====
                           `=---='

    ..................佛祖保佑, 永无BUG...................

*/

package zlog

import (
    "bytes"
    "context"
    "fmt"
    "go.opentelemetry.io/otel/trace"
    "golang.org/x/xerrors"
    "io"
    "log"
    "os"
    "sync"
)

type (
    LogLevel   int8
    ModelLevel int8

    SetOutput func(io.Writer)

    LogFileManage interface {
        IsCut(SetOutput) error // 是否切割文件
        GetFile() *os.File     // 获取文件操作对象
        FilePath() string      // 文件名
        Close() error          // 关闭文件
    }

    zlog struct {
        sync.Mutex
        logger        *log.Logger
        logLevel      LogLevel
        logFileManage LogFileManage
    }

    Option struct {
        logFileManage LogFileManage
    }

    LogFileManageOption func(*Option)
)

const (
    DEBUG LogLevel = 1 << iota
    INFO
    WARN
    ERROR
    FATAL
    ALLLogLevel = DEBUG | INFO | WARN | ERROR
)

const (
    OnlyStd ModelLevel = iota
    OnlyFile
    AllModelLevel
)

const (
    FmtStandard  = log.Llongfile | log.Ldate | log.Lmicroseconds
    FileStandard = os.O_WRONLY | os.O_CREATE | os.O_APPEND
    prefixName   = "[zlog] "
)

var (
    zlogx       = &zlog{logger: log.New(io.Writer(os.Stdout), prefixName, FmtStandard), logLevel: ALLLogLevel}
    once        sync.Once
    LogLevelMap = map[LogLevel]string{
        DEBUG: "DEBUG",
        INFO:  "INFO",
        WARN:  "WARN",
        ERROR: "ERROR",
        FATAL: "FATAL",
    }
)

func newZlog(logFilePath string, logLevel LogLevel, options []LogFileManageOption, writers ...io.Writer) *zlog {
    option := Option{logFileManage: NewDateCutMode(logFilePath)}
    for _, o := range options {
        o(&option)
    }
    ret := &zlog{
        logLevel:      logLevel,
        logFileManage: option.logFileManage,
    }
    writers = append(writers, ret.logFileManage.GetFile())
    ret.logger = log.New(io.MultiWriter(writers...), prefixName, FmtStandard)
    return ret
}

func WithLogFileManage(logFileManage LogFileManage) LogFileManageOption {
    return func(o *Option) {
        if o.logFileManage != nil {
            if err := o.logFileManage.Close(); err != nil {
                log.Fatalf("关闭文件`%v`失败, err = %v\n", o.logFileManage.FilePath(), err)
            }
        }
        o.logFileManage = logFileManage
    }
}

func InitZlog(logFilePath string, logLevel LogLevel, modelLevel ModelLevel, option ...LogFileManageOption) {
    once.Do(func() {
        switch modelLevel {
        case OnlyStd:
            zlogx.logLevel = logLevel
        case OnlyFile:
            zlogx = newZlog(logFilePath, logLevel, option)
        default:
            zlogx = newZlog(logFilePath, logLevel, option, os.Stdout)
        }
    })
}

func (z *zlog) Close() error {
    return z.logFileManage.Close()
}

func (z *zlog) Debugf(format string, args ...interface{}) {
    if z.logLevel&DEBUG > 0 {
        z.writef(DEBUG, format, args...)
    }
}

func (z *zlog) Infof(format string, args ...interface{}) {
    if z.logLevel&INFO > 0 {
        z.writef(INFO, format, args...)
    }
}

func (z *zlog) Warnf(format string, args ...interface{}) {
    if z.logLevel&WARN > 0 {
        z.writef(WARN, format, args...)
    }
}

func (z *zlog) Errorf(format string, args ...interface{}) {
    if z.logLevel&ERROR > 0 {
        z.writef(ERROR, format, args...)
    }
}

func (z *zlog) Fatalf(format string, args ...interface{}) {
    z.writef(FATAL, format, args...)
    panic(fmt.Sprintf(format, args...))
}

func (z *zlog) Debug(args ...interface{}) {
    if z.logLevel&DEBUG > 0 {
        z.write(DEBUG, args...)
    }
}

func (z *zlog) Info(args ...interface{}) {
    if z.logLevel&INFO > 0 {
        z.write(INFO, args...)
    }
}

func (z *zlog) Warn(args ...interface{}) {
    if z.logLevel&WARN > 0 {
        z.write(WARN, args...)
    }
}

func (z *zlog) Error(args ...interface{}) {
    if z.logLevel&ERROR > 0 {
        z.write(ERROR, args...)
    }
}

func (z *zlog) Fatal(args ...interface{}) {
    z.write(FATAL, args...)
    panic(fmt.Sprintln(args...))
}

func (z *zlog) setOutput(w io.Writer) {
    z.logger.SetOutput(w)
}

func (z *zlog) cut() {
    if z.logFileManage == nil {
        return
    }
    err := z.logFileManage.IsCut(func(writer io.Writer) {
        z.setOutput(writer)
    })
    if err != nil {
        log.Printf("日志切换发生错误, err = %+v", xerrors.Errorf("%w", err))
        Errorf("日志切换发生错误, err = %+v", xerrors.Errorf("%w", err))
    }
}

func (z *zlog) write(loglevel LogLevel, args ...interface{}) {
    z.cut()
    err := z.logger.Output(4, fmt.Sprintln(setPrefix(loglevel, args)...))
    if err != nil {
        log.Printf("日志写入错误, err = %v\n", err)
    }
}

func (z *zlog) writef(loglevel LogLevel, format string, args ...interface{}) {
    z.cut()
    err := z.logger.Output(4, fmt.Sprintf(setPrefixf(loglevel, format), args...))
    if err != nil {
        log.Printf("日志写入错误, err = %v\n", err)
    }
}

func setPrefix(level LogLevel, args []interface{}) []interface{} {
    levelStr := LogLevelMap[level]
    b := bytes.Buffer{}
    b.WriteString("[")
    b.WriteString(levelStr)
    b.WriteString("]")
    t := make([]interface{}, 0, len(args)+1)
    t = append(t, b.String())
    t = append(t, args...)
    return t
}

func setPrefixf(level LogLevel, format string) string {
    levelStr := LogLevelMap[level]
    b := bytes.Buffer{}
    b.WriteString("[")
    b.WriteString(levelStr)
    b.WriteString("] ")
    b.WriteString(format)
    return b.String()
}

func Close() error {
    if zlogx.logFileManage == nil {
        return nil
    }
    return zlogx.logFileManage.Close()
}

func Debugf(format string, args ...interface{}) {
    zlogx.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
    zlogx.Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
    zlogx.Warnf(format, args...)
}

func Errorf(format string, args ...interface{}) {
    zlogx.Errorf(format, args...)
}

func Fatalf(format string, args ...interface{}) {
    zlogx.Fatalf(format, args...)
}

func Debug(args ...interface{}) {
    zlogx.Debug(args...)
}

func Info(args ...interface{}) {
    zlogx.Info(args...)
}

func Warn(args ...interface{}) {
    zlogx.Warn(args...)
}

func Error(args ...interface{}) {
    zlogx.Error(args...)
}

func Fatal(args ...interface{}) {
    zlogx.Fatal(args...)
}

func WithContext(ctx context.Context) (spanId, traceId string) {
    spanContext := trace.SpanContextFromContext(ctx)
    if spanContext.HasSpanID() {
        spanId = spanContext.SpanID().String()
    }
    if spanContext.HasTraceID() {
        traceId = spanContext.TraceID().String()
    }
    fmt.Printf("spanid = %v, traceid = %v\n", spanId, traceId)
    return
}
