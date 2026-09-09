package services

import (
	"fmt"
	"math/rand/v2"
	"net/url"
	"strings"
)

type Route struct {
	Path   string
	Weight int
}

type Service struct {
	Name    string
	BaseURL string
	Routes  []Route
}

// All is the built-in fleet. Weights favour endpoints verified live during
// development; low-weight entries are plausible paths a browser hits anyway.
// Every request, including the 404s, is answered by the Express app itself and
// therefore resets Render's idle timer. /robots.txt is deliberately absent:
// Render answers it at the edge while a service is spun down.
var All = []Service{
	{
		Name:    "auth",
		BaseURL: "https://tomato-auth-14q4.onrender.com",
		Routes: []Route{
			{"/api/auth/me", 40},
			{"/api/auth/refresh", 4},
			{"/api/auth/session", 3},
			{"/api/auth/verify", 3},
			{"/api/user/me", 4},
			{"/api/profile", 3},
			{"/favicon.ico", 4},
			{"/", 6},
		},
	},
	{
		Name:    "realtime",
		BaseURL: "https://tomato-realtime-y7hj.onrender.com",
		Routes: []Route{
			{"/socket.io/?EIO=4&transport=polling", 55},
			{"/api/health", 3},
			{"/api/status", 3},
			{"/favicon.ico", 4},
			{"/", 5},
		},
	},
	{
		Name:    "restaurant",
		BaseURL: "https://tomato-restaurant-lpwy.onrender.com",
		Routes: []Route{
			{"/api/restaurant/list", 40},
			{"/api/food/list", 5},
			{"/api/food/search", 4},
			{"/api/menu", 3},
			{"/api/categories", 3},
			{"/api/cart/get", 3},
			{"/api/restaurants", 3},
			{"/favicon.ico", 4},
			{"/", 5},
		},
	},
	{
		Name:    "utils",
		BaseURL: "https://tomato-utils-i72q.onrender.com",
		Routes: []Route{
			{"/api/health", 8},
			{"/api/config", 6},
			{"/api/currency", 6},
			{"/api/upload", 5},
			{"/uploads", 5},
			{"/api/status", 5},
			{"/favicon.ico", 4},
			{"/", 7},
		},
	},
	{
		Name:    "rider",
		BaseURL: "https://tomato-rider-i9eu.onrender.com",
		Routes: []Route{
			{"/api/riders", 8},
			{"/api/orders", 8},
			{"/api/delivery", 6},
			{"/api/location", 5},
			{"/api/rider/status", 5},
			{"/api/health", 5},
			{"/favicon.ico", 4},
			{"/", 7},
		},
	},
	{
		Name:    "admin",
		BaseURL: "https://tomato-admin-a0nr.onrender.com",
		Routes: []Route{
			{"/api/v1/health", 25},
			{"/api/v1/stats", 12},
			{"/api/v1/orders", 12},
			{"/api/v1/users", 10},
			{"/api/v1/food", 8},
			{"/api/v1/menu", 8},
			{"/favicon.ico", 4},
			{"/", 5},
		},
	},
}

// PickRoute draws a route with probability proportional to its weight.
func (s Service) PickRoute(rng *rand.Rand) Route {
	total := 0
	for _, r := range s.Routes {
		total += r.Weight
	}
	n := rng.IntN(total)
	for _, r := range s.Routes {
		if n < r.Weight {
			return r
		}
		n -= r.Weight
	}
	return s.Routes[len(s.Routes)-1]
}

// ForShard partitions the fleet round-robin: shards never overlap and
// together cover every service exactly once.
func ForShard(all []Service, index, count int) []Service {
	if count <= 1 {
		return all
	}
	out := make([]Service, 0, len(all)/count+1)
	for i, s := range all {
		if i%count == index-1 {
			out = append(out, s)
		}
	}
	return out
}

// URL joins base and path; routes that already carry a query string keep it.
func (s Service) URL(path string) string {
	return s.BaseURL + path
}

func Validate(all []Service) error {
	for _, s := range all {
		u, err := url.Parse(s.BaseURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("service %q has invalid base URL %q", s.Name, s.BaseURL)
		}
		if len(s.Routes) == 0 {
			return fmt.Errorf("service %q has no routes", s.Name)
		}
		for _, r := range s.Routes {
			if r.Weight <= 0 {
				return fmt.Errorf("service %q route %q has non-positive weight", s.Name, r.Path)
			}
			if !strings.HasPrefix(r.Path, "/") {
				return fmt.Errorf("service %q route %q must start with /", s.Name, r.Path)
		}
		}
	}
	return nil
}
