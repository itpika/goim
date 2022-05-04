package job

import (
	"context"
	"fmt"
	"goim/pkg/logger"

	"goim/api/comet"
	pb "goim/api/logic"
	"goim/api/protocol"
	"goim/pkg/bytes"
)

func (j *Job) push(ctx context.Context, pushMsg *pb.PushMsg) (err error) {
	switch pushMsg.Type {
	case pb.PushMsg_PUSH:
		err = j.pushMsgToKeys(pushMsg)
	case pb.PushMsg_ROOM:
		err = j.getRoom(pushMsg.Room).Push(pushMsg.Operation, pushMsg.Msg)
	case pb.PushMsg_BROADCAST:
		err = j.broadcast(pushMsg.Operation, pushMsg.Msg, pushMsg.Speed)
	default:
		err = fmt.Errorf("no match push type: %s", pushMsg.Type)
	}
	return
}

// pushKeys push a message to a batch of subkeys.
func (j *Job) pushMsgToKeys(msg *pb.PushMsg) (err error) {
	buf := bytes.NewWriterSize(len(msg.Msg) + 64)
	p := &protocol.Proto{
		Ver:  1,
		Op:   protocol.OpRaw,
		Body: msg.Msg,
	}
	p.WriteTo(buf)
	p.Body = buf.Buffer()
	//p.Op = protocol.OpRaw
	var args = comet.PushMsgReq{
		Keys:    msg.Keys,
		ProtoOp: msg.Operation,
		Proto:   p,
	}
	if c, ok := j.cometServers[msg.Server]; ok {
		if err = c.Push(&args); err != nil {
			logger.Errorf("c.Push(%v) serverID:%s error(%v)", args, msg.Server, err)
		}
		logger.Infof("pushKey:%s comets:%d", msg.Server, len(j.cometServers))
	}
	return
}

// broadcast broadcast a message to all.
func (j *Job) broadcast(operation int32, body []byte, speed int32) (err error) {
	buf := bytes.NewWriterSize(len(body) + 64)
	p := &protocol.Proto{
		Ver:  1,
		Op:   operation,
		Body: body,
	}
	p.WriteTo(buf)
	p.Body = buf.Buffer()
	p.Op = protocol.OpRaw
	comets := j.cometServers
	speed /= int32(len(comets))
	var args = comet.BroadcastReq{
		ProtoOp: operation,
		Proto:   p,
		Speed:   speed,
	}
	for serverID, c := range comets {
		if err = c.Broadcast(&args); err != nil {
			logger.Errorf("c.Broadcast(%v) serverID:%s error(%v)", args, serverID, err)
		}
	}
	logger.Infof("broadcast comets:%d", len(comets))
	return
}

// broadcastRoomRawBytes broadcast aggregation messages to room.
func (j *Job) broadcastRoomRawBytes(roomID string, body []byte) (err error) {
	args := comet.BroadcastRoomReq{
		RoomID: roomID,
		Proto: &protocol.Proto{
			Ver:  1,
			Op:   protocol.OpRaw,
			Body: body,
		},
	}
	comets := j.cometServers
	for serverID, c := range comets {
		if err = c.BroadcastRoom(&args); err != nil {
			logger.Errorf("c.BroadcastRoom(%v) roomID:%s serverID:%s error(%v)", args, roomID, serverID, err)
		}
	}
	logger.Infof("broadcastRoom comets:%d", len(comets))
	return
}
