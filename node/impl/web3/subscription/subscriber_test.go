package subscription

import (
	"fmt"
	"testing"
	"time"

	r "github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SubscriberSuite struct {
	suite.Suite
}

func newSubscriberSuite() *SubscriberSuite {
	return &SubscriberSuite{}
}

type Complex struct {
	age  int
	name string
}

func (suite *SubscriberSuite) TestSubsriberInt() {
	t := suite.T()
	publich := NewPublisher()
	totalScore := 0
	subInt := &Subscriber{
		Identify: NewID(),
		Mailbox:  make(chan interface{}),
		Handler: func(m interface{}) {
			totalScore += m.(int)
		},
	}
	subInt.Subscribe()
	channelInt := make(chan interface{})
	err := publich.AddChannel("int", &channelInt)
	r.NoError(t, err)
	err = publich.AddSubscriber("int", subInt)
	r.NoError(t, err)
	channelInt <- 25
	channelInt <- 35
	channelInt <- 40
	time.Sleep(time.Second)
	r.Equal(t, 100, totalScore)
}

func (suite *SubscriberSuite) TestRemoveSubsriber() {
	t := suite.T()
	publich := NewPublisher()
	totalScore := 0
	subInt := &Subscriber{
		Identify: NewID(),
		Mailbox:  make(chan interface{}),
		Handler: func(m interface{}) {
			totalScore += m.(int)
		},
	}
	subInt.Subscribe()
	channelInt := make(chan interface{})
	err := publich.AddChannel("int", &channelInt)
	r.NoError(t, err)
	err = publich.AddSubscriber("int", subInt)
	r.NoError(t, err)
	err = publich.RemoveSubscriber("int", subInt.Identify)
	r.NoError(t, err)
	subInt = nil
	time.Sleep(time.Second)
	channelInt <- 25
	channelInt <- 35
	channelInt <- 40
	time.Sleep(time.Second)
	r.Equal(t, 0, totalScore)
}

func (suite *SubscriberSuite) TestMultiSubsriber() {
	t := suite.T()
	publich := NewPublisher()
	totalScore := 0
	subInt := &Subscriber{
		Identify: NewID(),
		Mailbox:  make(chan interface{}),
		Handler: func(m interface{}) {
			totalScore += m.(int)
		},
	}
	subInt.Subscribe()
	subIntAnother := &Subscriber{
		Identify: NewID(),
		Mailbox:  make(chan interface{}),
		Handler: func(m interface{}) {
			totalScore += m.(int)
		},
	}
	subIntAnother.Subscribe()
	channelInt := make(chan interface{})
	err := publich.AddChannel("int", &channelInt)
	r.NoError(t, err)
	err = publich.AddSubscriber("int", subInt)
	r.NoError(t, err)
	err = publich.AddSubscriber("int", subIntAnother)
	r.NoError(t, err)
	channelInt <- 25
	channelInt <- 35
	channelInt <- 40
	time.Sleep(time.Second)
	r.Equal(t, 200, totalScore)
}

func (suite *SubscriberSuite) TestCloneSingleSubsriber() {
	t := suite.T()
	publich := NewPublisher()
	totalScore := 0
	subInt := &Subscriber{
		Identify: NewID(),
		Mailbox:  make(chan interface{}),
		Handler: func(m interface{}) {
			totalScore += m.(int)
		},
	}
	subInt.Subscribe()
	channelInt := make(chan interface{})
	err := publich.AddChannel("int", &channelInt)
	r.NoError(t, err)
	cloneCount := 4
	// Add origin subscriber
	err = publich.AddSubscriber("int", subInt)
	r.NoError(t, err)
	// Try to add with same ID
	for i := 0; i < cloneCount; i++ {
		err := publich.AddSubscriber("int", subInt)
		r.Error(t, err)
	}
	// Try to add with unique ID
	for i := 0; i < cloneCount; i++ {
		subInt.Identify = NewID()
		err := publich.AddSubscriber("int", subInt)
		r.NoError(t, err)
	}
	channelInt <- 25
	channelInt <- 35
	channelInt <- 40
	time.Sleep(time.Second)
	r.Equal(t, (cloneCount+1)*100, totalScore)
}

func (suite *SubscriberSuite) TestSubsriberComplex() {
	t := suite.T()
	publich := NewPublisher()
	totalAges := 0
	names := make([]string, 0)
	subComplex := Subscriber{
		Mailbox: make(chan interface{}),
		Handler: func(m interface{}) {
			totalAges += m.(Complex).age
			names = append(names, m.(Complex).name)
		},
	}
	subComplex.Subscribe()
	channelComplex := make(chan interface{})
	err := publich.AddChannel("complex", &channelComplex)
	r.NoError(t, err)
	err = publich.AddSubscriber("complex", &subComplex)
	r.NoError(t, err)
	clientCount := 10
	for i := 0; i < clientCount; i++ {
		channelComplex <- Complex{age: 10, name: fmt.Sprintf("Client %v", i+1)}
	}
	time.Sleep(time.Second)
	r.Equal(t, 100, totalAges)
	for i := 0; i < clientCount; i++ {
		r.Equal(t, fmt.Sprintf("Client %v", i+1), names[i])
	}
}

func (suite *SubscriberSuite) TestSubsribeToUnknownTopic() {
	t := suite.T()
	publich := NewPublisher()
	totalAges := 0
	subComplex := Subscriber{
		Mailbox: make(chan interface{}),
		Handler: func(m interface{}) {
			totalAges += m.(int)
		},
	}
	subComplex.Subscribe()
	err := publich.AddSubscriber("unknown", &subComplex)
	r.Error(t, err)
}

func (suite *SubscriberSuite) TestRewriteChannel() {
	t := suite.T()
	publich := NewPublisher()
	channelComplex := make(chan interface{})
	err := publich.AddChannel("complex", &channelComplex)
	r.NoError(t, err)
	err = publich.AddChannel("complex", &channelComplex)
	r.Error(t, err)
}

func (suite *SubscriberSuite) TestRemoveChannel() {
	t := suite.T()
	publich := NewPublisher()
	channelComplex := make(chan interface{})
	err := publich.AddChannel("complex", &channelComplex)
	r.NoError(t, err)
	err = publich.RemoveChannel("complex111")
	r.Error(t, err)
	err = publich.RemoveChannel("complex")
	r.NoError(t, err)
	err = publich.AddChannel("complex", &channelComplex)
	r.NoError(t, err)
}

func TestMain(t *testing.T) {
	suite.Run(t, newSubscriberSuite())
}
