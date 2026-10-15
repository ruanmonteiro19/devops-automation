package instrumentation

import (
	"os"
	"testing"
)

func TestExportIsEmbeddedAndDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	path, e := Export(dir)
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(path)
	if e != nil || string(b) != string(NodeModule) {
		t.Fatal("export mismatch", e)
	}
	if _, e = Export(dir); e == nil {
		t.Fatal("existing installation overwritten")
	}
}
