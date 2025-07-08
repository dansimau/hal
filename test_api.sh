#!/bin/bash

# Test script for HAL Mobile API
# This script demonstrates different scenarios for the conditional startup screen

API_URL="http://localhost:8080/api/v1"

echo "🚀 Testing HAL Mobile API Conditional Startup Screen"
echo "=================================================="
echo

# Test 1: Health check
echo "📊 Test 1: Health Check"
echo "GET $API_URL/health"
curl -s "$API_URL/health" | jq '.' || echo "❌ Health check failed - is the server running?"
echo
echo

# Test 2: New user (should show welcome screen)
echo "👋 Test 2: New User (should show welcome screen)"
echo "POST $API_URL/startup-screen"
curl -s -X POST "$API_URL/startup-screen" \
  -H "Content-Type: application/json" \
  -d '{
    "app_version": "2.1.0",
    "platform": "ios",
    "device_id": "new-device-123"
  }' | jq '.'
echo
echo

# Test 3: Old app version (should show forced update)
echo "⚠️  Test 3: Old App Version (should show forced update)"
echo "POST $API_URL/startup-screen"
curl -s -X POST "$API_URL/startup-screen" \
  -H "Content-Type: application/json" \
  -d '{
    "app_version": "1.5.0",
    "platform": "android",
    "device_id": "old-device-456"
  }' | jq '.'
echo
echo

# Test 4: Current user (should not show screen)
echo "✅ Test 4: Current User (should not show screen)"
echo "POST $API_URL/startup-screen"
curl -s -X POST "$API_URL/startup-screen" \
  -H "Content-Type: application/json" \
  -d '{
    "app_version": "2.1.0",
    "platform": "ios",
    "device_id": "existing-device-789",
    "last_shown_at": "2024-01-15T10:30:00Z"
  }' | jq '.'
echo
echo

# Test 5: Very old version (should show forced update)
echo "🔒 Test 5: Very Old Version (should show forced update)"
echo "POST $API_URL/startup-screen"
curl -s -X POST "$API_URL/startup-screen" \
  -H "Content-Type: application/json" \
  -d '{
    "app_version": "1.0.0",
    "platform": "ios",
    "device_id": "very-old-device-000"
  }' | jq '.'
echo
echo

echo "✨ API testing completed!"
echo
echo "💡 To start the server, run:"
echo "   go run cmd/mobile-api/main.go --port 8080"
echo
echo "📱 To test with the React Native app:"
echo "   cd mobile-app && npm install && npm start"