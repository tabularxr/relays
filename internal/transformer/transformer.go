package transformer

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tabular/relay/pkg/types"
)

// Transformer converts StreamPackets to SpatialEvents
type Transformer struct {
	// Track anchors for generating consistent IDs
	anchorMap map[string]string // sessionID -> anchorID mapping
}

// New creates a new Transformer instance
func New() *Transformer {
	return &Transformer{
		anchorMap: make(map[string]string),
	}
}

// Transform converts a StreamPacket to a SpatialEvent
func (t *Transformer) Transform(packet types.StreamPacket) (*types.SpatialEvent, error) {
	// Generate event ID
	eventID := uuid.New().String()
	
	// Create base event
	event := &types.SpatialEvent{
		SessionID: packet.SessionID,
		EventID:   eventID,
		Timestamp: packet.Timestamp,
		Anchors:   []types.Anchor{},
		Meshes:    []types.MeshDiff{},
	}

	// Process based on packet type
	switch packet.Type {
	case "pose":
		return t.transformPose(event, packet)
	case "mesh":
		return t.transformMesh(event, packet)
	default:
		return event, nil
	}
}

// transformPose handles pose packet transformation
func (t *Transformer) transformPose(event *types.SpatialEvent, packet types.StreamPacket) (*types.SpatialEvent, error) {
	if packet.Data.Pose == nil {
		return event, nil
	}

	// Generate or retrieve anchor ID for this session
	anchorID := t.getOrCreateAnchorID(packet.SessionID)
	
	// Create anchor from pose
	anchor := types.Anchor{
		ID:        anchorID,
		Pose:      *packet.Data.Pose,
		Timestamp: packet.Timestamp,
	}

	event.Anchors = append(event.Anchors, anchor)
	return event, nil
}

// transformMesh handles mesh packet transformation
func (t *Transformer) transformMesh(event *types.SpatialEvent, packet types.StreamPacket) (*types.SpatialEvent, error) {
	if packet.Data.Mesh == nil {
		return event, nil
	}

	mesh := packet.Data.Mesh
	
	// Create mesh diff (initially as full mesh, not delta)
	meshDiff := types.MeshDiff{
		AnchorID:      mesh.AnchorID,
		VerticesDelta: mesh.Vertices,
		FacesDelta:    mesh.Faces,
		IsDelta:       false, // Full mesh initially
	}

	event.Meshes = append(event.Meshes, meshDiff)
	return event, nil
}

// getOrCreateAnchorID generates or retrieves an anchor ID for a session
func (t *Transformer) getOrCreateAnchorID(sessionID string) string {
	if anchorID, exists := t.anchorMap[sessionID]; exists {
		return anchorID
	}
	
	// Generate new anchor ID
	anchorID := "anchor_" + uuid.New().String()
	t.anchorMap[sessionID] = anchorID
	return anchorID
}

// NormalizeTimestamp ensures timestamp is in Unix milliseconds
func (t *Transformer) NormalizeTimestamp(timestamp int64) int64 {
	now := time.Now().UnixMilli()
	
	// If timestamp is in seconds, convert to milliseconds
	if timestamp < 1000000000000 { // Less than year 2001 in milliseconds
		timestamp *= 1000
	}
	
	// If timestamp is in the future or too far in the past, use current time
	if timestamp > now+60000 || timestamp < now-3600000 { // Within 1 minute future or 1 hour past
		return now
	}
	
	return timestamp
}

// ValidateEvent performs final validation on the transformed event
func (t *Transformer) ValidateEvent(event *types.SpatialEvent) error {
	if event.SessionID == "" {
		return fmt.Errorf("missing session ID")
	}
	if event.EventID == "" {
		return fmt.Errorf("missing event ID")
	}
	if event.Timestamp <= 0 {
		return fmt.Errorf("invalid timestamp")
	}
	return nil
}

// GetStats returns transformer statistics
func (t *Transformer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"active_sessions": len(t.anchorMap),
		"anchor_mappings": t.anchorMap,
	}
}

// ClearStaleSession removes old session mappings
func (t *Transformer) ClearStaleSession(sessionID string) {
	delete(t.anchorMap, sessionID)
}