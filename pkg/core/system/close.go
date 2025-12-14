package system

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	closes = []func(){}
	mu     = sync.Mutex{}
)

func RegisterClose(f func()) {
	mu.Lock()
	defer mu.Unlock()

	closes = append(closes, f)
}

func init() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGSEGV)

	go func() {
		<-ch
		for _, f := range closes {
			f()
		}
		os.Exit(0)
	}()
}
