package sqsjkr

import (
	"sync/atomic"

	"github.com/kayac/sqsjkr/throttle"
)

// Worker struct
type Worker struct {
	sjkr  SQSJkr
	id    int
	jobs  <-chan Job
	stats *Stats
}

// SpawnWorker spawn worker
func SpawnWorker(sjkr SQSJkr, wid int, js <-chan Job, s *Stats) {
	defer func(id int) {
		logger.Infof("[worker_id:%d] terminated command worker.", wid)
	}(wid)

	worker := Worker{
		sjkr:  sjkr,
		id:    wid,
		jobs:  js,
		stats: s,
	}
	logger.Infof("[worker_id:%d]spawn worker.", wid)

	worker.ReceiveMessage()
}

// ReceiveMessage receive messages
func (w Worker) ReceiveMessage() {
	// worker will be killed when errCnt is over 5.
	for job := range w.jobs {
		if err := w.sjkr.Throttler().Set(job.JobID()); err != nil {
			if err == throttle.ErrDuplicatedMessage {
				logger.Errorf("duplicated message id: %s", job.JobID())
				continue
			}
			logger.Errorf("reason=%s ,job=%v", err.Error(), job)
		}

		if err := w.executeJob(job); err != nil {
			logger.Errorf("[worker_id:%d] %s", w.id, err.Error())
		}

	}

	logger.Infof("terminate %d worker", w.id)
	return
}

func (w Worker) executeJob(job Job) error {
	// busy worker number count up
	w.stats.busy <- struct{}{}

	// decrement busy worker number when to return
	defer func() {
		<-w.stats.busy
	}()

	// Execute job
	logger.Infof("CMD event_id:%s command:%s", job.EventID(), job.Command())
	output, err := job.Execute(w.sjkr.Locker())
	if err != nil && output == nil {
		atomic.AddInt64(&w.stats.Invocations.Failed, 1)
		logger.Errorf("[event:%s] failed to invoke command, reason: %s", job.EventID(), err.Error())
		return err
	} else if err != nil {
		atomic.AddInt64(&w.stats.Invocations.Errored, 1)
		logger.Errorf("[event:%s] error when to invoke command, reason: %s", job.EventID(), err.Error())
		logger.Errorf(string(output))
		return err
	} else {
		atomic.AddInt64(&w.stats.Invocations.Succeeded, 1)
	}
	logger.Debugf("[event:%s] output:\n%s", job.EventID(), string(output))

	return nil
}
