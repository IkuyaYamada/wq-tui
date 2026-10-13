package ui

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// A node can carry the codex or Claude Code session working on it
// (session: in its header). gr resumes it in the directory it ran in.

// Where the agents keep their sessions; tests point these elsewhere.
var (
	codexSessions  = filepath.Join(homeOr("CODEX_HOME", ".codex"), "sessions")
	claudeSessions = filepath.Join(homeOr("CLAUDE_CONFIG_DIR", ".claude"), "projects")
)

func homeOr(env, dot string) string {
	if d := os.Getenv(env); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, dot)
}

// agentSession is one session file of either agent.
type agentSession struct {
	agent string // "codex" or "claude"
	id    string
	cwd   string
	path  string
	mod   time.Time
}

// findSession looks the session id up among both agents' files.
func findSession(id string) (agentSession, bool) {
	if id == "" || strings.ContainsAny(id, "/*?[") {
		return agentSession{}, false
	}
	if paths, _ := filepath.Glob(filepath.Join(codexSessions, "*", "*", "*", "rollout-*-"+id+".jsonl")); len(paths) > 0 {
		if s, ok := readSession("codex", paths[len(paths)-1]); ok {
			return s, true
		}
	}
	if paths, _ := filepath.Glob(filepath.Join(claudeSessions, "*", id+".jsonl")); len(paths) > 0 {
		if s, ok := readSession("claude", paths[0]); ok {
			return s, true
		}
	}
	return agentSession{}, false
}

// readSession reads a session file's id and working directory: codex puts
// them in its first line's payload, Claude Code in a cwd field of its
// early lines, its id being the file's name.
func readSession(agent, path string) (agentSession, bool) {
	st, err := os.Stat(path)
	if err != nil {
		return agentSession{}, false
	}
	s := agentSession{agent: agent, path: path, mod: st.ModTime()}
	if agent == "claude" {
		s.id = strings.TrimSuffix(filepath.Base(path), ".jsonl")
	}
	f, err := os.Open(path)
	if err != nil {
		return agentSession{}, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for i := 0; i < 50 && sc.Scan(); i++ {
		var line struct {
			Type    string `json:"type"`
			Cwd     string `json:"cwd"`
			Payload struct {
				ID  string `json:"id"`
				Cwd string `json:"cwd"`
			} `json:"payload"`
		}
		if json.Unmarshal(sc.Bytes(), &line) != nil {
			continue
		}
		if agent == "codex" && line.Type == "session_meta" {
			s.id, s.cwd = line.Payload.ID, line.Payload.Cwd
			break
		}
		if agent == "claude" && line.Cwd != "" {
			s.cwd = line.Cwd
			break
		}
	}
	return s, s.id != ""
}

// sessionDoneMsg reports that an agent run from a node has ended.
type sessionDoneMsg struct {
	id    string // the node
	agent string
	err   error
}

// resumeSession reopens the node's session where it ran: gr copies the
// command to run it in another terminal, so the board stays usable
// meanwhile; gR (inPlace) runs it here, wq stepping aside until it exits.
func (m *Model) resumeSession(inPlace bool) tea.Cmd {
	n := m.selected()
	if n == nil {
		return nil
	}
	if n.Session == "" {
		m.msg = "no session: put its id in the header's session:"
		return nil
	}
	s, ok := findSession(n.Session)
	if !ok {
		m.msg = "session " + n.Session + " not found"
		return nil
	}
	args := []string{"resume", s.id}
	if s.agent == "claude" {
		args = []string{"--resume", s.id}
	}
	if inPlace {
		return runAgent(n.ID, s.agent, s.cwd, args)
	}
	line := strings.Join(append([]string{s.agent}, args...), " ")
	if s.cwd != "" {
		line = "cd " + shellQuote(s.cwd) + " && " + line
	}
	if err := m.copyText(line); err != nil {
		m.msg = "gr: " + err.Error()
		return nil
	}
	m.msg = "copied " + line + " · paste it in another terminal (gR runs it here)"
	return nil
}

// shellQuote quotes s for a POSIX shell when it needs it.
func shellQuote(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._-+~@%:,=") == "" && s[0] != '~' {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func runAgent(id, agent, dir string, args []string) tea.Cmd {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	cmd := exec.Command(agent, args...)
	cmd.Dir = dir
	return tea.Exec(vimCmd{cmd}, func(err error) tea.Msg {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			err = nil // the agent's own exit status is not ours to report
		}
		return sessionDoneMsg{id: id, agent: agent, err: err}
	})
}

// sessionDone reports an agent that could not be run.
func (m *Model) sessionDone(msg sessionDoneMsg) {
	if msg.err != nil {
		m.msg = msg.agent + ": " + msg.err.Error()
	}
}
