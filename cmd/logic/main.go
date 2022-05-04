package main

import (
	"context"
	"flag"
	"goim/pkg/logger"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/bilibili/discovery/naming"
	resolver "github.com/bilibili/discovery/naming/grpc"
	"goim/internal/logic"
	"goim/internal/logic/conf"
	"goim/internal/logic/grpc"
	"goim/internal/logic/http"
	"goim/internal/logic/model"
	"goim/pkg/ip"
)

const (
	ver   = "2.0.0"
	appid = "goim.logic"
)

func main() {
	flag.Parse()
	conf, err := conf.Init()
	if err != nil {
		panic(err)
	}
	logger.Infof("goim-logic [version: %s env: %+v] start", ver, conf.Env)
	// grpc register naming
	dis := naming.New(conf.Discovery)
	resolver.Register(dis)
	// logic
	srv := logic.New(&conf)
	httpSrv := http.New(conf.HTTPServer, srv)
	rpcSrv := grpc.New(conf.RPCServer, srv)
	cancel := register(&conf, dis, srv)
	// signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		s := <-c
		logger.Infof("goim-logic get a signal %s", s.String())
		switch s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			if cancel != nil {
				cancel()
			}
			srv.Close()
			httpSrv.Close()
			rpcSrv.GracefulStop()
			logger.Infof("goim-logic [version: %s] exit", ver)
			return
		case syscall.SIGHUP:
		default:
			return
		}
	}
}

func register(conf *conf.Config, dis *naming.Discovery, srv *logic.Logic) context.CancelFunc {
	env := conf.Env
	addr := ip.InternalIP()
	_, port, _ := net.SplitHostPort(conf.RPCServer.Addr)
	ins := &naming.Instance{
		Region:   env.Region,
		Zone:     env.Zone,
		Env:      env.DeployEnv,
		Hostname: env.Host,
		AppID:    appid,
		Addrs: []string{
			"grpc://" + addr + ":" + port,
		},
		Metadata: map[string]string{
			model.MetaWeight: strconv.FormatInt(env.Weight, 10),
		},
	}
	cancel, err := dis.Register(ins)
	if err != nil {
		panic(err)
	}
	return cancel
}
