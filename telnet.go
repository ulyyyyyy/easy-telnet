package easy_telnet

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"net"
)

const (
	// IAC interpret as command
	IAC = 255
	// SB is subnegotiation of the indicated option follows
	SB = 250
	// SE is end of subnegotiation parameters
	SE = 240
	// WILL indicate the desire to begin
	WILL = 251
	// WONT indicates the refusal to perform,
	// continue performing, the indicated option
	WONT = 252
	// DO indicates the request that the other
	// party perform, or confirmation that you are
	// expecting the other party to perform, the indicated option
	DO = 253
	// DONT indicates the demand that the other
	// party stop performing, or confirmation that you
	// are no longer expecting the other party to
	// perform, the indicated option
	DONT = 254
)

var (
	defaultPort    = 23
	defaultTimeout = 10 * time.Second

	defaultUsernameRe = "[\\w\\d-_]+ username:"
	defaultPasswordRe = "Password:"
	defaultBannerRe   = "[\\w\\d-_]+@[\\w\\d-_]+:[\\w\\d/-_~]+(\\$|#)"
)

// Client is basic descriptor
type Client struct {
	Address string
	Port    int

	username string
	password string
	timeout  time.Duration
	verbose  bool

	logWriter *bufio.Writer

	promptUsername *regexp.Regexp
	promptPassword *regexp.Regexp
	promptBanner   *regexp.Regexp

	reader *bufio.Reader
	writer *bufio.Writer
	conn   net.Conn
}

func NewClient(address string, opts ...Option) *Client {
	cli := &Client{Address: address}
	dftOpts := []Option{
		WithPort(defaultPort),
		WithTimeout(defaultTimeout),
		WithVerbose(false),

		WithPromptUsername(defaultUsernameRe),
		WithPromptPassword(defaultPasswordRe),
		WithPromptBanner(defaultBannerRe),
	}

	dftOpts = append(dftOpts, opts...)

	for _, fn := range dftOpts {
		fn(cli)
	}

	return cli
}

func (tc *Client) setDefaultParams() {
	if tc.Port == 0 {
		tc.Port = defaultPort
	}
	if tc.timeout == 0 {
		tc.timeout = defaultTimeout
	}
	if tc.verbose && tc.logWriter == nil {
		tc.logWriter = bufio.NewWriter(os.Stdout)
	}
	if tc.promptUsername == nil {
		tc.promptUsername = regexp.MustCompile(defaultUsernameRe)
	}
	if tc.promptPassword == nil {
		tc.promptPassword = regexp.MustCompile(defaultPasswordRe)
	}
	if tc.promptBanner == nil {
		tc.promptBanner = regexp.MustCompile(defaultBannerRe)
	}
}

func (tc *Client) log(format string, params ...interface{}) {
	if tc.verbose {
		_, _ = fmt.Fprintf(tc.logWriter, "telnet: "+format+"\n", params...)
		_ = tc.logWriter.Flush()
	}
}

// Dial does open connect to telnet server
func (tc *Client) Dial() (err error) {
	tc.setDefaultParams()

	tc.log("Trying connect to %s:%d", tc.Address, tc.Port)
	tc.conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", tc.Address, tc.Port))
	if err != nil {
		return
	}

	tc.reader = bufio.NewReader(tc.conn)
	tc.writer = bufio.NewWriter(tc.conn)
	err = tc.conn.SetReadDeadline(time.Now().Add(tc.timeout))
	if err != nil {
		return
	}

	tc.log("Waiting for the first banner")
	err = tc.waitWelcomeSigns()

	return
}

func (tc *Client) Close() {
	tc.conn.Close()
}

func (tc *Client) skipSBSequence() (err error) {
	var peeked []byte

	for {
		_, err = tc.reader.Discard(1)
		if err != nil {
			return
		}

		peeked, err = tc.reader.Peek(2)
		if err != nil {
			return
		}

		if peeked[0] == IAC && peeked[1] == SE {
			_, err = tc.reader.Discard(2)
			break
		}
	}

	return
}

