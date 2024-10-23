package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	TRACE   = 4
	INFO    = 3
	WARNING = 2
	ERROR   = 1
	OFF     = 0
)

var logLevel = TRACE
var myLogger = log.New(os.Stdout, "", 0)

func GetLevelFromString(str string) int {
	switch str {
	case "TRACE":
		return TRACE
	case "INFO":
		return INFO
	case "WARNING":
		return WARNING
	case "ERROR":
		return ERROR
	case "OFF":
		return OFF
	default:
		return TRACE
	}
}

func SetLogLevel(level int) {
	logLevel = level
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.SetPrefix("[DEFAULT] ")
	if level == TRACE {
		log.SetOutput(os.Stderr)
	} else {
		log.SetOutput(io.Discard)
	}
}

func LogStack(params ...interface{}) {
	pc := make([]uintptr, 10)
	n := runtime.Callers(0, pc)
	if n == 0 {
		// No PCs available. This can happen if the first argument to
		// runtime.Callers is large.
		//
		// Return now to avoid processing the zero Frame that would
		// otherwise be returned by frames.Next below.
		return
	}

	pc = pc[:n] // pass only valid pcs to runtime.CallersFrames
	frames := runtime.CallersFrames(pc)

	// Loop to get frames.
	// A fixed number of PCs can expand to an indefinite number of Frames.
	fmt.Printf("[ERROR] %s - ", time.Now().Format("2006.01.02 15:04:05"))
	fmt.Print(params...)
	fmt.Print("\n")
	idx := 0
	for {
		frame, more := frames.Next()

		// Process this frame.
		//
		// To keep this example's output stable
		// even if there are changes in the testing package,
		// stop unwinding when we leave package runtime.
		// if !strings.Contains(frame.File, "runtime/") {
		// 	break
		// }
		if idx > 1 {
			fmt.Printf("\t %s %s:%d\n", frame.File, frame.Function, frame.Line)
		}
		idx++
		// Check whether there are more frames to process after this one.
		if !more {
			break
		}
	}
}

func logBase(prefix string, params []interface{}) {
	_, path, line, _ := runtime.Caller(2)
	filename := filepath.Base(path)
	formattedParams := []interface{}{
		"[" + prefix + "]",
		time.Now().Format("2006.01.02 15:04:05"),
		fmt.Sprintf("%s:%d#%d", filename, line, goid()),
	}
	myLogger.Println(append(formattedParams, params...)...)
}

func LogTrace(params ...interface{}) {
	if logLevel >= TRACE {
		logBase("TRACE", params)
	}
}

func LogWarning(params ...interface{}) {
	if logLevel >= WARNING {
		logBase("WARNING", params)
	}
}

func LogInfo(params ...interface{}) {
	if logLevel >= INFO {
		logBase("INFO", params)
	}
}

func LogError(params ...interface{}) {
	if logLevel >= ERROR {
		logBase("ERROR", params)
	}
}

func logBasef(prefix string, allParams []interface{}) {
	format, _ := allParams[0].(string)
	params := allParams[1:]
	_, path, line, _ := runtime.Caller(2)
	filename := filepath.Base(path)
	formattedParams :=
		"[" + prefix + "] " +
			time.Now().Format("2006.01.02 15:04:05") +
			fmt.Sprintf(" %s:%d#%d ", filename, line, goid()) +
			format

	myLogger.Printf(formattedParams, params...)
}

func LogTracef(params ...interface{}) {
	if logLevel >= TRACE {
		logBasef("TRACE", params)
	}
}

func LogWarningf(params ...interface{}) {
	if logLevel >= WARNING {
		logBasef("WARNING", params)
	}
}

func LogInfof(params ...interface{}) {
	if logLevel >= INFO {
		logBasef("INFO", params)
	}
}

func LogErrorf(params ...interface{}) {
	if logLevel >= ERROR {
		logBasef("ERROR", params)
	}
}

func goid() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
	}
	return id
}
