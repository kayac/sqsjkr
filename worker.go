package sqsjkr

import (
	"github.com/kayac/sqsjkr/throttle"
)

// Worker struct
type Worker struct {
	sjkr SQSJkr
	id   int
	jobs <-chan Job
	busy chan struct{}
}

// SpawnWorker spawn worker
func SpawnWorker(sjkr SQSJkr, wid int, js <-chan Job, busy chan struct{}) {
	defer func(id int) {
		logger.Infof("[worker_id:%d] terminated command worker.", wid)
	}(wid)

	worker := Worker{
		sjkr: sjkr,
		id:   wid,
		jobs: js,
		busy: busy,
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
	w.busy <- struct{}{}

	// decrement busy worker number when to return
	defer func() {
		<-w.busy
	}()

	// Execute job
	logger.Infof("CMD event_id:%s command:%s", job.EventID(), job.Command())
	output, err := job.Execute(w.sjkr.Locker())
	if err != nil && output == nil {
		logger.Errorf("[event:%s] failed to invoke command, reason: %s", job.EventID(), err.Error())
		return err
	} else if err != nil {
		logger.Errorf("[event:%s] error when to invoke command, reason: %s", job.EventID(), err.Error())
		logger.Errorf(string(output))
		return err
	}
	logger.Debugf("[event:%s] output:\n%s", job.EventID(), string(output))

	return nil
}
