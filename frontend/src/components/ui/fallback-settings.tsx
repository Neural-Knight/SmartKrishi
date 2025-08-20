import { useState, useEffect, useRef, Suspense, lazy } from 'react';
import { Button } from './button';
import { Label } from './label';
import { Switch } from './switch.tsx';
import { Card, CardContent, CardHeader, CardTitle } from './card';
import { Alert, AlertDescription } from './alert.tsx'
import { Loader2, Phone, CheckCircle, AlertTriangle } from 'lucide-react';
import type { ConfirmationResult } from 'firebase/auth';
import 'react-phone-number-input/style.css';

// Lazy load PhoneInput for better performance  
const PhoneInput = lazy(() => import('react-phone-number-input'));

// Dynamic import for Firebase (only load when needed)
const loadFirebaseAuth = () => import('@/lib/firebase');

// PhoneInput fallback component
const PhoneInputFallback = () => (
  <div className="flex h-12 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background">
    <span className="text-muted-foreground">Loading phone input...</span>
  </div>
);

interface FallbackSettings {
  auto_fallback_enabled: boolean;
  fallback_phone: string | null;
  fallback_phone_verified: boolean;
}

interface FallbackSettingsProps {
  onSettingsChange?: (settings: FallbackSettings) => void;
}

export function FallbackSettings({ onSettingsChange }: FallbackSettingsProps) {
  const [settings, setSettings] = useState<FallbackSettings>({
    auto_fallback_enabled: false,
    fallback_phone: null,
    fallback_phone_verified: false,
  });
  
  const [phoneNumber, setPhoneNumber] = useState('');
  const [loading, setLoading] = useState(false);
  const [verifying, setVerifying] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error' | 'info'; text: string } | null>(null);

  // Firebase OTP verification state
  const [confirmationResult, setConfirmationResult] = useState<ConfirmationResult | null>(null);
  const [showOtpInput, setShowOtpInput] = useState(false);
  const [otpValues, setOtpValues] = useState<string[]>(['', '', '', '', '', '']);
  const otpRefs = useRef<(HTMLInputElement | null)[]>([]);
  const recaptchaRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    loadSettings();
  }, []);

  const loadSettings = async () => {
    try {
      const token = localStorage.getItem('auth_token');
      if (!token) {
        // Error handling
        return;
      }
      const response = await fetch(`${import.meta.env.VITE_API_BASE_URL}/api/v1/fallback/settings`, {
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        }
      });

      if (response.ok) {
        const data = await response.json();
        setSettings(data);
        setPhoneNumber(data.fallback_phone || '');
      }
    } catch (error) {
      // Error handling
    }
  };

  const updateSettings = async (newSettings: Partial<FallbackSettings>) => {
    setLoading(true);
    setMessage(null);

    try {
      const token = localStorage.getItem('auth_token');
      if (!token) {
        throw new Error('No authentication token found');
      }
      const response = await fetch(`${import.meta.env.VITE_API_BASE_URL}/api/v1/fallback/settings`, {
        method: 'PUT',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(newSettings)
      });

      if (response.ok) {
        const updatedSettings = { ...settings, ...newSettings };
        setSettings(updatedSettings);
        onSettingsChange?.(updatedSettings);
        setMessage({ type: 'success', text: 'Settings updated successfully!' });
      } else {
        throw new Error('Failed to update settings');
      }
    } catch (error) {
      setMessage({ type: 'error', text: 'Failed to update settings. Please try again.' });
    } finally {
      setLoading(false);
    }
  };

  // OTP input handlers
  const handleOtpChange = (index: number, value: string) => {
    if (!/^\d*$/.test(value)) return; // Only allow digits
    
    const newOtpValues = [...otpValues];
    newOtpValues[index] = value;
    setOtpValues(newOtpValues);
    
    // Auto-focus next input
    if (value && index < 5) {
      otpRefs.current[index + 1]?.focus();
    }
  };

  const handleOtpKeyDown = (index: number, e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Backspace' && !otpValues[index] && index > 0) {
      otpRefs.current[index - 1]?.focus();
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const otpString = otpValues.join('');
      if (otpString.length === 6) {
        handleOtpVerification(otpString);
      }
    }
  };

  const handleOtpVerification = async (otpString: string) => {
    if (!confirmationResult) {
      setMessage({ type: 'error', text: 'No OTP confirmation available. Please request a new OTP.' });
      return;
    }

    setVerifying(true);
    setMessage(null);

    try {
      const credential = await confirmationResult.confirm(otpString);
      const { getFirebaseIdToken } = await loadFirebaseAuth();
      const firebaseToken = await getFirebaseIdToken(credential.user);

      // Verify with backend
      const token = localStorage.getItem('auth_token');
      if (!token) {
        throw new Error('No authentication token found');
      }

      const response = await fetch(`${import.meta.env.VITE_API_BASE_URL}/api/v1/fallback/verify-phone`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ firebase_token: firebaseToken })
      });

      if (response.ok) {
        const updatedSettings = { ...settings, fallback_phone_verified: true };
        setSettings(updatedSettings);
        onSettingsChange?.(updatedSettings);
        setShowOtpInput(false);
        setOtpValues(['', '', '', '', '', '']);
        setMessage({ type: 'success', text: 'Phone number verified successfully!' });
      } else {
        const errorData = await response.json().catch(() => ({ detail: 'Unknown error' }));
        // Error handling
        throw new Error(errorData.detail || `Server error: ${response.status}`);
      }
    } catch (error) {
      // Error handling
      setMessage({ 
        type: 'error', 
        text: error instanceof Error ? error.message : 'Invalid OTP. Please try again.' 
      });
    } finally {
      setVerifying(false);
    }
  };

  const verifyPhone = async () => {
    if (!phoneNumber.trim()) {
      setMessage({ type: 'error', text: 'Please enter a phone number first.' });
      return;
    }

    setVerifying(true);
    setMessage(null);

    try {
      // First update the phone number in backend
      await updateSettings({ fallback_phone: phoneNumber });

      // Then send OTP via Firebase
      const { setupRecaptcha, sendOTPToPhone } = await loadFirebaseAuth();
      const verifier = await setupRecaptcha('recaptcha-container', { forceNew: true, size: 'invisible' });
      const confirmation = await sendOTPToPhone(phoneNumber, verifier);
      
      setConfirmationResult(confirmation);
      setShowOtpInput(true);
      setMessage({ 
        type: 'success', 
        text: 'Verification OTP sent! Enter the 6-digit code below.' 
      });
      
      // Focus first OTP input
      setTimeout(() => {
        otpRefs.current[0]?.focus();
      }, 100);
      
    } catch (error) {
      // Error handling
      setMessage({ 
        type: 'error', 
        text: error instanceof Error ? error.message : 'Failed to send verification OTP. Please try again.' 
      });
    } finally {
      setVerifying(false);
    }
  };

  const toggleAutoFallback = async (enabled: boolean) => {
    if (enabled && (!settings.fallback_phone || !settings.fallback_phone_verified)) {
      setMessage({ 
        type: 'error', 
        text: 'Please add and verify a phone number before enabling automatic fallback.' 
      });
      return;
    }

    await updateSettings({ auto_fallback_enabled: enabled });
  };

  const getVerificationStatus = () => {
    if (!settings.fallback_phone) {
      return { icon: Phone, color: 'text-gray-400', text: 'No phone number set' };
    }
    if (!settings.fallback_phone_verified) {
      return { icon: AlertTriangle, color: 'text-yellow-500', text: 'Phone not verified' };
    }
    return { icon: CheckCircle, color: 'text-green-500', text: 'Phone verified' };
  };

  const verificationStatus = getVerificationStatus();
  const StatusIcon = verificationStatus.icon;

  return (
    <Card className="w-full max-w-2xl">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Phone className="h-5 w-5" />
          SMS Fallback Settings
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        {message && (
          <Alert className={message.type === 'error' ? 'border-red-200 bg-red-50' : 
                           message.type === 'success' ? 'border-green-200 bg-green-50' : 
                           'border-blue-200 bg-blue-50'}>
            <AlertDescription className={message.type === 'error' ? 'text-red-700' : 
                                      message.type === 'success' ? 'text-green-700' : 
                                      'text-blue-700'}>
              {message.text}
            </AlertDescription>
          </Alert>
        )}

        {/* Phone Number Configuration */}
        <div className="space-y-4">
          <div>
            <Label htmlFor="phone" className="text-base font-semibold">
              Fallback Phone Number
            </Label>
            <p className="text-sm text-gray-600 mb-3">
              This phone number will receive SMS messages when your internet connection is poor.
            </p>
          </div>
          
          <div className="flex flex-col sm:flex-row gap-3 items-stretch sm:items-end">
            <div className="flex-1">
              <Suspense fallback={<PhoneInputFallback />}>
                <PhoneInput
                  placeholder="+91 9876543210"
                  value={phoneNumber}
                  onChange={(value: string | undefined) => {
                    setPhoneNumber(value || '');
                  }}
                  defaultCountry="IN"
                  className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                />
              </Suspense>
            </div>
            <Button
              onClick={verifyPhone}
              disabled={verifying || !phoneNumber.trim()}
              variant={settings.fallback_phone_verified ? "outline" : "default"}
              className="w-full sm:w-auto flex-shrink-0"
            >
              {verifying && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
              {settings.fallback_phone_verified ? 'Re-verify' : 'Verify'}
            </Button>
          </div>

          <div className="flex items-center gap-2 text-sm">
            <StatusIcon className={`h-4 w-4 ${verificationStatus.color}`} />
            <span className={verificationStatus.color}>{verificationStatus.text}</span>
          </div>

          {/* OTP Input Section */}
          {showOtpInput && (
            <div className="space-y-4 p-4 bg-blue-50 border border-blue-200 rounded-lg">
              <div className="text-center">
                <Label className="text-base font-semibold text-blue-800">
                  Enter Verification Code
                </Label>
                <p className="text-sm text-blue-600 mt-1">
                  Enter the 6-digit code sent to {phoneNumber}
                </p>
              </div>
              
              <div className="flex justify-center space-x-2 sm:space-x-3">
                {otpValues.map((digit, index) => (
                  <input
                    key={index}
                    ref={(el) => {
                      otpRefs.current[index] = el;
                    }}
                    type="text"
                    maxLength={1}
                    value={digit}
                    onChange={(e) => handleOtpChange(index, e.target.value)}
                    onKeyDown={(e) => handleOtpKeyDown(index, e)}
                    className="w-8 h-8 sm:w-12 sm:h-12 text-center text-lg sm:text-xl font-bold border-2 border-blue-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-colors"
                    inputMode="numeric"
                  />
                ))}
              </div>
              
              <div className="flex justify-center space-x-3">
                <Button
                  onClick={() => {
                    const otpString = otpValues.join('');
                    if (otpString.length === 6) {
                      handleOtpVerification(otpString);
                    } else {
                      setMessage({ type: 'error', text: 'Please enter all 6 digits' });
                    }
                  }}
                  disabled={verifying || otpValues.join('').length !== 6}
                  className="bg-blue-600 hover:bg-blue-700"
                >
                  {verifying && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                  Verify Code
                </Button>
                
                <Button
                  onClick={() => {
                    setShowOtpInput(false);
                    setOtpValues(['', '', '', '', '', '']);
                    setMessage(null);
                  }}
                  variant="outline"
                >
                  Cancel
                </Button>
              </div>
            </div>
          )}

          {/* reCAPTCHA Container - Hidden but present for invisible reCAPTCHA */}
          <div id="recaptcha-container" ref={recaptchaRef} className="h-0 overflow-hidden"></div>
        </div>

        {/* Auto Fallback Toggle */}
        <div className="space-y-4 pt-4 border-t">
          <div>
            <Label className="text-base font-semibold">Automatic SMS Fallback</Label>
            <p className="text-sm text-gray-600 mb-4">
              Automatically switch to SMS when your connection becomes unstable. 
              You'll receive farming assistance via text messages.
            </p>
          </div>

          <div className="flex items-center justify-between p-4 bg-gray-50 rounded-lg">
            <div className="flex items-center gap-3">
              <div className={`h-3 w-3 rounded-full ${settings.auto_fallback_enabled ? 'bg-green-500' : 'bg-gray-300'}`} />
              <span className="font-medium">
                {settings.auto_fallback_enabled ? 'Auto Fallback Enabled' : 'Auto Fallback Disabled'}
              </span>
            </div>
            
            <Switch
              checked={settings.auto_fallback_enabled}
              onCheckedChange={toggleAutoFallback}
              disabled={loading}
            />
          </div>

          {settings.auto_fallback_enabled && (
            <Alert className="border-green-200 bg-green-50">
              <CheckCircle className="h-4 w-4 text-green-600" />
              <AlertDescription className="text-green-700">
                SMS fallback is active. You'll automatically receive SMS notifications when your connection is poor.
              </AlertDescription>
            </Alert>
          )}
        </div>

        {/* How It Works */}
        <div className="space-y-3 pt-4 border-t">
          <Label className="text-base font-semibold">How It Works</Label>
          <div className="text-sm text-gray-600 space-y-2">
            <div className="flex items-start gap-2">
              <div className="h-2 w-2 bg-blue-500 rounded-full mt-2 flex-shrink-0" />
              <span>We monitor your connection quality in the background</span>
            </div>
            <div className="flex items-start gap-2">
              <div className="h-2 w-2 bg-blue-500 rounded-full mt-2 flex-shrink-0" />
              <span>When connection becomes poor, SMS fallback automatically activates</span>
            </div>
            <div className="flex items-start gap-2">
              <div className="h-2 w-2 bg-blue-500 rounded-full mt-2 flex-shrink-0" />
              <span>Continue your farming conversations via SMS</span>
            </div>
            <div className="flex items-start gap-2">
              <div className="h-2 w-2 bg-blue-500 rounded-full mt-2 flex-shrink-0" />
              <span>When connection improves, you're notified to return to the app</span>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
