package town

import (
	"container/heap"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
	"gopkg.in/yaml.v3"
)

// ServiceGraphVersion is the supported central Act-1 graph schema version.
const ServiceGraphVersion = 2

// ServiceGraph connects semantic Town anchors through independently recorded edges.
type ServiceGraph struct {
	Version int          `yaml:"version"`
	AreaID  world.AreaID `yaml:"area_id"`
	Edges   []GraphEdge  `yaml:"edges"`
}

// GraphEdge references one directed route recording between distinct anchors.
// Reversible edges may be replayed backwards only because the operator explicitly approved it.
type GraphEdge struct {
	ID         string              `yaml:"id"`
	From       Anchor              `yaml:"from"`
	To         Anchor              `yaml:"to"`
	Route      string              `yaml:"route,omitempty"` // Legacy migration source; never selected by layout-bound playback.
	Variants   []GraphRouteVariant `yaml:"variants,omitempty"`
	Cost       int                 `yaml:"cost"`
	Reversible bool                `yaml:"reversible"`
}

// GraphRouteVariant binds one edge recording to an exact Town layout fingerprint.
type GraphRouteVariant struct {
	Layout string `yaml:"layout"`
	Route  string `yaml:"route"`
}

// Traversal is one selected graph edge and its playback direction.
type Traversal struct {
	Edge    GraphEdge
	Reverse bool
}

// LoadServiceGraph loads and validates a central Act-1 graph manifest.
func LoadServiceGraph(path string) (ServiceGraph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ServiceGraph{}, fmt.Errorf("read service graph %q: %w", path, err)
	}
	var graph ServiceGraph
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&graph); err != nil {
		return ServiceGraph{}, fmt.Errorf("parse service graph %q: %w", path, err)
	}
	if err := graph.Validate(); err != nil {
		return ServiceGraph{}, fmt.Errorf("validate service graph %q: %w", path, err)
	}
	return graph, nil
}

// Validate rejects incomplete, ambiguous, or non-Act-1 graphs.
func (g ServiceGraph) Validate() error {
	if g.Version != ServiceGraphVersion {
		return fmt.Errorf("version: got %d, want %d", g.Version, ServiceGraphVersion)
	}
	if g.AreaID != world.RogueEncampment {
		return fmt.Errorf("area_id: got %d, want Rogue Encampment", g.AreaID)
	}
	if len(g.Edges) == 0 {
		return fmt.Errorf("edges must not be empty")
	}
	ids := map[string]bool{}
	connected := map[Anchor]bool{}
	for i, edge := range g.Edges {
		if strings.TrimSpace(edge.ID) == "" {
			return fmt.Errorf("edges[%d]: id is required", i)
		}
		if ids[edge.ID] {
			return fmt.Errorf("edges[%d]: duplicate id %q", i, edge.ID)
		}
		ids[edge.ID] = true
		if !knownGraphAnchor(edge.From) || !knownGraphAnchor(edge.To) || edge.From == edge.To {
			return fmt.Errorf("edges[%d]: invalid anchors %q → %q", i, edge.From, edge.To)
		}
		if edge.Cost <= 0 {
			return fmt.Errorf("edges[%d].cost must be > 0", i)
		}
		if edge.Route != "" && (filepath.IsAbs(edge.Route) || filepath.Clean(edge.Route) != edge.Route || strings.HasPrefix(edge.Route, "..")) {
			return fmt.Errorf("edges[%d].route must stay inside graph directory", i)
		}
		layouts := map[string]bool{}
		for j, variant := range edge.Variants {
			if len(variant.Layout) != 64 || strings.TrimSpace(variant.Route) == "" || layouts[variant.Layout] {
				return fmt.Errorf("edges[%d].variants[%d]: unique SHA-256 layout and route are required", i, j)
			}
			if _, err := hex.DecodeString(variant.Layout); err != nil {
				return fmt.Errorf("edges[%d].variants[%d].layout: %w", i, j, err)
			}
			if filepath.IsAbs(variant.Route) || filepath.Clean(variant.Route) != variant.Route || strings.HasPrefix(variant.Route, "..") {
				return fmt.Errorf("edges[%d].variants[%d].route must stay inside graph directory", i, j)
			}
			layouts[variant.Layout] = true
		}
		connected[edge.From], connected[edge.To] = true, true
	}
	for _, anchor := range []Anchor{AnchorPortalArrival, AnchorStash, AnchorWaypoint, AnchorAkara, AnchorCharsi, AnchorCain} {
		if !connected[anchor] {
			return fmt.Errorf("anchor %q is not connected", anchor)
		}
		if !g.canReach(anchor, AnchorWaypoint) {
			return fmt.Errorf("anchor %q cannot reach waypoint", anchor)
		}
	}
	return nil
}

