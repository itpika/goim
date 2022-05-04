package logic

import (
	"context"
	"goim/pkg/logger"
	"goim/pkg/mq"
	"strconv"
	"time"

	"github.com/bilibili/discovery/naming"
	"goim/internal/logic/conf"
	"goim/internal/logic/dao"
	"goim/internal/logic/model"
)

const (
	_onlineTick     = time.Second * 10
	_onlineDeadline = time.Minute * 5
)

// Logic struct
type Logic struct {
	c      *conf.Config
	dis    *naming.Discovery
	dao    *dao.Dao
	pusher mq.Pushing
	// online
	totalIPs   int64
	totalConns int64
	roomCount  map[string]int32
	// load balancer
	nodes        []*naming.Instance
	loadBalancer *LoadBalancer
	regions      map[string]string // province -> region
}

// New init
func New(c *conf.Config) (l *Logic) {
	l = &Logic{
		c:            c,
		dao:          dao.New(c),
		dis:          naming.New(c.Discovery),
		pusher:       NewNatsPusher(c.Nats),
		loadBalancer: NewLoadBalancer(),
		regions:      make(map[string]string),
	}

	l.initRegions()
	l.initNodes()
	_ = l.loadOnline()
	go l.onlineproc()
	return l
}

// Ping ping resources is ok.
func (l *Logic) Ping(c context.Context) (err error) {
	return l.dao.Ping(c)
}

// Close close resources.
func (l *Logic) Close() {
	l.dao.Close()
}

func (l *Logic) initRegions() {
	for region, ps := range l.c.Regions {
		for _, province := range ps {
			l.regions[province] = region
		}
	}
}

func (l *Logic) initNodes() {
	res := l.dis.Build("goim.comet")
	event := res.Watch()
	select {
	case _, ok := <-event:
		if ok {
			l.newNodes(res)
		} else {
			panic("discovery watch failed")
		}
	case <-time.After(10 * time.Second):
		logger.Error("discovery start timeout")
	}
	go func() {
		for {
			if _, ok := <-event; !ok {
				return
			}
			l.newNodes(res)
		}
	}()
}

func (l *Logic) newNodes(res naming.Resolver) {
	if zoneIns, ok := res.Fetch(); ok {
		var (
			totalConns int64
			totalIPs   int64
			allIns     []*naming.Instance
		)
		for _, zins := range zoneIns.Instances {
			for _, ins := range zins {
				if ins.Metadata == nil {
					logger.Errorf("node instance metadata is empty(%+v)", ins)
					continue
				}
				offline, err := strconv.ParseBool(ins.Metadata[model.MetaOffline])
				if err != nil || offline {
					logger.Warnf("strconv.ParseBool(offline:%t) error(%v)", offline, err)
					continue
				}
				conns, err := strconv.ParseInt(ins.Metadata[model.MetaConnCount], 10, 32)
				if err != nil {
					logger.Errorf("strconv.ParseInt(conns:%d) error(%v)", conns, err)
					continue
				}
				ips, err := strconv.ParseInt(ins.Metadata[model.MetaIPCount], 10, 32)
				if err != nil {
					logger.Errorf("strconv.ParseInt(ips:%d) error(%v)", ips, err)
					continue
				}
				totalConns += conns
				totalIPs += ips
				allIns = append(allIns, ins)
			}
		}
		l.totalConns = totalConns
		l.totalIPs = totalIPs
		l.nodes = allIns
		l.loadBalancer.Update(allIns)
	}
}

func (l *Logic) onlineproc() {
	for {
		time.Sleep(_onlineTick)
		if err := l.loadOnline(); err != nil {
			logger.Errorf("onlineproc error(%v)", err)
		}
	}
}

func (l *Logic) loadOnline() (err error) {
	var (
		roomCount = make(map[string]int32)
	)
	for _, server := range l.nodes {
		var online *model.Online
		online, err = l.dao.ServerOnline(context.Background(), server.Hostname)
		if err != nil {
			return
		}
		if time.Since(time.Unix(online.Updated, 0)) > _onlineDeadline {
			_ = l.dao.DelServerOnline(context.Background(), server.Hostname)
			continue
		}
		for roomID, count := range online.RoomCount {
			roomCount[roomID] += count
		}
	}
	l.roomCount = roomCount
	return
}

// PushKeys push a message by keys.
func (l *Logic) PushKeys(c context.Context, op int32, keys []string, msg []byte) (err error) {
	servers, err := l.dao.ServersByKeys(c, keys)
	if err != nil {
		return
	}
	pushKeys := make(map[string][]string)
	for i, key := range keys {
		server := servers[i]
		if server != "" && key != "" {
			pushKeys[server] = append(pushKeys[server], key)
		}
	}
	for server := range pushKeys {
		if err = l.pusher.PushMsg(c, op, server, pushKeys[server], msg); err != nil {
			return
		}
	}
	return
}

// PushMids push a message by mid.
func (l *Logic) PushMids(c context.Context, op int32, mids []int64, msg []byte) (err error) {
	keyServers, _, err := l.dao.KeysByMids(c, mids)
	if err != nil {
		return
	}
	keysMap := make(map[string][]string)
	for key, server := range keyServers {
		if key == "" || server == "" {
			logger.Warnf("push key:%s server:%s is empty", key, server)
			continue
		}
		keysMap[server] = append(keysMap[server], key)
	}
	for server, keys := range keysMap {
		if err = l.pusher.PushMsg(c, op, server, keys, msg); err != nil {
			return
		}
	}
	return
}

// PushRoom push a message by room.
func (l *Logic) PushRoom(c context.Context, op int32, typ, room string, msg []byte) (err error) {
	return l.pusher.BroadcastRoomMsg(c, op, model.EncodeRoomKey(typ, room), msg)
}

// PushAll push a message to all.
func (l *Logic) PushAll(c context.Context, op, speed int32, msg []byte) (err error) {
	return l.pusher.BroadcastMsg(c, op, speed, msg)
}
