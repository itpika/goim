package main

import (
	"context"
	"fmt"
	"github.com/bilibili/discovery/naming"
	resolver "github.com/bilibili/discovery/naming/grpc"
	"goim/internal/comet"
	"goim/internal/comet/conf"
	"goim/internal/comet/grpc"
	"goim/internal/logic/model"
	"goim/pkg/ip"
	"goim/pkg/logger"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	ver   = "2.0.0"
	appid = "goim.comet"
)

func main() {
	conf, err := conf.Init()
	if err != nil {
		panic(err)
	}

	rand.Seed(time.Now().UTC().UnixNano())
	runtime.GOMAXPROCS(runtime.NumCPU())
	logger.Infof("goim-comet [version: %s env: %+v] start", ver, conf.Env)
	dis := naming.New(conf.Discovery)
	resolver.Register(dis)
	// new comet server
	srv := comet.NewServer(&conf)
	if err := comet.InitWhitelist(conf.Whitelist); err != nil {
		panic(err)
	}
	if err := comet.InitTCP(srv, conf.TCP.Bind, runtime.NumCPU()); err != nil {
		panic(err)
	}
	//if err := comet.InitWebsocket(srv, conf.Conf.Websocket.Bind, runtime.NumCPU()); err != nil {
	//	panic(err)
	//}
	//if conf.Conf.Websocket.TLSOpen {
	//	if err := comet.InitWebsocketWithTLS(srv, conf.Conf.Websocket.TLSBind, conf.Conf.Websocket.CertFile, conf.Conf.Websocket.PrivateFile, runtime.NumCPU()); err != nil {
	//		panic(err)
	//	}
	//}
	// new grpc server
	rpcSrv := grpc.New(conf.RPCServer, srv)
	cancel := register(&conf, dis, srv)
	// signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		s := <-c
		logger.Infof("goim-comet get a signal %s", s.String())
		switch s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			if cancel != nil {
				cancel()
			}
			rpcSrv.GracefulStop()
			srv.Close()
			logger.Infof("goim-comet [version: %s] exit", ver)
			return
		case syscall.SIGHUP:
		default:
			return
		}
	}
}

func register(conf *conf.Config, dis *naming.Discovery, srv *comet.Server) context.CancelFunc {
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
			model.MetaWeight:  strconv.FormatInt(env.Weight, 10),
			model.MetaOffline: strconv.FormatBool(env.Offline),
			model.MetaAddrs:   strings.Join(env.Addrs, ","),
		},
	}
	cancel, err := dis.Register(ins)
	if err != nil {
		panic(err)
	}
	// renew discovery metadata
	go func() {
		for {
			var (
				err   error
				conns int
				ips   = make(map[string]struct{})
			)
			for _, bucket := range srv.Buckets() {
				for ip := range bucket.IPCount() {
					ips[ip] = struct{}{}
				}
				conns += bucket.ChannelCount()
			}
			ins.Metadata[model.MetaConnCount] = fmt.Sprint(conns)
			ins.Metadata[model.MetaIPCount] = fmt.Sprint(len(ips))
			if err = dis.Set(ins); err != nil {
				logger.Errorf("dis.Set(%+v) error(%v)", ins, err)
				time.Sleep(time.Second)
				continue
			}
			time.Sleep(time.Second * 10)
		}
	}()
	return cancel
}
