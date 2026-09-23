package discordbot

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

type processLock struct {
	path string
	file *os.File
}

type lockRecord struct {
	Kind      string `json:"kind"`
	PID       int    `json:"pid"`
	CreatedAt string `json:"createdAt"`
}

func acquireBotLock(dir string) (*processLock, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "bot.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, errors.New("another bot process already holds bot.lock")
		}
		return nil, err
	}
	rec := lockRecord{Kind: "skill-issues-bot-lock", PID: os.Getpid(), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	body, _ := json.Marshal(rec)
	_ = f.Truncate(0)
	_, _ = f.WriteAt(append(body, '\n'), 0)
	_ = f.Sync()
	return &processLock{path: path, file: f}, nil
}

func (l *processLock) release() {
	if l == nil || l.file == nil {
		return
	}
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	_ = l.file.Close()
}

func writeHeartbeat(dir string) {
	_ = os.MkdirAll(dir, 0o700)
	path := filepath.Join(dir, "bot.pid")
	_ = os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600)
}
