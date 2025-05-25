package mcp_golang

type Level int

// level2str maps the level integer to the level string
var level2str = map[Level]string{
	LevelNil:       "Nil",
	LevelDebug:     "Debug",
	LevelInfo:      "Info",
	LevelNotice:    "Notice",
	LevelWarning:   "Warning",
	LevelError:     "Error",
	LevelCritical:  "Critical",
	LevelAlert:     "Alert",
	LevelEmergency: "Emergency",
}

// str2Level maps the level string to the level integer
var str2Level = map[string]Level{
	"Nil":       LevelNil,
	"Debug":     LevelDebug,
	"Info":      LevelInfo,
	"Notice":    LevelNotice,
	"Warning":   LevelWarning,
	"Error":     LevelError,
	"Critical":  LevelCritical,
	"Alert":     LevelAlert,
	"Emergency": LevelEmergency,
}

const (
	LevelNil Level = iota
	LevelDebug
	LevelInfo
	LevelNotice
	LevelWarning
	LevelError
	LevelCritical
	LevelAlert
	LevelEmergency
)

type LoggingMessageParams struct {
	Level  string      `json:"level" yaml:"level" mapstructure:"level"`
	Logger string      `json:"logger" yaml:"logger" mapstructure:"logger"`
	Data   interface{} `json:"data" yaml:"data" mapstructure:"data"`
}

func newLoggingMessageParams(level Level, logger string, data interface{}) LoggingMessageParams {
	return LoggingMessageParams{
		Level:  level2str[level],
		Logger: logger,
		Data:   data,
	}
}
