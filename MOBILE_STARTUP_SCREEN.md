# Mobile App Conditional Startup Screen Implementation

This implementation adds a conditional startup screen feature to mobile apps that can display dynamic content based on backend API responses. The screen can be used for forced app updates, promotional content, onboarding, and announcements without requiring app updates.

## 🏗️ Architecture Overview

```
Mobile App (React Native)
    ↓ API Request on startup
Backend API Server (Go)
    ↓ Returns configuration
Conditional Startup Screen
    ↓ User actions
App continues or blocks
```

## 📁 Project Structure

```
├── mobile_api/                 # Go backend API server
│   ├── server.go              # Main API server implementation
│   └── README.md              # API documentation
├── cmd/mobile-api/            # Executable command
│   └── main.go               # Server entry point
├── mobile-app/               # React Native mobile app
│   ├── src/
│   │   ├── types/           # TypeScript type definitions
│   │   ├── services/        # API service layer
│   │   ├── components/      # UI components
│   │   └── App.tsx         # Main app component
│   └── package.json        # Dependencies
├── test_api.sh              # API testing script
└── go.mod                   # Go dependencies
```

## 🚀 Quick Start

### 1. Start the Backend API Server

```bash
# Install Go dependencies
go mod tidy

# Start the server
go run cmd/mobile-api/main.go --port 8080
```

### 2. Test the API

```bash
# Run the test script
./test_api.sh

# Or test manually
curl -X POST http://localhost:8080/api/v1/startup-screen \
  -H "Content-Type: application/json" \
  -d '{
    "app_version": "1.0.0",
    "platform": "ios",
    "device_id": "test-device"
  }'
```

### 3. Run the Mobile App (Optional)

```bash
cd mobile-app
npm install
npm start

# In another terminal
npm run ios     # or npm run android
```

## 🎯 Key Features

### ✅ **Conditional Display Logic**
- Show screens based on app version, platform, user status
- Custom business logic for targeting specific user segments
- Time-based expiration for temporary campaigns

### ✅ **Forced App Updates**
- Block app usage for critical security updates
- Redirect users to appropriate app stores
- Different URLs for iOS and Android platforms

### ✅ **Dynamic Content**
- Update screen content without app releases
- Support for images, custom colors, and branding
- Multiple action buttons with different behaviors

### ✅ **Smart Tracking**
- Record when screens are shown to avoid spam
- Device-specific tracking using unique identifiers
- Configurable display frequency

## 📊 API Response Examples

### Forced Update Screen (Blocking)
```json
{
  "show_screen": true,
  "is_blocking": true,
  "title": "Update Required",
  "message": "A new version with important security updates is available.",
  "background_color": "#FF6B6B",
  "text_color": "#FFFFFF",
  "actions": [
    {
      "text": "Update Now",
      "type": "force_update",
      "url": "https://apps.apple.com/app/hal/id123456789",
      "is_primary": true
    }
  ]
}
```

### Welcome Screen (Non-blocking)
```json
{
  "show_screen": true,
  "is_blocking": false,
  "title": "Welcome to HAL Mobile!",
  "message": "Manage your home automation from anywhere.",
  "image_url": "https://example.com/welcome.png",
  "background_color": "#4A90E2",
  "text_color": "#FFFFFF",
  "expires_at": "2024-02-01T00:00:00Z",
  "actions": [
    {
      "text": "Get Started",
      "type": "dismiss",
      "is_primary": true
    },
    {
      "text": "Learn More",
      "type": "redirect",
      "url": "https://example.com/learn-more",
      "is_primary": false
    }
  ]
}
```

### No Screen
```json
{
  "show_screen": false
}
```

## 🔧 Customization

### Backend Logic (`mobile_api/server.go`)

The `generateStartupScreenConfig` function contains the business logic for determining when to show startup screens. Customize this based on your needs:

```go
func (s *MobileAPIServer) generateStartupScreenConfig(req StartupScreenRequest) StartupScreenConfig {
    // Add your custom logic here:
    
    // Version-based updates
    if s.isOldVersion(req.AppVersion) {
        return forceUpdateScreen()
    }
    
    // User targeting
    if s.shouldShowPromotion(req) {
        return promotionalScreen()
    }
    
    // A/B testing
    if s.isInExperimentGroup(req.DeviceID) {
        return experimentScreen()
    }
    
    // Default: no screen
    return StartupScreenConfig{ShowScreen: false}
}
```

### Mobile App Integration

The React Native app demonstrates how to:

1. **Call the API on startup** with device context
2. **Handle different response types** (blocking vs non-blocking)
3. **Display the modal screen** with dynamic styling
4. **Process user actions** (dismiss, redirect, force update)
5. **Track display history** to avoid repeated shows

## 🔐 Security Considerations

- **Input Validation**: All API inputs are validated
- **HTTPS**: Use HTTPS in production environments
- **Rate Limiting**: Implement rate limiting for API endpoints
- **URL Validation**: Sanitize URLs before opening them in the app
- **Authentication**: Add authentication for sensitive content

## 🧪 Testing Scenarios

The `test_api.sh` script demonstrates these scenarios:

1. **Health Check**: Verify API is running
2. **New User**: First-time app launch (shows welcome)
3. **Old Version**: App needs forced update (blocking)
4. **Current User**: Regular user (no screen)
5. **Very Old Version**: Critical update required (blocking)

## 📱 Mobile App Features

- **Conditional Rendering**: Shows startup screen based on API response
- **Action Handling**: Supports dismiss, redirect, and force update actions
- **Responsive Design**: Adapts to different screen sizes and orientations
- **Error Handling**: Graceful fallback when API is unavailable
- **Loading States**: Shows loading indicator during API calls

## 🚀 Production Deployment

### Backend
- Deploy the Go server to your cloud provider
- Set up HTTPS with proper SSL certificates
- Configure environment variables for different environments
- Add monitoring and logging
- Implement database storage for configurations

### Mobile App
- Update API_BASE_URL in the mobile app to your production server
- Test thoroughly on both iOS and Android
- Consider implementing offline fallbacks
- Add analytics to track startup screen effectiveness

## 📈 Analytics & Monitoring

Consider tracking these metrics:

- **Display Rate**: How often screens are shown
- **Action Rate**: Which actions users take
- **Update Success**: How many users successfully update
- **Conversion Rate**: Effectiveness of promotional content
- **Error Rate**: API failures and mobile app errors

## 🤝 Contributing

To extend this implementation:

1. **Add new action types** in both backend and mobile app
2. **Implement A/B testing** for different screen variants
3. **Add database persistence** for configurations
4. **Create an admin interface** for managing content
5. **Add push notification integration** for proactive updates

This implementation provides a solid foundation for dynamic mobile app content that can be updated without requiring app store releases.