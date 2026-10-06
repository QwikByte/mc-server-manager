package logs

import (
	"log/slog"
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

func TestBuffer(t *testing.T) {
	b := NewBuffer()
	log := slog.New(b.Handler(slog.LevelInfo))
	for range keep + 1 {
		log.Info("entry", "server", "s1")
	}
	all, _ := b.after("", 0)
	if len(all) != keep-keep/10+1 || all[len(all)-1].GetSeq() != keep+1 || all[0].GetServerId() != "s1" {
		t.Fatalf("kept %d entries up to %d", len(all), all[len(all)-1].GetSeq())
	}
	newer, changed := b.after(b.boot, keep)
	if len(newer) != 1 {
		t.Fatalf("after %d: %d entries", keep, len(newer))
	}
	if again, _ := b.after("an earlier boot", keep); len(again) != len(all) {
		t.Fatalf("a reader of an earlier boot gets %d entries", len(again))
	}
	log.Info("new")
	select {
	case <-changed:
	default:
		t.Fatal("readers are not told about new entries")
	}
}

func TestCallDetails(t *testing.T) {
	for name, want := range map[string]string{"CreateCSR": "Create CSR", "GetServerProperties": "Get server properties"} {
		if got := describe(name); got != want {
			t.Errorf("describe(%s) = %q, want %q", name, got, want)
		}
	}
	upload := &noryxv1.WriteFileRequest{Content: &noryxv1.WriteFileRequest_Header{Header: &noryxv1.WriteFileHeader{ServerId: "s1", Path: "plugins/x.yml"}}}
	network := &noryxv1.ConfigureNetworkRequest{Id: "s1", ForwardingSecret: "secret"}
	set := &noryxv1.ApplyFileSetRequest{
		ServerId: "s1", SetId: "set1", SetName: "Plugins", Revision: "hash", Secrets: map[string]string{"secret:db": "secret"},
		Files: []*noryxv1.FileSetFile{{Path: "a.yml", Content: "password: {{secret:db}}"}},
	}
	for req, want := range map[proto.Message][]string{
		upload: {"path=plugins/x.yml", "server=s1"}, network: {"server=s1"}, set: {"server=s1", "set_id=set1", "set_name=Plugins"},
	} {
		var got []string
		for _, a := range details(req.ProtoReflect(), nil, true) {
			got = append(got, a.Key+"="+a.Value.String())
		}
		if slices.Sort(got); !slices.Equal(got, want) {
			t.Errorf("%T: logged %v, want %v", req, got, want)
		}
	}
}
