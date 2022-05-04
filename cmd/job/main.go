package main

import (
	"flag"
	"goim/pkg/logger"
	"os"
	"os/signal"
	"syscall"

	"github.com/bilibili/discovery/naming"
	"goim/internal/job"
	"goim/internal/job/conf"

	resolver "github.com/bilibili/discovery/naming/grpc"
)

var (
	ver = "2.0.0"
)

func main() {
	flag.Parse()
	conf, err := conf.Init()
	if err != nil {
		panic(err)
	}
	logger.Infof("goim-job [version: %s env: %+v] start", ver, conf.Env)
	// grpc register naming
	dis := naming.New(conf.Discovery)
	resolver.Register(dis)
	// job
	j := job.New(&conf)
	// signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		s := <-c
		logger.Infof("goim-job get a signal %s", s.String())
		switch s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			j.Close()
			logger.Infof("goim-job [version: %s] exit", ver)
			return
		case syscall.SIGHUP:
		default:
			return
		}
	}
}
