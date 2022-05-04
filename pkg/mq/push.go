package mq

import "context"

type Pushing interface {
	// PushMsg push a message to databus.
	PushMsg(ctx context.Context, op int32, server string, keys []string, msg []byte) error

	// BroadcastRoomMsg push a message to databus.
	BroadcastRoomMsg(ctx context.Context, op int32, room string, msg []byte) error

	// BroadcastMsg push a message to databus.
	BroadcastMsg(ctx context.Context, op, speed int32, msg []byte) error
}
