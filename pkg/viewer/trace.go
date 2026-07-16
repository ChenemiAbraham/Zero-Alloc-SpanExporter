package viewer

import (
	"time"

	"github.com/yourusername/ltt/pkg/protocol"
)

// TraceTree represents a hierarchical trace structure
type TraceTree struct {
	Root  *TraceNode
	Nodes map[string]*TraceNode // spanID -> node
}

// TraceNode represents a single span in the tree
type TraceNode struct {
	Span     *protocol.SpanMessage
	Children []*TraceNode
	Parent   *TraceNode
	Depth    int

	// UI state
	Expanded bool
	Selected bool
}

// NewTraceTree creates a new empty trace tree
func NewTraceTree() *TraceTree {
	return &TraceTree{
		Nodes: make(map[string]*TraceNode),
	}
}

// AddSpan adds a span to the tree, organizing by parent relationships
func (t *TraceTree) AddSpan(span *protocol.SpanMessage) {
	spanID := spanIDString(span.SpanID)

	// Check if node already exists
	if _, exists := t.Nodes[spanID]; exists {
		return
	}

	node := &TraceNode{
		Span:     span,
		Expanded: true, // Default to expanded
		Children: make([]*TraceNode, 0),
	}

	t.Nodes[spanID] = node

	// Find parent
	parentID := spanIDString(span.ParentID)
	if parentID != "0000000000000000" {
		if parent, exists := t.Nodes[parentID]; exists {
			node.Parent = parent
			node.Depth = parent.Depth + 1
			parent.Children = append(parent.Children, node)
			return
		}
	}

	// No parent found, this is a root span
	if t.Root == nil {
		t.Root = node
		node.Depth = 0
	} else {
		// Multiple roots, attach to existing root
		node.Parent = t.Root
		node.Depth = 1
		t.Root.Children = append(t.Root.Children, node)
	}
}

// FlattenVisible returns a flat list of visible nodes (respecting expand/collapse)
func (t *TraceTree) FlattenVisible() []*TraceNode {
	if t.Root == nil {
		return nil
	}

	result := make([]*TraceNode, 0, len(t.Nodes))
	t.flattenRecursive(t.Root, &result)
	return result
}

func (t *TraceTree) flattenRecursive(node *TraceNode, result *[]*TraceNode) {
	*result = append(*result, node)

	if node.Expanded {
		for _, child := range node.Children {
			t.flattenRecursive(child, result)
		}
	}
}

// GetStats calculates trace statistics
func (t *TraceTree) GetStats() TraceStats {
	stats := TraceStats{
		TotalSpans: len(t.Nodes),
	}

	var totalDuration time.Duration
	errorCount := 0

	for _, node := range t.Nodes {
		duration := node.Span.EndTime.Sub(node.Span.StartTime)
		totalDuration += duration

		if node.Span.StatusCode != 0 { // 0 = OK
			errorCount++
		}
	}

	if len(t.Nodes) > 0 {
		stats.AvgLatency = totalDuration / time.Duration(len(t.Nodes))
	}

	stats.ErrorRate = float64(errorCount) / float64(len(t.Nodes)) * 100

	return stats
}

// TraceStats holds trace statistics
type TraceStats struct {
	TotalSpans int
	AvgLatency time.Duration
	ErrorRate  float64
}

// spanIDString converts a span ID byte array to hex string
func spanIDString(id [8]byte) string {
	var result string
	for _, b := range id {
		result += string(rune(b))
	}
	return result
}

// TraceFilter holds filter criteria
type TraceFilter struct {
	ServiceName   string
	OperationName string
	MinDuration   time.Duration
	MaxDuration   time.Duration
	OnlyErrors    bool
}

// Matches returns true if a span matches the filter
func (f *TraceFilter) Matches(span *protocol.SpanMessage) bool {
	if f.OperationName != "" && span.Name != f.OperationName {
		return false
	}

	duration := span.EndTime.Sub(span.StartTime)
	if f.MinDuration > 0 && duration < f.MinDuration {
		return false
	}
	if f.MaxDuration > 0 && duration > f.MaxDuration {
		return false
	}

	if f.OnlyErrors && span.StatusCode == 0 {
		return false
	}

	return true
}
