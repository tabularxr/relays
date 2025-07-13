package parser

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"log"

	"github.com/tabular/relay/pkg/types"
)

// Parser handles decompression and validation of incoming packets
type Parser struct {
	// Compression support (gzip-based for MVP)
}

// New creates a new Parser instance
func New() *Parser {
	return &Parser{}
}

// ParsePacket processes and validates a StreamPacket
func (p *Parser) ParsePacket(packet types.StreamPacket) (*types.StreamPacket, error) {
	// Validate basic packet structure
	if err := p.validatePacket(packet); err != nil {
		return nil, fmt.Errorf("invalid packet: %w", err)
	}

	// Process based on packet type
	switch packet.Type {
	case "pose":
		return p.parsePosePacket(packet)
	case "mesh":
		return p.parseMeshPacket(packet)
	default:
		return nil, fmt.Errorf("unknown packet type: %s", packet.Type)
	}
}

// validatePacket performs basic validation
func (p *Parser) validatePacket(packet types.StreamPacket) error {
	if packet.SessionID == "" {
		return fmt.Errorf("missing session_id")
	}
	if packet.Timestamp <= 0 {
		return fmt.Errorf("invalid timestamp")
	}
	if packet.Type == "" {
		return fmt.Errorf("missing packet type")
	}
	return nil
}

// parsePosePacket processes pose data
func (p *Parser) parsePosePacket(packet types.StreamPacket) (*types.StreamPacket, error) {
	if packet.Data.Pose == nil {
		return nil, fmt.Errorf("missing pose data")
	}

	pose := packet.Data.Pose
	
	// Validate pose data
	if err := p.validatePose(*pose); err != nil {
		return nil, fmt.Errorf("invalid pose: %w", err)
	}

	// Pose packets typically don't need decompression
	return &packet, nil
}

// parseMeshPacket processes mesh data with decompression
func (p *Parser) parseMeshPacket(packet types.StreamPacket) (*types.StreamPacket, error) {
	if packet.Data.Mesh == nil {
		return nil, fmt.Errorf("missing mesh data")
	}

	mesh := packet.Data.Mesh
	
	// Validate mesh data
	if len(mesh.Vertices) == 0 {
		return nil, fmt.Errorf("empty vertices data")
	}
	if mesh.AnchorID == "" {
		return nil, fmt.Errorf("missing anchor_id")
	}

	// Decompress vertices if they're Draco-compressed
	decompressedVertices, err := p.decompressDraco(mesh.Vertices)
	if err != nil {
		// If decompression fails, assume data is already uncompressed
		log.Printf("Draco decompression failed, using raw data: %v", err)
		decompressedVertices = mesh.Vertices
	}

	// Decompress faces if present and compressed
	var decompressedFaces []byte
	if len(mesh.Faces) > 0 {
		decompressedFaces, err = p.decompressDraco(mesh.Faces)
		if err != nil {
			log.Printf("Face decompression failed, using raw data: %v", err)
			decompressedFaces = mesh.Faces
		}
	}

	// Update packet with decompressed data
	newPacket := packet
	newPacket.Data.Mesh = &types.MeshData{
		Vertices: decompressedVertices,
		Faces:    decompressedFaces,
		AnchorID: mesh.AnchorID,
	}

	return &newPacket, nil
}

// validatePose validates pose data structure
func (p *Parser) validatePose(pose types.PoseData) error {
	// Check for reasonable position bounds (adjust as needed)
	if pose.X < -1000 || pose.X > 1000 ||
		pose.Y < -1000 || pose.Y > 1000 ||
		pose.Z < -1000 || pose.Z > 1000 {
		return fmt.Errorf("pose position out of bounds")
	}

	// Validate quaternion (should be normalized)
	qx, qy, qz, qw := pose.Rotation[0], pose.Rotation[1], pose.Rotation[2], pose.Rotation[3]
	magnitude := qx*qx + qy*qy + qz*qz + qw*qw
	if magnitude < 0.9 || magnitude > 1.1 {
		return fmt.Errorf("quaternion not normalized: magnitude=%f", magnitude)
	}

	return nil
}

// decompressDraco attempts to decompress compressed mesh data
// For MVP: Supports gzip compression (Draco libraries have complex dependencies)
func (p *Parser) decompressDraco(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	// Try gzip decompression first
	return p.decompressGzip(data)
}

// decompressGzip attempts to decompress gzip-encoded data
func (p *Parser) decompressGzip(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		// If gzip fails too, assume raw data
		log.Printf("Data not gzip-encoded either, treating as raw: %v", err)
		return data, nil
	}
	defer gzReader.Close()
	
	var decompressed bytes.Buffer
	_, err = decompressed.ReadFrom(gzReader)
	if err != nil {
		log.Printf("Gzip decompression failed: %v", err)
		return data, nil
	}
	
	result := decompressed.Bytes()
	log.Printf("Successfully decompressed gzip data: %d -> %d bytes", len(data), len(result))
	return result, nil
}

// GetStats returns parser statistics
func (p *Parser) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"parser_initialized": true,
		"compression_support": true,
		"gzip_support":       true,
	}
}