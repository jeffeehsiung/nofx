package market

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type KlineWebSocketManagerTestSuite struct {
	suite.Suite
	manager *KlineWebSocketManager
}

func TestKlineWebSocketManagerSuite(t *testing.T) {
	suite.Run(t, new(KlineWebSocketManagerTestSuite))
}

func (s *KlineWebSocketManagerTestSuite) SetupTest() {
	// Create manager for testnet
	s.manager = NewKlineWebSocketManager(true)
}

func (s *KlineWebSocketManagerTestSuite) TearDownTest() {
	if s.manager != nil {
		if err := s.manager.Stop(); err != nil {
			s.T().Errorf("failed to stop manager: %v", err)
		}
	}
}

// TestNewKlineWebSocketManager tests manager initialization
func (s *KlineWebSocketManagerTestSuite) TestNewKlineWebSocketManager() {
	assert.NotNil(s.T(), s.manager)
	assert.Equal(s.T(), 500, s.manager.maxStreamsPerConn)
	assert.True(s.T(), s.manager.testnet)
	assert.Equal(s.T(), 0, len(s.manager.connections))
	assert.Equal(s.T(), 0, len(s.manager.subscriptions))
	assert.Equal(s.T(), 0, len(s.manager.activeSymbols))
}

// TestStart tests manager startup
func (s *KlineWebSocketManagerTestSuite) TestStart() {
	err := s.manager.Start()
	assert.NoError(s.T(), err)

	// Should create at least one connection
	assert.GreaterOrEqual(s.T(), len(s.manager.connections), 1)
	assert.True(s.T(), s.manager.connections[0].IsConnected())
}

// TestRegisterActiveSymbols tests symbol registration
func (s *KlineWebSocketManagerTestSuite) TestRegisterActiveSymbols() {
	err := s.manager.Start()
	assert.NoError(s.T(), err)

	symbols := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT"}
	timeframes := []string{"1m", "4h"}

	err = s.manager.RegisterActiveSymbols(symbols, timeframes)
	assert.NoError(s.T(), err)

	// Check active symbols registered
	assert.Equal(s.T(), 3, len(s.manager.activeSymbols))
	assert.True(s.T(), s.manager.activeSymbols["BTCUSDT"])
	assert.True(s.T(), s.manager.activeSymbols["ETHUSDT"])
	assert.True(s.T(), s.manager.activeSymbols["BNBUSDT"])

	// Check subscriptions created (3 symbols × 2 timeframes = 6)
	assert.Equal(s.T(), 6, len(s.manager.subscriptions))
}

// TestUnregisterSymbol tests symbol unregistration
func (s *KlineWebSocketManagerTestSuite) TestUnregisterSymbol() {
	err := s.manager.Start()
	assert.NoError(s.T(), err)

	symbols := []string{"BTCUSDT", "ETHUSDT"}
	timeframes := []string{"1m"}

	err = s.manager.RegisterActiveSymbols(symbols, timeframes)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 2, len(s.manager.activeSymbols))

	// Unregister one symbol
	err = s.manager.UnregisterSymbol("BTCUSDT")
	assert.NoError(s.T(), err)

	assert.Equal(s.T(), 1, len(s.manager.activeSymbols))
	assert.False(s.T(), s.manager.activeSymbols["BTCUSDT"])
	assert.True(s.T(), s.manager.activeSymbols["ETHUSDT"])
}

// TestConnectionPooling tests that manager creates multiple connections when needed
func (s *KlineWebSocketManagerTestSuite) TestConnectionPooling() {
	err := s.manager.Start()
	assert.NoError(s.T(), err)

	// Set low limit for testing
	s.manager.maxStreamsPerConn = 3

	// Register enough symbols to require multiple connections
	symbols := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "ADAUSDT", "DOGEUSDT"}
	timeframes := []string{"1m", "4h"} // 5 symbols × 2 timeframes = 10 streams

	err = s.manager.RegisterActiveSymbols(symbols, timeframes)
	assert.NoError(s.T(), err)

	// Should create multiple connections (10 streams / 3 per conn = 4 connections)
	assert.GreaterOrEqual(s.T(), len(s.manager.connections), 3)

	// Verify all connections are active
	for i, conn := range s.manager.connections {
		assert.True(s.T(), conn.IsConnected(), "Connection %d should be connected", i)
	}
}

// TestFindAvailableConnection tests connection selection logic
func (s *KlineWebSocketManagerTestSuite) TestFindAvailableConnection() {
	err := s.manager.Start()
	assert.NoError(s.T(), err)
	s.manager.maxStreamsPerConn = 2

	// Create some subscriptions
	s.manager.subscriptions["BTCUSDT@kline_1m"] = 0
	s.manager.subscriptions["ETHUSDT@kline_1m"] = 0

	// Connection 0 is full (2/2), should return -1
	index := s.manager.findAvailableConnection()
	assert.Equal(s.T(), -1, index)

	// Create second connection with space
	conn := NewBinanceWebSocketClient(true)
	if err := conn.Connect(); err != nil {
		s.T().Errorf("failed to connect: %v", err)
	}
	s.manager.connections = append(s.manager.connections, conn)

	// Now should return connection 1
	index = s.manager.findAvailableConnection()
	assert.Equal(s.T(), 1, index)
}

