package common

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger

func init() {
	Log = logrus.New()

	// 设置日志格式为 JSON
	Log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// 设置日志输出到文件
	logFile := filepath.Join("logs", "app.log")
	if err := os.MkdirAll("logs", 0755); err != nil {
		Log.Fatal("无法创建日志目录:", err)
	}

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		Log.Fatal("无法打开日志文件:", err)
	}

	// 同时输出到文件和控制台
	Log.SetOutput(file)
	Log.AddHook(&ConsoleHook{})

	// 设置默认日志级别
	Log.SetLevel(logrus.InfoLevel)
}

// ConsoleHook 用于同时输出到控制台
type ConsoleHook struct{}

func (hook *ConsoleHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (hook *ConsoleHook) Fire(entry *logrus.Entry) error {
	line, err := entry.String()
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write([]byte(line))
	return err
}

// SetLogLevel 设置日志级别
func SetLogLevel(level string) {
	switch level {
	case "debug":
		Log.SetLevel(logrus.DebugLevel)
	case "info":
		Log.SetLevel(logrus.InfoLevel)
	case "warn":
		Log.SetLevel(logrus.WarnLevel)
	case "error":
		Log.SetLevel(logrus.ErrorLevel)
	default:
		Log.SetLevel(logrus.InfoLevel)
	}
}

// 添加调用者信息的钩子
type CallerHook struct{}

func (hook *CallerHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (hook *CallerHook) Fire(entry *logrus.Entry) error {
	if pc, file, line, ok := runtime.Caller(6); ok {
		entry.Data["file"] = filepath.Base(file)
		entry.Data["line"] = line
		entry.Data["func"] = runtime.FuncForPC(pc).Name()
	}
	return nil
} 