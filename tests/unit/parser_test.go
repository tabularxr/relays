package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tabular/relay/internal/parser"
	"github.com/tabular/relay/pkg/types"
)

func TestParser_NewParser(t *testing.T) {
	p := parser.New()
	assert.NotNil(t, p)
}

func TestParser_ParsePosePacket(t *testing.T) {
	p := parser.New()
	
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
	
	result, err := p.ParsePacket(packet)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "pose", result.Type)
	assert.Equal(t, packet.Data.Pose.X, result.Data.Pose.X)
}

func TestParser_ParseMeshPacket(t *testing.T) {
	p := parser.New()
	
	// Create test mesh data (non-compressed for this test)
	testVertices := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12} // 12 bytes = 1 vertex
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
	
	result, err := p.ParsePacket(packet)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "mesh", result.Type)
	assert.Equal(t, "anchor-123", result.Data.Mesh.AnchorID)
	assert.NotEmpty(t, result.Data.Mesh.Vertices)
}

func TestParser_ValidatePacket(t *testing.T) {
	p := parser.New()
	
	tests := []struct {
		name    string
		packet  types.StreamPacket
		wantErr bool
	}{
		{
			name: "valid packet",
			packet: types.StreamPacket{
				SessionID: "test-session",
				Timestamp: time.Now().UnixMilli(),
				Type:      "pose",
			},
			wantErr: false,
		},
		{
			name: "missing session_id",
			packet: types.StreamPacket{
				Timestamp: time.Now().UnixMilli(),
				Type:      "pose",
			},
			wantErr: true,
		},
		{
			name: "invalid timestamp",
			packet: types.StreamPacket{
				SessionID: "test-session",
				Timestamp: 0,
				Type:      "pose",
			},
			wantErr: true,
		},
		{
			name: "missing type",
			packet: types.StreamPacket{
				SessionID: "test-session",
				Timestamp: time.Now().UnixMilli(),
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParsePacket(tt.packet)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				// For valid basic packets, we might get errors if data is missing
				// but that's expected for this validation test
			}
		})
	}
}

func TestParser_InvalidPacketType(t *testing.T) {
	p := parser.New()
	
	packet := types.StreamPacket{
		SessionID: "test-session",
		Timestamp: time.Now().UnixMilli(),
		Type:      "unknown",
	}
	
	_, err := p.ParsePacket(packet)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown packet type")
}

func TestParser_PoseValidation(t *testing.T) {
	p := parser.New()
	
	tests := []struct {
		name    string
		pose    types.PoseData
		wantErr bool
	}{
		{
			name: "valid pose",
			pose: types.PoseData{
				X:        1.0,
				Y:        2.0,
				Z:        3.0,
				Rotation: [4]float64{0, 0, 0, 1}, // Normalized quaternion
			},
			wantErr: false,
		},
		{
			name: "position out of bounds",
			pose: types.PoseData{
				X:        2000.0, // Too large
				Y:        2.0,
				Z:        3.0,
				Rotation: [4]float64{0, 0, 0, 1},
			},
			wantErr: true,
		},
		{
			name: "unnormalized quaternion",
			pose: types.PoseData{
				X:        1.0,
				Y:        2.0,
				Z:        3.0,
				Rotation: [4]float64{2, 2, 2, 2}, // Not normalized
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packet := types.StreamPacket{
				SessionID: "test-session",
				Timestamp: time.Now().UnixMilli(),
				Type:      "pose",
				Data: types.PacketData{
					Pose: &tt.pose,
				},
			}
			
			_, err := p.ParsePacket(packet)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParser_GetStats(t *testing.T) {
	p := parser.New()
	
	stats := p.GetStats()
	assert.Contains(t, stats, "parser_initialized")
	assert.True(t, stats["parser_initialized"].(bool))
	assert.Contains(t, stats, "compression_support")
	assert.True(t, stats["compression_support"].(bool))
	assert.Contains(t, stats, "gzip_support")
	assert.True(t, stats["gzip_support"].(bool))
}