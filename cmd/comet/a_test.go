package main

import (
	"encoding/binary"
	"fmt"
	"github.com/nats-io/nats.go"
	"math/rand"
	"net"
	"os"
	"testing"
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

func Test_B(t *testing.T) {
	conn, err := nats.Connect("127.0.0.1:4222")
	if err != nil {
		panic(err)
	}
	jetStream, err := conn.JetStream()
	streamInfo, err := jetStream.AddStream(&nats.StreamConfig{
		Name:      "pika",
		Retention: nats.LimitsPolicy,
		Storage:   nats.MemoryStorage,
	})

	go func() {
		for i := 0; i < 10; i++ {
			ack, err := jetStream.Publish(streamInfo.Config.Name, []byte(fmt.Sprintf("kma-%d", i)))
			if err != nil {
				panic(err)
			}
			//fmt.Printf("send [%v] to stream: %v, sequence: %v\n", fmt.Sprintf("kma-%d", i), ack.Stream, ack.Sequence)
			ack.Domain = ""
			time.Sleep(time.Second)
		}
		os.Exit(1)
	}()

	sub, err := jetStream.PullSubscribe(streamInfo.Config.Name, "get1")
	go func() {
		for {
			msgs, err := sub.Fetch(1, nats.MaxWait(1*time.Hour))
			if err != nil {
				panic(err)
			}
			if len(msgs) < 1 {
				continue
			}
			msgs[0].InProgress()
			fmt.Println("consumer 1 receive :", string(msgs[0].Data))
			if err := msgs[0].AckSync(); err != nil {
				panic(err)
			}

		}
	}()

	sub2, err := jetStream.SubscribeSync(streamInfo.Config.Name, nats.Durable("get1"), nats.MaxDeliver(3))
	if err != nil {

		panic(err)
	}
	go func() {
		for {
			msg, err := sub2.NextMsg(1 * time.Hour)
			if err != nil {
				panic(err)
			}
			msg.InProgress()
			fmt.Println("consumer 2 receive :", string(msg.Data))
			if err := msg.AckSync(); err != nil {
				panic(err)
			}
		}
	}()
	for {
	}

}

var GetUid = func() func() int64 {
	rand.Seed(int64(time.Now().Nanosecond()))
	return func() int64 {
		b := make([]byte, 6)
		rand.Read(b)
		return int64(binary.BigEndian.Uint16(b))
	}
}()

func TestO(t *testing.T) {
	addr, _ := net.ResolveTCPAddr("tcp", ":7777")
	go func() {
		listen, err := net.ListenTCP("tcp", addr)
		if err != nil {
			panic(err)
		}
		tcpCon, err := listen.AcceptTCP()
		if err != nil {
			panic(err)
		}
		for {
			tcpCon.Write([]byte("pika"))
			time.Sleep(2 * time.Second)
		}
	}()

	go func() {
		conn, err := net.DialTCP("tcp", nil, addr)
		if err != nil {
			panic(err)
		}
		for {
			bs, err := ReadVal(conn, 3)
			if err != nil {
				panic(err)
			}
			//fmt.Printf("cli read: n %d, val %s \n", n, string(bs[:n]))
			fmt.Printf("cli read: val %s \n", string(bs))
		}
	}()

	for {

	}
}

func ReadVal(conn *net.TCPConn, num int) ([]byte, error) {
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

func TestName(t *testing.T) {
}
