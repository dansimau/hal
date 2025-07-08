export interface StartupScreenConfig {
  show_screen: boolean;
  is_blocking: boolean;
  title: string;
  message: string;
  button_text: string;
  image_url?: string;
  background_color?: string;
  text_color?: string;
  expires_at?: string;
  min_app_version?: string;
  max_app_version?: string;
  actions?: ScreenAction[];
  metadata?: Record<string, string>;
}

export interface ScreenAction {
  text: string;
  type: 'dismiss' | 'redirect' | 'force_update';
  url?: string;
  is_primary: boolean;
}

export interface StartupScreenRequest {
  app_version: string;
  platform: 'ios' | 'android';
  device_id: string;
  last_shown_at?: string;
  user_id?: string;
}

export interface ApiResponse<T> {
  data?: T;
  error?: string;
  status: number;
}