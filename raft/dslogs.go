package raft

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"
)

type logTopic string

var mu sync.Mutex

const (
	dClient   logTopic = "CLNT"
	dCommit   logTopic = "CMIT"
	dDrop     logTopic = "DROP"
	dError    logTopic = "ERRO"
	dInfo     logTopic = "INFO"
	dLeader   logTopic = "LEAD"
	dLog      logTopic = "LOG1"
	dLog2     logTopic = "LOG2"
	dPersist  logTopic = "PERS"
	dSnap     logTopic = "SNAP"
	dTerm     logTopic = "TERM"
	dTest     logTopic = "TEST"
	dTimer    logTopic = "TIMR"
	dTrace    logTopic = "TRCE"
	dVote     logTopic = "VOTE"
	dWarn     logTopic = "WARN"
	dFollower logTopic = "FLWR"
)

var debugStart time.Time
var debugVerbosity int

// Retrieve the verbosity level from an environment variable
func setVerbosity(l string) error {
	err := os.Setenv("VERBOSE", l)
	if err != nil {
		return err
	}

	return nil
}

// Retrieve the verbosity level from an environment variable
func getVerbosity() int {
	v := os.Getenv("VERBOSE")
	level := 0
	if v != "" {
		var err error
		level, err = strconv.Atoi(v)
		if err != nil {
			log.Fatalf("Invalid verbosity %v", v)
		}
	}
	return level
}

func init() {
	debugVerbosity = getVerbosity()
	debugStart = time.Now()

	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
}

func Debug(topic logTopic, format string, a ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	/**
	debug := getVerbosity()
	if debug >= 1 {
		time := time.Since(debugStart).Microseconds()
		time /= 100
		prefix := fmt.Sprintf("%06d %v ", time, string(topic))
		format = prefix + format
		log.Printf(format, a...)
	}*/

	time := time.Since(debugStart).Microseconds()
	time /= 100
	prefix := fmt.Sprintf("%06d %v ", time, string(topic))
	format = prefix + format
	log.Printf(format, a...)
}
