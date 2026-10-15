package vql

import "testing"

func TestPipelineTypedFilterAndAggregation(t *testing.T) {
	rows := []Row{{"name": "a|b", "n": 2.0, "group": "api"}, {"name": "c", "n": 4.0, "group": "api"}, {"name": "true", "n": "4", "group": "web"}}
	r, e := Execute(`fetch spans | filter n >= 2 | summarize count(), avg(n) by group`, rows)
	if e != nil || len(r.Rows) != 1 || r.Rows[0]["count()"] != 2 || r.Rows[0]["avg(n)"] != 3.0 {
		t.Fatalf("%+v %v", r, e)
	}
	r, e = Execute(`fetch hosts | filter name == "a|b" | fields name`, rows)
	if e != nil || len(r.Rows) != 1 || r.Rows[0]["name"] != "a|b" {
		t.Fatalf("%+v %v", r, e)
	}
	r, e = Execute(`fetch hosts | filter name == true`, rows)
	if e != nil || len(r.Rows) != 0 {
		t.Fatalf("typed equality: %+v %v", r, e)
	}
	for _, q := range []string{`fetch hosts | fields bad!`, `fetch hosts | summarize nope() by group`, `fetch hosts | filter n > NaN`, `fetch hosts | limit 1001`, `fetch unknown`, `fetch hosts | filter name == "open`} {
		if _, e := Execute(q, nil); e == nil {
			t.Errorf("accepted %s", q)
		}
	}
}
func TestOutputBoundAndNoMutation(t *testing.T) {
	rows := make([]Row, 16000)
	for i := range rows {
		rows[i] = Row{"n": i}
	}
	r, e := Execute(`fetch hosts | sort n desc | limit 10`, rows)
	if e != nil || len(r.Rows) != 10 || !r.Truncated || r.Rows[0]["n"] != 14999 || rows[0]["n"] != 0 {
		t.Fatalf("%+v %v", r, e)
	}
}
