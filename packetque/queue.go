package packetque
// packetque implements a thread-safe queue

import (
	"errors"
	"sync/atomic"

	"github.com/giorgisio/goav/avcodec"
	"github.com/veandco/go-sdl2/sdl"
)

type PacketNode struct {
	packet *avcodec.Packet
	next   *PacketNode
}

type PacketQueue struct {
	first, last *PacketNode
	size        int32 // bytes of all packets
	packetNum   int32
	mutex       *sdl.Mutex
	cond        *sdl.Cond
	stop        atomic.Value   // bool
}

func CreatePacketQueue() (*PacketQueue, error) {
	var err error
	q := &PacketQueue{}
	q.size = 0
	q.packetNum = 0
	q.cond = sdl.CreateCond()
	q.mutex, err = sdl.CreateMutex()
	q.stop.Store(false)
	if err != nil {
		return nil, err
	}
	return q, nil
}

func (q *PacketQueue) Put(packet *avcodec.Packet) error {
	if q == nil {
		return errors.New("queue is nil")
	}
	if packet == nil {
		return errors.New("packet is nil")
	}

	packetNode := &PacketNode{
		packet: packet,
	}

	if err := q.mutex.Lock(); err != nil {
		return err
	}
	// fmt.Printf("Put a packet into queue: %+v\n", packet)
	if q.last != nil {
		q.last.next = packetNode
	} else {
		q.first = packetNode
	}
	q.last = packetNode
	q.packetNum++
	q.size += int32(packet.Size())
	if err := q.cond.Signal(); err != nil {
		return err
	}

	if err := q.mutex.Unlock(); err != nil {
		return err
	}
	return nil
}

// Get return last packet of the queue,
// if needBlock value is false, it will wait util queue non-empty
// if must get one packet, needBlock parameter is needed to be true
// PS: if needBlock is false, the two return value could be nil
func (q *PacketQueue) Get(needBlock bool) (res *avcodec.Packet, err error) {
	if q == nil {
		return nil, errors.New("queue is nil")
	}

	if q.IsStop() {
		return nil, nil
	}

	if err := q.mutex.Lock(); err != nil {
		return nil, err
	}

	defer func() {
		if err = q.mutex.Unlock(); err != nil {
			res = nil
		}
	}()

	for {
		stop := q.stop.Load().(bool)
		if stop {
			return nil, errors.New("queue stopped")
		}
		if q.first != nil {
			res = q.first.packet
			q.packetNum--
			q.size -= int32(res.Size())
			q.first = q.first.next
			if q.first == nil {
				q.last = nil
			}
			// fmt.Println("get a packet from queue")
			break
		} else if !needBlock {
			break
		} else {
			if err = q.cond.Wait(q.mutex); err != nil {
				return nil, err
			}
		}
	}

	return res, err
}

// stop
func (q *PacketQueue) Stop() {
	q.stop.Store(true)
}

func (q *PacketQueue) IsStop() bool {
	return q.stop.Load().(bool)
}