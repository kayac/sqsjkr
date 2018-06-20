package sqsjkr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/kayac/sqsjkr/lock"
)

// Duration struct
type Duration struct {
	time.Duration
}

// UnmarshalJSON Duration field to decode json
func (d *Duration) UnmarshalJSON(b []byte) (err error) {
	if b[0] == '"' {
		sd := string(b[1 : len(b)-1])
		d.Duration, err = time.ParseDuration(sd)
		return
	}

	var sec int64
	sec, err = json.Number(string(b)).Int64()
	d.Duration = time.Duration(sec) * time.Second
	return
}

// DefaultJob is created by one SQS message's body
type DefaultJob struct {
	jobID         string // JobID is created by sqs messageID
	environment   map[string]string
	command       string
	eventID       string
	lifeTime      time.Duration
	sentTimestamp time.Time
	abortIfLocked bool
	lockID        string
	trigger       string
}

// Job is sqsjkr job struct
type Job interface {
	Execute(lock.Locker) ([]byte, error)
	JobID() string
	EventID() string
	Command() string
}

// MessageBody for decoding json
type MessageBody struct {
	Command                string            `json:"command"`
	Environments           map[string]string `json:"envs"`
	EventID                string            `json:"event_id"`
	LifeTime               Duration          `json:"life_time"`
	LockID                 string            `json:"lock_id"`
	AbortIfLocked          bool              `json:"abort_if_locked"`
	DisableLifeTimeTrigger bool              `json:"disable_life_time_trigger"`
}

func (m MessageBody) String() string {
	var b strings.Builder
	json.NewEncoder(&b).Encode(m)
	return strings.TrimSuffix(b.String(), "\n")
}

// Execute executes command
func (j *DefaultJob) Execute(lkr lock.Locker) ([]byte, error) {
	// 1. Checks job's lifetime.
	if j.isOverLifeTime() {
		if j.trigger == "" {
			// trigger is disabled or not defined
			return nil, ErrOverLifeTime
		}

		msg := fmt.Sprintf("job_id:%s, event_id:%s, command:%s, life_time:%s, sent_timestamp:%s",
			j.jobID, j.eventID, j.command, j.lifeTime.String(), j.sentTimestamp.String())

		out, err := invokeTrigger(j.trigger, msg)
		logger.Debugf("trigger output: %s", string(out))
		if err != nil {
			return nil, err
		}

		return nil, ErrOverLifeTime
	}

	// 2. Locks Job (if job's lockID have been locked already, retry to Execute() after 5sec).
	if j.lockID != "" && j.eventID != "" && lkr != nil {
		err := lkr.Lock(j.lockID, j.eventID)
		if err != nil {
			logger.Errorf(err.Error())
			if j.abortIfLocked {
				return nil, err
			}
			time.Sleep(JobRetryInterval)
			return j.Execute(lkr)
		}
	}

	// 3. Validation.
	if err := j.validate(); err != nil {
		return nil, err
	}

	// 4. Executes job.
	env := os.Environ()
	for key, val := range j.environment {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}
	cmd := exec.Command("sh", "-c", j.command)
	cmd.Env = env
	output, err := cmd.CombinedOutput()

	// 5. Unlocks job.
	if j.lockID != "" && lkr != nil {
		if derr := lkr.Unlock(j.lockID); derr != nil {
			// TODO: should implement notification
			logger.Errorf(derr.Error())
		}
	}

	return output, err
}

// JobID return job's id which is unique (sqs message id).
func (j DefaultJob) JobID() string {
	return j.jobID
}

// Command return job's command
func (j DefaultJob) Command() string {
	return j.command
}

// EventID return event_id
func (j DefaultJob) EventID() string {
	return j.eventID
}

func (j DefaultJob) isOverLifeTime() bool {
	diffTime := time.Now().Sub(j.sentTimestamp)

	if j.lifeTime == 0 {
		return false
	}

	if diffTime > j.lifeTime {
		logger.Warnf("over life time: life_time:%v, diffTime:%v, overTime:%v",
			j.lifeTime, diffTime, diffTime-j.lifeTime)
		return true
	}
	return false
}

// Validates job can exec command
func (j *DefaultJob) validate() error {
	if j.command == "" {
		return fmt.Errorf("Job command undefined.")
	}
	if j.jobID == "" {
		return fmt.Errorf("JobID undefined.")
	}
	return nil
}

// NewJob create job
func NewJob(msg *sqs.Message, trigger string) (Job, error) {
	var body MessageBody
	if err := json.Unmarshal([]byte(*msg.Body), &body); err != nil {
		logger.Errorf("Cannot parse message body: %s", err.Error())
		return nil, err
	}

	sentTimestamp, err := strconv.ParseInt(*msg.Attributes["SentTimestamp"], 10, 64)
	if err != nil {
		logger.Errorf("Cannot parse attribute SentTimestamp: %s", err.Error())
		return nil, err
	}
	sentTime := time.Unix(sentTimestamp/1000, sentTimestamp%1000*int64(time.Millisecond))

	logger.Infof(
		"new job by message id:%s body:%s sentTimestamp:%s",
		*msg.MessageId,
		body.String(),
		sentTime,
	)

	dj := &DefaultJob{
		jobID:         *msg.MessageId,
		command:       body.Command,
		environment:   body.Environments,
		eventID:       body.EventID,
		lockID:        body.LockID,
		abortIfLocked: body.AbortIfLocked,
		lifeTime:      body.LifeTime.Duration,
		sentTimestamp: sentTime,
	}
	if !body.DisableLifeTimeTrigger {
		dj.trigger = trigger
	}
	return dj, nil
}

// invokeTrigger execute trigger command
func invokeTrigger(command, msg string) ([]byte, error) {
	cmd := exec.Command("sh", "-c", command)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed: %v %s", cmd, err.Error())
	}

	input := []byte(msg)
	src := bytes.NewReader(input)
	_, err = io.Copy(stdin, src)

	if e, ok := err.(*os.PathError); ok && e.Err == syscall.EPIPE {
		logger.Errorf(err.Error())
	} else if err != nil {
		logger.Errorf(err.Error())
		logger.Errorf(fmt.Errorf("failed to write STDIN").Error())
	}
	stdin.Close()

	return cmd.CombinedOutput()
}
