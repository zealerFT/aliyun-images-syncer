package waitutil

import (
	"fmt"
	"log"
	"runtime"
)

// logPanic logs the caller tree when a panic occurs.
func logPanic(r interface{}) {
	callers := getCallers(r)
	if _, ok := r.(string); ok {
		log.Printf("observed a panic: %s\n%v", r, callers)
	} else {
		log.Printf("observed a panic: %#v (%v)\n%v", r, r, callers)
	}
}

func getCallers(r interface{}) string {
	callers := ""
	for i := 0; true; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		callers = callers + fmt.Sprintf("%v:%v\n", file, line)
	}

	return callers
}

// PanicHandlers is a list of functions which will be invoked when a panic happens.
var PanicHandlers = []func(interface{}){logPanic}

// HandleCrash simply catches a crash and logs an error. Meant to be called via
// defer.  Additional context-specific handlers can be provided, and will be
// called in case of panic.  HandleCrash actually crashes, after calling the
// handlers and logging the panic message.
func HandleCrash(additionalHandlers ...func(interface{})) {
	if r := recover(); r != nil {
		for _, fn := range PanicHandlers {
			fn(r)
		}
		for _, fn := range additionalHandlers {
			fn(r)
		}
		// Actually proceed to panic.
		panic(r)
	}
}
