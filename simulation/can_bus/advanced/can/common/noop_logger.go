package common

import (
	"io"
	"log"
)

func NewNoopLogger() *log.Logger {
	return log.New(io.Discard, "", log.LstdFlags)
}
