import React, { useState, useRef, useEffect, Suspense, lazy } from 'react';
import { useNavigate } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import * as z from 'zod';
import { motion, AnimatePresence } from 'framer-motion';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Eye, EyeOff, Smartphone, Mail, ArrowLeft } from 'lucide-react';
import { useAuthStore } from '@/store/authStore';
import { authService } from '@/services/auth';
import type { ConfirmationResult } from 'firebase/auth';
import { isValidPhoneNumber } from 'react-phone-number-input';

// Lazy load PhoneInput for better performance
const PhoneInput = lazy(() => import('react-phone-number-input'));

// Dynamic import for Firebase (only load when needed)
const loadFirebaseAuth = () => import('@/lib/firebase');

// Create conditional logger
// Development variables removed
// Conditional logging removed for production

// Update the validation schemas to use a single unified schema
const mobileSchema = z.object({
  phone_number: z
    .string()
    .refine((value) => isValidPhoneNumber(value || ''), {
      message: 'Please enter a valid phone number',
    }),
  username: z.string().optional(),
});

// Create a type that includes both required and optional username
type MobileFormData = {
  phone_number: string;
  username?: string;
};

// Validation schemas
const emailSchema = z.object({
  email: z.string().email('Please enter a valid email address'),
  password: z.string().min(6, 'Password must be at least 6 characters'),
});

const emailSignupSchema = z.object({
  name: z.string().min(2, 'Name must be at least 2 characters'),
  email: z.string().email('Please enter a valid email address'),
  password: z.string().min(6, 'Password must be at least 6 characters'),
  confirmPassword: z.string(),
}).refine((data) => data.password === data.confirmPassword, {
  message: "Passwords do not match",
  path: ["confirmPassword"],
});

const otpSchema = z.object({
  otp: z.string().length(6, 'OTP must be 6 digits'),
});

type EmailFormData = z.infer<typeof emailSchema>;
type EmailSignupFormData = z.infer<typeof emailSignupSchema>;
type OTPFormData = z.infer<typeof otpSchema>;

type LoginMode = 'mobile' | 'email';
type AuthStep = 'login' | 'otp';

