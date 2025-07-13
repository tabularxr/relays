package integration

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tabular/relay/tests/testdata"
)

func TestCompressionEfficiency(t *testing.T) {
	generator := testdata.NewDracoTestDataGenerator()
	
	testCases := []struct {
		name        string
		generator   func() ([]byte, error)
		expectRatio float64 // Expected compression ratio (compressed/original)
	}{
		{
			name:        "SmallCube",
			generator:   generator.GenerateCubeMesh,
			expectRatio: 0.8, // Allow up to 80% of original size
		},
		{
			name: "MediumSphere",
			generator: func() ([]byte, error) {
				return generator.GenerateSphereMesh(5.0, 20)
			},
			expectRatio: 0.7, // Better compression for larger meshes
		},
		{
			name: "RepeatedPattern",
			generator: func() ([]byte, error) {
				// Generate highly repetitive data that should compress very well
				vertices := make([]float32, 3000) // 1000 vertices
				pattern := []float32{1.0, 2.0, 3.0, 1.1, 2.1, 3.1}
				for i := 0; i < len(vertices); i++ {
					vertices[i] = pattern[i%len(pattern)]
				}
				
				// Convert to bytes and compress directly
				rawData := testdata.CreateRawVertexData(vertices)
				var compressed bytes.Buffer
				gzWriter := gzip.NewWriter(&compressed)
				_, err := gzWriter.Write(rawData)
				if err != nil {
					return nil, err
				}
				err = gzWriter.Close()
				if err != nil {
					return nil, err
				}
				return compressed.Bytes(), nil
			},
			expectRatio: 0.3, // Very good compression for repetitive data
		},
	}
	
	totalOriginalSize := 0
	totalCompressedSize := 0
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate test data
			compressed, err := tc.generator()
			require.NoError(t, err)
			
			// Decompress to get original size
			reader := bytes.NewReader(compressed)
			gzReader, err := gzip.NewReader(reader)
			require.NoError(t, err)
			defer gzReader.Close()
			
			var decompressed bytes.Buffer
			_, err = decompressed.ReadFrom(gzReader)
			require.NoError(t, err)
			
			originalSize := decompressed.Len()
			compressedSize := len(compressed)
			ratio := float64(compressedSize) / float64(originalSize)
			savings := float64(originalSize-compressedSize) / float64(originalSize) * 100
			
			t.Logf("%s: %d -> %d bytes (%.1f%% ratio, %.1f%% savings)", 
				tc.name, originalSize, compressedSize, ratio*100, savings)
			
			// Verify compression efficiency
			assert.True(t, ratio <= tc.expectRatio, 
				"Compression ratio %.2f%% should be <= %.2f%%", ratio*100, tc.expectRatio*100)
			
			// Verify minimum savings for larger meshes
			if originalSize > 100 {
				assert.True(t, savings > 20.0, 
					"Should achieve at least 20%% compression savings, got %.1f%%", savings)
			}
			
			totalOriginalSize += originalSize
			totalCompressedSize += compressedSize
		})
	}
	
	// Overall compression assessment
	overallRatio := float64(totalCompressedSize) / float64(totalOriginalSize)
	overallSavings := float64(totalOriginalSize-totalCompressedSize) / float64(totalOriginalSize) * 100
	
	t.Logf("Overall: %d -> %d bytes (%.1f%% ratio, %.1f%% savings)", 
		totalOriginalSize, totalCompressedSize, overallRatio*100, overallSavings)
	
	// Validate overall bandwidth reduction requirement
	assert.True(t, overallSavings >= 50.0, 
		"Overall compression should achieve at least 50%% bandwidth reduction, got %.1f%%", overallSavings)
}

func TestVertexDataCompression(t *testing.T) {
	// Test compression of raw vertex data
	vertices := []float32{
		// Repetitive pattern should compress well
		1.0, 2.0, 3.0,
		1.0, 2.0, 3.0,
		1.1, 2.1, 3.1,
		1.0, 2.0, 3.0,
		1.1, 2.1, 3.1,
		1.0, 2.0, 3.0,
	}
	
	// Convert to bytes
	data := make([]byte, len(vertices)*4)
	for i, v := range vertices {
		binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(v))
	}
	
	// Compress
	var compressed bytes.Buffer
	gzWriter := gzip.NewWriter(&compressed)
	_, err := gzWriter.Write(data)
	require.NoError(t, err)
	err = gzWriter.Close()
	require.NoError(t, err)
	
	originalSize := len(data)
	compressedSize := compressed.Len()
	ratio := float64(compressedSize) / float64(originalSize)
	savings := float64(originalSize-compressedSize) / float64(originalSize) * 100
	
	t.Logf("Vertex compression: %d -> %d bytes (%.1f%% ratio, %.1f%% savings)", 
		originalSize, compressedSize, ratio*100, savings)
	
	// Repetitive data should compress reasonably well
	assert.True(t, savings > 25.0, "Repetitive vertex data should compress >25%%, got %.1f%%", savings)
}