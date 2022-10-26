package log

import (
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

const (
	//DEBUG is a constant of string type
	DEBUG = "DEBUG"
	INFO  = "INFO"
	WARN  = "WARN"
	ERROR = "ERROR"
	FATAL = "FATAL"
)

var sugarLogger *zap.SugaredLogger

func InitLogger(writerType, logLevel string, textFormat bool, filePath string, rotateSize, backupCount, maxAge int) {
	// 写入位置
	var writeSyncer zapcore.WriteSyncer
	if writerType == "file" {
		writeSyncer = getFileWriter(filePath, rotateSize, backupCount, maxAge)
	} else {
		writeSyncer = getConsoleWriter()
	}

	// 编码格式
	encoder := getEncoder(textFormat)
	// 需传入Encoder、WriterSyncer、Log Level
	core := zapcore.NewCore(encoder, writeSyncer, getLogLevel(logLevel))
	// 使用zap.New(…)方法来手动传递所有配置
	// 增加 Caller 信息
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	sugarLogger = logger.Sugar()
}

func getLogLevel(logLevel string) zapcore.Level {
	switch logLevel {
	case DEBUG:
		return zapcore.DebugLevel
	case INFO:
		return zapcore.InfoLevel
	case WARN:
		return zapcore.WarnLevel
	case ERROR:
		return zapcore.ErrorLevel
	case FATAL:
		return zapcore.FatalLevel
	default:
		return zapcore.WarnLevel
	}
}

func Sync() {
	sugarLogger.Sync()
}

func Debug(args ...interface{}) {
	sugarLogger.Debug(args)
}

func Info(args ...interface{}) {
	sugarLogger.Info(args)
}

func Warn(args ...interface{}) {
	sugarLogger.Warn(args)
}

func Error(args ...interface{}) {
	sugarLogger.Error(args)
}

func Fatal(args ...interface{}) {
	sugarLogger.Fatal(args)
}

func Debugf(format string, args ...interface{}) {
	sugarLogger.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	sugarLogger.Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
	sugarLogger.Warnf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	sugarLogger.Errorf(format, args...)
}

func Fatalf(format string, args ...interface{}) {
	sugarLogger.Fatalf(format, args...)
}

// 自定义日志格式
func getEncoder(textFormat bool) zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	// ISO8601TimeEncoder 序列化时间。以毫秒为精度的 ISO8601 格式字符串的时间。
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	// CapitalLevelEncoder 将Level序列化为全大写字符串。例如， InfoLevel被序列化为“INFO”。
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	if textFormat {
		return zapcore.NewConsoleEncoder(encoderConfig)
	}
	return zapcore.NewJSONEncoder(encoderConfig)

}

// 将日志写到 test.log 文件中
func getFileWriter(filePath string, rotateSize, backupCount, maxAge int) zapcore.WriteSyncer {
	if maxAge == 0 {
		maxAge = 7
	}
	lumberJackLogger := &lumberjack.Logger{
		Filename:   filePath,    // 日志文件的位置
		MaxSize:    rotateSize,  // 以 MB 为单位
		MaxBackups: backupCount, // 在进行切割之前，日志文件的最大大小（以MB为单位）
		MaxAge:     maxAge,      // 保留旧文件的最大天数
		Compress:   false,       // 是否压缩/归档旧文件
		LocalTime:  true,
	}
	return zapcore.AddSync(lumberJackLogger)
}

func getConsoleWriter() zapcore.WriteSyncer {
	return zapcore.AddSync(os.Stdout)
}
