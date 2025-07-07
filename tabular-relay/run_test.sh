#!/bin/bash

# Tabular Relay Test Script
# This script tests the tabular-relay by running the test client

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default configuration
RELAY_URL="ws://localhost:8080/ws/streamkit"
STAGS_URL="http://localhost:8000/ingest"
CONCURRENT=3
INTERVAL="100ms"
VERBOSE=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -r|--relay-url)
            RELAY_URL="$2"
            shift 2
            ;;
        -s|--stags-url)
            STAGS_URL="$2"
            shift 2
            ;;
        -c|--concurrent)
            CONCURRENT="$2"
            shift 2
            ;;
        -i|--interval)
            INTERVAL="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  -r, --relay-url URL     Relay WebSocket URL (default: $RELAY_URL)"
            echo "  -s, --stags-url URL     Stags ingest URL (default: $STAGS_URL)"
            echo "  -c, --concurrent N      Number of concurrent connections (default: $CONCURRENT)"
            echo "  -i, --interval DURATION Interval between sends (default: $INTERVAL)"
            echo "  -v, --verbose           Verbose output"
            echo "  -h, --help              Show this help"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

echo -e "${BLUE}==================================================${NC}"
echo -e "${BLUE}           Tabular Relay Test Script${NC}"
echo -e "${BLUE}==================================================${NC}"
echo
echo -e "${YELLOW}Configuration:${NC}"
echo "  Relay URL: $RELAY_URL"
echo "  Stags URL: $STAGS_URL"
echo "  Concurrent connections: $CONCURRENT"
echo "  Send interval: $INTERVAL"
echo "  Verbose: $VERBOSE"
echo

# Check if test data exists
echo -e "${YELLOW}Checking test data...${NC}"
TESTDATA_DIR="testdata"
if [[ ! -d "$TESTDATA_DIR" ]]; then
    echo -e "${RED}Error: Test data directory '$TESTDATA_DIR' not found${NC}"
    exit 1
fi

PACKET_FILES=(
    "$TESTDATA_DIR/sample_packet_1.json"
    "$TESTDATA_DIR/sample_packet_2.json"
    "$TESTDATA_DIR/sample_packet_3.json"
    "$TESTDATA_DIR/sample_packet_4.json"
    "$TESTDATA_DIR/sample_packet_5.json"
)

for file in "${PACKET_FILES[@]}"; do
    if [[ ! -f "$file" ]]; then
        echo -e "${RED}Error: Test packet file '$file' not found${NC}"
        exit 1
    fi
done

echo -e "${GREEN}✓ All test packet files found${NC}"

# Check if relay is running
echo -e "${YELLOW}Checking if relay is running...${NC}"
RELAY_HOST=$(echo "$RELAY_URL" | sed -n 's/.*:\/\/\([^:]*\):.*/\1/p')
RELAY_PORT=$(echo "$RELAY_URL" | sed -n 's/.*:\([0-9]*\)\/.*/\1/p')

if [[ -z "$RELAY_HOST" || -z "$RELAY_PORT" ]]; then
    echo -e "${RED}Error: Could not parse relay URL${NC}"
    exit 1
fi

if ! nc -z "$RELAY_HOST" "$RELAY_PORT" 2>/dev/null; then
    echo -e "${RED}Error: Relay is not running on $RELAY_HOST:$RELAY_PORT${NC}"
    echo "Please start the relay server first:"
    echo "  cd tabular-relay && go run cmd/relay/main.go"
    exit 1
fi

echo -e "${GREEN}✓ Relay is running${NC}"

# Build test client if needed
echo -e "${YELLOW}Building test client...${NC}"
if [[ ! -f "test_client" ]] || [[ "test_client.go" -nt "test_client" ]]; then
    if ! go build -o test_client test_client.go; then
        echo -e "${RED}Error: Failed to build test client${NC}"
        exit 1
    fi
fi
echo -e "${GREEN}✓ Test client built${NC}"

# Run the test
echo -e "${YELLOW}Running test...${NC}"
echo

TEST_ARGS=(
    "-relay-url=$RELAY_URL"
    "-stags-url=$STAGS_URL"
    "-concurrent=$CONCURRENT"
    "-interval=$INTERVAL"
)

if [[ "$VERBOSE" == "true" ]]; then
    TEST_ARGS+=("-verbose")
fi

# Run test client and capture output
TEST_OUTPUT=$(./test_client "${TEST_ARGS[@]}" 2>&1)
TEST_EXIT_CODE=$?

echo "$TEST_OUTPUT"

# Analyze results
echo
echo -e "${BLUE}==================================================${NC}"
if [[ $TEST_EXIT_CODE -eq 0 ]] && echo "$TEST_OUTPUT" | grep -q "TEST PASSED"; then
    echo -e "${GREEN}🎉 ALL TESTS PASSED! 🎉${NC}"
    echo -e "${GREEN}The tabular-relay is working correctly.${NC}"
    
    # Extract statistics
    PACKETS_SENT=$(echo "$TEST_OUTPUT" | grep "Packets Sent:" | sed 's/.*: //')
    STAGS_RESPONSES=$(echo "$TEST_OUTPUT" | grep "Stags Responses:" | sed 's/.*: //')
    
    echo
    echo -e "${YELLOW}Test Summary:${NC}"
    echo "  Packets sent: $PACKETS_SENT"
    echo "  Stags responses: $STAGS_RESPONSES"
    echo "  End-to-end pipeline: ✓ Working"
    
else
    echo -e "${RED}❌ TESTS FAILED ❌${NC}"
    echo -e "${RED}Please check the relay logs and try again.${NC}"
    
    # Show helpful debugging information
    echo
    echo -e "${YELLOW}Debugging Tips:${NC}"
    echo "1. Check relay logs for errors"
    echo "2. Verify relay configuration"
    echo "3. Ensure Stags endpoint is accessible"
    echo "4. Check network connectivity"
    
    exit 1
fi

echo -e "${BLUE}==================================================${NC}"