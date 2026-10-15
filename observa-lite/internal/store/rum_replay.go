package store

import (
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"regexp"
	"strings"
)

var replayTag = regexp.MustCompile(`^[a-z][a-z0-9-]{0,40}$`)
var replayStyleValue = regexp.MustCompile(`^[a-zA-Z0-9#(),.% +\-]+$`)
var replayStyleKeys = map[string]bool{}

func init() {
	for _, k := range strings.Fields("display position top left right bottom width height min-width max-width min-height max-height box-sizing margin margin-top margin-left margin-right margin-bottom padding padding-top padding-left padding-right padding-bottom border border-width border-style border-color border-radius color background-color font-size font-weight font-family line-height text-align white-space overflow overflow-x overflow-y flex flex-direction flex-wrap justify-content align-items align-self gap grid-template-columns grid-template-rows opacity visibility transform transform-origin") {
		replayStyleKeys[k] = true
	}
}
func replayStyle(v any) string {
	s, _ := v.(string)
	if len(s) > 5000 {
		return ""
	}
	safe := []string{}
	for _, decl := range strings.Split(s, ";") {
		kv := strings.SplitN(decl, ":", 2)
		if len(kv) != 2 {
			continue
		}
		k, v := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
		if replayStyleKeys[k] && replayStyleValue.MatchString(v) && !strings.Contains(strings.ToLower(v), "url") {
			safe = append(safe, k+":"+v)
		}
	}
	return strings.Join(safe, ";")
}
func num(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return 0
	}
}
func safeID(v any) any {
	n := num(v)
	if n < -1 || n > 1e9 {
		return float64(-1)
	}
	return n
}
func replayAttrs(v any) map[string]any {
	out := map[string]any{}
	m, _ := v.(map[string]any)
	for k, v := range m {
		switch k {
		case "style":
			out[k] = replayStyle(v)
		case "rr_width", "rr_height":
			s, _ := v.(string)
			if len(s) < 30 && replayStyleValue.MatchString(s) {
				out[k] = s
			}
		case "type":
			s, _ := v.(string)
			switch s {
			case "button", "submit", "checkbox", "radio", "text", "password", "email", "number":
				out[k] = s
			}
		case "colspan", "rowspan":
			s, _ := v.(string)
			if len(s) <= 3 {
				out[k] = s
			}
		}
	}
	return out
}
func safeText(v any, allow bool) string {
	if !allow {
		return "***"
	}
	s, _ := v.(string)
	if len(s) > 2048 {
		s = s[:2048]
	}
	return s
}
func replayNode(v any, a model.RUMApplication, budget *int, depth int) (map[string]any, error) {
	*budget--
	if *budget < 0 || depth > 80 {
		return nil, fmt.Errorf("replay excede limite de nós")
	}
	n, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("nó inválido")
	}
	t := int(num(n["type"]))
	out := map[string]any{"type": t, "id": safeID(n["id"])}
	switch t {
	case 0:
	case 1:
		out["name"] = "html"
		out["publicId"] = ""
		out["systemId"] = ""
	case 2:
		tag, _ := n["tagName"].(string)
		tag = strings.ToLower(tag)
		if !replayTag.MatchString(tag) {
			tag = "div"
		}
		switch tag {
		case "script", "style", "link", "meta", "base", "iframe", "frame", "object", "embed", "svg", "math", "video", "audio", "canvas", "img", "source":
			return map[string]any{"type": 2, "id": safeID(n["id"]), "tagName": "div", "attributes": replayAttrs(n["attributes"]), "childNodes": []any{}}, nil
		}
		if strings.Contains(tag, "-") {
			tag = "div"
		}
		out["tagName"] = tag
		out["attributes"] = replayAttrs(n["attributes"])
	case 3:
		out["textContent"] = safeText(n["textContent"], a.Masking == "allowlist" && n["vetraPublic"] == true)
		return out, nil
	default:
		out["type"] = 3
		out["textContent"] = ""
		return out, nil
	}
	children := []any{}
	raw, _ := n["childNodes"].([]any)
	for _, c := range raw {
		child, e := replayNode(c, a, budget, depth+1)
		if e != nil {
			return nil, e
		}
		children = append(children, child)
	}
	out["childNodes"] = children
	return out, nil
}
func SanitizeReplay(events []map[string]any, a model.RUMApplication) ([]map[string]any, error) {
	out := []map[string]any{}
	for _, e := range events {
		t := int(num(e["type"]))
		d, _ := e["data"].(map[string]any)
		safe := map[string]any{}
		budget := 6000
		switch t {
		case 0, 1:
		case 2:
			n, err := replayNode(d["node"], a, &budget, 0)
			if err != nil {
				return nil, err
			}
			safe["node"] = n
			safe["initialOffset"] = map[string]any{"top": 0, "left": 0}
		case 4:
			href, _ := d["href"].(string)
			if rumExcluded(a, href) {
				return nil, fmt.Errorf("página excluída")
			}
			safe = map[string]any{"href": RUMSafeURL(href, a.CapturePath), "width": num(d["width"]), "height": num(d["height"])}
		case 3:
			source := int(num(d["source"]))
			safe["source"] = source
			switch source {
			case 0:
				for _, key := range []string{"texts", "attributes", "removes", "adds"} {
					list, _ := d[key].([]any)
					if len(list) > 6000 {
						return nil, fmt.Errorf("mutações excessivas")
					}
					items := []any{}
					for _, raw := range list {
						v, _ := raw.(map[string]any)
						item := map[string]any{"id": safeID(v["id"])}
						switch key {
						case "texts":
							item["value"] = safeText(v["value"], a.Masking == "allowlist" && v["vetraPublic"] == true)
						case "attributes":
							item["attributes"] = replayAttrs(v["attributes"])
						case "removes":
							item["parentId"] = safeID(v["parentId"])
						case "adds":
							delete(item, "id")
							item["parentId"] = safeID(v["parentId"])
							item["nextId"] = safeID(v["nextId"])
							n, err := replayNode(v["node"], a, &budget, 0)
							if err != nil {
								return nil, err
							}
							item["node"] = n
						}
						items = append(items, item)
					}
					safe[key] = items
				}
			case 1, 6:
				positions, _ := d["positions"].([]any)
				if len(positions) > 500 {
					return nil, fmt.Errorf("movimentos excessivos")
				}
				safe["positions"] = []any{}
				for _, p := range positions {
					v, _ := p.(map[string]any)
					safe["positions"] = append(safe["positions"].([]any), map[string]any{"id": safeID(v["id"]), "x": num(v["x"]), "y": num(v["y"]), "timeOffset": num(v["timeOffset"])})
				}
			case 2, 3, 4, 5:
				for _, key := range []string{"id", "type", "x", "y", "width", "height"} {
					if _, ok := d[key]; ok {
						safe[key] = num(d[key])
					}
				}
				if source == 5 {
					safe["text"] = "***"
					safe["isChecked"] = false
				}
			default:
				continue
			}
		default:
			continue
		}
		out = append(out, map[string]any{"type": t, "timestamp": num(e["timestamp"]), "data": safe})
	}
	return out, nil
}
