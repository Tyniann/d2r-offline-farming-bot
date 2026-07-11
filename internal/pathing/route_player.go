package pathing

import (
	"context"
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// RoutePlayer chains validated segments in one continuously monitored session.
// Resume is only internal at a confirmed segment boundary; construction always starts at segment zero.
type RoutePlayer struct {
	navigator SegmentNavigator
	route     Route
	index     int
	segment   *RouteSegmentPlayer
	done      bool
}

// NewRoutePlayer creates full playback beginning at the first route segment.
func NewRoutePlayer(navigator SegmentNavigator, route Route) (*RoutePlayer, error) {
	if err := route.Validate(); err != nil {
		return nil, fmt.Errorf("route player: %w", err)
	}
	segment, err := NewRouteSegmentPlayer(navigator, route, 0)
	if err != nil {
		return nil, err
	}
	return &RoutePlayer{navigator: navigator, route: route, segment: segment}, nil
}

// Tick advances the active segment and opens the next only after confirmed completion.
func (p *RoutePlayer) Tick(ctx context.Context, state world.State) (bool, error) {
	if p.done {
		return true, nil
	}
	done, err := p.segment.Tick(ctx, state)
	if err != nil {
		p.navigator.Reset()
		return false, fmt.Errorf("segment %q: %w", p.segment.Segment().ID, err)
	}
	if !done {
		return false, nil
	}
	p.index++
	if p.index >= len(p.route.Segments) {
		p.done = true
		p.navigator.Reset()
		return true, nil
	}
	next, err := NewRouteSegmentPlayer(p.navigator, p.route, p.index)
	if err != nil {
		return false, err
	}
	p.segment = next
	return false, nil
}

// SegmentIndex returns the active zero-based segment index.
func (p *RoutePlayer) SegmentIndex() int { return p.index }

// Segment returns the active segment, or the final segment after completion.
func (p *RoutePlayer) Segment() RouteSegment { return p.segment.Segment() }

// Transitioning reports whether the active segment is awaiting its Area transition.
func (p *RoutePlayer) Transitioning() bool { return p.segment.Transitioning() }

// PointIndex returns the active segment's next point index.
func (p *RoutePlayer) PointIndex() int { return p.segment.PointIndex() }

// CurrentTarget returns the active segment's next recorded point.
func (p *RoutePlayer) CurrentTarget() (RoutePoint, bool) { return p.segment.CurrentTarget() }

// Reset aborts playback and clears the delegated navigator.
func (p *RoutePlayer) Reset() { p.navigator.Reset(); p.done = true }