// RouteForLayout selects a lowest-cost route using only variants for layout.
func (g ServiceGraph) RouteForLayout(layout string, start Anchor, required []Anchor, end Anchor) ([]Traversal, error) {
	if len(layout) != 64 {
		return nil, fmt.Errorf("%s", ReasonTownLayoutUnavailable)
	}
	bound := g
	bound.Edges = make([]GraphEdge, 0, len(g.Edges))
	for _, edge := range g.Edges {
		for _, variant := range edge.Variants {
			if variant.Layout == layout {
				edge.Route = variant.Route
				bound.Edges = append(bound.Edges, edge)
				break
			}
		}
	}
	if len(bound.Edges) == 0 {
		return nil, fmt.Errorf("%s", ReasonTownLayoutRouteMissing)
	}
	if route, err := bound.routeAvailable(start, required, end); err == nil {
		return route, nil
	}
	return nil, fmt.Errorf("%s", ReasonTownLayoutRouteMissing)
}

func (g ServiceGraph) canReach(start, end Anchor) bool {
	seen := map[Anchor]bool{start: true}
	queue := []Anchor{start}
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		if at == end {
			return true
		}
		for _, next := range g.neighbors(at) {
			if !seen[next.to] {
				seen[next.to] = true
				queue = append(queue, next.to)
			}
		}
	}
	return false
}

// Edge returns a graph edge by stable ID.
func (g ServiceGraph) Edge(id string) (GraphEdge, bool) {
	for _, edge := range g.Edges {
		if edge.ID == id {
			return edge, true
		}
	}
	return GraphEdge{}, false
}

// Route finds the lowest-cost path from start through every required anchor to end.
// Required anchors are unordered so the graph, rather than service declaration order,
// chooses the route that avoids unnecessary detours.
func (g ServiceGraph) Route(start Anchor, required []Anchor, end Anchor) ([]Traversal, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}
	return g.routeAvailable(start, required, end)
}

func (g ServiceGraph) routeAvailable(start Anchor, required []Anchor, end Anchor) ([]Traversal, error) {
	if !knownGraphAnchor(start) || !knownGraphAnchor(end) {
		return nil, fmt.Errorf("unknown start or end anchor")
	}
	start = canonicalGraphAnchor(start)
	end = canonicalGraphAnchor(end)
	bit := map[Anchor]uint64{}
	for _, anchor := range required {
		if !knownGraphAnchor(anchor) {
			return nil, fmt.Errorf("unknown required anchor %q", anchor)
		}
		anchor = canonicalGraphAnchor(anchor)
		if _, ok := bit[anchor]; !ok {
			bit[anchor] = uint64(1) << len(bit)
		}
	}
	all := uint64(0)
	for _, value := range bit {
		all |= value
	}
	mask := uint64(0)
	if value, ok := bit[start]; ok {
		mask |= value
	}
	startKey := routeKey{start, mask}
	dist := map[routeKey]int{startKey: 0}
	prev := map[routeKey]previous{}
	q := &routeQueue{{at: start, mask: mask, cost: 0}}
	heap.Init(q)
	for q.Len() > 0 {
		current := heap.Pop(q).(routeState)
		currentKey := routeKey{current.at, current.mask}
		if best := dist[currentKey]; current.cost != best {
			continue
		}
		if current.at == end && current.mask == all {
			return rebuildRoute(prev, currentKey, startKey), nil
		}
		for _, next := range g.neighbors(current.at) {
			nextMask := current.mask
			if value, ok := bit[next.to]; ok {
				nextMask |= value
			}
			nextKey := routeKey{next.to, nextMask}
			nextCost := current.cost + next.edge.Cost
			if old, ok := dist[nextKey]; ok && old <= nextCost {
				continue
			}
			dist[nextKey] = nextCost
			prev[nextKey] = previous{fromAt: current.at, fromMask: current.mask, traversal: Traversal{Edge: next.edge, Reverse: next.reverse}}
			heap.Push(q, routeState{at: next.to, mask: nextMask, cost: nextCost})
		}
	}
	return nil, fmt.Errorf("no graph route from %q through %v to %q", start, required, end)
}

