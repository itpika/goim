package logic

import (
	"context"
	"github.com/golang/protobuf/proto"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	pb "goim/api/logic"
	"goim/internal/logic/conf"
	"goim/pkg/logger"
)

type NatsPusher struct {
	cfg *conf.Nats
	js  nats.JetStreamContext
}

func NewNatsPusher(conf *conf.Nats) *NatsPusher {
	var np = &NatsPusher{
		cfg: conf,
	}

	conn, err := nats.Connect(conf.Addr)
	if err != nil {
		panic(err)
	}
	np.js, err = conn.JetStream()
	info, err := np.js.StreamInfo(conf.Stream)
	if err != nil && err != nats.ErrStreamNotFound {
		panic(err)
	}
	if info != nil {
		return np
	}
	cfg := &nats.StreamConfig{
		Name:      conf.Stream,
		Retention: nats.LimitsPolicy,
		Storage:   nats.FileStorage,
	}
	if conf.InMemory {
		cfg.Storage = nats.MemoryStorage
	}
	_, err = np.js.AddStream(cfg)
	if err != nil {
		panic(err)
	}

	return np
}

// PushMsg push a message to databus.
func (np *NatsPusher) PushMsg(ctx context.Context, op int32, server string, keys []string, msg []byte) error {
	pushMsg := &pb.PushMsg{
		Type:      pb.PushMsg_PUSH,
		Operation: op,
		Server:    server,
		Keys:      keys,
		Msg:       msg,
	}
	b, err := proto.Marshal(pushMsg)
	if err != nil {
		return err
	}
	if _, err = np.js.Publish(np.cfg.Stream, b); err != nil {
		logger.Errorf("PushMsg error", zap.Error(err))
		return err
	}
	return nil
}

// BroadcastRoomMsg push a message to databus.
func (np *NatsPusher) BroadcastRoomMsg(ctx context.Context, op int32, room string, msg []byte) error {
	pushMsg := &pb.PushMsg{
		Type:      pb.PushMsg_ROOM,
		Operation: op,
		Room:      room,
		Msg:       msg,
	}
	b, err := proto.Marshal(pushMsg)
	if err != nil {
		return err
	}
	if _, err = np.js.Publish(np.cfg.Stream, b); err != nil {
		logger.Errorf("BroadcastRoomMsg error", zap.Error(err))
		return err
	}
	return nil
}

// BroadcastMsg push a message to databus.
func (np *NatsPusher) BroadcastMsg(ctx context.Context, op, speed int32, msg []byte) error {
	pushMsg := &pb.PushMsg{
		Type:      pb.PushMsg_BROADCAST,
		Operation: op,
		Speed:     speed,
		Msg:       msg,
	}
	b, err := proto.Marshal(pushMsg)
	if err != nil {
		return err
	}
	if _, err = np.js.Publish(np.cfg.Stream, b); err != nil {
		logger.Errorf("BroadcastMsg error", zap.Error(err))
		return err
	}
	return nil
}
