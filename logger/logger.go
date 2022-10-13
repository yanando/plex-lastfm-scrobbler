package logger

import "log"

var Debug bool = false

func LogDebug(msg string, v ...any) {
	if Debug {
		log.Printf("DEBUG | "+msg+"\n", v...)
	}
}

func LogInfo(msg string, v ...any) {
	log.Printf("INFO | "+msg+"\n", v...)
}

func LogError(msg string, v ...any) {
	log.Printf("ERROR | "+msg+"\n", v...)
}
