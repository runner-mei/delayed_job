package delayed_job

import (
	"errors"
	"fmt"
	"time"
)

type multiplexedHandler struct {
	backend *dbBackend
	rules   []*Job
}

func newMultiplexedHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == ctx {
		return nil, errors.New("ctx is nil")
	}
	if nil == params {
		return nil, errors.New("params is nil")
	}

	o, ok := ctx["backend"]
	if !ok || nil == o {
		return nil, errors.New("backend in the ctx is required")
	}

	backend, ok := o.(*dbBackend)
	if !ok {
		return nil, fmt.Errorf("backend in the ctx is not a backend - %T", o)
	}

	if nil == backend {
		return nil, errors.New("backend in the ctx is nil")
	}

	is_valid_rule := boolWithDefault(params, "is_valid_rule", false)
	gpriority := intWithDefault(params, "priority", *default_priority)
	gqueue := stringWithDefault(params, "queue", *default_queue_name)
	gmax_attempts := intWithDefault(params, "max_attempts", *default_max_attempts)
	grun_at := timeWithDefault(params, "run_at", time.Time{})
	args := params["arguments"]

	o, ok = params["rules"]
	if !ok || nil == o {
		return nil, errors.New("'rules' is required.")
	}

	array, ok := o.([]interface{})
	if !ok {
		return nil, errors.New("'rules' is not a array")
	}

	if 0 == len(array) {
		return &multiplexedHandler{backend: backend}, nil
	}

	var m_args map[string]interface{}
	if nil != args {
		m_args, ok = args.(map[string]interface{})
	}

	rules := make([]*Job, 0, len(array))
	for idx, v := range array {
		options, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("rules[%v] is not a map", idx)
		}
		priority := intWithDefault(options, "priority", gpriority)
		queue := stringWithDefault(options, "queue", gqueue)
		repeat_count := intWithDefault(options, "repeat_count", 0)
		repeat_interval := stringWithDefault(options, "repeat_interval", "")
		max_attempts := intWithDefault(options, "max_attempts", gmax_attempts)
		run_at := timeWithDefault(options, "run_at", grun_at)

		if nil != args {
			if own_args, ok := options["arguments"]; !ok || nil == own_args {
				options["arguments"] = args
			} else if mm, ok := own_args.(map[string]interface{}); ok && nil != mm && nil != m_args {
				for k, v := range m_args {
					mm[k] = v
				}
				options["arguments"] = mm
			}
		}

		j, e := newJob(backend, priority, repeat_count, repeat_interval, max_attempts, queue, run_at, options, is_valid_rule)
		if nil != e {
			return nil, fmt.Errorf("rules[%d] is invalid, %v", idx, e)
		}

		rules = append(rules, j)
	}
	return &multiplexedHandler{backend: backend, rules: rules}, nil
}

func (self *multiplexedHandler) Perform() error {
	if nil == self.backend {
		return errors.New("backend is nil.")
	}

	return self.backend.create(self.rules...)
}

func init() {
	Handlers["multiplexed"] = newMultiplexedHandler
}
