package settings

import (
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QwikByte/mc-server-manager/internal/master/database"
)

func TestPanelAddr(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	load := func() *Service {
		s, err := Load(t.Context(), db, Master{PanelDefaultAddr: "127.0.0.1:0"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	listen := func(s *Service) {
		ln, err := s.ListenPanel()
		if err != nil {
			t.Fatal(err)
		}
		ln.Close()
	}
	set := func(s *Service, addr string) error {
		next := s.Get()
		next.PanelAddr = addr
		_, err := s.Update(t.Context(), next)
		return err
	}
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	_, busyPort, _ := net.SplitHostPort(busy.Addr().String())
	free := freeAddr(t)

	// Without a setting, the panel listens at the address from the command line.
	s := load()
	listen(s)
	if s.master.PanelAddr != "127.0.0.1:0" || s.master.PanelAddrError != "" {
		t.Fatalf("master = %+v", s.master)
	}
	for _, bad := range []string{"localhost:8080", "127.0.0.1", "127.0.0.1:0", "127.0.0.1:70000", busy.Addr().String()} {
		if err := set(s, bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if err := set(s, busy.Addr().String()); !strings.Contains(err.Error(), "address already in use") {
		t.Errorf("busy port: %v", err)
	}
	if err := set(s, " "+free+" "); err != nil || s.Get().PanelAddr != free {
		t.Fatalf("set %s: %v, settings = %+v", free, err, s.Get())
	}

	// After a restart, the panel listens at the address from the settings.
	s = load()
	listen(s)
	if s.master.PanelAddr != free || s.master.PanelAddrError != "" {
		t.Fatalf("master = %+v", s.master)
	}

	// The port the panel listens at now is only free after a restart.
	s.master.PanelAddr = busy.Addr().String()
	if err := set(s, "0.0.0.0:"+busyPort); err != nil {
		t.Errorf("other interfaces at the current port: %v", err)
	}

	// If another program took the port in the meantime, the panel falls back.
	if err := set(s, free); err != nil {
		t.Fatal(err)
	}
	taken, err := net.Listen("tcp", free)
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	if err := set(s, free); err != nil {
		t.Errorf("unchanged address checked again: %v", err)
	}
	s = load()
	listen(s)
	if s.master.PanelAddr != "127.0.0.1:0" || !strings.Contains(s.master.PanelAddrError, "address already in use") {
		t.Fatalf("master = %+v", s.master)
	}
}

func freeAddr(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}
