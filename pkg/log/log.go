// Package log 提供基于 zerolog 的结构化日志：支持文件轮转、
// 多输出与 context 级字段注入。
package log

import (
	"context"
	"io"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/samber/do/v2"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

// ctxLoggerKey 是请求级日志器在 context 中的键。
const ctxLoggerKey = "zerologger"

// consoleTimeFormat 是控制台输出的时间格式。
const consoleTimeFormat = "2006-01-02 15:04:05.000000000"

// Logger 包装 zerolog，支持通过 context 传递请求级字段。
type Logger struct {
	zerolog.Logger
}

// Package registers the log provider into the injector.
var Package = do.Package(do.Lazy(New))

// New 按配置构造日志器：级别、输出目标（console/file/both）、
// 编码与轮转参数；非 prod 环境附带调用位置。
func New(i do.Injector) (*Logger, error) {
	conf := do.MustInvoke[*viper.Viper](i)
	level, err := zerolog.ParseLevel(conf.GetString("log.log_level"))
	if err != nil {
		level = zerolog.InfoLevel
	}

	hook := lumberjack.Logger{
		Filename:   conf.GetString("log.log_file_name"), // Log file path
		MaxSize:    conf.GetInt("log.max_size"),         // Maximum size unit for each log file: M
		MaxBackups: conf.GetInt("log.max_backups"),      // The maximum number of backups that can be saved for log files
		MaxAge:     conf.GetInt("log.max_age"),          // Maximum number of days the file can be saved
		Compress:   conf.GetBool("log.compress"),        // Compression or not
	}

	// default(both) log to console and file
	var dest io.Writer = io.MultiWriter(os.Stdout, &hook)
	switch conf.GetString("log.mode") {
	case "console":
		dest = os.Stdout
	case "file":
		dest = &hook
	}

	var out io.Writer = dest
	if conf.GetString("log.encoding") == "console" {
		out = zerolog.ConsoleWriter{Out: dest, TimeFormat: consoleTimeFormat}
	}

	logger := zerolog.New(out).With().Timestamp().Logger().Level(level)
	if conf.GetString("env") != "prod" {
		logger = logger.With().Caller().Logger()
	}
	return &Logger{logger}, nil
}

// WithValue adds a field to the logger stored in the specified context
// and returns a context carrying the enriched logger.
func (l *Logger) WithValue(ctx context.Context, key string, value any) context.Context {
	logger := l.contextLogger(ctx).With().Interface(key, value).Logger()
	if c, ok := ctx.(*gin.Context); ok {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxLoggerKey, &logger))
		return c
	}
	return context.WithValue(ctx, ctxLoggerKey, &logger)
}

// WithContext returns a logger enriched by values previously attached via WithValue.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	return &Logger{Logger: *l.contextLogger(ctx)}
}

// contextLogger 取出 ctx 中注入的请求级日志器，缺省回退到实例自身。
func (l *Logger) contextLogger(ctx context.Context) *zerolog.Logger {
	if c, ok := ctx.(*gin.Context); ok {
		ctx = c.Request.Context()
	}
	if ctxLogger, ok := ctx.Value(ctxLoggerKey).(*zerolog.Logger); ok {
		return ctxLogger
	}
	return &l.Logger
}
