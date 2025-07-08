import AsyncStorage from '@react-native-async-storage/async-storage';
import DeviceInfo from 'react-native-device-info';
import { Platform } from 'react-native';
import { StartupScreenConfig, StartupScreenRequest, ApiResponse } from '../types/StartupScreen';

const API_BASE_URL = 'http://localhost:8080/api/v1';
const LAST_SHOWN_KEY = 'startup_screen_last_shown';

export class ApiService {
  private static instance: ApiService;

  public static getInstance(): ApiService {
    if (!ApiService.instance) {
      ApiService.instance = new ApiService();
    }
    return ApiService.instance;
  }

  /**
   * Fetches the startup screen configuration from the backend
   */
  async getStartupScreenConfig(): Promise<ApiResponse<StartupScreenConfig>> {
    try {
      const request = await this.buildStartupScreenRequest();
      
      const response = await fetch(`${API_BASE_URL}/startup-screen`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(request),
      });

      const data = await response.json();

      if (!response.ok) {
        return {
          error: data.message || 'Failed to fetch startup screen config',
          status: response.status,
        };
      }

      return {
        data,
        status: response.status,
      };
    } catch (error) {
      console.error('Error fetching startup screen config:', error);
      return {
        error: error instanceof Error ? error.message : 'Network error',
        status: 0,
      };
    }
  }

  /**
   * Builds the startup screen request with device and app information
   */
  private async buildStartupScreenRequest(): Promise<StartupScreenRequest> {
    const [appVersion, deviceId, lastShownAt] = await Promise.all([
      DeviceInfo.getVersion(),
      DeviceInfo.getUniqueId(),
      this.getLastShownAt(),
    ]);

    return {
      app_version: appVersion,
      platform: Platform.OS as 'ios' | 'android',
      device_id: deviceId,
      last_shown_at: lastShownAt,
    };
  }

  /**
   * Gets the last time the startup screen was shown
   */
  private async getLastShownAt(): Promise<string | undefined> {
    try {
      const lastShown = await AsyncStorage.getItem(LAST_SHOWN_KEY);
      return lastShown || undefined;
    } catch (error) {
      console.error('Error getting last shown time:', error);
      return undefined;
    }
  }

  /**
   * Records that the startup screen was shown
   */
  async recordStartupScreenShown(): Promise<void> {
    try {
      const now = new Date().toISOString();
      await AsyncStorage.setItem(LAST_SHOWN_KEY, now);
    } catch (error) {
      console.error('Error recording startup screen shown:', error);
    }
  }

  /**
   * Checks the health of the API server
   */
  async checkHealth(): Promise<boolean> {
    try {
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 5000);
      
      const response = await fetch(`${API_BASE_URL}/health`, {
        method: 'GET',
        signal: controller.signal,
      });
      
      clearTimeout(timeoutId);
      return response.ok;
    } catch (error) {
      console.error('Health check failed:', error);
      return false;
    }
  }
}