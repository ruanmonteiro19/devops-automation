package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ruanmonteiro19/observa-lite/internal/model"
)

func testDashboard() model.Dashboard {
	return model.Dashboard{ID: "ops", Title: "Operations", Panels: []model.Panel{{ID: "p1", Title: "Requests", Kind: "apm-stat", SourceID: "observa-apm", Query: `{"service_id":"svc:shop:checkout","metric":"requests"}`}}}
}
func TestDashboardPersistenceAndDetachedCopies(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state.json")
	st, err := New(file)
	if err != nil {
		t.Fatal(err)
	}
	dashboard := testDashboard()
	if err = st.PutDashboard(dashboard); err != nil {
		t.Fatal(err)
	}
	dashboard.Panels[0].Title = "input mutation"
	loaded, ok := st.Dashboard("ops")
	if !ok || loaded.Panels[0].Title != "Requests" {
		t.Fatal("input aliases store")
	}
	loaded.Panels[0].Title = "output mutation"
	snapshot := st.Snapshot()
	snapshot.Dashboards[0].Panels[0].Title = "snapshot mutation"
	loaded, _ = st.Dashboard("ops")
	if loaded.Panels[0].Title != "Requests" {
		t.Fatal("output aliases store")
	}
	loaded.Title = "Updated"
	loaded.Panels[0].Kind = "apm-series"
	if err = st.PutDashboard(loaded); err != nil {
		t.Fatal(err)
	}
	reloaded, err := New(file)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := reloaded.Dashboard("ops")
	if !ok || got.Title != "Updated" || got.Panels[0].Kind != "apm-series" {
		t.Fatalf("edit did not persist: %+v", got)
	}
	if err = reloaded.DeleteDashboard("ops"); err != nil {
		t.Fatal(err)
	}
	if err = reloaded.DeleteDashboard("ops"); !errors.Is(err, ErrDashboardNotFound) {
		t.Fatal(err)
	}
	blank := testDashboard()
	blank.ID = ""
	blank.Panels[0] = model.Panel{ID: "html", Title: "HTML", Kind: "HTML", HTML: "<b>ok</b>", JS: "window.example=1"}
	if err = st.PutDashboard(blank); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range st.Snapshot().Dashboards {
		if strings.HasPrefix(v.ID, "dash-") {
			found = true
			if v.Panels[0].Kind != "html" || v.Panels[0].JS != "window.example=1" {
				t.Fatal("legacy HTML changed")
			}
		}
	}
	if !found {
		t.Fatal("empty id was not generated")
	}
}
func TestDashboardValidationAndCapacity(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	for name, modify := range map[string]func(*model.Dashboard){
		"blank title":        func(v *model.Dashboard) { v.Title = " " },
		"long title":         func(v *model.Dashboard) { v.Title = strings.Repeat("x", 257) },
		"long id":            func(v *model.Dashboard) { v.ID = strings.Repeat("x", 129) },
		"panel limit":        func(v *model.Dashboard) { v.Panels = make([]model.Panel, 51) },
		"unknown kind":       func(v *model.Dashboard) { v.Panels[0].Kind = "pretend-live" },
		"duplicate panel id": func(v *model.Dashboard) { v.Panels = append(v.Panels, v.Panels[0]) },
		"wrong source":       func(v *model.Dashboard) { v.Panels[0].SourceID = "external" },
		"bad service":        func(v *model.Dashboard) { v.Panels[0].Query = `{"service_id":"checkout","metric":"requests"}` },
		"bad metric":         func(v *model.Dashboard) { v.Panels[0].Query = `{"service_id":"svc::checkout","metric":"fabricated"}` },
		"query injection": func(v *model.Dashboard) {
			v.Panels[0].Query = `{"service_id":"svc::checkout","metric":"requests","url":"http://external"}`
		},
		"html bound": func(v *model.Dashboard) { v.Panels[0].HTML = strings.Repeat("x", 100001) },
		"serialized bound": func(v *model.Dashboard) {
			v.Panels[0].Kind = "stat"
			v.Panels[0].Query = strings.Repeat("x", MaxDashboardBytes)
		},
	} {
		t.Run(name, func(t *testing.T) {
			v := testDashboard()
			modify(&v)
			if err := st.PutDashboard(v); !errors.Is(err, ErrDashboardValidation) {
				t.Fatalf("validation error missing: %v", err)
			}
		})
	}
	for i := 0; i < MaxDashboards; i++ {
		id := fmt.Sprintf("d%d", i)
		v := testDashboard()
		v.ID = id
		st.data.Dashboards[id] = v
	}
	if err = st.PutDashboard(testDashboard()); !errors.Is(err, ErrDashboardLimit) {
		t.Fatalf("capacity error missing: %v", err)
	}
	edit := testDashboard()
	edit.ID = "d0"
	if err = st.PutDashboard(edit); err != nil {
		t.Fatalf("update at capacity rejected: %v", err)
	}
}
func TestDashboardWritesRollbackOnPersistenceFailure(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state.json")
	st, err := New(file)
	if err != nil {
		t.Fatal(err)
	}
	v := testDashboard()
	if err = st.PutDashboard(v); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(file+".tmp", 0700); err != nil {
		t.Fatal(err)
	}
	edited := v
	edited.Title = "Not committed"
	if err = st.PutDashboard(edited); err == nil {
		t.Fatal("edit ignored disk failure")
	}
	loaded, _ := st.Dashboard(v.ID)
	if loaded.Title != v.Title {
		t.Fatal("failed edit mutated memory")
	}
	created := v
	created.ID = "new"
	if err = st.PutDashboard(created); err == nil {
		t.Fatal("create ignored disk failure")
	}
	if _, ok := st.Dashboard("new"); ok {
		t.Fatal("failed create mutated memory")
	}
	if err = st.DeleteDashboard(v.ID); err == nil {
		t.Fatal("delete ignored disk failure")
	}
	if _, ok := st.Dashboard(v.ID); !ok {
		t.Fatal("failed delete mutated memory")
	}
}

func TestDashboardPanelLayoutPersistsAndRejectsInvalidWidth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "layout.json")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	d := model.Dashboard{ID: "layout", Title: "Layout", Panels: []model.Panel{{ID: "a", Title: "First", Kind: "stat", Width: "full"}, {ID: "b", Title: "Second", Kind: "stat", Width: "half"}}}
	if err := s.PutDashboard(d); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	got := reopened.Snapshot().Dashboards
	if len(got) != 1 || got[0].Panels[0].Width != "full" || got[0].Panels[1].Width != "half" {
		t.Fatalf("layout lost: %+v", got)
	}
	d.Panels[0].Width = "invalid"
	if err := s.PutDashboard(d); err == nil {
		t.Fatal("invalid layout accepted")
	}
}
