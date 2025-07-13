package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tabular/relay/internal/transformer"
	"github.com/tabular/relay/pkg/types"
)

func TestTransformer_NewTransformer(t *testing.T) {
	tr := transformer.New()
	assert.NotNil(t, tr)
}

func TestTransformer_TransformPosePacket(t *testing.T) {
	tr := transformer.New()
	
	packet := types.StreamPacket{
		SessionID:   "test-session",
		FrameNumber: 1,
		Timestamp:   time.Now().UnixMilli(),
		Type:        "pose",
		Data: types.PacketData{
			Pose: &types.PoseData{
				X:        1.0,
				Y:        2.0,
				Z:        3.0,
				Rotation: [4]float64{0, 0, 0, 1},
			},
		},
	}
	
	event, err := tr.Transform(packet)
	require.NoError(t, err)
	assert.NotNil(t, event)
	
	// Check basic event structure
	assert.Equal(t, "test-session", event.SessionID)
	assert.NotEmpty(t, event.EventID)
	assert.Equal(t, packet.Timestamp, event.Timestamp)
	
	// Check anchors
	assert.Len(t, event.Anchors, 1)
	anchor := event.Anchors[0]
	assert.NotEmpty(t, anchor.ID)
	assert.Equal(t, packet.Data.Pose.X, anchor.Pose.X)
	assert.Equal(t, packet.Data.Pose.Y, anchor.Pose.Y)
	assert.Equal(t, packet.Data.Pose.Z, anchor.Pose.Z)
	
	// Check meshes (should be empty for pose packet)
	assert.Len(t, event.Meshes, 0)
}

func TestTransformer_TransformMeshPacket(t *testing.T) {
	tr := transformer.New()
	
	testVertices := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	testFaces := []byte{0, 1, 2, 3}
	
	packet := types.StreamPacket{
		SessionID:   "test-session",
		FrameNumber: 1,
		Timestamp:   time.Now().UnixMilli(),
		Type:        "mesh",
		Data: types.PacketData{
			Mesh: &types.MeshData{
				Vertices: testVertices,
				Faces:    testFaces,
				AnchorID: "anchor-123",
			},
		},
	}
	
	event, err := tr.Transform(packet)
	require.NoError(t, err)
	assert.NotNil(t, event)
	
	// Check basic event structure
	assert.Equal(t, "test-session", event.SessionID)
	assert.NotEmpty(t, event.EventID)
	
	// Check anchors (should be empty for mesh packet)
	assert.Len(t, event.Anchors, 0)
	
	// Check meshes
	assert.Len(t, event.Meshes, 1)
	mesh := event.Meshes[0]
	assert.Equal(t, "anchor-123", mesh.AnchorID)
	assert.Equal(t, testVertices, mesh.VerticesDelta)
	assert.Equal(t, testFaces, mesh.FacesDelta)
	assert.False(t, mesh.IsDelta) // Should be full mesh initially
}

func TestTransformer_ConsistentAnchorID(t *testing.T) {
	tr := transformer.New()
	
	sessionID := "test-session"
	
	// Create two pose packets for the same session
	packet1 := types.StreamPacket{
		SessionID: sessionID,
		Timestamp: time.Now().UnixMilli(),
		Type:      "pose",
		Data: types.PacketData{
			Pose: &types.PoseData{
				X:        1.0,
				Y:        2.0,
				Z:        3.0,
				Rotation: [4]float64{0, 0, 0, 1},
			},
		},
	}
	
	packet2 := types.StreamPacket{
		SessionID: sessionID,
		Timestamp: time.Now().UnixMilli() + 1000,
		Type:      "pose",
		Data: types.PacketData{
			Pose: &types.PoseData{
				X:        2.0,
				Y:        3.0,
				Z:        4.0,
				Rotation: [4]float64{0, 0, 0, 1},
			},
		},
	}
	
	event1, err := tr.Transform(packet1)
	require.NoError(t, err)
	
	event2, err := tr.Transform(packet2)
	require.NoError(t, err)
	
	// Both events should have the same anchor ID for the same session
	assert.Equal(t, event1.Anchors[0].ID, event2.Anchors[0].ID)
}

