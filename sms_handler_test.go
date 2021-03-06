package delayed_job

import (
	"flag"

	"testing"
)

var (
	phone_number = flag.String("phone_numbers", "1222", "the phone number")
	sms_content  = flag.String("sms_content", "恒维软件himp", "the message")
	sms_skipped  = flag.Bool("sms_skipped", true, "the message")
)

func TestSMSHandler(t *testing.T) {
	if *sms_skipped {
		t.Skip("sms is skip")
	}

	handler, e := newHandler(nil, map[string]interface{}{"type": "sms",
		"phone_numbers": *phone_number,
		"content":       *sms_content})
	if nil != e {
		t.Error(e)
		return
	}

	e = handler.Perform()
	if nil != e {
		t.Error(e)
		return
	}
}
