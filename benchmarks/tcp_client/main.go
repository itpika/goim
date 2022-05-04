package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"goim/api/protocol"
	"goim/pkg/logger"
	"math/rand"
	"net"
	"os"
	"time"
)

const (
	MaxBodySize = int32(1 << 12)
	// size
	PackSize      = 4
	HeaderSize    = 2
	VerSize       = 2
	OpSize        = 4
	SeqSize       = 4
	HeartSize     = 4
	RawHeaderSize = PackSize + HeaderSize + VerSize + OpSize + SeqSize
	MaxPackSize   = MaxBodySize + int32(RawHeaderSize)

	HeaderLenOffsize = PackSize + HeaderSize
	VerOffsize       = HeaderLenOffsize + VerSize
	OpOffsize        = VerOffsize + OpSize
	SeOffsize        = OpOffsize + SeqSize
)

type AuthParams struct {
	Mid      int64   `json:"mid"`
	Key      string  `json:"key"`
	RoomID   string  `json:"room_id"`
	Platform string  `json:"platform"`
	Accepts  []int32 `json:"accepts"`
}

func main() {
	conn, err := GenerateConn()
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	auth(conn)
	go func() {
		if err = heartBeat(conn); err != nil {
			panic(err)
		}
	}()
	for {
		headPack, err := ReadVal(conn, RawHeaderSize)
		if err != nil {
			panic(err)
		}
		packLen, headerLen, ver, op, seq := UnPackHead(headPack)
		if headerLen != RawHeaderSize {
			fmt.Println("error head len", headerLen)
			continue
		}
		bodyLen := packLen - headerLen
		bodyPack, err := ReadVal(conn, bodyLen)

		switch int32(op) {
		case protocol.OpHeartbeatReply:
			onlineNum := binary.BigEndian.Uint32(bodyPack)
			logger.Infof("heartbeat reply, version: %d, seq: %d, room onlineNum: %d", ver, seq, onlineNum)
		case protocol.OpRaw:
			logger.Infof("message receive: seq: %d, op: %d, msg: %s", seq, op, string(bodyPack))
		case accepts[0], accepts[1], accepts[2]:
			logger.Infof("room [%d] receive: seq: %d, msg: %s", op, seq, string(bodyPack))
		}
	}

}

func heartBeat(conn *net.TCPConn) error {
	i := 0
	for {
		headPack := PackHead(0, 1, int(protocol.OpHeartbeat), i)
		n, err := conn.Write(headPack)
		if err != nil {
			return err
		}
		if n != len(headPack) {
			return errors.New("error heartbeat num")
		}
		i++
		time.Sleep(time.Second * 20)
	}
}

var accepts = []int32{1000, 1001, 1002}

func auth(conn *net.TCPConn) {
	ap := AuthParams{
		Mid:      GetUid(),
		RoomID:   "live://1000",
		Platform: "TCP",
		Accepts:  accepts,
	}
	bodyPack, err := json.Marshal(ap)
	if err != nil {
		panic(err)
	}
	headPack := PackHead(len(bodyPack), 1, int(protocol.OpAuth), 1)
	_, err = conn.Write(headPack)
	if err != nil {
		panic(err)
	}
	_, err = conn.Write(bodyPack)
	if err != nil {
		panic(err)
	}
	for {
		headPack, err := ReadVal(conn, RawHeaderSize)
		if err != nil {
			panic(err)
		}
		packLen, headerLen, ver, op, seq := UnPackHead(headPack)
		if headerLen != RawHeaderSize {
			fmt.Println("error head len", headerLen)
			continue
		}
		if op == int(protocol.OpAuthReply) {
			bodyLen := packLen - headerLen
			bodyPack, err = ReadVal(conn, bodyLen)
			if err != nil {
				panic(err)
			}
			logger.Infof("mid: %d, [%s] success auth, version: %d, seq: %d", ap.Mid, string(bodyPack), ver, seq)
			break
		}

	}
}

func PackHead(bodyLen, ver, op, seq int) []byte {
	headPack := make([]byte, RawHeaderSize)
	binary.BigEndian.PutUint32(headPack[0:], uint32(int32(bodyLen)+RawHeaderSize))
	binary.BigEndian.PutUint16(headPack[4:], uint16(RawHeaderSize))
	binary.BigEndian.PutUint16(headPack[6:], uint16(ver))
	binary.BigEndian.PutUint32(headPack[8:], uint32(op))
	binary.BigEndian.PutUint32(headPack[12:], uint32(seq))
	return headPack
}
func UnPackHead(headPack []byte) (packLen, headerLen, ver, op, seq int) {
	packLen = int(binary.BigEndian.Uint32(headPack[0:PackSize]))
	headerLen = int(binary.BigEndian.Uint16(headPack[PackSize:HeaderLenOffsize]))
	ver = int(binary.BigEndian.Uint16(headPack[HeaderLenOffsize:VerOffsize]))
	op = int(binary.BigEndian.Uint32(headPack[VerOffsize:OpOffsize]))
	seq = int(binary.BigEndian.Uint32(headPack[OpOffsize:SeOffsize]))
	return
}

func GenerateConn() (*net.TCPConn, error) {
	rAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:"+os.Args[1])
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTCP("tcp", nil, rAddr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

var GetUid = func() func() int64 {
	rand.Seed(int64(time.Now().Nanosecond()))
	return func() int64 {
		b := make([]byte, 6)
		rand.Read(b)
		return int64(binary.BigEndian.Uint16(b))
	}
}()

func ReadVal(conn *net.TCPConn, num int) ([]byte, error) {
	if num == 0 {
		return []byte{}, nil
	}
	bs := make([]byte, num)
	var (
		start   = 0
		end     = num
		readOff = 0
	)
	for {
		n, err := conn.Read(bs[start:end])
		if err != nil {
			return nil, err
		}
		if readOff += n; readOff == num {
			return bs, nil
		}
		start += n
	}
	return bs, nil
}
