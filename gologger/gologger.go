package gologger

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type ctxKey string

const ReqIDKey ctxKey = "reqID"

func init() {
	l := NewLogger()
	zerolog.DefaultContextLogger = &l
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		function := ""
		fun := runtime.FuncForPC(pc)
		if fun != nil {
			funName := fun.Name()
			slash := strings.LastIndex(funName, "/")
			if slash > 0 {
				funName = funName[slash+1:]
			}
			function = " " + funName + "()"
		}
		return file + ":" + strconv.Itoa(line) + function
	}
}

func GetEnvOrDefault(env, defaultVal string) string {
	e := os.Getenv(env)
	if e == "" {
		return defaultVal
	} else {
		return e
	}
}

// Makes context.Canceled errors a warn (for when people abandon requests)
func LvlForErr(err error) zerolog.Level {
	if errors.Is(err, context.Canceled) {
		return zerolog.WarnLevel
	}
	return zerolog.ErrorLevel
}

func NewLogger() zerolog.Logger {
	if os.Getenv("LOG_TIME_MS") == "1" {
		// Log with milliseconds
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	} else {
		zerolog.TimeFieldFormat = time.RFC3339Nano
	}

	zerolog.LevelFieldName = GetEnvOrDefault("LOG_LEVEL_KEY", "level")

	zerolog.TimestampFieldName = "time"

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	logger = logger.Hook(CallerHook{})

	if os.Getenv("LOG_JSON") != "1" {
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
	if os.Getenv("LOG_TRACE") == "1" {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	} else if os.Getenv("LOG_INFO") == "1" {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	return logger
}

type CallerHook struct{}

func (h CallerHook) Run(e *zerolog.Event, _ zerolog.Level, _ string) {
	e.Caller(3)
}
