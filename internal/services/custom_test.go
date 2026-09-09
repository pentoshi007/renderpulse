package services

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestComposeDefaultsToBuiltinFleet(t *testing.T) {
	fleet, err := Compose(AddOptions{})
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if len(fleet) != len(All) {
		t.Fatalf("default fleet = %d services, want %d", len(fleet), len(All))
	}
}

func TestComposeRemove(t *testing.T) {
	if _, err := Compose(AddOptions{Removes: []string{"rider", "nosuch"}}); err == nil {
		t.Fatal("removing an unknown service must fail")
	}
	fleet, err := Compose(AddOptions{Removes: []string{"rider"}})
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	for _, s := range fleet {
		if s.Name == "rider" {
			t.Error("rider should have been removed")
		}
	}
	if len(fleet) != len(All)-1 {
		t.Errorf("fleet = %d services, want %d", len(fleet), len(All)-1)
	}
}

func TestComposeAddReplacesBuiltinByName(t *testing.T) {
	fleet, err := Compose(AddOptions{Adds: []string{"auth=https://other.example.com"}})
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if len(fleet) != len(All) {
		t.Errorf("replacing a built-in must not grow the fleet: %d", len(fleet))
	}
	for _, s := range fleet {
		if s.Name == "auth" && s.BaseURL != "https://other.example.com" {
			t.Errorf("auth not replaced, base url = %s", s.BaseURL)
		}
	}
}

func TestComposeNoBuiltinOnlyAdds(t *testing.T) {
	fleet, err := Compose(AddOptions{NoBuiltin: true, Adds: []string{"a=https://a.example.com", "b=https://b.example.com/"}})
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if len(fleet) != 2 {
		t.Fatalf("fleet = %d services, want 2", len(fleet))
	}
	if fleet[1].BaseURL != "https://b.example.com" {
		t.Errorf("trailing slash not stripped: %q", fleet[1].BaseURL)
	}
	for _, s := range fleet {
		if !reflect.DeepEqual(s.Routes, GenericRoutes) {
			t.Errorf("service %s should get the generic route pool", s.Name)
		}
	}
}

func TestComposeBareURLAddDerivesName(t *testing.T) {
	fleet, err := Compose(AddOptions{NoBuiltin: true, Adds: []string{"https://blog.onrender.com"}})
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if fleet[0].Name != "blog" {
		t.Errorf("bare URL name = %q, want blog", fleet[0].Name)
	}
}

func TestComposeEmptyFleetFails(t *testing.T) {
	if _, err := Compose(AddOptions{NoBuiltin: true}); err == nil {
		t.Error("--no-builtin with nothing added must fail")
	}
	if _, err := Compose(AddOptions{Removes: []string{"auth", "realtime", "restaurant", "utils", "rider", "admin"}}); err == nil {
		t.Error("removing every service must fail")
	}
}

func TestComposeServicesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "svc.json")
	bad := `[{"name":"api","url":"ftp://nope.example.com"}]`
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Compose(AddOptions{File: path}); err == nil {
		t.Error("non-http(s) url must be rejected")
	}

	body := `[{"name":"api","url":"api.example.com","routes":[{"path":"/v1","weight":3},{"path":"/v2"}]}]`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	fleet, err := Compose(AddOptions{File: path})
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	var api *Service
	for i := range fleet {
		if fleet[i].Name == "api" {
			api = &fleet[i]
		}
	}
	if api == nil {
		t.Fatal("file service api missing from fleet")
	}
	if api.BaseURL != "https://api.example.com" {
		t.Errorf("schemeless url not normalized: %q", api.BaseURL)
	}
	want := []Route{{Path: "/v1", Weight: 3}, {Path: "/v2", Weight: 1}}
	if !reflect.DeepEqual(api.Routes, want) {
		t.Errorf("routes = %+v, want %+v", api.Routes, want)
	}
}

func TestComposeBadJSONFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Compose(AddOptions{File: path}); err == nil {
		t.Error("invalid JSON must fail")
	}
}

func TestForShardCoversComposedFleet(t *testing.T) {
	fleet, err := Compose(AddOptions{Adds: []string{"g=https://g.example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for idx := 1; idx <= 2; idx++ {
		for _, s := range ForShard(fleet, idx, 2) {
			seen[s.Name]++
		}
	}
	if len(seen) != len(fleet) {
		t.Fatalf("shards covered %d of %d services", len(seen), len(fleet))
	}
	for name, n := range seen {
		if n != 1 {
			t.Errorf("service %s covered %d times, want 1", name, n)
		}
	}
}
