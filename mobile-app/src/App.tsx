import React, { useState, useEffect } from 'react';
import {
  View,
  Text,
  StyleSheet,
  ActivityIndicator,
  SafeAreaView,
  Alert,
} from 'react-native';
import SplashScreen from 'react-native-splash-screen';
import { StartupScreen } from './components/StartupScreen';
import { ApiService } from './services/ApiService';
import { StartupScreenConfig } from './types/StartupScreen';

interface AppState {
  isLoading: boolean;
  startupConfig: StartupScreenConfig | null;
  showStartupScreen: boolean;
  apiError: string | null;
}

const App: React.FC = () => {
  const [state, setState] = useState<AppState>({
    isLoading: true,
    startupConfig: null,
    showStartupScreen: false,
    apiError: null,
  });

  const apiService = ApiService.getInstance();

  useEffect(() => {
    initializeApp();
  }, []);

  const initializeApp = async () => {
    try {
      // Check API health first
      const isApiHealthy = await apiService.checkHealth();
      
      if (!isApiHealthy) {
        console.warn('API is not available, skipping startup screen check');
        setState(prev => ({
          ...prev,
          isLoading: false,
          apiError: 'API service is not available',
        }));
        hideSplashScreen();
        return;
      }

      // Fetch startup screen configuration
      const response = await apiService.getStartupScreenConfig();
      
      if (response.error) {
        console.error('Failed to fetch startup screen config:', response.error);
        setState(prev => ({
          ...prev,
          isLoading: false,
          apiError: response.error || 'Unknown error',
        }));
        hideSplashScreen();
        return;
      }

      const config = response.data;
      
      if (config && config.show_screen) {
        // Check if the screen has expired
        if (config.expires_at) {
          const expiresAt = new Date(config.expires_at);
          const now = new Date();
          
          if (now > expiresAt) {
            console.log('Startup screen has expired, not showing');
            setState(prev => ({ ...prev, isLoading: false }));
            hideSplashScreen();
            return;
          }
        }

        // Record that we're showing the startup screen
        await apiService.recordStartupScreenShown();

        setState(prev => ({
          ...prev,
          isLoading: false,
          startupConfig: config,
          showStartupScreen: true,
        }));
      } else {
        setState(prev => ({ ...prev, isLoading: false }));
      }
      
      hideSplashScreen();
    } catch (error) {
      console.error('Error during app initialization:', error);
      setState(prev => ({
        ...prev,
        isLoading: false,
        apiError: error instanceof Error ? error.message : 'Initialization failed',
      }));
      hideSplashScreen();
    }
  };

  const hideSplashScreen = () => {
    // Hide the native splash screen
    try {
      SplashScreen.hide();
    } catch (error) {
      console.log('SplashScreen.hide() failed:', error);
    }
  };

  const handleStartupScreenDismiss = () => {
    setState(prev => ({
      ...prev,
      showStartupScreen: false,
    }));
  };

  const handleRetry = () => {
    setState(prev => ({
      ...prev,
      isLoading: true,
      apiError: null,
    }));
    initializeApp();
  };

  if (state.isLoading) {
    return (
      <View style={styles.loadingContainer}>
        <ActivityIndicator size="large" color="#4A90E2" />
        <Text style={styles.loadingText}>Loading...</Text>
      </View>
    );
  }

  return (
    <SafeAreaView style={styles.container}>
      {/* Main App Content */}
      <View style={styles.mainContent}>
        <Text style={styles.welcomeTitle}>HAL Home Automation</Text>
        <Text style={styles.welcomeSubtitle}>
          Welcome to your smart home control center
        </Text>
        
        {state.apiError && (
          <View style={styles.errorContainer}>
            <Text style={styles.errorTitle}>API Connection Issue</Text>
            <Text style={styles.errorText}>{state.apiError}</Text>
            <Text style={styles.errorHint}>
              Make sure the backend server is running on localhost:8080
            </Text>
          </View>
        )}
        
        <View style={styles.infoContainer}>
          <Text style={styles.infoTitle}>Demo Features:</Text>
          <Text style={styles.infoText}>
            • Conditional startup screens based on app version{'\n'}
            • Dynamic content without app updates{'\n'}
            • Forced update blocking for critical updates{'\n'}
            • Promotional and onboarding content
          </Text>
        </View>
      </View>

      {/* Startup Screen Modal */}
      {state.startupConfig && (
        <StartupScreen
          config={state.startupConfig}
          visible={state.showStartupScreen}
          onDismiss={handleStartupScreenDismiss}
        />
      )}
    </SafeAreaView>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f5f5f5',
  },
  loadingContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#f5f5f5',
  },
  loadingText: {
    marginTop: 10,
    fontSize: 16,
    color: '#666',
  },
  mainContent: {
    flex: 1,
    padding: 20,
    justifyContent: 'center',
  },
  welcomeTitle: {
    fontSize: 32,
    fontWeight: 'bold',
    textAlign: 'center',
    color: '#333',
    marginBottom: 10,
  },
  welcomeSubtitle: {
    fontSize: 18,
    textAlign: 'center',
    color: '#666',
    marginBottom: 40,
  },
  errorContainer: {
    backgroundColor: '#ffebee',
    padding: 15,
    borderRadius: 8,
    marginBottom: 20,
    borderLeftWidth: 4,
    borderLeftColor: '#f44336',
  },
  errorTitle: {
    fontSize: 16,
    fontWeight: 'bold',
    color: '#c62828',
    marginBottom: 5,
  },
  errorText: {
    fontSize: 14,
    color: '#d32f2f',
    marginBottom: 5,
  },
  errorHint: {
    fontSize: 12,
    color: '#757575',
    fontStyle: 'italic',
  },
  infoContainer: {
    backgroundColor: '#e3f2fd',
    padding: 15,
    borderRadius: 8,
    borderLeftWidth: 4,
    borderLeftColor: '#2196f3',
  },
  infoTitle: {
    fontSize: 16,
    fontWeight: 'bold',
    color: '#1565c0',
    marginBottom: 10,
  },
  infoText: {
    fontSize: 14,
    color: '#1976d2',
    lineHeight: 20,
  },
});

export default App;