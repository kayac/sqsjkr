package sqsjkr

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/kayac/sqsjkr/lock"
)

var DefaultTestBodyMsg = `{
    "command":        "./test/job_script.sh",
    "envs":           {"PATH":"/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin"},
    "life_time":      "1m",
    "event_id":       "test_event",
    "lock_id":        "lock1",
    "abort_if_locked": false
}`
var TooLargeStdOutBodyMsg = `{
    "command":"       ./test/large_stdout.sh",
    "envs":           {"PATH":"/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin"},
    "life_time":      "1m",
    "event_id":       "test_event",
    "lock_id":        "lock1",
    "abort_if_locked": false
}`
var LockUnlockMsg = `{
    "command":        "sleep 1; /bin/echo -n 'hello'",
    "envs":           {"PATH":"/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin"},
    "life_time":      "1m",
    "event_id":       "test_event",
    "lock_id":        "lock2",
    "abort_if_locked": false
}`
var AbortIfLockedMsg = `{
    "command":        "sleep 1",
    "envs":           {"PATH":"/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin"},
    "life_time":      "1m",
    "event_id":       "test_event",
    "lock_id":        "lock3",
    "abort_if_locked": true
}`

var jobtestLocker lock.Locker

func init() {
	jobtestLocker = &TestLocker{
		lockTable: map[string]bool{},
	}
}

func TestExecute(t *testing.T) {
	msg := initMsg()
	job, err := NewJobForTest(msg)
	if err != nil {
		t.Errorf("%s", err)
	}

	if job.EventID() != "test_event" {
		t.Errorf("wrong event_id: got=%s, expect='test_event'", job.EventID())
	}

	output, err := job.Execute(jobtestLocker)
	if err != nil {
		t.Error(err)
	}

	expect := "/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin\nHello, SQS Job Kicker!\n"
	if string(output) != expect {
		t.Errorf("Wrong result of stdout and stderr\n\n   got: %s\n   expect: %s", string(output), expect)
	}
}

func TestErrorExecuteWithWrongBody(t *testing.T) {
	wmsg := buildMsg(`{"unknown": "unknown"}`)
	job, err := NewJobForTest(wmsg)
	if err != nil {
		t.Errorf("%s", err)
	}

	output, err := job.Execute(jobtestLocker)
	if err == nil {
		t.Errorf("Should returns error! err: %s, result: %s", err, string(output))
	}
}

func TestTooLargeStdOut(t *testing.T) {
	wmsg := buildMsg(TooLargeStdOutBodyMsg)
	job, err := NewJobForTest(wmsg)
	if err != nil {
		t.Errorf("%s", err)
	}

	output, err := job.Execute(jobtestLocker)
	if err != nil {
		t.Errorf("%s, %s", err, string(output))
	}
}

func TestOverLifeTime(t *testing.T) {
	msg := initMsg()
	// set SentTimestamp 2010/4/1/ 00:00:00.00
	sentTimestamp := strconv.Itoa(1270047600000)
	msg.Attributes["SentTimestamp"] = &sentTimestamp

	job, _ := NewJobForTest(msg)
	_, err := job.Execute(jobtestLocker)

	if err == nil {
		t.Errorf("should not be error is nil")
	} else if err != ErrOverLifeTime {
		t.Errorf("should be error:%s. error=%s", ErrOverLifeTime, err.Error())
	}
}

func TestInvokeTrigger(t *testing.T) {
	msg := "job_id:1, event_id:test_event, command:'echo hoge', life_time:1, sent_timestamp:1"
	out, err := invokeTrigger("./test/trigger_test.sh", msg)
	if err != nil {
		t.Error(err)
	}

	if string(out) != fmt.Sprintf("Trigger: %s", msg) {
		t.Errorf("Wrong result of trigger: expect='Trigger: %s', got='%s'", msg, string(out))
	}
}

func TestLockUnlockJob(t *testing.T) {
	wmsg := buildMsg(LockUnlockMsg)
	job1, err := NewJobForTest(wmsg)
	job2, err := NewJobForTest(wmsg)

	go job1.Execute(jobtestLocker)
	time.Sleep(time.Second * 1)
	output, err := job2.Execute(jobtestLocker)

	if string(output) != "hello" {
		t.Errorf("output is wrong: got=%s, expected=hello", string(output))
	}

	if err != nil {
		t.Errorf("%s, %s", err, string(output))
	}
}

func TestAbortIfLockedJob(t *testing.T) {
	wmsg := buildMsg(AbortIfLockedMsg)
	job1, err := NewJobForTest(wmsg)
	job2, err := NewJobForTest(wmsg)
	if err != nil {
		t.Errorf("%s", err)
	}

	go job1.Execute(jobtestLocker)
	time.Sleep(time.Second * 1)

	startTime := time.Now()
	_, err = job2.Execute(jobtestLocker)
	endTime := time.Now()

	if err == nil {
		t.Errorf("expect err is 'already lock', but err is nil")
	}

	if endTime.Sub(startTime) >= JobRetryInterval {
		t.Errorf("could not give up immediately.")
	}
}

func initMsg() *sqs.Message {
	return buildMsg(DefaultTestBodyMsg)
}

func buildMsg(body string) *sqs.Message {
	msg := &sqs.Message{}
	msgid := "12435"
	md5b := "aaa"
	rh := "xxx"
	sentTimestamp := strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)

	msg.MessageId = &msgid
	msg.ReceiptHandle = &rh
	msg.MD5OfBody = &md5b
	msg.Body = &body
	msg.Attributes = map[string]*string{
		"SentTimestamp": &sentTimestamp,
	}

	return msg
}

func NewJobForTest(msg *sqs.Message) (Job, error) {
	var body MessageBody
	if err := json.Unmarshal([]byte(*msg.Body), &body); err != nil {
		return nil, err
	}

	sentTimestamp, err := strconv.ParseInt(*msg.Attributes["SentTimestamp"], 10, 64)
	if err != nil {
		return nil, err
	}
	sentTime := time.Unix(sentTimestamp/1000, sentTimestamp%1000*int64(time.Millisecond))

	trigger := "./test/trigger_test.sh"

	dj := &DefaultJob{
		jobID:         *msg.MessageId,
		command:       body.Command,
		environment:   body.Environments,
		eventID:       body.EventID,
		lockID:        body.LockID,
		abortIfLocked: body.AbortIfLocked,
		lifeTime:      body.LifeTime.Duration,
		sentTimestamp: sentTime,
		trigger:       trigger,
	}

	return dj, nil
}
