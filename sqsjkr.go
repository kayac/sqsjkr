package sqsjkr

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/kayac/sqsjkr/lock"
	"github.com/kayac/sqsjkr/throttle"
)

// Global variables
var (
	logger Logger
)

// TrapSignals list
var TrapSignals = []os.Signal{
	syscall.SIGHUP,
	syscall.SIGINT,
	syscall.SIGQUIT,
	syscall.SIGTERM,
}

// DefaultSQSJkr default
type DefaultSQSJkr struct {
	SQS             *sqs.SQS
	RetentionPeriod time.Duration
	qURL            string
	recvParams      *sqs.ReceiveMessageInput
	jobs            chan Job
	locker          lock.Locker
	throttler       throttle.Throttler
	conf            *Config
}

// StatsItem struct
type StatsItem struct {
	IdleWorkerNum uint32 `json:"idle_worker"`
	BusyWorkerNum uint32 `json:"busy_worker"`
}

// Stats represents stats.
type Stats struct {
	Workers struct {
		Busy int64 `json:"busy"`
		Idle int64 `json:"idle"`
	} `json:"workers"`
	Invocations struct {
		Succeeded int64 `json:"succeeded"`
		Failed    int64 `json:"failed"`
		Errored   int64 `json:"errored"`
	} `json:"invocations"`

	busy chan struct{}
}

// SQSJkr interfaces
type SQSJkr interface {
	Run(context.Context) error
	Config() *Config
	JobStream() chan Job
	Locker() lock.Locker
	Throttler() throttle.Throttler
	SetLocker(locker lock.Locker)
	SetThrottler(throttle throttle.Throttler)
}

// Run sqsjkr daemon
func (sjkr *DefaultSQSJkr) Run(ctx context.Context) error {

	sjkr.recvParams = &sqs.ReceiveMessageInput{
		QueueUrl:              aws.String(sjkr.qURL),
		MaxNumberOfMessages:   aws.Int64(MaxRetrieveMessageNum),
		VisibilityTimeout:     aws.Int64(VisibilityTimeout),
		WaitTimeSeconds:       aws.Int64(WaitTimeSec),
		MessageAttributeNames: aws.StringSlice([]string{"All"}),
		AttributeNames:        aws.StringSlice([]string{"All"}),
	}

	err := sjkr.receiveMessage(ctx)

	return err
}

// Config return SQSJkr config
func (sjkr *DefaultSQSJkr) Config() *Config {
	return sjkr.conf
}

// JobStream return Job chan.
func (sjkr *DefaultSQSJkr) JobStream() chan Job {
	return sjkr.jobs
}

// DeleteMessage delete message from SQS
func (sjkr *DefaultSQSJkr) deleteMessage(msg *sqs.Message) error {
	params := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(sjkr.qURL),
		ReceiptHandle: msg.ReceiptHandle,
	}

	_, err := sjkr.SQS.DeleteMessage(params)
	if err != nil {
		return err
	}

	return nil
}

func (sjkr *DefaultSQSJkr) receiveMessage(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			logger.Infof("cancel sqsjkr context")
			close(sjkr.jobs)
			return nil
		default:
			resp, err := sjkr.SQS.ReceiveMessage(sjkr.recvParams)
			if err != nil {
				logger.Errorf(err.Error())
			}

			for _, msg := range resp.Messages {
				logger.Debugf("[msg_id:%s] body:%#v, md5body:%s",
					*msg.MessageId, *msg.Body, *msg.MD5OfBody)
				logger.Debugf("[attributes] sender_id:%s, sent_time_stamp:%s, appx_1st_recv_time:%s, appx_recv_cnt:%s",
					*msg.Attributes["SenderId"],
					*msg.Attributes["SentTimestamp"],
					*msg.Attributes["ApproximateFirstReceiveTimestamp"],
					*msg.Attributes["ApproximateReceiveCount"],
				)

				job, err := NewJob(msg, sjkr.conf.Kicker.Trigger)
				if err == nil {
					sjkr.jobs <- job
				} else {
					logger.Errorf(err.Error())
				}

				sjkr.deleteMessage(msg)
			}
		}
	}
}

// SetLocker set DefaultSQSJkr's Locker
func (sjkr *DefaultSQSJkr) SetLocker(l lock.Locker) {
	sjkr.locker = l
}

// Locker return DefaultSQSJkr's Locker
func (sjkr *DefaultSQSJkr) Locker() lock.Locker {
	return sjkr.locker
}

// SetThrottler set DefaultSQSJkr's Throttler
func (sjkr *DefaultSQSJkr) SetThrottler(t throttle.Throttler) {
	sjkr.throttler = t
}

// Throttler return DefaultSQSJkr's Throttler
func (sjkr *DefaultSQSJkr) Throttler() throttle.Throttler {
	return sjkr.throttler
}

