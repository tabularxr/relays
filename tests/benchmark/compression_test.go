package benchmark

import (
	"math"
	"testing"

	"github.com/tabular/relay/internal/parser"
	"github.com/tabular/relay/internal/updater"
	"github.com/tabular/relay/pkg/types"
	"github.com/tabular/relay/tests/testdata"
)

func BenchmarkDracoDecompression(b *testing.B) {
	generator := testdata.NewDracoTestDataGenerator()
	parser := parser.New()
	
	// Generate test data
	cubeData, err := generator.GenerateCubeMesh()
	if err != nil {
		b.Fatalf("Failed to generate cube mesh: %v", err)
	}
	
	sphereData, err := generator.GenerateSphereMesh(5.0, 20)
	if err != nil {
		b.Fatalf("Failed to generate sphere mesh: %v", err)
	}
	
	largeData, err := generator.GenerateLargeMesh()
	if err != nil {
		b.Fatalf("Failed to generate large mesh: %v", err)
	}
	
	b.Run("SmallCube", func(b *testing.B) {
		// Create a test packet to trigger decompression through parser
		packet := testdata.CreateTestMeshPacket("test-session", "test-anchor", cubeData)
		streamPacket := convertToStreamPacket(packet)
		
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := parser.ParsePacket(streamPacket)
			if err != nil {
				b.Fatalf("Parse failed: %v", err)
			}
		}
		b.SetBytes(int64(len(cubeData)))
	})
	
	b.Run("MediumSphere", func(b *testing.B) {
		packet := testdata.CreateTestMeshPacket("test-session", "test-anchor", sphereData)
		streamPacket := convertToStreamPacket(packet)
		
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := parser.ParsePacket(streamPacket)
			if err != nil {
				b.Fatalf("Parse failed: %v", err)
			}
		}
		b.SetBytes(int64(len(sphereData)))
	})
	
	b.Run("LargeMesh", func(b *testing.B) {
		packet := testdata.CreateTestMeshPacket("test-session", "test-anchor", largeData)
		streamPacket := convertToStreamPacket(packet)
		
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := parser.ParsePacket(streamPacket)
			if err != nil {
				b.Fatalf("Parse failed: %v", err)
			}
		}
		b.SetBytes(int64(len(largeData)))
	})
}

func BenchmarkDracoCompression(b *testing.B) {
	updater := updater.New("http://test", 1, 0)
	generator := testdata.NewDracoTestDataGenerator()
	
	// Generate raw vertex data for compression testing
	smallVertices := testdata.CreateRawVertexData([]float32{
		-1, -1, 1, 1, -1, 1, 1, 1, 1, -1, 1, 1, // Front face
	})
	
	sphereVertices := generateSphereVertices(1.0, 20)
	rawSphereData := testdata.CreateRawVertexData(sphereVertices)
	
	largeVertices := generateGridVertices(100, 100)
	rawLargeData := testdata.CreateRawVertexData(largeVertices)
	
	b.Run("SmallMesh", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _, err := updater.compressMeshData(smallVertices)
			if err != nil {
				b.Fatalf("Compression failed: %v", err)
			}
		}
		b.SetBytes(int64(len(smallVertices)))
	})
	
	b.Run("MediumSphere", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _, err := updater.compressMeshData(rawSphereData)
			if err != nil {
				b.Fatalf("Compression failed: %v", err)
			}
		}
		b.SetBytes(int64(len(rawSphereData)))
	})
	
	b.Run("LargeMesh", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _, err := updater.compressMeshData(rawLargeData)
			if err != nil {
				b.Fatalf("Compression failed: %v", err)
			}
		}
		b.SetBytes(int64(len(rawLargeData)))
	})
}

func TestCompressionRatios(t *testing.T) {
	updater := updater.New("http://test", 1, 0)
	generator := testdata.NewDracoTestDataGenerator()
	
	testCases := []struct {
		name     string
		vertices []float32
	}{
		{
			name:     "Cube",
			vertices: []float32{-1, -1, 1, 1, -1, 1, 1, 1, 1, -1, 1, 1},
		},
		{
			name:     "SmallSphere",
			vertices: generateSphereVertices(1.0, 10),
		},
		{
			name:     "LargeSphere",
			vertices: generateSphereVertices(5.0, 50),
		},
		{
			name:     "Grid100x100",
			vertices: generateGridVertices(100, 100),
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rawData := testdata.CreateRawVertexData(tc.vertices)
			compressed, bytesSaved, err := updater.compressMeshData(rawData)
			
			if err != nil {
				t.Fatalf("Compression failed: %v", err)
			}
			
			originalSize := len(rawData)
			compressedSize := len(compressed)
			compressionRatio := float64(compressedSize) / float64(originalSize)
			savingsPercent := float64(bytesSaved) / float64(originalSize) * 100
			
			t.Logf("%s: %d -> %d bytes (%.1f%% ratio, %.1f%% savings)", 
				tc.name, originalSize, compressedSize, compressionRatio*100, savingsPercent)
			
			// Verify significant compression for larger meshes
			if len(tc.vertices) > 100 {
				if compressionRatio >= 0.8 {
					t.Logf("Warning: Low compression ratio %.1f%% for %s", compressionRatio*100, tc.name)
				}
				
				// For realistic meshes, expect at least some compression
				if compressionRatio > 1.0 {
					t.Errorf("Compression ratio > 100%% indicates expansion, not compression")
				}
			}
		})
	}
}

// Helper functions for generating test geometry

func generateSphereVertices(radius float32, segments int) []float32 {
	var vertices []float32
	
	for i := 0; i <= segments; i++ {
		lat := 3.14159 * float32(i) / float32(segments) - 3.14159/2
		for j := 0; j <= segments; j++ {
			lng := 2 * 3.14159 * float32(j) / float32(segments)
			
			x := float32(math.Cos(float64(lat)) * math.Cos(float64(lng))) * radius
			y := float32(math.Sin(float64(lat))) * radius
			z := float32(math.Cos(float64(lat)) * math.Sin(float64(lng))) * radius
			
			vertices = append(vertices, x, y, z)
		}
	}
	
	return vertices
}

func generateGridVertices(width, height int) []float32 {
	vertices := make([]float32, width*height*3)
	
	idx := 0
	for i := 0; i < width; i++ {
		for j := 0; j < height; j++ {
			x := float32(i) / float32(width-1) * 10.0 - 5.0
			z := float32(j) / float32(height-1) * 10.0 - 5.0
			y := float32(math.Sin(float64(x)*0.5) * math.Cos(float64(z)*0.5))
			
			vertices[idx] = x
			vertices[idx+1] = y
			vertices[idx+2] = z
			idx += 3
		}
	}
	
	return vertices
}

// convertToStreamPacket converts test packet map to StreamPacket struct
func convertToStreamPacket(packet map[string]interface{}) types.StreamPacket {
	data := packet["data"].(map[string]interface{})
	mesh := data["mesh"].(map[string]interface{})
	
	return types.StreamPacket{
		SessionID:   packet["session_id"].(string),
		FrameNumber: packet["frame_number"].(int),
		Timestamp:   int64(packet["timestamp"].(int)),
		Type:        packet["type"].(string),
		Data: types.PacketData{
			Mesh: &types.MeshData{
				Vertices: mesh["vertices"].([]byte),
				Faces:    mesh["faces"].([]byte),
				AnchorID: mesh["anchor_id"].(string),
			},
		},
	}
}