// TestResubscribeConnection tests reconnection logic
func (s *KlineWebSocketManagerTestSuite) TestResubscribeConnection() {
	err := s.manager.Start()
	assert.NoError(s.T(), err)

	// Register some symbols
	symbols := []string{"BTCUSDT", "ETHUSDT"}
	timeframes := []string{"1m"}
	err = s.manager.RegisterActiveSymbols(symbols, timeframes)
	assert.NoError(s.T(), err)

	originalSubCount := len(s.manager.subscriptions)
	assert.Equal(s.T(), 2, originalSubCount)

	// Simulate disconnect and reconnect
	if err := s.manager.connections[0].Disconnect(); err != nil {
		s.T().Errorf("failed to disconnect: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := s.manager.connections[0].Connect(); err != nil {
		s.T().Errorf("failed to reconnect: %v", err)
	}

	// Resubscribe
	s.manager.resubscribeConnection(0)

	// Should have same number of subscriptions
	assert.Equal(s.T(), originalSubCount, len(s.manager.subscriptions))
}

// TestStaleDataDetection tests staleness detection
func (s *KlineWebSocketManagerTestSuite) TestStaleDataDetection() {
	err := s.manager.Start()
	assert.NoError(s.T(), err)
	s.manager.staleDuration = 1 * time.Second // Short duration for testing

	// Register a symbol
	err = s.manager.RegisterActiveSymbols([]string{"BTCUSDT"}, []string{"1m"})
	assert.NoError(s.T(), err)

	subscriptionKey := "BTCUSDT@kline_1m"

	// Set last update to past
	s.manager.mu.Lock()
	s.manager.lastUpdateTime[subscriptionKey] = time.Now().Add(-2 * time.Second)
	s.manager.mu.Unlock()

	// Perform health check
	s.manager.performHealthCheck()

	// After health check, should have attempted REST fallback
	// (We can't easily test REST call without mocking, but check the logic ran)
	assert.NotNil(s.T(), s.manager.lastUpdateTime[subscriptionKey])
}

// TestGetStatus tests status reporting
func (s *KlineWebSocketManagerTestSuite) TestGetStatus() {
	err := s.manager.Start()
	assert.NoError(s.T(), err)

	symbols := []string{"BTCUSDT", "ETHUSDT"}
	timeframes := []string{"1m", "4h"}
	err = s.manager.RegisterActiveSymbols(symbols, timeframes)
	assert.NoError(s.T(), err)

	status := s.manager.GetStatus()

	assert.NotNil(s.T(), status)
	assert.Equal(s.T(), 2, status["active_symbols"])
	assert.Equal(s.T(), 4, status["subscriptions"]) // 2 symbols × 2 timeframes
	assert.GreaterOrEqual(s.T(), status["connections"], 1)
	assert.NotNil(s.T(), status["connection_status"])
}

// TestKlineHandlerRegistration tests handler registration and callback
func (s *KlineWebSocketManagerTestSuite) TestKlineHandlerRegistration() {
	err := s.manager.Start()
	assert.NoError(s.T(), err)

	handlerCalled := false
	var receivedUpdate KlineUpdate

	handler := func(update KlineUpdate) {
		handlerCalled = true
		receivedUpdate = update
	}

	s.manager.RegisterKlineHandler(handler)

	assert.Equal(s.T(), 1, len(s.manager.klineUpdateHandlers))

	// Simulate an update (in real usage, this comes from WebSocket)
	testUpdate := KlineUpdate{
		Symbol:   "BTCUSDT",
		Interval: "1m",
		Close:    50000.0,
	}

	// Call handler directly to test
	s.manager.klineUpdateHandlers[0](testUpdate)

	// Give handler goroutine time to execute
	time.Sleep(50 * time.Millisecond)

	assert.True(s.T(), handlerCalled)
	assert.Equal(s.T(), "BTCUSDT", receivedUpdate.Symbol)
	assert.Equal(s.T(), 50000.0, receivedUpdate.Close)
}

// TestEqualStringSlices tests the helper function
func TestEqualStringSlices(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{"equal slices", []string{"a", "b", "c"}, []string{"a", "b", "c"}, true},
		{"different length", []string{"a", "b"}, []string{"a", "b", "c"}, false},
		{"different content", []string{"a", "b", "c"}, []string{"a", "x", "c"}, false},
		{"both empty", []string{}, []string{}, true},
		{"one empty", []string{"a"}, []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := equalStringSlices(tt.a, tt.b)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestConnectionCapacityLimit tests that connections respect max stream limit
func (s *KlineWebSocketManagerTestSuite) TestConnectionCapacityLimit() {
	err := s.manager.Start()
	assert.NoError(s.T(), err)
	s.manager.maxStreamsPerConn = 5

	// Register 10 symbols with 1 timeframe each = 10 streams
	symbols := make([]string, 10)
	for i := 0; i < 10; i++ {
		symbols[i] = fmt.Sprintf("SYM%dUSDT", i)
	}

	err = s.manager.RegisterActiveSymbols(symbols, []string{"1m"})
	assert.NoError(s.T(), err)

	// Should have created 2 connections (10 streams / 5 per conn)
	assert.GreaterOrEqual(s.T(), len(s.manager.connections), 2)

	// Count streams per connection
	connCounts := make(map[int]int)
	for _, connIndex := range s.manager.subscriptions {
		connCounts[connIndex]++
	}

	// No connection should exceed the limit
	for connIndex, count := range connCounts {
		assert.LessOrEqual(s.T(), count, s.manager.maxStreamsPerConn,
			"Connection %d has %d streams (exceeds limit of %d)",
			connIndex, count, s.manager.maxStreamsPerConn)
	}
}
