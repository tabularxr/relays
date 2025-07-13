package testdata

import (
	"compress/gzip"
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
)

// DracoTestDataGenerator creates realistic test mesh data for testing
// Note: Using gzip compression for MVP (Draco has complex native dependencies)
type DracoTestDataGenerator struct {
	// No fields needed for simple compression
}

// NewDracoTestDataGenerator creates a new test data generator
func NewDracoTestDataGenerator() *DracoTestDataGenerator {
	return &DracoTestDataGenerator{}
}

// GenerateCubeMesh creates a simple cube mesh for testing
func (g *DracoTestDataGenerator) GenerateCubeMesh() ([]byte, error) {
	// Create a simple cube with 8 vertices
	vertices := []float32{
		// Front face
		-1.0, -1.0,  1.0,
		 1.0, -1.0,  1.0,
		 1.0,  1.0,  1.0,
		-1.0,  1.0,  1.0,
		// Back face
		-1.0, -1.0, -1.0,
		 1.0, -1.0, -1.0,
		 1.0,  1.0, -1.0,
		-1.0,  1.0, -1.0,
	}
	
	return g.createCompressedMesh(vertices)
}

// GenerateSphereMesh creates a sphere mesh for testing
func (g *DracoTestDataGenerator) GenerateSphereMesh(radius float32, segments int) ([]byte, error) {
	var vertices []float32
	
	// Generate sphere vertices using spherical coordinates
	for i := 0; i <= segments; i++ {
		lat := math.Pi * float64(i) / float64(segments) - math.Pi/2
		for j := 0; j <= segments; j++ {
			lng := 2 * math.Pi * float64(j) / float64(segments)
			
			x := float32(math.Cos(lat) * math.Cos(lng)) * radius
			y := float32(math.Sin(lat)) * radius
			z := float32(math.Cos(lat) * math.Sin(lng)) * radius
			
			vertices = append(vertices, x, y, z)
		}
	}
	
	return g.createCompressedMesh(vertices)
}

// GenerateRandomMesh creates a random mesh for testing
func (g *DracoTestDataGenerator) GenerateRandomMesh(numVertices int) ([]byte, error) {
	vertices := make([]float32, numVertices*3)
	
	for i := 0; i < numVertices*3; i++ {
		vertices[i] = rand.Float32()*20 - 10 // Random values between -10 and 10
	}
	
	return g.createCompressedMesh(vertices)
}

// GenerateLargeMesh creates a large mesh for performance testing
func (g *DracoTestDataGenerator) GenerateLargeMesh() ([]byte, error) {
	// Generate a 100x100 grid of vertices (10,000 vertices)
	const gridSize = 100
	vertices := make([]float32, gridSize*gridSize*3)
	
	idx := 0
	for i := 0; i < gridSize; i++ {
		for j := 0; j < gridSize; j++ {
			x := float32(i) / float32(gridSize-1) * 10.0 - 5.0 // -5 to 5
			z := float32(j) / float32(gridSize-1) * 10.0 - 5.0 // -5 to 5
			y := float32(math.Sin(float64(x)*0.5) * math.Cos(float64(z)*0.5)) // Wavy surface
			
			vertices[idx] = x
			vertices[idx+1] = y
			vertices[idx+2] = z
			idx += 3
		}
	}
	
	return g.createCompressedMesh(vertices)
}

// createCompressedMesh converts float32 vertices to compressed bytes
// Since we don't have Draco encoding available, we'll use gzip compression
// and add a simple header to simulate Draco format
func (g *DracoTestDataGenerator) createCompressedMesh(vertices []float32) ([]byte, error) {
	if len(vertices)%3 != 0 {
		return nil, fmt.Errorf("vertices length must be multiple of 3")
	}
	
	// Convert to byte array
	rawData := CreateRawVertexData(vertices)
	
	// Compress with gzip
	var compressed bytes.Buffer
	gzWriter := gzip.NewWriter(&compressed)
	
	_, err := gzWriter.Write(rawData)
	if err != nil {
		gzWriter.Close()
		return nil, fmt.Errorf("compression failed: %w", err)
	}
	
	err = gzWriter.Close()
	if err != nil {
		return nil, fmt.Errorf("compression close failed: %w", err)
	}
	
	return compressed.Bytes(), nil
}

// CreateRawVertexData converts float32 vertices to raw byte format
func CreateRawVertexData(vertices []float32) []byte {
	data := make([]byte, len(vertices)*4)
	for i, v := range vertices {
		binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(v))
	}
	return data
}

// CreateTestMeshPacket creates a test mesh packet with the given data
func CreateTestMeshPacket(sessionID, anchorID string, vertexData []byte) map[string]interface{} {
	return map[string]interface{}{
		"session_id":   sessionID,
		"frame_number": 1,
		"timestamp":    1234567890000,
		"type":         "mesh",
		"data": map[string]interface{}{
			"mesh": map[string]interface{}{
				"vertices":  vertexData,
				"faces":     []byte{0, 1, 2, 1, 2, 3}, // Simple triangle indices
				"anchor_id": anchorID,
			},
		},
	}
}