const UnifiedAuthPage: React.FC = () => {
  const navigate = useNavigate();
  const { login, setLoading, isLoading } = useAuthStore();
  
  // Main state
  const [loginMode, setLoginMode] = useState<LoginMode>('mobile');
  const [authStep, setAuthStep] = useState<AuthStep>('login');
  const [isSignupMode, setIsSignupMode] = useState(false);
  
  // Performance optimization state
  const [backgroundImageLoaded, setBackgroundImageLoaded] = useState(false);
  
  // Mobile auth state
  const [phoneNumber, setPhoneNumber] = useState<string>('');
  const [isNewUser, setIsNewUser] = useState(false);
  const [confirmationResult, setConfirmationResult] = useState<ConfirmationResult | null>(null);
  
  // OTP state
  const [otpValues, setOtpValues] = useState<string[]>(['', '', '', '', '', '']);
  const otpRefs = useRef<(HTMLInputElement | null)[]>([]);
  
  // Timer state
  const [resendTimer, setResendTimer] = useState(0);
  const [canResend, setCanResend] = useState(true);
  
  // UI state
  const [error, setError] = useState<string>('');
  const [successMessage, setSuccessMessage] = useState<string>('');
  const [showPassword, setShowPassword] = useState(false);

  // Forms
  const mobileForm = useForm<MobileFormData>({
    resolver: zodResolver(mobileSchema),
    defaultValues: { phone_number: '', username: '' },
  });

  const emailForm = useForm<EmailFormData>({
    resolver: zodResolver(emailSchema),
    defaultValues: { email: '', password: '' },
  });

  const emailSignupForm = useForm<EmailSignupFormData>({
    resolver: zodResolver(emailSignupSchema),
    defaultValues: { name: '', email: '', password: '', confirmPassword: '' },
  });

  const otpForm = useForm<OTPFormData>({
    resolver: zodResolver(otpSchema),
    defaultValues: { otp: '' },
  });

  // Timer effect
  useEffect(() => {
    let interval: NodeJS.Timeout;
    if (resendTimer > 0) {
      interval = setInterval(() => {
        setResendTimer((prev) => {
          if (prev <= 1) {
            setCanResend(true);
            return 0;
          }
          return prev - 1;
        });
      }, 1000);
    }
    return () => clearInterval(interval);
  }, [resendTimer]);

  // Performance optimization: preload background image and Firebase
  useEffect(() => {
    // Preload background image
    const img = new Image();
    img.onload = () => setBackgroundImageLoaded(true);
    img.onerror = () => setBackgroundImageLoaded(true); // Still show page even if image fails
    img.src = '/farmerbackground.png';
    
    // Preload Firebase when mobile mode is likely to be used
    if (loginMode === 'mobile') {
      loadFirebaseAuth().catch(() => {
        // Warning handled
      });
    }
  }, [loginMode]);

  // Dynamic CSS loading for PhoneInput
  useEffect(() => {
    if (loginMode === 'mobile') {
      import('react-phone-number-input/style.css');
    }
  }, [loginMode]);

  // Auto-focus first OTP input
  useEffect(() => {
    if (authStep === 'otp') {
      const timer = setTimeout(() => {
        otpRefs.current[0]?.focus();
      }, 100);
      return () => clearTimeout(timer);
    }
  }, [authStep]);

  const startResendTimer = () => {
    setResendTimer(15);
    setCanResend(false);
  };

  const clearMessages = () => {
    setError('');
    setSuccessMessage('');
  };

  // OTP input handlers
  const handleOtpChange = (index: number, value: string) => {
    if (!/^\d*$/.test(value)) return;
    
    const newOtpValues = [...otpValues];
    newOtpValues[index] = value;
    setOtpValues(newOtpValues);
    
    const otpString = newOtpValues.join('');
    otpForm.setValue('otp', otpString);
    
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
        otpForm.handleSubmit(handleOTPSubmit)();
      }
    }
  };
  
  // Mobile auth handlers
  const handleMobileSubmit = async (data: MobileFormData) => {
    // Log removed

    // Log removed

    // Clear any previous errors
    mobileForm.clearErrors();

    // Manual validation for signup mode
    if (isSignupMode) {
      if (!data.username || data.username.trim().length < 2) {
        mobileForm.setError('username', {
          type: 'manual',
          message: 'Full name is required and must be at least 2 characters'
        });
        return;
      }
    }

    // Manual validation for phone number (additional check)
    if (!data.phone_number || !isValidPhoneNumber(data.phone_number)) {
      mobileForm.setError('phone_number', {
        type: 'manual',
        message: 'Please enter a valid phone number'
      });
      return;
    }

    clearMessages();
    setLoading(true);
    
    try {
      // Check if user exists first (faster API call)
      const response = await fetch(`${import.meta.env.VITE_API_BASE_URL}/api/v1/auth/mobile-init`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          phone_number: data.phone_number,
          username: data.username || 'User',
        }),
      });

      const result = await response.json();
      if (!response.ok) throw new Error(result.detail || 'Failed to initialize');

      // Handle existing user in signup mode
      if (isSignupMode && !result.is_new_user) {
        setError('Phone number already registered. Sign in instead.');
        setLoading(false);
        return;
      }

      // Handle new user in login mode
      if (!isSignupMode && result.is_new_user) {
        setError('Account not found. Please sign up first.');
        setLoading(false);
        return;
      }

      // Load Firebase only when needed
      const { setupRecaptcha, sendOTPToPhone } = await loadFirebaseAuth();
      const verifier = await setupRecaptcha('recaptcha-container', { forceNew: true, size: 'invisible' });
      const confirmation = await sendOTPToPhone(data.phone_number, verifier);
      
      setConfirmationResult(confirmation);
      setPhoneNumber(data.phone_number);
      setIsNewUser(result.is_new_user);
      setAuthStep('otp');
      startResendTimer();
      setSuccessMessage('OTP sent successfully! Check your phone.');
      
    } catch (err) {
      // Error handling
      setError(err instanceof Error ? err.message : 'Failed to send OTP');
    } finally {
      setLoading(false);
    }
  };

  const handleOTPSubmit = async (data: OTPFormData) => {
    clearMessages();
    setLoading(true);

    try {
      if (!confirmationResult) {
        throw new Error('No OTP confirmation available');
      }

      const credential = await confirmationResult.confirm(data.otp);
      const { getFirebaseIdToken } = await loadFirebaseAuth();
      const firebaseToken = await getFirebaseIdToken(credential.user);

      const endpoint = isNewUser 
        ? `${import.meta.env.VITE_API_BASE_URL}/api/v1/auth/mobile-signup`
        : `${import.meta.env.VITE_API_BASE_URL}/api/v1/auth/mobile-verify`;

      const body = isNewUser 
        ? {
            phone_number: phoneNumber,
            username: mobileForm.getValues('username') || 'User',
            firebase_token: firebaseToken,
          }
        : {
            phone_number: phoneNumber,
            otp: firebaseToken,
          };

      const response = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });

      const result = await response.json();
      if (!response.ok) throw new Error(result.detail || 'Failed to verify OTP');

      // Get user data and login
      try {
        const userData = await authService.getCurrentUser();
        login(result.access_token, userData);
      } catch {
        // Fallback user data
        const userData = {
          id: Date.now(),
          name: mobileForm.getValues('username') || 'User',
          phone_number: phoneNumber,
          auth_provider: 'mobile' as const,
          is_active: true,
          created_at: new Date().toISOString(),
        };
        login(result.access_token, userData);
      }

      navigate('/dashboard');
      
    } catch (err) {
      // Error handling
      setError(err instanceof Error ? err.message : 'Invalid OTP');
    } finally {
      setLoading(false);
    }
  };

  const handleResendOTP = async () => {
    if (!canResend) return;
    clearMessages();
    setLoading(true);

    try {
      const { setupRecaptcha, sendOTPToPhone } = await loadFirebaseAuth();
      const verifier = await setupRecaptcha('recaptcha-container', { forceNew: false, size: 'invisible' });
      const confirmation = await sendOTPToPhone(phoneNumber, verifier);
      setConfirmationResult(confirmation);
      
      setSuccessMessage('OTP resent successfully!');
      startResendTimer();
      setOtpValues(['', '', '', '', '', '']);
      otpForm.setValue('otp', '');
      otpRefs.current[0]?.focus();
      
    } catch (err) {
      // Error handling
      setError(err instanceof Error ? err.message : 'Failed to resend OTP');
    } finally {
      setLoading(false);
    }
  };

  // Email auth handler
  const handleEmailSubmit = async (data: EmailFormData) => {
    clearMessages();
    setLoading(true);

    try {
      useAuthStore.getState().setAuthenticating(true);

      const authResult = await authService.login(data);
      const userData = await authService.getCurrentUser();
      login(authResult.access_token, userData);
      navigate('/dashboard');
      
    } catch (err: any) {
      // Error handling
      setError(err.response?.data?.detail || err.message || 'Login failed');
    } finally {
      setLoading(false);
      useAuthStore.getState().setAuthenticating(false);
    }
  };

  // Email signup handler
  const handleEmailSignupSubmit = async (data: EmailSignupFormData) => {
    clearMessages();
    setLoading(true);

    try {
      useAuthStore.getState().setAuthenticating(true);
      
      const response = await authService.signup({
        name: data.name,
        email: data.email,
        password: data.password
      });
      
      const userData = await authService.getCurrentUser();
      login(response.access_token, userData);
      navigate('/dashboard');
      
    } catch (err: any) {
      // Error handling
      setError(err.response?.data?.detail || err.message || 'Signup failed');
    } finally {
      setLoading(false);
      useAuthStore.getState().setAuthenticating(false);
    }
  };

  const switchMode = (mode: LoginMode) => {
    setLoginMode(mode);
    clearMessages();
    // Reset form completely
    if (mode === 'mobile') {
      mobileForm.reset({ phone_number: '', username: '' });
      mobileForm.clearErrors();
      setPhoneNumber('');
    }
  };

  const goBackToLogin = () => {
    setAuthStep('login');
    setOtpValues(['', '', '', '', '', '']);
    otpForm.reset();
    setResendTimer(0);
    setCanResend(true);
    clearMessages();
  };

  // Loading component for PhoneInput
  const PhoneInputFallback = () => (
    <div className="bg-white/50 border border-gray-300 rounded-xl py-3 px-4 animate-pulse">
      <div className="h-6 bg-gray-200 rounded w-48"></div>
    </div>
  );

  return (
    <div className="min-h-screen bg-gradient-to-br from-emerald-100 via-green-50 to-teal-100 overflow-hidden relative">
      {/* Optimized animated background elements */}
      <motion.div 
        className="absolute top-20 left-10 w-32 h-32 bg-emerald-300/20 rounded-full blur-3xl"
        animate={{ 
          scale: [1, 1.1, 1],
          opacity: [0.3, 0.5, 0.3],
        }}
        transition={{ 
          duration: 6,
          repeat: Infinity,
          ease: "easeInOut"
        }}
      />
      <motion.div 
        className="absolute bottom-20 right-10 w-40 h-40 bg-green-300/20 rounded-full blur-3xl"
        animate={{ 
          scale: [1, 0.9, 1],
          opacity: [0.2, 0.4, 0.2],
        }}
        transition={{ 
          duration: 8,
          repeat: Infinity,
          ease: "easeInOut",
          delay: 2
        }}
      />

      <div className="flex min-h-screen">
        {/* Left side - Image and branding */}
        <motion.div 
          className="hidden lg:flex lg:w-1/2 relative overflow-hidden"
          initial={{ x: -100, opacity: 0 }}
          animate={{ x: 0, opacity: 1 }}
          transition={{ duration: 1 }}
        >
          <div className="absolute inset-0 bg-gradient-to-br from-emerald-600/90 via-green-600/80 to-teal-700/90"></div>
          {backgroundImageLoaded && (
            <div 
              className="absolute inset-0 bg-cover bg-center transition-opacity duration-500"
              style={{ backgroundImage: "url('/farmerbackground.png')" }}
            />
          )}
          
          {/* Simple Content Container */}
          <div className="relative z-10 flex flex-col justify-center items-center h-full px-16">
            <motion.div
              initial={{ y: 30, opacity: 0 }}
              animate={{ y: 0, opacity: 1 }}
              transition={{ duration: 0.8, delay: 0.3 }}
              className="text-center max-w-lg"
            >
              {/* Logo Section */}
              <div className="mb-12">
                <div className="text-8xl mb-6">🌱</div>
                <h1 className="text-5xl font-bold mb-4 bg-gradient-to-r from-emerald-400 via-green-300 to-teal-400 bg-clip-text text-transparent">
                  SmartKrishi
                </h1>
                <p className="text-2xl text-white/90 font-light">
                  Smart Farming, Made Simple
                </p>
              </div>

              {/* Simple Feature List */}
              <div className="space-y-6">
                <motion.div 
                  className="flex items-center justify-center space-x-4 text-white/85"
                  initial={{ opacity: 0, x: -20 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ duration: 0.6, delay: 0.8 }}
                >
                  <div className="text-3xl">🤖</div>
                  <span className="text-xl">AI-Powered Insights</span>
                </motion.div>

                <motion.div 
                  className="flex items-center justify-center space-x-4 text-white/85"
                  initial={{ opacity: 0, x: -20 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ duration: 0.6, delay: 1.0 }}
                >
                  <div className="text-3xl">🌦️</div>
                  <span className="text-xl">Weather Intelligence</span>
                </motion.div>

                <motion.div 
                  className="flex items-center justify-center space-x-4 text-white/85"
                  initial={{ opacity: 0, x: -20 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ duration: 0.6, delay: 1.2 }}
                >
                  <div className="text-3xl">🔍</div>
                  <span className="text-xl">Disease Detection</span>
                </motion.div>
              </div>
            </motion.div>
          </div>
        </motion.div>

        {/* Right side - Auth form */}
        <div className="w-full lg:w-1/2 flex items-center justify-center p-6">
          <motion.div
            initial={{ x: 100, opacity: 0 }}
            animate={{ x: 0, opacity: 1 }}
            transition={{ duration: 1 }}
            className="w-full max-w-md"
          >
            {/* Mobile logo for small screens */}
            <div className="lg:hidden text-center mb-8">
              <h1 className="text-3xl font-bold text-emerald-600 mb-2">🌱 SmartKrishi</h1>
              <p className="text-gray-600">Smart Farming, Made Simple</p>
            </div>

            {/* Glassmorphic card */}
            <motion.div
              className="backdrop-blur-lg bg-white/30 border border-white/20 rounded-3xl p-8 shadow-2xl"
              initial={{ scale: 0.9, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              transition={{ duration: 0.5, delay: 0.2 }}
            >
              <AnimatePresence mode="wait">
                {authStep === 'login' ? (
                  <motion.div
                    key="login"
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -20 }}
                    transition={{ duration: 0.4 }}
                  >
                    {/* Header */}
                    <div className="text-center mb-6">
                      <h2 className="text-2xl font-bold text-gray-800 mb-2">
                        {isSignupMode ? 'Create Account' : 'Welcome Back'}
                      </h2>
                      <p className="text-gray-600">
                        {isSignupMode ? 'Join the smart farming revolution' : 'Sign in to your account'}
                      </p>
                    </div>

                    {/* Login mode tabs */}
                    <div className="flex mb-6 bg-white/50 rounded-2xl p-1">
                      <button
                        type="button"
                        onClick={() => switchMode('mobile')}
                        className={`flex-1 flex items-center justify-center py-3 px-4 rounded-xl transition-all duration-300 ${
                          loginMode === 'mobile'
                            ? 'bg-gradient-to-r from-emerald-500 to-green-600 text-white shadow-lg'
                            : 'text-gray-600 hover:text-gray-800'
                        }`}
                      >
                        <Smartphone className="w-4 h-4 mr-2" />
                        Mobile
                      </button>
                      <button
                        type="button"
                        onClick={() => switchMode('email')}
                        className={`flex-1 flex items-center justify-center py-3 px-4 rounded-xl transition-all duration-300 ${
                          loginMode === 'email'
                            ? 'bg-gradient-to-r from-emerald-500 to-green-600 text-white shadow-lg'
                            : 'text-gray-600 hover:text-gray-800'
                        }`}
                      >
                        <Mail className="w-4 h-4 mr-2" />
                        Email
                      </button>
                    </div>

                    {/* Error/Success messages */}
                    <AnimatePresence>
                      {error && (
                        <motion.div
                          initial={{ opacity: 0, y: -10 }}
                          animate={{ opacity: 1, y: 0 }}
                          exit={{ opacity: 0, y: -10 }}
                          className="mb-4 p-3 bg-red-100/80 border border-red-200 rounded-xl"
                        >
                          <p className="text-red-600 text-sm">{error}</p>
                        </motion.div>
                      )}
                      {successMessage && (
                        <motion.div
                          initial={{ opacity: 0, y: -10 }}
                          animate={{ opacity: 1, y: 0 }}
                          exit={{ opacity: 0, y: -10 }}
                          className="mb-4 p-3 bg-green-100/80 border border-green-200 rounded-xl"
                        >
                          <p className="text-green-600 text-sm">{successMessage}</p>
                        </motion.div>
                      )}
                    </AnimatePresence>

                    {/* Forms */}
                    <AnimatePresence mode="wait">
                      {loginMode === 'mobile' ? (
                        <motion.form
                          key="mobile-form"
                          initial={{ opacity: 0, x: -20 }}
                          animate={{ opacity: 1, x: 0 }}
                          exit={{ opacity: 0, x: -20 }}
                          transition={{ duration: 0.3 }}
                          onSubmit={mobileForm.handleSubmit(handleMobileSubmit)}
                          className="space-y-4"
                        >
                          {isSignupMode && (
                            <div className="space-y-2">
                              <Label className="text-gray-700 font-medium">Full Name</Label>
                              <Input
                                type="text"
                                placeholder="Enter your full name"
                                {...mobileForm.register('username')}
                                className="bg-white/50 border border-gray-300 rounded-xl py-3 focus:ring-2 focus:ring-emerald-500 focus:border-emerald-500 transition-all duration-300"
                              />
                              {mobileForm.formState.errors.username && (
                                <p className="text-red-500 text-xs">{mobileForm.formState.errors.username.message}</p>
                              )}
                            </div>
                          )}

                          <div className="space-y-2">
                            <Label className="text-gray-700 font-medium">Mobile Number</Label>
                            <Suspense fallback={<PhoneInputFallback />}>
                              <PhoneInput
                                placeholder="9876543210"
                                value={phoneNumber}
                                onChange={(value: string | undefined) => {
                                  setPhoneNumber(value || '');
                                  mobileForm.setValue('phone_number', value || '');
                                }}
                                defaultCountry="IN"
                                className="bg-white/50 border border-gray-300 rounded-xl focus-within:ring-2 focus-within:ring-emerald-500 transition-all duration-300"
                              />
                            </Suspense>
                            {mobileForm.formState.errors.phone_number && (
                              <p className="text-red-500 text-xs">{mobileForm.formState.errors.phone_number.message}</p>
                            )}
                          </div>
                          <Button
                            type="submit"
                            disabled={isLoading}
                            className="w-full bg-gradient-to-r from-emerald-500 to-green-600 hover:from-emerald-600 hover:to-green-700 text-white py-3 rounded-xl font-medium shadow-lg hover:shadow-xl transition-all duration-300 transform hover:scale-[1.02]"
                          >
                            {isLoading ? 'Sending OTP...' : 'Send OTP'}
                          </Button>
                        </motion.form>
                      ) : (
                        <motion.form
                          key="email-form"
                          initial={{ opacity: 0, x: 20 }}
                          animate={{ opacity: 1, x: 0 }}
                          exit={{ opacity: 0, x: 20 }}
                          transition={{ duration: 0.3 }}
                          onSubmit={isSignupMode ? emailSignupForm.handleSubmit(handleEmailSignupSubmit) : emailForm.handleSubmit(handleEmailSubmit)}
                          className="space-y-4"
                        >
                          {isSignupMode && (
                            <div className="space-y-2">
                              <Label className="text-gray-700 font-medium">Full Name</Label>
                              <Input
                                type="text"
                                placeholder="Enter your full name"
                                {...emailSignupForm.register('name')}
                                className="bg-white/50 border border-gray-300 rounded-xl py-3 focus:ring-2 focus:ring-emerald-500 focus:border-emerald-500 transition-all duration-300"
                              />
                              {emailSignupForm.formState.errors.name && (
                                <p className="text-red-500 text-xs">{emailSignupForm.formState.errors.name.message}</p>
                              )}
                            </div>
                          )}

                          <div className="space-y-2">
                            <Label className="text-gray-700 font-medium">Email</Label>
                            <Input
                              type="email"
                              placeholder="farmer@example.com"
                              {...(isSignupMode ? emailSignupForm.register('email') : emailForm.register('email'))}
                              className="bg-white/50 border border-gray-300 rounded-xl py-3 focus:ring-2 focus:ring-emerald-500 focus:border-emerald-500 transition-all duration-300"
                            />
                            {(isSignupMode ? emailSignupForm.formState.errors.email : emailForm.formState.errors.email) && (
                              <p className="text-red-500 text-xs">
                                {(isSignupMode ? emailSignupForm.formState.errors.email?.message : emailForm.formState.errors.email?.message)}
                              </p>
                            )}
                          </div>

                          <div className="space-y-2">
                            <Label className="text-gray-700 font-medium">Password</Label>
                            <div className="relative">
                              <Input
                                type={showPassword ? 'text' : 'password'}
                                placeholder="Enter your password"
                                {...(isSignupMode ? emailSignupForm.register('password') : emailForm.register('password'))}
                                className="bg-white/50 border border-gray-300 rounded-xl py-3 pr-10 focus:ring-2 focus:ring-emerald-500 focus:border-emerald-500 transition-all duration-300"
                              />
                              <button
                                type="button"
                                onClick={() => setShowPassword(!showPassword)}
                                className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-700"
                              >
                                {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                              </button>
                            </div>
                            {(isSignupMode ? emailSignupForm.formState.errors.password : emailForm.formState.errors.password) && (
                              <p className="text-red-500 text-xs">
                                {(isSignupMode ? emailSignupForm.formState.errors.password?.message : emailForm.formState.errors.password?.message)}
                              </p>
                            )}
                          </div>

                          {isSignupMode && (
                            <div className="space-y-2">
                              <Label className="text-gray-700 font-medium">Confirm Password</Label>
                              <div className="relative">
                                <Input
                                  type={showPassword ? 'text' : 'password'}
                                  placeholder="Confirm your password"
                                  {...emailSignupForm.register('confirmPassword')}
                                  className="bg-white/50 border border-gray-300 rounded-xl py-3 pr-10 focus:ring-2 focus:ring-emerald-500 focus:border-emerald-500 transition-all duration-300"
                                />
                                <button
                                  type="button"
                                  onClick={() => setShowPassword(!showPassword)}
                                  className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-700"
                                >
                                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                                </button>
                              </div>
                              {emailSignupForm.formState.errors.confirmPassword && (
                                <p className="text-red-500 text-xs">{emailSignupForm.formState.errors.confirmPassword.message}</p>
                              )}
                            </div>
                          )}

                          <Button
                            type="submit"
                            disabled={isLoading}
                            className="w-full bg-gradient-to-r from-emerald-500 to-green-600 hover:from-emerald-600 hover:to-green-700 text-white py-3 rounded-xl font-medium shadow-lg hover:shadow-xl transition-all duration-300 transform hover:scale-[1.02]"
                          >
                            {isLoading ? (isSignupMode ? 'Creating Account...' : 'Signing in...') : (isSignupMode ? 'Create Account' : 'Sign In')}
                          </Button>
                        </motion.form>
                      )}
                    </AnimatePresence>

                    {/* Auth mode toggle */}
                    <div className="mt-6 text-center">
                      <p className="text-gray-600 text-sm">
                        {isSignupMode ? 'Already have an account?' : "Don't have an account?"}{' '}
                        <button
                          type="button"
                          onClick={() => setIsSignupMode(!isSignupMode)}
                          className="text-emerald-600 hover:text-emerald-700 font-medium hover:underline"
                        >
                          {isSignupMode ? 'Sign in' : 'Sign up'}
                        </button>
                      </p>
                    </div>

                    {/* Navigation links */}
                    <div className="mt-4 text-center">
                      <button
                        type="button"
                        onClick={() => navigate('/')}
                        className="text-gray-500 hover:text-gray-700 text-sm hover:underline"
                      >
                        ← Back to Home
                      </button>
                    </div>
                  </motion.div>
                ) : (
                  <motion.div
                    key="otp"
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -20 }}
                    transition={{ duration: 0.4 }}
                  >
                    {/* OTP Header */}
                    <div className="text-center mb-6">
                      <h2 className="text-2xl font-bold text-gray-800 mb-2">Verify OTP</h2>
                      <p className="text-gray-600">Enter the 6-digit code sent to</p>
                      <p className="font-medium text-emerald-600">{phoneNumber}</p>
                    </div>

                    {/* Back button */}
                    <div className="flex items-center mb-6">
                      <Button
                        type="button"
                        variant="ghost"
                        onClick={goBackToLogin}
                        className="text-emerald-600 hover:text-emerald-700 hover:bg-emerald-50 p-2 rounded-xl"
                      >
                        <ArrowLeft className="w-4 h-4 mr-2" />
                        Back
                      </Button>
                    </div>

                    {/* Error/Success messages */}
                    <AnimatePresence>
                      {error && (
                        <motion.div
                          initial={{ opacity: 0, y: -10 }}
                          animate={{ opacity: 1, y: 0 }}
                          exit={{ opacity: 0, y: -10 }}
                          className="mb-4 p-3 bg-red-100/80 border border-red-200 rounded-xl"
                        >
                          <p className="text-red-600 text-sm">{error}</p>
                        </motion.div>
                      )}
                      {successMessage && (
                        <motion.div
                          initial={{ opacity: 0, y: -10 }}
                          animate={{ opacity: 1, y: 0 }}
                          exit={{ opacity: 0, y: -10 }}
                          className="mb-4 p-3 bg-green-100/80 border border-green-200 rounded-xl"
                        >
                          <p className="text-green-600 text-sm">{successMessage}</p>
                        </motion.div>
                      )}
                    </AnimatePresence>

                    {/* OTP Inputs */}
                    <div className="mb-6">
                      <Label className="block text-center text-gray-700 font-medium mb-4">
                        Enter 6-digit OTP
                      </Label>
                      <div className="flex justify-center space-x-2 sm:space-x-3">
                        {otpValues.map((digit, index) => (
                          <motion.input
                            key={index}
                            ref={(el) => { otpRefs.current[index] = el; }}
                            type="text"
                            maxLength={1}
                            value={digit}
                            onChange={(e) => handleOtpChange(index, e.target.value)}
                            onKeyDown={(e) => handleOtpKeyDown(index, e)}
                            className="w-10 h-10 sm:w-12 sm:h-12 text-center text-lg sm:text-xl font-bold bg-white/70 border-2 border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500 focus:border-emerald-500 outline-none transition-all duration-300 shadow-sm"
                            inputMode="numeric"
                            whileFocus={{ scale: 1.05 }}
                            transition={{ duration: 0.2 }}
                          />
                        ))}
                      </div>
                    </div>

                    {/* Verify Button */}
                    <Button
                      type="button"
                      onClick={() => {
                        const otpString = otpValues.join('');
                        if (otpString.length === 6) {
                          otpForm.setValue('otp', otpString);
                          otpForm.handleSubmit(handleOTPSubmit)();
                        } else {
                          setError('Please enter all 6 digits');
                        }
                      }}
                      disabled={isLoading || otpValues.join('').length !== 6}
                      className="w-full bg-gradient-to-r from-emerald-500 to-green-600 hover:from-emerald-600 hover:to-green-700 text-white py-3 rounded-xl font-medium shadow-lg hover:shadow-xl transition-all duration-300 transform hover:scale-[1.02] disabled:opacity-50 disabled:transform-none"
                    >
                      {isLoading ? 'Verifying...' : 'Verify OTP'}
                    </Button>

                    {/* Resend OTP */}
                    <div className="mt-4 text-center">
                      {resendTimer > 0 ? (
                        <p className="text-gray-600 text-sm">
                          Resend OTP in{' '}
                          <span className="font-medium text-emerald-600">{resendTimer}s</span>
                        </p>
                      ) : (
                        <button
                          type="button"
                          onClick={handleResendOTP}
                          disabled={!canResend || isLoading}
                          className="text-emerald-600 hover:text-emerald-700 font-medium hover:underline disabled:text-gray-400 disabled:hover:no-underline text-sm"
                        >
                          Resend OTP
                        </button>
                      )}
                    </div>
                  </motion.div>
                )}
              </AnimatePresence>
            </motion.div>
          </motion.div>
        </div>
      </div>

      {/* Hidden reCAPTCHA container */}
      <div id="recaptcha-container" className="h-0 overflow-hidden"></div>
    </div>
  );
};

export default UnifiedAuthPage;