type neighbor struct {
	to      Anchor
	edge    GraphEdge
	reverse bool
}

func (g ServiceGraph) neighbors(at Anchor) []neighbor {
	out := make([]neighbor, 0)
	for _, edge := range g.Edges {
		if edge.From == at {
			out = append(out, neighbor{to: edge.To, edge: edge})
		}
		if edge.Reversible && edge.To == at {
			out = append(out, neighbor{to: edge.From, edge: edge, reverse: true})
		}
	}
	return out
}

type previous struct {
	fromAt    Anchor
	fromMask  uint64
	traversal Traversal
}
type routeKey struct {
	at   Anchor
	mask uint64
}
type routeState struct {
	at    Anchor
	mask  uint64
	cost  int
	index int
}
type routeQueue []routeState

func (q routeQueue) Len() int           { return len(q) }
func (q routeQueue) Less(i, j int) bool { return q[i].cost < q[j].cost }
func (q routeQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i]; q[i].index = i; q[j].index = j }
func (q *routeQueue) Push(value any)    { *q = append(*q, value.(routeState)) }
func (q *routeQueue) Pop() any {
	old := *q
	value := old[len(old)-1]
	*q = old[:len(old)-1]
	return value
}

func rebuildRoute(prev map[routeKey]previous, current, start routeKey) []Traversal {
	reversed := make([]Traversal, 0)
	for current != start {
		step := prev[current]
		reversed = append(reversed, step.traversal)
		current = routeKey{step.fromAt, step.fromMask}
	}
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return reversed
}

func knownGraphAnchor(anchor Anchor) bool {
	switch anchor {
	case AnchorSpawn, AnchorPortalArrival, AnchorStash, AnchorWaypoint, AnchorAkara, AnchorCharsi, AnchorCain:
		return true
	}
	return false
}

func canonicalGraphAnchor(anchor Anchor) Anchor {
	if anchor == AnchorSpawn {
		return AnchorStash
	}
	return anchor
}

// EgressRoute describes the intentionally minimal foreign-act route manifest.
type EgressRoute struct {
	Act     OriginAct `yaml:"act"`
	Area    string    `yaml:"area"`
	RouteID string    `yaml:"route_id"`
	Anchors []Anchor  `yaml:"anchors"`
}

// LoadEgressRoute loads and validates a foreign-act egress manifest.
func LoadEgressRoute(path string) (EgressRoute, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return EgressRoute{}, fmt.Errorf("read egress route %q: %w", path, err)
	}
	var route EgressRoute
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&route); err != nil {
		return EgressRoute{}, fmt.Errorf("parse egress route %q: %w", path, err)
	}
	if err := route.Validate(); err != nil {
		return EgressRoute{}, fmt.Errorf("validate egress route %q: %w", path, err)
	}
	return route, nil
}

// Validate rejects egress manifests that try to become a second service graph.
func (r EgressRoute) Validate() error {
	if r.Act == OriginActUnknown || r.Act == OriginAct1 || strings.TrimSpace(r.Area) == "" || strings.TrimSpace(r.RouteID) == "" {
		return fmt.Errorf("act, area, and route_id are required for a foreign egress")
	}
	if len(r.Anchors) != 2 || r.Anchors[0] != AnchorPortalArrival || r.Anchors[1] != AnchorWaypoint {
		return fmt.Errorf("egress anchors must be portal_arrival, waypoint")
	}
	return nil
}
