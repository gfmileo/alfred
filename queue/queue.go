// Package queue abstracts the various external (or internal) message queues we are using for notifications
package queue

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/demisto/alfred/conf"
	"github.com/demisto/alfred/domain"
)

var (
	// ErrTimeout is returned if receive encounters a timeout
	ErrTimeout = errors.New("Timeout occured")
	// ErrClosed is returned if you try to access a closed queue
	ErrClosed = errors.New("Queue is already closed")
)

// ConfigurationMessage including the user and configuration
type ConfigurationMessage struct {
	Team          string
	Configuration domain.Configuration
}

// Queue abstracts the external / internal queues
type Queue interface {
	PushConf(t string, c *domain.Configuration) error
	PopConf(timeout time.Duration) (string, *domain.Configuration, error)
	PushWork(work *domain.WorkRequest) error
	PopWork(timeout time.Duration) (*domain.WorkRequest, error)
	PushWorkReply(replyQueue string, reply *domain.WorkReply) error
	PopWorkReply(replyQueue string, timeout time.Duration) (*domain.WorkReply, error)
	Close() error
}

// New queue is returned depending on environment
func New() (Queue, error) {
	var q Queue
	var err error
	switch {
	case conf.Options.AWS.ID != "":
		// q, err = newSQS()
		return nil, errors.New("AWS SQS not fully functional right now")
	case conf.Options.G.Project != "":
		q, err = newPubSub()
	default:
		q = &queueChannel{
			Conf:         make(chan *ConfigurationMessage, 100),
			Work:         make(chan *domain.WorkRequest, 100),
			WorkReply:    make(chan *domain.WorkReply, 100),
			WebWorkReply: make(chan *domain.WorkReply, 100),
		}
	}
	return q, err
}

type queueChannel struct {
	Conf         chan *ConfigurationMessage
	Work         chan *domain.WorkRequest
	WorkReply    chan *domain.WorkReply
	WebWorkReply chan *domain.WorkReply
	closed       bool
}

func (q *queueChannel) PushConf(t string, c *domain.Configuration) error {
	q.Conf <- &ConfigurationMessage{Team: t, Configuration: *c}
	return nil
}

// Pop a value from the queue - the simple channl implementation ignores timeout
func (q *queueChannel) PopConf(timeout time.Duration) (string, *domain.Configuration, error) {
	conf := <-q.Conf
	// If someone closed the channel
	if conf == nil {
		return "", nil, errors.New("Closed")
	}
	return conf.Team, &conf.Configuration, nil
}

func (q *queueChannel) PushWork(data *domain.WorkRequest) error {
	q.Work <- data
	return nil
}

func (q *queueChannel) PopWork(timeout time.Duration) (*domain.WorkRequest, error) {
	work := <-q.Work
	return work, nil
}

func (q *queueChannel) PushWorkReply(replyQueue string, reply *domain.WorkReply) error {
	if strings.HasSuffix(replyQueue, "-web") {
		q.WebWorkReply <- reply
	} else {
		q.WorkReply <- reply
	}
	return nil
}

func (q *queueChannel) PopWorkReply(replyQueue string, timeout time.Duration) (*domain.WorkReply, error) {
	var work *domain.WorkReply
	if strings.HasSuffix(replyQueue, "-web") {
		work = <-q.WebWorkReply
	} else {
		work = <-q.WorkReply
	}
	return work, nil
}

// ReplyQueueName returns the default name
func ReplyQueueName() (string, error) {
	host, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return "Q" + host, nil
}

func (q *queueChannel) Close() error {
	if !q.closed {
		close(q.Conf)
		close(q.Work)
		close(q.WorkReply)
		close(q.WebWorkReply)
	}
	q.closed = true
	return nil
}
