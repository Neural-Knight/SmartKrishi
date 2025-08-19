import React, { useState, useLayoutEffect, useRef, useEffect } from "react"
import { Card, CardContent } from "@/components/ui/card"
import ChatInput from "@/components/ui/chat-input"
import { EnhancedMessage } from "../components/ui/enhanced-message";
import ResponsiveNavbar from "@/components/ui/responsive-navbar"
import HistoryDrawer from "@/components/ui/history-drawer"
import SettingsModal from "@/components/ui/settings-modal"
import { FallbackSettings } from "@/components/ui/fallback-settings"
import { 
  Leaf,
  Pickaxe, 
  Sun, 
  Bug, 
  DollarSign,
  Sparkles,
  Activity,
  Upload
} from "lucide-react"
import { useNavigate } from "react-router-dom"
import { useAuthStore } from "@/store/authStore"
import { authService } from "@/services/auth"
import { chatService } from '../services/chatService';
import { useStreamingChat } from '../hooks/useStreamingChat';
import { useReasoning } from '../hooks/useReasoning';
import type { ChatMessage } from '../services/chatService';

interface SuggestionCard {
  id: string
  icon: React.ReactNode
  text: string
  prompt: string
}

const getIconForSuggestion = (id: string) => {
  switch (id) {
    case 'crop-care':
    case 'crop-diseases':
      return <Activity className="w-6 h-6 text-red-600" />;
    case 'soil-health':
      return <Pickaxe className="w-6 h-6 text-gray-600" />;
    case 'weather-advice':
      return <Sun className="w-6 h-6 text-yellow-500" />;
    case 'pest-management':
    case 'pest-control':
      return <Bug className="w-6 h-6 text-red-500" />;
    case 'market-prices':
    case 'market-insights':
      return <DollarSign className="w-6 h-6 text-green-600" />;
    default:
      return <Sparkles className="w-6 h-6 text-blue-500" />;
  }
};

