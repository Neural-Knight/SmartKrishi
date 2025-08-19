import React, { useState, useEffect } from 'react';
import { X, AlertCircle, FileX, Scale } from 'lucide-react';

interface ErrorToastProps {
  message: string;
  type: 'file-type' | 'file-size' | 'general';
  onClose: () => void;
  autoClose?: boolean;
  duration?: number;
}

export const ErrorToast: React.FC<ErrorToastProps> = ({
  message,
  type,
  onClose,
  autoClose = true,
  duration = 5000
}) => {
  const [isVisible, setIsVisible] = useState(true);
  const [isExiting, setIsExiting] = useState(false);

  useEffect(() => {
    if (autoClose) {
      const timer = setTimeout(() => {
        handleClose();
      }, duration);
      return () => clearTimeout(timer);
    }
  }, [autoClose, duration]);

  const handleClose = () => {
    setIsExiting(true);
    setTimeout(() => {
      setIsVisible(false);
      onClose();
    }, 300); // Match animation duration
  };

  const getIcon = () => {
    switch (type) {
      case 'file-type':
        return <FileX className="w-5 h-5 text-red-500 flex-shrink-0" />;
      case 'file-size':
        return <Scale className="w-5 h-5 text-red-500 flex-shrink-0" />;
      default:
        return <AlertCircle className="w-5 h-5 text-red-500 flex-shrink-0" />;
    }
  };

  const getTitle = () => {
    switch (type) {
      case 'file-type':
        return 'Invalid File Type';
      case 'file-size':
        return 'File Too Large';
      default:
        return 'Upload Error';
    }
  };

  if (!isVisible) return null;

  return (
    <div 
      className={`
        fixed top-4 right-4 z-50 max-w-sm w-full mx-4 sm:mx-0
        transform transition-all duration-300 ease-in-out
        ${isExiting 
          ? 'translate-x-full opacity-0' 
          : 'translate-x-0 opacity-100'
        }
      `}
    >
      <div className="bg-red-50 border border-red-200 rounded-lg shadow-lg p-4 relative">
        {/* Close button */}
        <button
          onClick={handleClose}
          className="absolute top-2 right-2 p-1 rounded-full hover:bg-red-100 transition-colors"
          aria-label="Close notification"
        >
          <X className="w-4 h-4 text-red-400 hover:text-red-600" />
        </button>

        {/* Content */}
        <div className="flex items-start space-x-3 pr-6">
          {getIcon()}
          <div className="flex-1 min-w-0">
            <h4 className="text-sm font-semibold text-red-800 mb-1">
              {getTitle()}
            </h4>
            <p className="text-sm text-red-700 leading-relaxed">
              {message}
            </p>
          </div>
        </div>

        {/* Progress bar for auto-close */}
        {autoClose && (
          <div className="absolute bottom-0 left-0 right-0 h-1 bg-red-100 rounded-b-lg overflow-hidden">
            <div 
              className="h-full bg-red-400 rounded-b-lg animate-pulse"
              style={{
                width: '0%',
                animation: `shrinkWidth ${duration}ms linear forwards`
              }}
            />
          </div>
        )}

        {/* Inline keyframe animation */}
        <style dangerouslySetInnerHTML={{
          __html: `
            @keyframes shrinkWidth {
              from { width: 100%; }
              to { width: 0%; }
            }
          `
        }} />
      </div>
    </div>
  );
};

// Toast container to manage multiple toasts
interface ToastContainerProps {
  toasts: Array<{
    id: string;
    message: string;
    type: 'file-type' | 'file-size' | 'general';
  }>;
  onRemoveToast: (id: string) => void;
}

export const ToastContainer: React.FC<ToastContainerProps> = ({
  toasts,
  onRemoveToast
}) => {
  return (
    <div className="fixed top-4 right-4 z-50 space-y-2">
      {toasts.map((toast, index) => (
        <div
          key={toast.id}
          style={{ 
            transform: `translateY(${index * 4}px)`,
            zIndex: 50 - index
          }}
        >
          <ErrorToast
            message={toast.message}
            type={toast.type}
            onClose={() => onRemoveToast(toast.id)}
          />
        </div>
      ))}
    </div>
  );
};
