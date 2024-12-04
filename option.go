package easy_telnet

import (
	"regexp"
	"time"
)

type Option func(g *Client)

func WithPort(port int) Option {
	return func(g *Client) {
		g.Port = port
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(g *Client) {
		g.timeout = timeout
	}
}

func WithUserName(username string) Option {
	return func(g *Client) {
		g.username = username
	}
}

func WithPassword(password string) Option {
	return func(g *Client) {
		g.password = password
	}
}

func WithVerbose(verbose bool) Option {
	return func(g *Client) {
		g.verbose = verbose
	}
}

func WithPromptUsername(prompt string) Option {
	reProm := regexp.MustCompile(prompt)
	return func(g *Client) {
		g.promptUsername = reProm
	}
}

func WithPromptPassword(prompt string) Option {
	reProm := regexp.MustCompile(prompt)
	return func(g *Client) {
		g.promptPassword = reProm
	}
}

func WithPromptBanner(prompt string) Option {
	reProm := regexp.MustCompile(prompt)
	return func(g *Client) {
		g.promptBanner = reProm
	}
}
