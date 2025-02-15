package logger

import "github.com/kpango/glg"

var LogChan = make(chan LogPayload)

const (
	LOG_TYPE_ERROR = iota
	LOG_TYPE_DEFAULT
)

type LogPayload struct {
	Type int
	Body interface{}
}

func Error(v ...interface{}) {
	LogChan <- LogPayload{
		Type: LOG_TYPE_ERROR,
		Body: v,
	}
}

func Log(v ...interface{}) {
	LogChan <- LogPayload{
		Type: LOG_TYPE_DEFAULT,
		Body: v,
	}
}

func Watcher() {
	for payload := range LogChan {
		if payload.Type == LOG_TYPE_DEFAULT {
			_ = glg.Log(payload.Body)
		} else {
			_ = glg.Error(payload.Body)
		}
	}
}
