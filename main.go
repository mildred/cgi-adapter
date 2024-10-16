package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Config struct {
	PathInfoStrip int
}

func HandleCGI(cfg *Config, args []string) error {
	var res http.Response
	req, err := http.ReadRequest(bufio.NewReader(os.Stdin))
	if err != nil {
		return err
	}

	// https://www.rfc-editor.org/rfc/rfc3875#section-4
	os.Setenv("AUTH_TYPE", "")
	os.Setenv("CONTENT_LENGTH", strconv.FormatInt(req.ContentLength, 10))
	os.Setenv("CONTENT_TYPE", req.Header.Get("Content-Type"))
	os.Setenv("GATEWAY_INTERFACE", "CGI/1.1")
	if cfg.PathInfoStrip >= 0 {
		splits := strings.SplitN(req.URL.Path, "/", cfg.PathInfoStrip+2)
		if len(splits) > cfg.PathInfoStrip+1 {
			os.Setenv("PATH_INFO", "/"+splits[cfg.PathInfoStrip+1])
		} else {
			os.Setenv("PATH_INFO", "")
		}
	} else {
		os.Setenv("PATH_INFO", "")
	}
	os.Setenv("PATH_TRANSLATED", os.Getenv("PATH_INFO")) // TODO: find better
	os.Setenv("QUERY_STRING", req.URL.RawQuery)
	os.Setenv("REMOTE_ADDR", req.RemoteAddr)
	os.Setenv("REMOTE_HOST", "")
	os.Setenv("REMOTE_IDENT", "")
	os.Setenv("REMOTE_USER", "")
	os.Setenv("REQUEST_METHOD", req.Method)
	if cfg.PathInfoStrip >= 0 {
		os.Setenv("SCRIPT_NAME", req.URL.Path)
		path := ""
		for i, pathcomp := range strings.Split(req.URL.Path, "/") {
			path = path + "/" + pathcomp
			if i >= cfg.PathInfoStrip {
				break
			}
		}
		os.Setenv("SCRIPT_NAME", path)
	} else {
		os.Setenv("SCRIPT_NAME", req.URL.Path)
	}
	os.Setenv("SERVER_NAME", req.URL.Hostname())
	os.Setenv("SERVER_PORT", req.URL.Port())
	os.Setenv("SERVER_PROTOCOL", "INCLUDED")
	os.Setenv("SERVER_SOFTWARE", "cgi-adapter/1.0")

	cmd := exec.Command(args[0], args[1:len(args)-1]...)
	cmd.Stdin = req.Body
	cmd.Stderr = os.Stderr

	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	scan := bufio.NewReader(out)
	for {
		linebuf, err := scan.ReadBytes('\n')
		if err != nil {
			return err
		}

		line := strings.TrimRight(string(linebuf), "\r\n")
		if line == "" {
			break
		}

		h := strings.SplitN(line, ":", 2)
		if len(h) == 2 {
			res.Header.Add(h[0], strings.TrimLeft(h[1], " "))
		}
	}

	content_length := res.Header.Get("Content-Length")
	if content_length != "" {
		res.ContentLength, err = strconv.ParseInt(content_length, 10, 0)
		if err != nil {
			res.ContentLength = -1
			fmt.Fprintf(os.Stderr, "Failed to parse Content-Length %s: %e", content_length, err)
		}
	} else {
		res.ContentLength = -1
	}

	res.Request = req
	res.Close = true
	res.Body = io.NopCloser(scan)

	res.Write(os.Stdout)

	return nil
}

func main() {
	var cfg Config
	flag.IntVar(&cfg.PathInfoStrip, "path-info-strip", -1, "How many segments to strip to get the PATH_INFO")
	flag.Parse()
	var args = flag.Args()

	err := HandleCGI(&cfg, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %e", err)
		os.Exit(1)
	}
}