export default function DashboardPage() {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [message, setMessage] = useState('');
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);
  const [showSuggestions, setShowSuggestions] = useState(true);
  const [isHistoryOpen, setIsHistoryOpen] = useState(false);
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [currentChatId, setCurrentChatId] = useState<string | null>(null);
  const [suggestionCards, setSuggestionCards] = useState<SuggestionCard[]>([]);
  const [readingMessageId, setReadingMessageId] = useState<string | null>(null);
  const [showFallbackSettings, setShowFallbackSettings] = useState(false);
  const [isGlobalDragOver, setIsGlobalDragOver] = useState(false);
  const navigate = useNavigate()
  const { user, logout } = useAuthStore()
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Initialize reasoning hook
  const reasoning = useReasoning();

  // New streaming hook
  const streaming = useStreamingChat({
    onNewMessage: (msg) => {
      setMessages(prev => {
        // Check if message already exists to prevent duplicates
        const existingIndex = prev.findIndex(m => m.id === msg.id);
        if (existingIndex >= 0) {
          return prev; // Message already exists, don't add duplicate
        }
        return [...prev, msg];
      });
    },
    onMessageUpdate: (msg) => {
      setMessages(prev => {
        const existingIndex = prev.findIndex(m => m.id === msg.id);
        if (existingIndex >= 0) {
          const updated = [...prev];
          updated[existingIndex] = msg;
          return updated;
        }
        // If message doesn't exist, add it (could be from streaming)
        return [...prev, msg];
      });
      
      // Update reasoning hook with new reasoning steps
      if (msg.reasoning_steps && msg.reasoning_steps.length > 0) {
        reasoning.updateReasoningSteps(msg.reasoning_steps);
      }
    },
    onError: (error) => {
      // Error handling
      setMessages(prev => [...prev, {
        id: `error-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
        role: 'assistant',
        content: `Sorry, I encountered an error: ${error}`,
        timestamp: new Date()
      }]);
    },
    onChatCreated: (chatId) => {
      // Development log removed
      setCurrentChatId(chatId);
    }
  });

  // Load suggestions from API
  useEffect(() => {
    const loadSuggestions = async () => {
      try {
        const suggestions = await chatService.getSuggestions();
        setSuggestionCards(suggestions.map(s => ({
          id: s.id,
          icon: getIconForSuggestion(s.id),
          text: s.text,
          prompt: s.prompt
        })));
      } catch (error) {
        // Error handling
        // Fallback suggestions
        setSuggestionCards([
          {
            id: "crop-care",
            icon: <Leaf className="w-6 h-6 text-green-600" />,
            text: "Crop Care Tips",
            prompt: "What are the best practices for caring for my crops during this season?"
          },
          {
            id: "weather-advice",
            icon: <Sun className="w-6 h-6 text-yellow-500" />,
            text: "Weather Insights",
            prompt: "How will the current weather conditions affect my farming activities?"
          },
          {
            id: "pest-management",
            icon: <Bug className="w-6 h-6 text-red-500" />,
            text: "Pest Control",
            prompt: "Help me identify and manage pests affecting my crops."
          },
          {
            id: "market-prices",
            icon: <DollarSign className="w-6 h-6 text-green-600" />,
            text: "Market Prices",
            prompt: "What are the current market prices for my crops and when should I sell?"
          }
        ]);
      }
    };

    loadSuggestions();
  }, []);

  // Global drag and drop event handlers
  useEffect(() => {
    const handleGlobalDragOver = (e: DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      
      // Check if the dragged items contain files
      if (e.dataTransfer?.types.includes('Files')) {
        setIsGlobalDragOver(true);
      }
    };

    const handleGlobalDragLeave = (e: DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      
      // Only hide overlay if leaving the window entirely
      if (e.clientX <= 0 || e.clientY <= 0 || 
          e.clientX >= window.innerWidth || e.clientY >= window.innerHeight) {
        setIsGlobalDragOver(false);
      }
    };

    const handleGlobalDrop = (e: DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setIsGlobalDragOver(false);
      
      if (streaming.isStreaming) return;
      
      const files = Array.from(e.dataTransfer?.files || []);
      if (files.length > 0) {
        handleFileUpload(files);
      }
    };

    document.addEventListener('dragover', handleGlobalDragOver);
    document.addEventListener('dragleave', handleGlobalDragLeave);
    document.addEventListener('drop', handleGlobalDrop);

    return () => {
      document.removeEventListener('dragover', handleGlobalDragOver);
      document.removeEventListener('dragleave', handleGlobalDragLeave);
      document.removeEventListener('drop', handleGlobalDrop);
    };
  }, [streaming.isStreaming]);

  // Auto-scroll to bottom after every new message
  useLayoutEffect(() => {
    if (messagesEndRef.current) {
      messagesEndRef.current.scrollIntoView({
        behavior: 'smooth',
        block: 'end'
      });
    }
  }, [messages]);

  const handleNewChat = () => {
    // Stop any ongoing streaming
    streaming.stopStreaming();
    setMessages([]);
    setMessage('');
    setSelectedFiles([]);
    setCurrentChatId(null);
    setShowSuggestions(true);
    setReadingMessageId(null);
    
    // Clear reasoning state for new chat
    reasoning.clearReasoning();
  };

  const handleChatSelect = async (chatId: string) => {
    try {
      // Stop any ongoing streaming
      streaming.stopStreaming();
      
      // Clear reasoning state when switching chats
      reasoning.clearReasoning();
      
      const chat = await chatService.getChat(chatId);
      // Convert backend message format to frontend format with reasoning steps
      const chatMessages: ChatMessage[] = chat.messages
        .sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime()) // Ensure proper ordering
        .map(msg => ({
            id: msg.id,
            role: msg.role as 'user' | 'assistant',
            content: msg.content,
            timestamp: new Date(msg.created_at),
          // Include reasoning steps if they exist and filter out non-reasoning content
            reasoning_steps: msg.reasoning_steps
              ?.filter((step: any) => 
              // Only include actual reasoning steps, not response chunks or final responses
                step.step_type && 
                step.step_type !== 'response_chunk' && 
                step.step_type !== 'response' && 
                step.step_type !== 'end' &&
              step.step_type !== 'log' &&  // Filter out log events
                step.content && 
                step.content.trim() !== ''
              )
              ?.map((step: any) => ({
                id: step.id,
                step_type: step.step_type,
                step_order: step.step_order,
                stage: step.stage,
                content: step.content,
                message: step.content,
                tool_name: step.tool_name,
                tool: step.tool_name,
                tool_args: step.tool_args,
                tool_result: step.tool_result,
                step_metadata: step.step_metadata,
                created_at: step.created_at
              })) || []
        }));
      
      setMessages(chatMessages);
      setCurrentChatId(chatId);
      setShowSuggestions(chatMessages.length === 0);
      
      // Load reasoning for this chat to sync the reasoning hook
      try {
        await reasoning.loadChatReasoning(chatId);
      } catch (error) {
        // Warning handled
      }
    } catch (error) {
      // Error handling
    }
  };

  const handleLogout = async () => {
    try {
      await authService.logout()
      logout()
      navigate('/login')
    } catch (error) {
      // Error handling
    }
  };

  const handleSuggestionClick = (prompt: string) => {
    setMessage(prompt)
  }

  const handleFileUpload = (files: File[]) => {
    // Validate files before adding
    const validFiles = files.filter(file => {
      if (!chatService.isFileTypeAllowed(file.name)) {
        // File validation error - could show toast notification instead
        return false;
      }
      if (!chatService.validateFileSize(file)) {
        // File validation error - could show toast notification instead
        return false;
      }
      return true;
    });

    if (validFiles.length > 0) {
      setSelectedFiles(prevFiles => [...prevFiles, ...validFiles]);
    }
  }

  const handleFileRemove = (index: number) => {
    setSelectedFiles(prevFiles => prevFiles.filter((_, i) => i !== index));
  }

  const handleCopyMessage = (_content: string) => {
    // Already handled by the MessageActions component
    // Development log removed
  }

  const handleEditMessage = (messageId: string) => {
    const messageToEdit = messages.find(m => m.id === messageId);
    if (messageToEdit && messageToEdit.role === 'user') {
      setMessage(messageToEdit.content);
      // Remove the message and all subsequent messages for re-generation
      const messageIndex = messages.findIndex(m => m.id === messageId);
      setMessages(prev => prev.slice(0, messageIndex));
      setSelectedFiles([]);
    }
  }

  const handleLikeMessage = (_messageId: string) => {
    // Development log removed
    // TODO: Implement feedback to backend
  }

  const handleDislikeMessage = (_messageId: string) => {
    // Development log removed
    // TODO: Implement feedback to backend
  }

  const handleReadAloud = (content: string, messageId: string) => {
    if ('speechSynthesis' in window) {
      // Stop any current reading
      window.speechSynthesis.cancel();
      setReadingMessageId(messageId);
      
      const utterance = new SpeechSynthesisUtterance(content);
      utterance.rate = 0.8;
      utterance.pitch = 1.0;
      utterance.volume = 0.8;
      
      utterance.onend = () => {
        setReadingMessageId(null);
      };
      
      utterance.onerror = () => {
        setReadingMessageId(null);
      };
      
      window.speechSynthesis.speak(utterance);
    } else {
      // Warning handled
    }
  }

  const handleStopReading = () => {
    if ('speechSynthesis' in window) {
      window.speechSynthesis.cancel();
      setReadingMessageId(null);
    }
  }

  const handleSendMessage = async () => {
    if ((!message.trim() && selectedFiles.length === 0) || streaming.isStreaming) return;

    const messageText = message.trim();
    const filesToUpload = [...selectedFiles]; // Copy files before clearing
    // Development log removed
    
    // Clear input immediately for better UX
    setMessage('');
    setSelectedFiles([]);
    setShowSuggestions(false);

    try {
      if (filesToUpload.length > 0) {
        // Validate files before uploading
        const invalidFiles = filesToUpload.filter(file => !chatService.isFileTypeAllowed(file.name));
        if (invalidFiles.length > 0) {
          // File validation error - could show toast notification instead
          setSelectedFiles(filesToUpload); // Restore files if validation fails
          return;
        }

        const oversizedFiles = filesToUpload.filter(file => !chatService.validateFileSize(file));
        if (oversizedFiles.length > 0) {
          // File validation error - could show toast notification instead
          setSelectedFiles(filesToUpload); // Restore files if validation fails
          return;
        }

        // Create and show user message with files IMMEDIATELY
        const userMessageWithFiles: ChatMessage = {
          id: `user-${Date.now()}`,
          role: 'user',
          content: messageText || `Analyze these files: ${filesToUpload.map(f => f.name).join(', ')}`,
          timestamp: new Date(),
          files: filesToUpload.map(file => ({
            id: `temp-${Date.now()}-${Math.random()}`, // Temporary ID
            original_filename: file.name,
            file_type: file.name.split('.').pop()?.toLowerCase() || 'unknown',
            file_size: file.size,
            processing_status: 'uploading',
            agent_file_id: undefined,
            created_at: new Date().toISOString(),
            user_id: 0, // Will be filled by backend
            chat_id: currentChatId || '',
            message_id: undefined,
            updated_at: new Date().toISOString(),
            is_deleted: false
          }))
        };
        
        // Add user message to UI immediately
        setMessages(prev => [...prev, userMessageWithFiles]);

        // Handle multiple file upload with streaming
        const promptText = messageText || `Analyze these files: ${filesToUpload.map(f => f.name).join(', ')}`;
        
        await streaming.uploadFileWithStreaming(filesToUpload, promptText, currentChatId || undefined, true);
      } else {
        // Handle text message with streaming
        // Note: sendMessageWithStreaming will add the user message automatically
        await streaming.sendMessageWithStreaming(
          messageText, 
          currentChatId || undefined,
          {
            include_logs: true // Enable reasoning by default
          }
        );
      }
    } catch (error) {
      // Error handling
    }
  }

return (
    <div className="h-screen flex flex-col lg:flex-row bg-gradient-to-br from-gray-50 via-green-50 to-white">
      {/* Global Drag and Drop Overlay */}
      {isGlobalDragOver && (
        <div className="fixed inset-0 z-50 bg-emerald-100/90 backdrop-blur-sm flex flex-col items-center justify-center border-4 border-dashed border-emerald-500">
          <div className="text-center">
            <Upload className="w-16 h-16 text-emerald-600 mb-4 animate-bounce mx-auto" />
            <h3 className="text-2xl font-bold text-emerald-700 mb-2">
              Drop files anywhere to upload
            </h3>
            <p className="text-emerald-600 text-lg">
              Supports images, PDF, DOCX, XLSX, CSV files
            </p>
            <div className="mt-4 px-6 py-2 bg-white/80 rounded-full border border-emerald-200">
              <p className="text-emerald-700 text-sm">
                Files will be added to your chat automatically
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Responsive Navigation */}
      <ResponsiveNavbar 
        onNewChat={handleNewChat}
        onHistoryClick={() => setIsHistoryOpen(true)}
        onSettingsClick={() => setIsSettingsOpen(true)}
      />

      {/* Main Content */}
      <div className="flex-1 flex flex-col bg-gradient-to-br from-gray-50 via-green-50 to-white">
        {showSuggestions ? (
          <div className="flex-1 overflow-y-auto p-4 lg:p-8">
            <div className="flex flex-col items-center justify-center min-h-full animate-fadeIn">
              <div className="text-center max-w-2xl mb-8">
                <div className="mb-4">
                  <span className="text-4xl lg:text-6xl drop-shadow-sm">🌱</span>
                </div>
                <h1 className="text-2xl lg:text-3xl font-bold text-gray-800 mb-2">
                  Welcome to <span className="bg-gradient-to-r from-green-600 to-emerald-500 bg-clip-text text-transparent">SmartKrishi</span>
                </h1>
                <p className="text-base lg:text-lg text-gray-600">
                  Your AI-powered farming assistant with advanced reasoning. Ask me anything about agriculture, crops, or farming techniques.
                </p>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6 w-full max-w-4xl px-4">
                {suggestionCards.map((card) => (
                  <Card 
                    key={card.id}
                    className="cursor-pointer backdrop-blur-lg bg-white/70 border border-green-100 hover:border-green-300 hover:shadow-xl hover:scale-[1.02] transition-all duration-300 rounded-xl"
                    onClick={() => handleSuggestionClick(card.prompt)}
                  >
                    <CardContent className="p-4 lg:p-6 flex items-center space-x-4">
                      <div className="flex-shrink-0">{card.icon}</div>
                      <div className="flex-1">
                        <h3 className="font-semibold text-gray-800 mb-1">{card.text}</h3>
                        <p className="text-sm text-gray-600 line-clamp-2">{card.prompt}</p>
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>
            </div>
          </div>
        ) : (
          <div className="flex-1 overflow-y-auto bg-gradient-to-br from-gray-50 via-green-50 to-white">
            <div className="w-full max-w-4xl mx-auto px-4 lg:px-6">
              {messages.map((msg) => (
                <EnhancedMessage
                  key={msg.id}
                  message={msg}
                  onCopy={handleCopyMessage}
                  onEdit={handleEditMessage}
                  onLike={handleLikeMessage}
                  onDislike={handleDislikeMessage}
                  onReadAloud={(content: string, messageId: string) => handleReadAloud(content, messageId)}
                  onStopReading={handleStopReading}
                  isReading={readingMessageId === msg.id}
                />
              ))}
              <div ref={messagesEndRef} style={{ height: '1px' }} />
            </div>
          </div>
        )}

        {/* Fixed Chat Input at Bottom */}
        <div className="flex-shrink-0 bg-transparent border-none px-4 lg:px-0">
          <ChatInput
            value={message}
            onChange={setMessage}
            onSend={handleSendMessage}
            onFileUpload={handleFileUpload}
            selectedFiles={selectedFiles}
            onFileRemove={handleFileRemove}
            disabled={streaming.isStreaming}
          />
        </div>
      </div>

      {/* Drawers & Modals */}
      <HistoryDrawer
        isOpen={isHistoryOpen}
        onClose={() => setIsHistoryOpen(false)}
        onChatSelect={handleChatSelect}
        onNewChat={handleNewChat}
        currentChatId={currentChatId || undefined}
      />
      <SettingsModal
        isOpen={isSettingsOpen}
        onClose={() => setIsSettingsOpen(false)}
        userName={user?.name || 'Guest'}
        onLogout={handleLogout}
      />
      
      {/* SMS Fallback Settings Modal */}
      {showFallbackSettings && (
        <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
          <div className="bg-white rounded-xl max-w-2xl w-full max-h-[90vh] overflow-y-auto">
            <div className="p-4 border-b flex items-center justify-between">
              <h3 className="text-lg font-semibold">SMS Fallback Settings</h3>
              <button
                onClick={() => setShowFallbackSettings(false)}
                className="p-2 hover:bg-gray-100 rounded-lg transition-colors text-gray-500 hover:text-gray-700"
              >
                ×
              </button>
            </div>
            <div className="p-6">
              <FallbackSettings />
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
