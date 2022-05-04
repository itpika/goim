package logger

import (
	"fmt"
	//"glosku-entry/support/app"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
	"log"
)

var gLogger *zap.Logger

func init() {

	cfg := zap.Config{
		Level: zap.NewAtomicLevel(),
		// Development puts the logger in development mode, which changes the
		// behavior of DPanicLevel and takes stacktraces more liberally.
		Development: true,
		// DisableCaller stops annotating logs with the calling function's file
		// name and line number. By default, all logs are annotated.
		DisableCaller: false, // 打印行号
		// DisableStacktrace completely disables automatic stacktrace capturing. By
		// default, stacktraces are captured for WarnLevel and above logs in
		// development and ErrorLevel and above in production.
		DisableStacktrace: true,
		// Sampling sets a sampling policy. A nil SamplingConfig disables sampling.
		//Sampling: &zap.SamplingConfig{
		//	Initial: 0,
		//	Thereafter: 0,
		//},
		// Encoding sets the logger's encoding. Valid values are "json" and
		// "console", as well as any third-party encodings registered via
		// RegisterEncoder.
		Encoding: "console",
		// EncoderConfig sets options for the chosen encoder. See
		// zapcore.EncoderConfig for details.
		//zap.NewDevelopmentEncoderConfig(),
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "T",
			LevelKey:       "L",
			NameKey:        "N",
			CallerKey:      "C",
			MessageKey:     "M",
			StacktraceKey:  "S",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.FullCallerEncoder,
		},
		// OutputPaths is a list of URLs or file paths to write logging output to.
		// See Open for details.
		OutputPaths: []string{"stdout"},
		// ErrorOutputPaths is a list of URLs to write internal logger errors to.
		// The default is standard error.
		//
		// Note that this setting only affects internal errors; for sample code that
		// sends error-level logs to a different location from info- and debug-level
		// logs, see the package-level AdvancedConfiguration example.
		ErrorOutputPaths: []string{"stdout"},
		// InitialFields is a collection of fields to add to the root logger.
		InitialFields: nil, //map[string]interface{}{},
	}

	var (
		err error
	)

	//gLogger, err = zap.NewDevelopment()
	gLogger, err = cfg.Build()
	if err != nil {
		panic(err)
	}

	gLogger.Debug("logger", zap.Any("cfg", cfg))

	gLogger = gLogger.WithOptions(zap.AddCallerSkip(1))

	//app.OnExit(OnAppExit)
}

func OnAppExit() {
	var err = gLogger.Sync() // flushes buffer, if any
	if err != nil {
		log.Println(err)
	}
}

// Debug logs a message at DebugLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func Debug(msg string, fields ...zap.Field) {
	gLogger.Debug(msg, fields...)
}

// Info logs a message at InfoLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func Info(msg string, fields ...zap.Field) {
	gLogger.Info(msg, fields...)
}

// Warn logs a message at WarnLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func Warn(msg string, fields ...zap.Field) {
	gLogger.Warn(msg, fields...)
}

// Error logs a message at ErrorLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func Error(msg string, fields ...zap.Field) {
	gLogger.Error(msg, fields...)
}

// DPanic logs a message at DPanicLevel. The message includes any fields
// passed at the log site, as well as any fields accumulated on the logger.
//
// If the logger is in development mode, it then panics (DPanic means
// "development panic"). This is useful for catching errors that are
// recoverable, but shouldn't ever happen.
func DPanic(msg string, fields ...zap.Field) {
	gLogger.DPanic(msg, fields...)
}

// Panic logs a message at PanicLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
//
// The logger then panics, even if logging at PanicLevel is disabled.
func Panic(msg string, fields ...zap.Field) {
	gLogger.Panic(msg, fields...)
}

// Fatal logs a message at FatalLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
//
// The logger then calls os.Exit(1), even if logging at FatalLevel is
// disabled.
func Fatal(msg string, fields ...zap.Field) {
	gLogger.Fatal(msg, fields...)
}

