package subscription

import (
	crand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math/rand"
	"strings"
	"sync"
	"time"
)

var globalGen = randomIDGenerator()

// ID defines a pseudo random number that is used to identify RPC subscriptions.
type ID string

// NewID returns a new, random ID.
func NewID() ID {
	return globalGen()
}

// randomIDGenerator returns a function generates a random IDs.
func randomIDGenerator() func() ID {
	var buf = make([]byte, 8)
	var seed int64
	if _, err := crand.Read(buf); err == nil {
		seed = int64(binary.BigEndian.Uint64(buf))
	} else {
		seed = int64(time.Now().Nanosecond())
	}

	var (
		mu  sync.Mutex
		rng = rand.New(rand.NewSource(seed))
	)
	return func() ID {
		mu.Lock()
		defer mu.Unlock()
		id := make([]byte, 16)
		rng.Read(id)
		return encodeID(id)
	}
}

func encodeID(b []byte) ID {
	id := hex.EncodeToString(b)
	id = strings.TrimLeft(id, "0")
	if id == "" {
		id = "0" // ID's are RPC quantities, no leading zero's and 0 is 0x0.
	}
	return ID("0x" + id)
}

type Subscriber struct {
	Identify ID
	alive    bool
	Mailbox  chan interface{}
	Handler  func(m interface{})
}

func (sub *Subscriber) Subscribe() {
	sub.alive = true
	go func() {
		for sub.alive {
			mail := <-sub.Mailbox
			sub.Handler(mail)
		}
	}()
}

func (sub *Subscriber) Unsubscribe() {
	sub.alive = false
}

type Channel struct {
	Subscribers map[ID]*Subscriber
	ChannelInfo *chan interface{}
}

type Publisher struct {
	content map[string]*Channel
}

//New func
func NewPublisher() *Publisher {
	return &Publisher{
		content: map[string]*Channel{},
	}
}

func (s *Publisher) ExistChannel(topic string) bool {
	_, exist := s.content[topic]
	return exist
}

func (s *Publisher) AddChannelWithSubs(topic string, chanInfo *chan interface{}, subs map[ID]*Subscriber) error {
	if s.ExistChannel(topic) {
		return errors.New("Channel with this topic already exist, topic:" + topic)
	}
	channel := &Channel{
		Subscribers: subs,
		ChannelInfo: chanInfo,
	}
	s.content[topic] = channel
	go func() {
		for {
			message := <-*channel.ChannelInfo
			for _, subscr := range channel.Subscribers {
				subscr.Mailbox <- message
			}
		}
	}()
	return nil
}

func (s *Publisher) AddChannel(topic string, chanInfo *chan interface{}) error {
	return s.AddChannelWithSubs(topic, chanInfo, map[ID]*Subscriber{})
}

func (s *Publisher) RemoveChannel(topic string) error {
	if !s.ExistChannel(topic) {
		return errors.New("Channel with this topic doesn't exist, topic:" + topic)
	}
	close(*s.content[topic].ChannelInfo)
	delete(s.content, topic)
	return nil
}

//AddSubscriber method
func (s *Publisher) AddSubscriber(topic string, sub *Subscriber) error {
	ch, exist := s.content[topic]
	if !exist {
		return errors.New("Channel with this topic doesn't exist, topic:" + topic)
	}
	if _, exist := ch.Subscribers[sub.Identify]; exist {
		return errors.New("Subscriber with ID " + string(sub.Identify) + " already exist")
	}
	ch.Subscribers[sub.Identify] = sub
	return nil
}

//RemoveSubscriber method
func (s *Publisher) RemoveSubscriber(topic string, id ID) error {
	ch, exist := s.content[topic]
	if !exist {
		return errors.New("Channel with this topic doesn't exist, topic:" + topic)
	}
	ch.Subscribers[id].Unsubscribe()
	delete(ch.Subscribers, id)
	return nil
}

//Publish method
func (s *Publisher) Publish(topic string, msg interface{}) error {
	ch, exist := s.content[topic]
	if !exist {
		return errors.New("Channel with this topic doesn't exist, topic:" + topic)
	}
	*ch.ChannelInfo <- msg
	return nil
}
