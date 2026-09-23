package discordbot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireBotLockExclusive(t *testing.T) {
	dir := t.TempDir()
	first, err := acquireBotLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer first.release()
	if _, err := acquireBotLock(dir); err == nil {
		t.Fatal("second lock must fail")
	}
	first.release()
	second, err := acquireBotLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	second.release()
	if _, err := os.Stat(filepath.Join(dir, "bot.lock")); err != nil {
		t.Fatal(err)
	}
}