func (tc *Client) skipCommand() (err error) {
	var peeked []byte

	peeked, err = tc.reader.Peek(1)
	if err != nil {
		return
	}

	switch peeked[0] {
	case WILL, WONT, DO, DONT:
		_, err = tc.reader.Discard(2)
	case SB:
		err = tc.skipSBSequence()
	}

	return
}

// ReadByte receives byte from remote server, avoiding commands
func (tc *Client) ReadByte() (b byte, err error) {
	for {
		b, err = tc.reader.ReadByte()
		if err != nil || b != IAC {
			break
		}

		err = tc.skipCommand()
		if err != nil {
			break
		}
	}

	return
}

// ReadUntil reads bytes until a specific symbol.
// Delimiter character will be written to result buffer
func (tc *Client) ReadUntil(data *[]byte, delim byte) (n int, err error) {
	var b byte

	for b != delim {
		b, err = tc.ReadByte()
		if err != nil {
			break
		}

		*data = append(*data, b)
		n++
	}

	return
}

func findNewLinePos(data []byte) int {
	var pb byte

	for i := len(data) - 1; i >= 0; i-- {
		cb := data[i]
		if pb == '\n' && cb == '\r' {
			return i
		}

		pb = cb
	}

	return -1
}

// ReadUntilPrompt reads data until process function stops.
// If process function returns true, reading will be stopped
// Process function give chunk of line i.e. from start of line
// to last white space or whole line, if next line delimiter is found
func (tc *Client) ReadUntilPrompt(process func(data []byte) bool) (output []byte, err error) {
	var n int
	var delimPos int
	var linePos int
	var chunk []byte

	output = make([]byte, 0, 64*1024)

	for {
		// Usually, if system print a prompt,
		// it requires inputing data and
		// prompt has ':' or whitespace in end of line.
		// However, may be cases which have another behaviors.
		// So client may freeze
		n, err = tc.ReadUntil(&output, ' ')
		if err != nil {
			return
		}

		delimPos += n
		n = findNewLinePos(output)
		if n != -1 {
			linePos = n + 2
		}

		chunk = output[linePos:delimPos]

		if process(chunk) {
			break
		}
	}

	return
}

// ReadUntilBanner reads until banner, i.e. whole output from command
func (tc *Client) ReadUntilBanner() (output []byte, err error) {
	output, err = tc.ReadUntilPrompt(func(data []byte) bool {
		m := tc.promptBanner.Find(data)
		return len(m) > 0
	})

	output = tc.promptBanner.ReplaceAll(output, []byte{})
	output = bytes.Trim(output, " ")

	return
}

func (tc *Client) findInputPrompt(re *regexp.Regexp, response string, buffer []byte) bool {
	match := re.Find(buffer)
	if len(match) == 0 {
		return false
	}

	_, err := tc.Write([]byte(response + "\r\n"))
	if err != nil {
		return false
	}

	return true
}

// waitWelcomeSigns waits for appearance of the first banner
// If detect login prompt, it will authorize
func (tc *Client) waitWelcomeSigns() (err error) {
	_, err = tc.ReadUntilPrompt(func(data []byte) bool {
		if tc.findInputPrompt(tc.promptUsername, tc.username, data) {
			tc.log("Found Username prompt")
			return false
		}
		if tc.password != "" && tc.findInputPrompt(tc.promptPassword, tc.password, data) {
			tc.log("Found password prompt")
			return false
		}

		m := tc.promptBanner.Find(data)
		return len(m) > 0
	})

	return
}

// Write sends raw data to remove telnet server
func (tc *Client) Write(data []byte) (n int, err error) {
	n, err = tc.writer.Write(data)
	if err == nil {
		err = tc.writer.Flush()
	}

	return
}

// Execute sends command on remote server and returns whole output
func (tc *Client) Execute(name string, args ...string) (stdout []byte, err error) {
	_, err = tc.reader.Discard(tc.reader.Buffered())
	if err != nil {
		return
	}

	request := []byte(name + " " + strings.Join(args, " ") + "\r\n")
	tc.log("Send command: %s", request[:len(request)-2])
	if _, err = tc.Write(request); err != nil {
		return nil, err
	}

	stdout, err = tc.ReadUntilBanner()
	if err != nil {
		return
	}
	tc.log("Received data with size = %d", len(stdout))

	return
}
