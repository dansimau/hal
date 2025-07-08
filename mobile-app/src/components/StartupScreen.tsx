import React from 'react';
import {
  View,
  Text,
  TouchableOpacity,
  Image,
  StyleSheet,
  Modal,
  Linking,
  Alert,
  Dimensions,
  StatusBar,
} from 'react-native';
import { StartupScreenConfig, ScreenAction } from '../types/StartupScreen';

interface StartupScreenProps {
  config: StartupScreenConfig;
  visible: boolean;
  onDismiss: () => void;
}

const { width: screenWidth, height: screenHeight } = Dimensions.get('window');

export const StartupScreen: React.FC<StartupScreenProps> = ({
  config,
  visible,
  onDismiss,
}) => {
  const handleActionPress = async (action: ScreenAction) => {
    switch (action.type) {
      case 'dismiss':
        onDismiss();
        break;
      
      case 'redirect':
        if (action.url) {
          try {
            await Linking.openURL(action.url);
            if (!config.is_blocking) {
              onDismiss();
            }
          } catch (error) {
            Alert.alert('Error', 'Unable to open link');
          }
        }
        break;
      
      case 'force_update':
        if (action.url) {
          try {
            await Linking.openURL(action.url);
            // Don't dismiss for force updates - user needs to update
          } catch (error) {
            Alert.alert('Error', 'Unable to open app store');
          }
        }
        break;
    }
  };

  const backgroundColor = config.background_color || '#4A90E2';
  const textColor = config.text_color || '#FFFFFF';

  const renderActions = () => {
    if (!config.actions || config.actions.length === 0) {
      return (
        <TouchableOpacity
          style={[styles.button, { backgroundColor: textColor }]}
          onPress={onDismiss}
          disabled={config.is_blocking}
        >
          <Text style={[styles.buttonText, { color: backgroundColor }]}>
            {config.button_text || 'Continue'}
          </Text>
        </TouchableOpacity>
      );
    }

    return config.actions.map((action, index) => (
      <TouchableOpacity
        key={index}
        style={[
          styles.button,
          action.is_primary ? styles.primaryButton : styles.secondaryButton,
          { 
            backgroundColor: action.is_primary ? textColor : 'transparent',
            borderColor: textColor,
          }
        ]}
        onPress={() => handleActionPress(action)}
      >
        <Text
          style={[
            styles.buttonText,
            {
              color: action.is_primary ? backgroundColor : textColor,
            }
          ]}
        >
          {action.text}
        </Text>
      </TouchableOpacity>
    ));
  };

  return (
    <Modal
      visible={visible}
      animationType="fade"
      transparent={false}
      onRequestClose={config.is_blocking ? undefined : onDismiss}
    >
      <StatusBar
        backgroundColor={backgroundColor}
        barStyle={textColor === '#FFFFFF' ? 'light-content' : 'dark-content'}
      />
      <View style={[styles.container, { backgroundColor }]}>
        <View style={styles.content}>
          {config.image_url && (
            <Image
              source={{ uri: config.image_url }}
              style={styles.image}
              resizeMode="contain"
            />
          )}
          
          <Text style={[styles.title, { color: textColor }]}>
            {config.title}
          </Text>
          
          <Text style={[styles.message, { color: textColor }]}>
            {config.message}
          </Text>
          
          <View style={styles.actionsContainer}>
            {renderActions()}
          </View>
          
          {config.is_blocking && (
            <Text style={[styles.blockingNote, { color: textColor }]}>
              This update is required to continue using the app
            </Text>
          )}
        </View>
      </View>
    </Modal>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    paddingHorizontal: 20,
  },
  content: {
    width: '100%',
    maxWidth: 400,
    alignItems: 'center',
  },
  image: {
    width: screenWidth * 0.6,
    height: screenWidth * 0.4,
    marginBottom: 30,
  },
  title: {
    fontSize: 28,
    fontWeight: 'bold',
    textAlign: 'center',
    marginBottom: 20,
  },
  message: {
    fontSize: 16,
    textAlign: 'center',
    lineHeight: 24,
    marginBottom: 40,
  },
  actionsContainer: {
    width: '100%',
    gap: 15,
  },
  button: {
    paddingVertical: 15,
    paddingHorizontal: 30,
    borderRadius: 8,
    alignItems: 'center',
    minWidth: 200,
  },
  primaryButton: {
    borderWidth: 0,
  },
  secondaryButton: {
    borderWidth: 2,
  },
  buttonText: {
    fontSize: 16,
    fontWeight: '600',
  },
  blockingNote: {
    fontSize: 12,
    textAlign: 'center',
    marginTop: 20,
    opacity: 0.8,
  },
});