func D(fields ...zap.Field) {
	gLogger.Debug("", fields...)
}

func I(fields ...zap.Field) {
	gLogger.Info("", fields...)
}

func W(fields ...zap.Field) {
	gLogger.Warn("", fields...)
}

func E(fields ...zap.Field) {
	gLogger.Error("", fields...)
}

func P(fields ...zap.Field) {
	gLogger.Panic("", fields...)
}
func F(fields ...zap.Field) {
	gLogger.Fatal("", fields...)
}

func Infof(format string, args ...interface{}) {
	gLogger.Info(fmt.Sprintf(format, args...))
}
func Errorf(format string, args ...interface{}) {
	gLogger.Error(fmt.Sprintf(format, args...))
}
func Warnf(format string, args ...interface{}) {
	gLogger.Warn(fmt.Sprintf(format, args...))
}

//------------------------------------------------------------------------------------------------
type RateLogger struct {
	limiter *rate.Limiter
}

func NewRateLogger(r rate.Limit, b int) *RateLogger {
	return &RateLogger{limiter: rate.NewLimiter(r, b)}
}

// Debug logs a message at DebugLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func (s *RateLogger) Debug(msg string, fields ...zap.Field) {
	if !s.limiter.Allow() {
		return
	}
	gLogger.Debug(msg, fields...)
}

// Info logs a message at InfoLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func (s *RateLogger) Info(msg string, fields ...zap.Field) {
	if !s.limiter.Allow() {
		return
	}
	gLogger.Info(msg, fields...)
}

// Warn logs a message at WarnLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func (s *RateLogger) Warn(msg string, fields ...zap.Field) {
	if !s.limiter.Allow() {
		return
	}
	gLogger.Warn(msg, fields...)
}

// Error logs a message at ErrorLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func (s *RateLogger) Error(msg string, fields ...zap.Field) {
	if !s.limiter.Allow() {
		return
	}
	gLogger.Error(msg, fields...)
}

// DPanic logs a message at DPanicLevel. The message includes any fields
// passed at the log site, as well as any fields accumulated on the logger.
//
// If the logger is in development mode, it then panics (DPanic means
// "development panic"). This is useful for catching errors that are
// recoverable, but shouldn't ever happen.
func (s *RateLogger) DPanic(msg string, fields ...zap.Field) {
	if !s.limiter.Allow() {
		return
	}
	gLogger.DPanic(msg, fields...)
}

// Panic logs a message at PanicLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
//
// The logger then panics, even if logging at PanicLevel is disabled.
func (s *RateLogger) Panic(msg string, fields ...zap.Field) {
	if !s.limiter.Allow() {
		return
	}
	gLogger.Panic(msg, fields...)
}

// Fatal logs a message at FatalLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
//
// The logger then calls os.Exit(1), even if logging at FatalLevel is
// disabled.
func (s *RateLogger) Fatal(msg string, fields ...zap.Field) {
	if !s.limiter.Allow() {
		return
	}
	gLogger.Fatal(msg, fields...)
}

func (s *RateLogger) D(fields ...zap.Field) {
	if !s.limiter.Allow() {
		return
	}
	gLogger.Debug("", fields...)
}

func (s *RateLogger) I(fields ...zap.Field) {
	if !s.limiter.Allow() {
		return
	}
	gLogger.Info("", fields...)
}

func (s *RateLogger) W(fields ...zap.Field) {
	if !s.limiter.Allow() {
		return
	}
	gLogger.Warn("", fields...)
}

func (s *RateLogger) E(fields ...zap.Field) {
	if !s.limiter.Allow() {
		return
	}
	gLogger.Error("", fields...)
}

func (s *RateLogger) P(fields ...zap.Field) {
	if !s.limiter.Allow() {
		return
	}
	gLogger.Panic("", fields...)
}
func (s *RateLogger) F(fields ...zap.Field) {
	if !s.limiter.Allow() {
		return
	}
	gLogger.Fatal("", fields...)
}