func TestTransformer_DifferentSessionsDifferentAnchors(t *testing.T) {
	tr := transformer.New()
	
	packet1 := types.StreamPacket{
		SessionID: "session-1",
		Timestamp: time.Now().UnixMilli(),
		Type:      "pose",
		Data: types.PacketData{
			Pose: &types.PoseData{
				X:        1.0,
				Y:        2.0,
				Z:        3.0,
				Rotation: [4]float64{0, 0, 0, 1},
			},
		},
	}
	
	packet2 := types.StreamPacket{
		SessionID: "session-2",
		Timestamp: time.Now().UnixMilli(),
		Type:      "pose",
		Data: types.PacketData{
			Pose: &types.PoseData{
				X:        1.0,
				Y:        2.0,
				Z:        3.0,
				Rotation: [4]float64{0, 0, 0, 1},
			},
		},
	}
	
	event1, err := tr.Transform(packet1)
	require.NoError(t, err)
	
	event2, err := tr.Transform(packet2)
	require.NoError(t, err)
	
	// Different sessions should have different anchor IDs
	assert.NotEqual(t, event1.Anchors[0].ID, event2.Anchors[0].ID)
}

func TestTransformer_NormalizeTimestamp(t *testing.T) {
	tr := transformer.New()
	
	tests := []struct {
		name      string
		timestamp int64
		check     func(int64) bool
	}{
		{
			name:      "milliseconds timestamp",
			timestamp: time.Now().UnixMilli(),
			check: func(result int64) bool {
				return result == time.Now().UnixMilli() || result == time.Now().UnixMilli()-1
			},
		},
		{
			name:      "seconds timestamp",
			timestamp: time.Now().Unix(),
			check: func(result int64) bool {
				expected := time.Now().Unix() * 1000
				return result >= expected-1000 && result <= expected+1000
			},
		},
		{
			name:      "future timestamp",
			timestamp: time.Now().UnixMilli() + 120000, // 2 minutes in future
			check: func(result int64) bool {
				now := time.Now().UnixMilli()
				return result >= now-1000 && result <= now+1000
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.NormalizeTimestamp(tt.timestamp)
			assert.True(t, tt.check(result), "Timestamp normalization failed for %d", tt.timestamp)
		})
	}
}

func TestTransformer_ValidateEvent(t *testing.T) {
	tr := transformer.New()
	
	tests := []struct {
		name    string
		event   types.SpatialEvent
		wantErr bool
	}{
		{
			name: "valid event",
			event: types.SpatialEvent{
				SessionID: "test-session",
				EventID:   "event-123",
				Timestamp: time.Now().UnixMilli(),
			},
			wantErr: false,
		},
		{
			name: "missing session ID",
			event: types.SpatialEvent{
				EventID:   "event-123",
				Timestamp: time.Now().UnixMilli(),
			},
			wantErr: true,
		},
		{
			name: "missing event ID",
			event: types.SpatialEvent{
				SessionID: "test-session",
				Timestamp: time.Now().UnixMilli(),
			},
			wantErr: true,
		},
		{
			name: "invalid timestamp",
			event: types.SpatialEvent{
				SessionID: "test-session",
				EventID:   "event-123",
				Timestamp: 0,
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tr.ValidateEvent(&tt.event)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTransformer_GetStats(t *testing.T) {
	tr := transformer.New()
	
	// Transform a packet to create a session
	packet := types.StreamPacket{
		SessionID: "test-session",
		Timestamp: time.Now().UnixMilli(),
		Type:      "pose",
		Data: types.PacketData{
			Pose: &types.PoseData{
				X:        1.0,
				Y:        2.0,
				Z:        3.0,
				Rotation: [4]float64{0, 0, 0, 1},
			},
		},
	}
	
	_, err := tr.Transform(packet)
	require.NoError(t, err)
	
	stats := tr.GetStats()
	assert.Contains(t, stats, "active_sessions")
	assert.Equal(t, 1, stats["active_sessions"])
	assert.Contains(t, stats, "anchor_mappings")
}

func TestTransformer_ClearStaleSession(t *testing.T) {
	tr := transformer.New()
	
	// Create a session
	packet := types.StreamPacket{
		SessionID: "test-session",
		Timestamp: time.Now().UnixMilli(),
		Type:      "pose",
		Data: types.PacketData{
			Pose: &types.PoseData{
				X:        1.0,
				Y:        2.0,
				Z:        3.0,
				Rotation: [4]float64{0, 0, 0, 1},
			},
		},
	}
	
	_, err := tr.Transform(packet)
	require.NoError(t, err)
	
	// Verify session exists
	stats := tr.GetStats()
	assert.Equal(t, 1, stats["active_sessions"])
	
	// Clear the session
	tr.ClearStaleSession("test-session")
	
	// Verify session is cleared
	stats = tr.GetStats()
	assert.Equal(t, 0, stats["active_sessions"])
}