package easy_telnet

import (
	"testing"
	"time"
)

func TestConn(t *testing.T) {
	tc := NewClient("127.0.0.1",
		WithPort(2333),
		WithVerbose(true),
		WithUserName("admin"),
		WithTimeout(5*time.Second),

		WithPromptBanner("Shell>"),
	)

	if err := tc.Dial(); err != nil {
		t.Fatal(err)
	}

	stdout, err := tc.Execute("test")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(stdout))
}
