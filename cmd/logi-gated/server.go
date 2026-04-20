package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
)

var SocketPath = func() string {
	if u, err := user.Current(); err == nil && u.Uid != "" {
		return "/tmp/logigate-" + u.Uid + ".sock"
	}
	return "/tmp/logigate.sock"
}()

type Server struct {
	cfg *Config
}

func NewServer(cfg *Config) *Server {
	return &Server{cfg: cfg}
}

func (s *Server) Serve() {
	_ = os.Remove(SocketPath)
	l, err := net.Listen("unix", SocketPath)
	if err != nil {
		log.Fatalf("socket: %v", err)
	}
	_ = os.Chmod(SocketPath, 0666)
	log.Printf("listening on %s", SocketPath)
	for {
		c, err := l.Accept()
		if err != nil {
			continue
		}
		go s.handle(c)
	}
}

func (s *Server) handle(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	line, err := r.ReadString('\n')
	if err != nil {
		return
	}
	line = strings.TrimSpace(line)
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return
	}
	cmd := strings.ToUpper(fields[0])
	switch cmd {
	case "STATUS":
		enabled, dwell, cooldown, trigger, channel := s.cfg.Get()
		disp := GetDisplayState()
		resp := map[string]any{
			"enabled":     enabled,
			"qualified":   disp.Qualified,
			"active":      enabled && disp.Qualified,
			"dwell_ms":    dwell,
			"cooldown_ms": cooldown,
			"trigger":     trigger,
			"channel":     channel,
		}
		data, _ := json.Marshal(resp)
		fmt.Fprintln(c, string(data))
	case "SWITCH":
		if len(fields) < 2 {
			fmt.Fprintln(c, "ERR missing channel")
			return
		}
		ch, err := strconv.Atoi(fields[1])
		if err != nil || ch < 1 || ch > 3 {
			fmt.Fprintln(c, "ERR invalid channel")
			return
		}
		go Switch(ch)
		fmt.Fprintln(c, "OK")
	case "ENABLE":
		s.cfg.SetEnabled(true)
		fmt.Fprintln(c, "OK")
	case "DISABLE":
		s.cfg.SetEnabled(false)
		fmt.Fprintln(c, "OK")
	case "TOGGLE":
		enabled, _, _, _, _ := s.cfg.Get()
		s.cfg.SetEnabled(!enabled)
		fmt.Fprintf(c, "OK enabled=%v\n", !enabled)
	case "SET":
		if len(fields) < 3 {
			fmt.Fprintln(c, "ERR usage: SET trigger <name> | SET channel <1|2|3>")
			return
		}
		switch fields[1] {
		case "trigger":
			if !s.cfg.SetTrigger(fields[2]) {
				fmt.Fprintln(c, "ERR invalid trigger")
				return
			}
			fmt.Fprintln(c, "OK")
		case "channel":
			ch, err := strconv.Atoi(fields[2])
			if err != nil || !s.cfg.SetChannel(ch) {
				fmt.Fprintln(c, "ERR invalid channel")
				return
			}
			fmt.Fprintln(c, "OK")
		default:
			fmt.Fprintln(c, "ERR unknown SET key")
		}
	case "SCAN":
		out, _ := exec.Command(LogiGateBin, "scan").CombinedOutput()
		c.Write(out)
	default:
		fmt.Fprintln(c, "ERR unknown command")
	}
}
