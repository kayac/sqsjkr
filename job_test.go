package sqsjkr

import (
	"fmt"
	"os"
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
var DisableLifeTimeTriggerMsg = `{
    "command":        "sleep 1",
    "envs":           {"PATH":"/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin"},
    "life_time":      "1s",
    "event_id":       "test_event",
    "abort_if_locked": true,
    "disable_life_time_trigger": true
}`

var jobtestLocker lock.Locker
var testTrigger = "./test/trigger_test.sh"

func init() {
	jobtestLocker = &TestLocker{
		lockTable: map[string]bool{},
	}
}

func TestExecute(t *testing.T) {
	msg := initMsg()
	job, err := NewJob(msg, testTrigger)
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
	job, err := NewJob(wmsg, testTrigger)
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
	job, err := NewJob(wmsg, testTrigger)
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
	filename := "test/.triggered"
	defer os.Remove(filename)
	job, _ := NewJob(msg, "touch "+filename)
	_, err := job.Execute(jobtestLocker)

	if err == nil {
		t.Errorf("should not be error is nil")
	} else if err != ErrOverLifeTime {
		t.Errorf("should be error:%s. error=%s", ErrOverLifeTime, err.Error())
	}
	if _, err := os.Stat(filename); err != nil {
		t.Errorf("%s must be exist", filename)
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
	job1, err := NewJob(wmsg, testTrigger)
	job2, err := NewJob(wmsg, testTrigger)

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
	job1, err := NewJob(wmsg, testTrigger)
	job2, err := NewJob(wmsg, testTrigger)
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

func TestDisableLifeTimeTriggerJob(t *testing.T) {
	wmsg := buildMsg(DisableLifeTimeTriggerMsg)
	filename := "test/.triggered"
	defer os.Remove(filename)
	job, err := NewJob(wmsg, "touch "+filename)
	if err != nil {
		t.Errorf("%s", err)
	}

	time.Sleep(time.Second * 2)
	b, err := job.Execute(jobtestLocker)
	if err != ErrOverLifeTime {
		t.Errorf("unexpected err: %s expected:ErrOverLifeTime", err)
	}
	if b != nil {
		t.Errorf("unexpected result: %s expected:nil", b)
	}
	if stat, err := os.Stat(filename); err == nil {
		t.Errorf("%s must not be exists: %#v", filename, stat)
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
