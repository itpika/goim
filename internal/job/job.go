package job

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"goim/pkg/logger"
	"sync"
	"time"

	"github.com/bilibili/discovery/naming"
	"github.com/golang/protobuf/proto"
	pb "goim/api/logic"
	"goim/internal/job/conf"

	cluster "github.com/bsm/sarama-cluster"
)

// Job is push job.
type Job struct {
	c            *conf.Config
	consumer     *cluster.Consumer
	mqConsumer   *nats.Subscription
	cometServers map[string]*Comet

	rooms      map[string]*Room
	roomsMutex sync.RWMutex
}

// New new a push job.
func New(c *conf.Config) *Job {
	j := &Job{
		c: c,
		//consumer: newKafkaSub(c.Kafka),
		mqConsumer: newNatsSub(c.Nats),
		rooms:      make(map[string]*Room),
	}
	j.watchComet(c.Discovery)
	go j.ExecConsume(context.Background())
	return j
}

//[nats]
//stream = "goim-push-topic"
//addr = "127.0.0.1:4222"

func newKafkaSub(c *conf.Kafka) *cluster.Consumer {
	config := cluster.NewConfig()
	config.Consumer.Return.Errors = true
	config.Group.Return.Notifications = true
	consumer, err := cluster.NewConsumer(c.Brokers, c.Group, []string{c.Topic}, config)
	if err != nil {
		panic(err)
	}
	return consumer
}

func newNatsSub(conf *conf.Nats) *nats.Subscription {
	conn, err := nats.Connect(conf.Addr)
	if err != nil {
		panic(err)
	}
	jetStream, err := conn.JetStream()
	subscr, err := jetStream.PullSubscribe(conf.Stream, conf.Sub)
	if err != nil {
		panic(err)
	}
	return subscr
}

// Close close resounces.
func (j *Job) Close() error {
	if j.consumer != nil {
		return j.consumer.Close()
	}
	return nil
}

// Consume messages, watch signals
func (j *Job) ExecConsume(ctx context.Context) {
	for {
		msgs, err := j.mqConsumer.Fetch(1, nats.MaxWait(j.c.Nats.Subtimeout*time.Second))
		if err != nil {
			switch err {
			case nats.ErrTimeout:
				continue
			case context.DeadlineExceeded, context.Canceled:
				// Work out whether it was the JetStream context that expired
				// or whether it was our supplied context.
				select {
				case <-ctx.Done():
					// The supplied context expired, so we want to stop the
					// consumer altogether.
					return
				default:
					// The JetStream context expired, so the fetch probably
					// just timed out and we should try again.
					continue
				}
			default:
				logger.E(zap.Error(err))
				continue
			}
		}
		if len(msgs) < 1 {
			continue
		}
		msg := msgs[0]
		if err = msg.InProgress(); err != nil {
			logger.E(zap.Error(err))
			continue
		}

		// process push message
		pushMsg := new(pb.PushMsg)
		if err := proto.Unmarshal(msg.Data, pushMsg); err != nil {
			logger.Errorf("proto.Unmarshal(%v) error(%v)", msgs[0].Data, err)
			continue
		}
		if err := j.push(context.Background(), pushMsg); err != nil {
			logger.Errorf("j.push(%v) error(%v)", pushMsg, err)
			if err = msg.Nak(); err != nil {
				logger.Warnf("msg.Nak: %w", err)
			}
			continue
		}

		if err = msg.AckSync(); err != nil {
			logger.Warnf("msg.AckSync: %w", err)
		}

		logger.Infof("consume: %s/%s", j.c.Nats.Stream, j.c.Nats.Sub)
	}
}

// Consume messages, watch signals
func (j *Job) Consume() {
	for {
		select {
		case err := <-j.consumer.Errors():
			logger.Errorf("consumer error(%v)", err)
		case n := <-j.consumer.Notifications():
			logger.Infof("consumer rebalanced(%v)", n)
		case msg, ok := <-j.consumer.Messages():
			if !ok {
				return
			}
			j.consumer.MarkOffset(msg, "")
			// process push message
			pushMsg := new(pb.PushMsg)
			if err := proto.Unmarshal(msg.Value, pushMsg); err != nil {
				logger.Errorf("proto.Unmarshal(%v) error(%v)", msg, err)
				continue
			}
			if err := j.push(context.Background(), pushMsg); err != nil {
				logger.Errorf("j.push(%v) error(%v)", pushMsg, err)
			}
			logger.Infof("consume: %s/%d/%d\t%s\t%+v", msg.Topic, msg.Partition, msg.Offset, msg.Key, pushMsg)
		}
	}
}

func (j *Job) watchComet(c *naming.Config) {
	dis := naming.New(c)
	resolver := dis.Build("goim.comet")
	event := resolver.Watch()
	select {
	case _, ok := <-event:
		if !ok {
			panic("watchComet init failed")
		}
		if ins, ok := resolver.Fetch(); ok {
			if err := j.newAddress(ins.Instances); err != nil {
				panic(err)
			}
			logger.Infof("watchComet init newAddress:%+v", ins)
		}
	case <-time.After(10 * time.Second):
		logger.Error("watchComet init instances timeout")
	}
	go func() {
		for {
			if _, ok := <-event; !ok {
				logger.Info("watchComet exit")
				return
			}
			ins, ok := resolver.Fetch()
			if ok {
				if err := j.newAddress(ins.Instances); err != nil {
					logger.Errorf("watchComet newAddress(%+v) error(%+v)", ins, err)
					continue
				}
				logger.Infof("watchComet change newAddress:%+v", ins)
			}
		}
	}()
}

func (j *Job) newAddress(insMap map[string][]*naming.Instance) error {
	ins := insMap[j.c.Env.Zone]
	if len(ins) == 0 {
		return fmt.Errorf("watchComet instance is empty")
	}
	comets := map[string]*Comet{}
	for _, in := range ins {
		if old, ok := j.cometServers[in.Hostname]; ok {
			comets[in.Hostname] = old
			continue
		}
		c, err := NewComet(in, j.c.Comet)
		if err != nil {
			logger.Errorf("watchComet NewComet(%+v) error(%v)", in, err)
			return err
		}
		comets[in.Hostname] = c
		logger.Infof("watchComet AddComet grpc:%+v", in)
	}
	for key, old := range j.cometServers {
		if _, ok := comets[key]; !ok {
			old.cancel()
			logger.Infof("watchComet DelComet:%s", key)
		}
	}
	j.cometServers = comets
	return nil
}
