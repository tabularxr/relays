package types

import "time"

// StreamPacket represents incoming data from StreamKit
type StreamPacket struct {
	SessionID   string      `json:"session_id"`
	FrameNumber int         `json:"frame_number"`
	Timestamp   int64       `json:"timestamp"`
	Type        string      `json:"type"` // "pose" | "mesh"
	Data        PacketData  `json:"data"`
}

// PacketData contains either pose or mesh data
type PacketData struct {
	Pose *PoseData `json:"pose,omitempty"`
	Mesh *MeshData `json:"mesh,omitempty"`
}

// PoseData represents spatial positioning
type PoseData struct {
	X        float64   `json:"x"`
	Y        float64   `json:"y"`
	Z        float64   `json:"z"`
	Rotation [4]float64 `json:"rotation"` // Quaternion [x,y,z,w]
}

// MeshData represents 3D mesh geometry
type MeshData struct {
	Vertices []byte `json:"vertices"` // Draco-compressed
	Faces    []byte `json:"faces"`    // Draco-compressed
	AnchorID string `json:"anchor_id"`
}

// SpatialEvent represents processed data sent to STAG
type SpatialEvent struct {
	SessionID string    `json:"session_id"`
	EventID   string    `json:"event_id"`
	Timestamp int64     `json:"timestamp"`
	Anchors   []Anchor  `json:"anchors"`
	Meshes    []MeshDiff `json:"meshes"`
}

// Anchor represents a spatial reference point
type Anchor struct {
	ID        string    `json:"id"`
	Pose      PoseData  `json:"pose"`
	Timestamp int64     `json:"timestamp"`
}

// MeshDiff represents mesh changes for versioning
type MeshDiff struct {
	AnchorID      string  `json:"anchor_id"`
	VerticesDelta []byte  `json:"vertices_delta,omitempty"`
	FacesDelta    []byte  `json:"faces_delta,omitempty"`
	IsDelta       bool    `json:"is_delta"`
}

// Connection represents a WebSocket client
type Connection struct {
	ID        string
	SessionID string
	LastSeen  time.Time
	APIKey    string
}

// Config holds application configuration
type Config struct {
	Server struct {
		Port string `mapstructure:"port"`
		Host string `mapstructure:"host"`
	} `mapstructure:"server"`
	
	STAG struct {
		URL     string        `mapstructure:"url"`
		Timeout time.Duration `mapstructure:"timeout"`
	} `mapstructure:"stag"`
	
	WebSocket struct {
		BufferSize        int           `mapstructure:"buffer_size"`
		HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
	} `mapstructure:"websocket"`
	
	Batch struct {
		MaxSize int           `mapstructure:"max_size"`
		Timeout time.Duration `mapstructure:"timeout"`
	} `mapstructure:"batch"`
}