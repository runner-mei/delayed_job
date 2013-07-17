package delayed_job

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)

var (
	sequeuce_lock sync.Mutex
	sequence_id   = uint64(0)
	sequeuce_seed = strconv.FormatInt(time.Now().Unix(), 10) + "_"
)

func generate_id() string {
	sequeuce_lock.Lock()
	defer sequeuce_lock.Unlock()
	sequence_id += 1
	if sequence_id >= 18446744073709551610 {
		sequence_id = 0
		sequeuce_seed = strconv.FormatInt(time.Now().Unix(), 10) + "_"
	}
	return sequeuce_seed + strconv.FormatUint(sequence_id, 10)
}

type Job struct {
	backend *dbBackend

	id         int64
	priority   int
	attempts   int
	queue      string
	handler    string
	handler_id string
	last_error string
	run_at     time.Time
	failed_at  time.Time
	locked_at  time.Time
	locked_by  string
	created_at time.Time
	updated_at time.Time

	handler_attributes map[string]interface{}
	handler_object     Handler
}

func newJob(backend *dbBackend, priority, attempts int, queue string, run_at time.Time, args map[string]interface{}) (*Job, error) {
	defaultValue := generate_id()
	id := stringWithDefault(args, "_uid", defaultValue)
	if 0 == len(id) {
		id = defaultValue
	}

	s, e := json.MarshalIndent(args, "", "  ")
	if nil != e {
		return nil, deserializationError(e)
	}
	j := &Job{backend: backend,
		priority:   priority,
		attempts:   attempts,
		queue:      queue,
		handler:    string(s),
		handler_id: id,
		run_at:     run_at}

	_, e = j.payload_object()
	if nil != e {
		return nil, e
	}
	return j, nil
}

func (self *Job) isFailed() bool {
	return self.failed_at.IsZero()
}

func (self *Job) name() string {
	options, e := self.attributes()
	if nil == e && nil != options {
		if v, ok := options["display_name"]; ok {
			return fmt.Sprint(v)
		}
	}
	return "unknow"
}

func (self *Job) attributes() (map[string]interface{}, error) {
	if nil != self.handler_attributes {
		return self.handler_attributes, nil
	}
	if 0 == len(self.handler) {
		return nil, deserializationError(errors.New("handle is empty"))
	}
	e := json.Unmarshal([]byte(self.handler), &self.handler_attributes)
	if nil != e {
		return nil, deserializationError(e)
	}
	return self.handler_attributes, nil
}

func (self *Job) payload_object() (Handler, error) {
	if nil != self.handler_object {
		return self.handler_object, nil
	}
	options, e := self.attributes()
	if nil != e {
		return nil, e
	}

	self.handler_object, e = newHandler(options)
	if nil != e {
		return nil, e
	}
	return self.handler_object, nil
}

func (self *Job) invokeJob() error {
	job, e := self.payload_object()
	if nil != e {
		return e
	}
	return job.Perform()
}

func (self *Job) reschedule_at() time.Time {
	var duration time.Duration

	options, e := self.attributes()
	if nil != e {
		goto default_duration
	}

	duration = durationWithDefault(options, "try_interval", 0)
	if duration < 5*time.Second {
		goto default_duration
	}
	return self.backend.db_time_now().Add(duration)

default_duration:
	attempts := time.Duration(self.attempts*10) * time.Second
	return self.backend.db_time_now().Add(attempts).Add(5 + time.Second)
}

func (self *Job) max_attempts() int {
	options, e := self.attributes()
	if nil != e {
		return -1
	}

	if m, ok := options["max_attempts"]; ok {
		i, e := strconv.ParseInt(fmt.Sprint(m), 10, 0)
		if nil == e {
			return int(i)
		}
	}
	return -1
}

func (self *Job) rescheduleIt(next_time time.Time) error {
	self.attempts += 1
	self.run_at = next_time
	self.locked_at = time.Time{}
	self.locked_by = ""
	return self.backend.update(self.id, map[string]interface{}{"attempts": self.attempts + 1,
		"run_at":    next_time,
		"locked_at": nil,
		"locked_by": nil})
}

func (self *Job) failIt() error {
	now := self.backend.db_time_now()
	self.failed_at = now
	return self.backend.update(self.id, map[string]interface{}{"failed_at": now})
}

func (self *Job) destroyIt() error {
	return self.backend.destroy(self.id)
}
