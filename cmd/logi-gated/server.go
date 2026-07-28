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
		snap := s.cfg.Get()
		disp := GetDisplayState()
		activeBucket := BucketFor(disp.ExternalCount)

		// Serialize every setup: armed flag + ordered trigger slots.
		setups := map[string]any{}
		for _, b := range Buckets {
			sv, ok := snap.Setups[b]
			if !ok {
				continue
			}
			// Report slots in stable order (slot 1, slot 2). Zones is a map, so
			// pull the ordered slots from the config directly.
			slots := s.cfg.SetupTriggers(b)
			trigList := []map[string]any{}
			for _, t := range slots {
				trigList = append(trigList, map[string]any{"zone": t.Zone, "channel": t.Channel})
			}
			setups[b] = map[string]any{"armed": sv.Armed(), "triggers": trigList}
		}

		activeSetup, hasActive := snap.Setups[activeBucket]
		armed := hasActive && activeSetup.Armed()
		devCount := DeviceCount()
		hasDevices := devCount != 0 // <0 (unknown) or >0 counts as "not gated off"
		resp := map[string]any{
			"enabled":        snap.Enabled,
			"external_count": disp.ExternalCount,
			"device_count":   devCount,
			"active_bucket":  activeBucket,
			"active":         snap.Enabled && armed && hasDevices,
			"dwell_ms":       snap.DwellMs,
			"cooldown_ms":    snap.CoolMs,
			"setups":         setups,
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
		enabled := s.cfg.IsEnabled()
		s.cfg.SetEnabled(!enabled)
		fmt.Fprintf(c, "OK enabled=%v\n", !enabled)
	case "SET":
		// SET trigger <bucket> <slot 1|2> <zone|off>
		// SET channel <bucket> <slot 1|2> <1|2|3>
		// (A setup is armed iff it has a trigger — there is no separate armed toggle.)
		if len(fields) < 4 {
			fmt.Fprintln(c, "ERR usage: SET trigger <bucket> <1|2> <zone|off> | SET channel <bucket> <1|2> <1|2|3>")
			return
		}
		switch fields[1] {
		case "trigger":
			if len(fields) < 5 {
				fmt.Fprintln(c, "ERR usage: SET trigger <bucket> <1|2> <zone|off>")
				return
			}
			slot, err := strconv.Atoi(fields[3])
			if err != nil {
				fmt.Fprintln(c, "ERR invalid slot")
				return
			}
			ok := false
			if fields[4] == "off" {
				ok = s.cfg.ClearTrigger(fields[2], slot)
			} else {
				ok = s.cfg.SetTriggerZone(fields[2], slot, fields[4])
			}
			if !ok {
				fmt.Fprintln(c, "ERR invalid trigger")
				return
			}
			fmt.Fprintln(c, "OK")
		case "channel":
			if len(fields) < 5 {
				fmt.Fprintln(c, "ERR usage: SET channel <bucket> <1|2> <1|2|3>")
				return
			}
			slot, err := strconv.Atoi(fields[3])
			if err != nil {
				fmt.Fprintln(c, "ERR invalid slot")
				return
			}
			ch, err := strconv.Atoi(fields[4])
			if err != nil || !s.cfg.SetTriggerChannel(fields[2], slot, ch) {
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
