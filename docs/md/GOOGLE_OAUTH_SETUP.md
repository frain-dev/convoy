# Google OAuth Setup for Convoy

This guide will help you set up Google OAuth authentication for your Convoy application using the new Google Identity Services (GIS) library.

## Prerequisites

- A Google Cloud Platform account
- Convoy application running on port 5005 (serves both UI and server when bundled)

## Step 1: Google Cloud Console Setup

### 1.1 Create or Select a Project
1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one

### 1.2 Create OAuth 2.0 Credentials
1. Go to **"APIs & Services"** → **"Credentials"**
2. Click **"Create Credentials"** → **"OAuth 2.0 Client IDs"**
3. Choose **"Web application"**
4. Fill in the following details:

#### Authorized JavaScript Origins
```
http://localhost
http://localhost:5005
```
or for production:
```
https://your-domain.com
```

#### Authorized Redirect URIs
```
http://localhost:5005/ui/auth/google/callback
```
or for production:
```
https://your-domain.com/ui/auth/google/callback
```

### 1.4 Copy Credentials
- Copy your **Client ID**
- Keep this secure - you'll need it for the next step

## Step 2: Convoy Configuration

### 2.1 Update convoy.json
Add the following configuration to your `convoy.json`:

```json
{
  "auth": {
    "google_oauth": {
      "enabled": true,
      "client_id": "your-google-client-id.apps.googleusercontent.com",
      "redirect_url": "http://localhost:5005/ui/auth/google/callback"
    }
  }
}
```

**Note**: Google OAuth requires a valid enterprise license with the `GOOGLE_OAUTH` feature to function.

### 2.2 Environment Variables (Alternative)
You can also use environment variables:

```bash
export CONVOY_GOOGLE_OAUTH_ENABLED=true
export CONVOY_GOOGLE_OAUTH_CLIENT_ID="your-google-client-id.apps.googleusercontent.com"
export CONVOY_GOOGLE_OAUTH_REDIRECT_URL="http://localhost:5005/ui/auth/google/callback"
```



## Step 3: Testing

### 3.1 Start Your Service
```bash
# Backend with bundled UI (port 5005)
cd /path/to/convoy
./server
```

### 3.2 Test Google Sign-In
1. Navigate to `http://localhost:5005`
2. Click **"Sign in with Google"**
3. You should be redirected to Google's OAuth consent screen
4. After authentication, you'll be redirected back to Convoy

## Troubleshooting

### Common Issues

#### 1. "Not a valid origin for the client"
**Problem**: Google blocks requests from unregistered origins

**Solution**: Ensure your domain (e.g., `http://localhost:5005` for testing or `https://your-domain.com` for production) is added to **Authorized JavaScript Origins**

#### 2. "idpiframe_initialization_failed"
**Problem**: Using deprecated OAuth libraries

**Solution**: This implementation uses the new Google Identity Services library

#### 3. Redirect URI Mismatch
**Problem**: Redirect URI doesn't match what's configured

**Solution**: Ensure the redirect URL in `convoy.json` matches exactly what's in Google Console

### Debug Steps

1. **Check Browser Console**: Look for JavaScript errors
2. **Check Network Tab**: Verify API calls are successful
3. **Check Backend Logs**: Look for authentication errors
4. **Verify Configuration**: Ensure all settings match between Google Console and Convoy

## Security Notes


- **HTTPS**: In production, use HTTPS for all OAuth flows
- **State Parameter**: Consider implementing state parameter validation for additional security
- **Scopes**: Only request necessary scopes (`openid email profile`)

## Production Considerations

- Update origins to your production domain in `convoy.json`
- Use environment variables for sensitive configuration
- Implement proper error handling and user feedback
- Consider implementing refresh token logic
- Monitor OAuth usage and errors
- **License Management**: Ensure your enterprise license includes the `GOOGLE_OAUTH` feature
- **Configuration Control**: Use the `enabled` flag to control Google OAuth availability

## Migration from Old OAuth Libraries

This implementation uses Google's new Identity Services library, which:
- ✅ Is future-proof and follows current Google standards
- ✅ Provides better security and user experience
- ✅ Supports modern OAuth flows
- ✅ Is actively maintained by Google

## Enterprise Features

### License Requirements
- **Google OAuth**: Requires `GOOGLE_OAUTH` license feature

### Configuration Control
Google OAuth can be controlled via configuration:
```json
"auth": {
    "google_oauth": { "enabled": false }
}
```



## Support

If you encounter issues:
1. Check this troubleshooting guide
2. Review Google's [Identity Services documentation](https://developers.google.com/identity/gsi/web)
3. Check Convoy's GitHub issues
4. Ensure your Google Cloud Console configuration is correct
