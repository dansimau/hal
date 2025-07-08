# HAL Mobile API - Conditional Startup Screen

This package provides a REST API server for mobile apps to fetch conditional startup screen configurations. The startup screen can be used for various purposes including forced app updates, promotional content, onboarding, and announcements.

## Features

- **Conditional Display**: Show startup screens based on app version, platform, user status, or custom logic
- **Forced Updates**: Block app usage for critical updates by setting `is_blocking: true`
- **Dynamic Content**: Update screen content without releasing a new app version
- **Expiration Support**: Set expiration times for temporary campaigns
- **Multiple Actions**: Support for multiple buttons with different action types
- **Platform Awareness**: Different configurations for iOS and Android
- **Device Tracking**: Track when screens were last shown to avoid spam

## API Endpoints

### POST /api/v1/startup-screen

Fetches the startup screen configuration for a mobile app.

**Request Body:**
```json
{
  "app_version": "1.5.0",
  "platform": "ios",
  "device_id": "unique-device-identifier",
  "last_shown_at": "2024-01-15T10:30:00Z",
  "user_id": "optional-user-id"
}
```

**Response:**
```json
{
  "show_screen": true,
  "is_blocking": false,
  "title": "Welcome to HAL Mobile!",
  "message": "Manage your home automation from anywhere. Set up your devices and create custom automations with ease.",
  "button_text": "Get Started",
  "image_url": "https://example.com/welcome-image.png",
  "background_color": "#4A90E2",
  "text_color": "#FFFFFF",
  "expires_at": "2024-01-16T10:30:00Z",
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
  ],
  "metadata": {
    "campaign_id": "welcome_2024",
    "content_type": "onboarding"
  }
}
```

### GET /api/v1/health

Health check endpoint.

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Configuration Fields

| Field | Type | Description |
|-------|------|-------------|
| `show_screen` | boolean | Whether to show the startup screen |
| `is_blocking` | boolean | If true, prevents app usage until action is taken |
| `title` | string | Main title text |
| `message` | string | Descriptive message text |
| `button_text` | string | Default button text (used if no actions array) |
| `image_url` | string (optional) | URL for header image |
| `background_color` | string (optional) | Hex color for background |
| `text_color` | string (optional) | Hex color for text |
| `expires_at` | string (optional) | ISO timestamp when screen expires |
| `min_app_version` | string (optional) | Minimum app version for this screen |
| `max_app_version` | string (optional) | Maximum app version for this screen |
| `actions` | array (optional) | Array of action buttons |
| `metadata` | object (optional) | Additional key-value data |

## Action Types

- **dismiss**: Closes the startup screen
- **redirect**: Opens a URL (external link, deep link, etc.)
- **force_update**: Opens app store for updates (typically used with `is_blocking: true`)

## Usage Examples

### Running the Server

```bash
# Install dependencies
go mod tidy

# Run the server
go run cmd/mobile-api/main.go --port 8080
```

### Example Use Cases

#### 1. Forced App Update
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

#### 2. Promotional Content
```json
{
  "show_screen": true,
  "is_blocking": false,
  "title": "New Feature Available!",
  "message": "Check out our new voice control feature.",
  "image_url": "https://example.com/feature-image.png",
  "expires_at": "2024-02-01T00:00:00Z",
  "actions": [
    {
      "text": "Try It Now",
      "type": "redirect",
      "url": "app://hal/features/voice",
      "is_primary": true
    },
    {
      "text": "Maybe Later",
      "type": "dismiss",
      "is_primary": false
    }
  ]
}
```

#### 3. No Screen (Default)
```json
{
  "show_screen": false
}
```

## Customization

The business logic for determining when to show startup screens is implemented in the `generateStartupScreenConfig` method in `mobile_api/server.go`. You can customize this logic based on your needs:

- App version comparisons
- User segmentation
- A/B testing
- Geographic targeting
- Time-based campaigns
- Feature flags

## Integration with Mobile Apps

The mobile app should:

1. Call the startup screen API on app launch
2. Check the `show_screen` flag
3. Display the screen if needed
4. Handle different action types appropriately
5. Respect the `is_blocking` flag for forced updates
6. Record when screens are shown to avoid repeated displays

See the `mobile-app/` directory for a complete React Native example implementation.

## Security Considerations

- Validate all input parameters
- Use HTTPS in production
- Implement rate limiting
- Add authentication if needed for sensitive content
- Sanitize any user-provided URLs before opening them

## Testing

```bash
# Test the health endpoint
curl http://localhost:8080/api/v1/health

# Test startup screen endpoint
curl -X POST http://localhost:8080/api/v1/startup-screen \
  -H "Content-Type: application/json" \
  -d '{
    "app_version": "1.0.0",
    "platform": "ios",
    "device_id": "test-device"
  }'
```