// New DefaultSQSJkr
func New(c *Config) (*DefaultSQSJkr, error) {
	// set queue url
	qURL := fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/%s",
		c.Account.Region,
		c.Account.ID,
		c.SQS.QueueName,
	)

	// initialize SQS
	var q *sqs.SQS
	var awsConf *aws.Config
	if c.Account.Profile != "" {
		cred := credentials.NewSharedCredentials("", c.Account.Profile)
		awsConf = &aws.Config{
			Region:      &c.Account.Region,
			Credentials: cred,
		}
	} else {
		awsConf = &aws.Config{
			Region: &c.Account.Region,
		}
	}
	q = sqs.New(session.New(), awsConf)

	// retrives SQS queue attributes
	input := &sqs.GetQueueAttributesInput{
		AttributeNames: aws.StringSlice([]string{"All"}),
		QueueUrl:       aws.String(qURL),
	}
	attrs, err := q.GetQueueAttributes(input)
	if err != nil {
		return nil, err
	}

	// parse a value of message retention period
	strVal := *attrs.Attributes[sqs.QueueAttributeNameMessageRetentionPeriod]
	sec, err := strconv.ParseInt(strVal, 10, 64)
	if err != nil {
		logger.Errorf("%s", err.Error())
		return nil, err
	}
	rperiod := time.Second * time.Duration(sec)

	return &DefaultSQSJkr{
		jobs:            make(chan Job),
		conf:            c,
		qURL:            qURL,
		SQS:             q,
		RetentionPeriod: rperiod,
	}, nil
}

// Run SQSJkr daemon
func Run(ctx context.Context, sjkr SQSJkr, level string) error {
	logger = NewLogger()
	logger.SetLevel(level)

	// config validate
	if err := sjkr.Config().Validate(); err != nil {
		return err
	}

	stats := &Stats{
		busy: make(chan struct{}, sjkr.Config().Kicker.MaxConcurrentNum),
	}
	handlerV1 := func(w http.ResponseWriter, r *http.Request) {
		busyNum := len(stats.busy)
		mi := StatsItem{
			IdleWorkerNum: uint32(cap(stats.busy) - busyNum),
			BusyWorkerNum: uint32(busyNum),
		}

		w.Header().Set("Content-type", ApplicationJSON)
		enc := json.NewEncoder(w)
		if err := enc.Encode(mi); err != nil {
			logger.Errorf(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
		}
	}

	handlerV2 := func(w http.ResponseWriter, r *http.Request) {
		busyNum := len(stats.busy)
		s := Stats{}
		s.Workers.Idle = int64(cap(stats.busy) - busyNum)
		s.Workers.Busy = int64(busyNum)
		s.Invocations = stats.Invocations

		w.Header().Set("Content-type", ApplicationJSON)
		enc := json.NewEncoder(w)
		if err := enc.Encode(s); err != nil {
			logger.Errorf(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
		}
	}

	// unix domain or http
	var l net.Listener
	var err error
	sockPath := sjkr.Config().Kicker.StatsSocket
	if sockPath != "" {
		l, err = net.Listen("unix", sockPath)
	} else {
		port := sjkr.Config().Kicker.StatsPort
		if port == 0 {
			port = DefaultStatsPort
		}
		l, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	}
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/stats/metrics/v2", handlerV2)
	mux.HandleFunc("/stats/metrics", handlerV1)

	srv := &http.Server{Handler: mux}
	go func() {
		err := srv.Serve(l)
		if err == http.ErrServerClosed {
			logger.Infof("%s", err)
		} else if err != nil {
			logger.Logger.Fatal(err)
		}
	}()

	// init trap signals
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, TrapSignals...)

	// set locker and throttler if not to set
	if sjkr.Locker() == nil {
		sjkr.SetLocker(new(lock.DefaultLocker))
	}
	if sjkr.Throttler() == nil {
		sjkr.SetThrottler(new(throttle.DefaultThrottler))
	}

	// context
	ctx, cancel := context.WithCancel(ctx)

	// start sqsjkr daemon
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		sjkr.Run(ctx)
		logger.Infof("stop sqsjkr daemon")
		wg.Done()
	}()

	// start workers
	for i := 0; i < sjkr.Config().Kicker.MaxConcurrentNum; i++ {
		wg.Add(1)
		go func(wid int) {
			SpawnWorker(sjkr, wid, sjkr.JobStream(), stats)
			wg.Done()
		}(i)
	}

	// wait for exit signals
	go func() {
		select {
		case s := <-signalCh:
			cancel()
			logger.Infof("signal: %s(%d), shutdown sqsjkr", s, s)
		}
	}()

	wg.Wait()
	srv.Shutdown(ctx)
	logger.Infof("stopped sqsjkr")

	return nil
}
