package sqsjkr

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/kayac/sqsjkr/lock"
	"github.com/kayac/sqsjkr/throttle"
)

type TestSQSJkr struct {
	jobs      chan Job
	locker    lock.Locker
	throttler throttle.Throttler
	conf      *Config
}

var (
	TestMessages []*sqs.Message
	Score        = 0
)

// Mock ReceiveMessage
func (tsjkr TestSQSJkr) receiveMessage(ctx context.Context) {
	for i := 0; i < 10; i++ {
		var job Job
		if i == 9 {
			job = NewTestJob("job-duplicated")
		} else {
			job = NewTestJob(fmt.Sprintf("job-%d", i))
		}
		tsjkr.jobs <- job
	}

	// jod-duplicated job's sleep 200 msec, so this last job will not be executed.
	job := NewTestJob("job-duplicated")
	tsjkr.jobs <- job

	<-ctx.Done()
	close(tsjkr.jobs)
}

func (tsjkr TestSQSJkr) Run(ctx context.Context) error {
	tsjkr.receiveMessage(ctx)

	return nil
}

func (tsjkr TestSQSJkr) Config() *Config {
	return tsjkr.conf
}

// JobStream return Job chan.
func (tsjkr TestSQSJkr) JobStream() chan Job {
	return tsjkr.jobs
}

func (tsjkr TestSQSJkr) Throttler() throttle.Throttler {
	return tsjkr.throttler
}
func (tsjkr TestSQSJkr) Locker() lock.Locker {
	return tsjkr.locker
}

func (tsjkr TestSQSJkr) SetLocker(l lock.Locker) {
}

func (tsjkr TestSQSJkr) SetThrottler(t throttle.Throttler) {
}

// TestJob for test
type TestJob struct {
	jobID string
}

func (tj TestJob) Execute(locker lock.Locker) ([]byte, error) {
	if tj.jobID == "job-duplicated" {
		time.Sleep(time.Millisecond * 200)
	}
	Score++
	return []byte(fmt.Sprintf("success %s", tj.jobID)), nil
}

func (tj TestJob) JobID() string {
	return tj.jobID
}

func (tj TestJob) Command() string {
	return "echo ok"
}

func (tj TestJob) EventID() string {
	return "event_id"
}

// This test includes tests to check duplicated sqs messages and lock events.
func TestReceiveAndExecCommand(t *testing.T) {
	var err error
	wg := sync.WaitGroup{}

	conf, _ := LoadConfig("./test/sqsjkr.toml")

	testSQSjkr := TestSQSJkr{
		jobs: make(chan Job),
		locker: &TestLocker{
			lockTable: map[string]bool{},
		},
		throttler: &TestThrottle{
			table: map[string]bool{},
		},
		conf: conf,
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// Start SQSJkr daemon
	wg.Add(1)
	go func() {
		err = Run(ctx, testSQSjkr, "debug")

		// checks Daemon error
		if err != nil {
			t.Error(err)
			fmt.Println("[error]: ", err)
		}
		wg.Done()
	}()

	cancel()
	wg.Wait()

	if Score != 10 {
		t.Errorf("Could not delete message correctly: expected=10 got=%d ", Score)
	}
}

func NewTestJob(id string) Job {
	return &TestJob{
		jobID: id,
	}
}
