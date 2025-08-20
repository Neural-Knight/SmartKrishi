import React, { useState } from 'react';
import { 
  ChevronDown, 
  ChevronRight, 
  Copy,
  Check,
  ThumbsUp,
  ThumbsDown,
  Volume2,
  VolumeX,
  Edit,
  Brain,
  Code,
  Globe,
  Search,
  MessageSquare,
  FileText,
  AlertCircle,
  CheckCircle,
  Wrench,
  FileImage,
  File as FileIcon,
  Download
} from 'lucide-react';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from './collapsible';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { ChatMessage, ReasoningStep } from '../../services/chatService';

interface EnhancedMessageProps {
  message: ChatMessage;
  onCopy?: (content: string) => void;
  onEdit?: (messageId: string) => void;
  onLike?: (messageId: string) => void;
  onDislike?: (messageId: string) => void;
  onReadAloud?: (content: string, messageId: string) => void;
  onStopReading?: () => void;
  isReading?: boolean;
}

// Event type to icon mapping
const getEventIcon = (eventType: string) => {
  const iconMap: Record<string, React.ReactNode> = {
    'log': <FileText className="w-3 h-3 text-blue-500" />,
    'plan': <Brain className="w-3 h-3 text-purple-500" />,
    'thinking': <Brain className="w-3 h-3 text-yellow-500" />,
    'tool_call': <Wrench className="w-3 h-3 text-green-500" />,
    'code_execution': <Code className="w-3 h-3 text-orange-500" />,
    'google_search_call': <Search className="w-3 h-3 text-red-500" />,
    'google_search_response': <Search className="w-3 h-3 text-red-400" />,
    'grounding_web_search_queries': <Globe className="w-3 h-3 text-blue-400" />,
    'grounding_chunks': <FileText className="w-3 h-3 text-teal-500" />,
    'grounding_supports': <CheckCircle className="w-3 h-3 text-green-400" />,
    'web_search': <Search className="w-3 h-3 text-blue-500" />,
    'search': <Search className="w-3 h-3 text-blue-500" />,
    'code': <Code className="w-3 h-3 text-orange-500" />,
    'execution': <Code className="w-3 h-3 text-orange-500" />,
    'error': <AlertCircle className="w-3 h-3 text-red-600" />,
    'end': <CheckCircle className="w-3 h-3 text-green-600" />
  };
  return iconMap[eventType] || <MessageSquare className="w-3 h-3 text-gray-500" />;
};

// Format event content for display
const formatEventContent = (event: ReasoningStep): string => {
  switch (event.step_type) {
    case 'plan':
      if (event.plan) {
        let planContent = '';
        try {
          const planData = typeof event.plan === 'string' ? JSON.parse(event.plan) : event.plan;
          
          if (planData.primary_intent) {
            planContent += `**Primary Intent:** ${planData.primary_intent}\n\n`;
          }
          if (planData.location) {
            planContent += `**Location:** ${planData.location}\n\n`;
          }
          if (planData.crop) {
            planContent += `**Crop:** ${planData.crop}\n\n`;
          }
          if (planData.tools_needed && Array.isArray(planData.tools_needed)) {
            planContent += `**Tools Needed:** ${planData.tools_needed.join(', ')}\n\n`;
          }
          
          return planContent || event.content || 'Planning...';
        } catch (e) {
          // Fallback to original format if parsing fails
          const plan = typeof event.plan === 'string' ? event.plan : JSON.stringify(event.plan, null, 2);
          return `**Plan:** ${plan}`;
        }
      }
      return event.content || 'Planning...';
    case 'thinking':
      return event.content || 'Processing...';
    case 'tool_call':
      const tool = event.tool_name || event.tool || 'Unknown Tool';
      const result = event.tool_result ? `\n\n**Result:** \`\`\`\n${typeof event.tool_result === 'string' ? event.tool_result : JSON.stringify(event.tool_result, null, 2)}\n\`\`\`` : '';
      return `**Tool:** ${tool}${event.tool_args ? `\n**Args:** \`${event.tool_args}\`` : ''}${result}`;
    case 'code_execution':
      if (event.stage === 'code') {
        return `**Code:**\n\`\`\`${event.language || 'python'}\n${event.code || event.content}\n\`\`\``;
      } else if (event.stage === 'result') {
        return `**Result:**\n\`\`\`\n${event.result || event.content}\n\`\`\``;
      }
      return event.content || 'Executing code...';
    case 'google_search_call':
      return `**Searching:** "${event.query || event.content}"`;
    case 'google_search_response':
      return `**Search Results:** ${event.results ? 'Retrieved' : 'Processing...'}`;
    case 'grounding_web_search_queries':
      const queries = event.queries || [];
      return `**Web Searches:** ${queries.map((q: string) => `"${q}"`).join(', ')}`;
    case 'grounding_chunks':
      if (event.sources) {
        const sources = event.sources;
        const sourceList = sources.map((source: any, index: number) => {
          if (typeof source === 'object' && source.uri && source.title) {
            return `${index + 1}. [${source.title}](${source.uri})`;
          } else if (typeof source === 'string') {
            return `${index + 1}. ${source}`;
          }
          return `${index + 1}. ${JSON.stringify(source)}`;
        }).join('\n');
        return `**Sources:**\n\n${sourceList}`;
      }
      return `**Sources:** No sources available`;
    // case 'grounding_supports':
    //   if (event.supports && Array.isArray(event.supports)) {
    //     const supports = event.supports.map((support: any, index: number) => 
    //       `${index + 1}. ${support.title || support.url || support}`
    //     ).join('\n');
    //     return `**Citations:**\n${supports}`;
    //   }
    //   return `**Citations:** Supporting evidence provided`;
    case 'web_search':
    case 'search':
      return `**Web Search:** "${event.query || event.content}"`;
    case 'code':
    case 'execution':
      if (event.code) {
        return `**Code Execution:**\n\`\`\`${event.language || 'python'}\n${event.code}\n\`\`\``;
      }
      return `**Code Execution:** ${event.content || 'Running code...'}`;
    case 'error':
      return `**Error:** ${event.message || event.content}`;
    case 'end':
      return 'Response completed';
    default:
      return event.content || `${event.step_type} event`;
  }
};

// File attachment display component
const FileAttachment = ({ file }: { file: any }) => {
  const getFileIcon = () => {
    const fileName = file.original_filename.toLowerCase();
    
    if (fileName.endsWith('.png') || fileName.endsWith('.jpg') || fileName.endsWith('.jpeg') || 
        fileName.endsWith('.webp') || fileName.endsWith('.heic') || fileName.endsWith('.heif')) {
      return <FileImage className="w-4 h-4 text-blue-500" />;
    } else if (fileName.endsWith('.pdf')) {
      return <FileText className="w-4 h-4 text-red-500" />;
    } else if (fileName.endsWith('.docx')) {
      return <FileText className="w-4 h-4 text-blue-600" />;
    } else if (fileName.endsWith('.xlsx')) {
      return <FileIcon className="w-4 h-4 text-green-600" />;
    } else if (fileName.endsWith('.csv')) {
      return <FileIcon className="w-4 h-4 text-orange-500" />;
    } else {
      return <FileText className="w-4 h-4 text-gray-500" />;
    }
  };

  const formatFileSize = (bytes: number) => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  return (
    <div className="inline-flex items-center bg-white/90 border border-white/50 rounded-lg p-2 mr-2 mb-2 shadow-sm">
      <div className="flex-shrink-0">
        {getFileIcon()}
      </div>
      <div className="flex-1 min-w-0 mx-2">
        <p className="text-xs font-medium text-gray-800 truncate max-w-24 sm:max-w-32">
          {file.original_filename}
        </p>
        <p className="text-xs text-gray-600">
          {formatFileSize(file.file_size)}
          {file.processing_status === 'completed' && ' • processed'}
        </p>
      </div>
      {file.processing_status === 'completed' && (
        <Download className="w-3 h-3 text-gray-600" />
      )}
    </div>
  );
};

const ThinkingAnimation: React.FC = () => (
  <div className="flex items-center space-x-2 py-2">
    <div className="flex space-x-1">
      <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce"></div>
      <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '0.1s' }}></div>
      <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '0.2s' }}></div>
    </div>
    <span className="text-sm text-gray-500">Thinking...</span>
  </div>
);

const StreamingText: React.FC<{ content: string }> = ({ content }) => {
  return (
    <div className="relative">
      <div className="prose prose-sm max-w-none">
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          components={{
            code({ className, children, ...props }: any) {
              const match = /language-(\w+)/.exec(className || '');
              return match ? (
                <pre className="bg-gray-100 p-3 rounded-md overflow-x-auto">
                  <code className={className} {...props}>
                    {children}
                  </code>
                </pre>
              ) : (
                <code className="bg-gray-100 px-1 py-0.5 rounded text-sm" {...props}>
                  {children}
                </code>
              );
            },
          }}
        >
          {content}
        </ReactMarkdown>
      </div>
      <span className="inline-block w-2 h-2 bg-green-500 rounded-full ml-1 animate-pulse"></span>
    </div>
  );
};

// Timeline-based reasoning section
const ReasoningTimeline: React.FC<{ 
  steps: ReasoningStep[]; 
  isExpanded: boolean; 
  onToggle: () => void; 
  isThinking?: boolean;
}> = ({ steps, isExpanded, onToggle, isThinking = false }) => {
  if (steps.length === 0) return null;

  const sortedSteps = [...steps].sort((a, b) => a.step_order - b.step_order);

  return (
    <div className="mb-4 w-full">
      <Collapsible open={isExpanded} onOpenChange={onToggle}>
        <CollapsibleTrigger 
          className="flex items-center space-x-2 text-sm cursor-pointer p-2 w-full"
        >
          <Brain className={`w-4 h-4 ${isThinking ? 'text-green-500 animate-pulse' : 'text-gray-600'}`} />
          <span className={`transition-colors ${
            isThinking 
              ? 'text-gray-700 bg-gradient-to-r from-gray-600 via-green-500 to-gray-600 bg-clip-text text-transparent bg-200% animate-shimmer' 
              : 'text-gray-600 hover:text-gray-800'
          }`}>
            {isThinking ? 'Thinking' : `Show reasoning (${steps.length} steps)`}
          </span>
          {!isThinking && (isExpanded ? <ChevronDown className="w-4 h-4 ml-auto" /> : <ChevronRight className="w-4 h-4 ml-auto" />)}
        </CollapsibleTrigger>
        <CollapsibleContent className="mt-2 w-full">
          <div className="relative pl-6 sm:pl-8 pb-4 w-full">
            {/* Animated vertical timeline line */}
            <div className={`absolute left-3 sm:left-4 top-0 bottom-0 w-0.5 ${
              isThinking 
                ? 'bg-gradient-to-b from-green-500 via-green-300 to-gray-200 animate-pulse' 
                : 'bg-gray-200'
            }`}></div>
            
            {sortedSteps.map((step, index) => (
              <div key={step.id || index} className="relative mb-4 last:mb-0 w-full">
                {/* Timeline dot with glow effect */}
                <div className={`absolute -left-6 sm:-left-7 top-2 w-5 h-5 sm:w-6 sm:h-6 bg-white border-2 rounded-full flex items-center justify-center transition-all duration-300 ${
                  isThinking && index === sortedSteps.length - 1
                    ? 'border-green-400 shadow-lg shadow-green-200 animate-pulse' 
                    : 'border-gray-300'
                }`}>
                  <div className="scale-75 sm:scale-100">
                    {getEventIcon(step.step_type)}
                  </div>
                </div>
                
                {/* Step content with better styling */}
                <div className={`bg-gradient-to-br from-green-50/50 via-white to-emerald-50/30 border border-green-100/50 p-2 sm:p-3 rounded-lg ml-1 transition-all duration-300 w-full ${
                  isThinking && index === sortedSteps.length - 1
                    ? 'ring-2 ring-green-200 shadow-md border-green-200' 
                    : 'hover:shadow-sm hover:border-green-200/70'
                }`}>
                  <div className="prose prose-xs sm:prose-sm max-w-none text-gray-700">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>
                      {formatEventContent(step)}
                    </ReactMarkdown>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </CollapsibleContent>
      </Collapsible>
    </div>
  );
};

// User message actions (copy and edit buttons)
const UserMessageActions: React.FC<{
  messageId: string;
  content: string;
  onCopy?: (content: string) => void;
  onEdit?: (messageId: string) => void;
}> = ({ messageId, content, onCopy, onEdit }) => {
  const [isCopied, setIsCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(content);
      setIsCopied(true);
      onCopy?.(content);
      setTimeout(() => setIsCopied(false), 2000);
    } catch (error) {
      // Error handling
    }
  };

  return (
    <div className="flex items-center space-x-1 mt-2 justify-end opacity-0 group-hover:opacity-100 transition-opacity">
      <button
        onClick={handleCopy}
        className="p-1.5 hover:bg-green-100 rounded transition-colors"
        title="Copy message"
      >
        {isCopied ? (
          <Check className="w-3 h-3 text-green-600" />
        ) : (
          <Copy className="w-3 h-3 text-gray-500 hover:text-green-600" />
        )}
      </button>
      
      {onEdit && (
        <button
          onClick={() => onEdit(messageId)}
          className="p-1.5 hover:bg-green-100 rounded transition-colors"
          title="Edit message"
        >
          <Edit className="w-3 h-3 text-gray-500 hover:text-green-600" />
        </button>
      )}
    </div>
  );
};

// AI message actions (only copy, like, dislike, read aloud)
const AIMessageActions: React.FC<{
  messageId: string;
  content: string;
  onCopy?: (content: string) => void;
  onLike?: (messageId: string) => void;
  onDislike?: (messageId: string) => void;
  onReadAloud?: (content: string, messageId: string) => void;
  onStopReading?: () => void;
  isReading?: boolean;
}> = ({
  messageId,
  content,
  onCopy,
  onLike,
  onDislike,
  onReadAloud,
  onStopReading,
  isReading = false
}) => {
  const [isCopied, setIsCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(content);
      setIsCopied(true);
      onCopy?.(content);
      setTimeout(() => setIsCopied(false), 2000);
    } catch (error) {
      // Error handling
    }
  };

  const handleReadAloud = () => {
    if (isReading && onStopReading) {
      onStopReading();
    } else if (onReadAloud) {
      onReadAloud(content, messageId);
    }
  };

  return (
    <div className="flex items-center space-x-1 sm:space-x-2 mt-3 pt-3 border-t border-gray-100 opacity-0 group-hover:opacity-100 transition-opacity">
      <button
        onClick={handleCopy}
        className="p-1 hover:bg-gray-100 rounded transition-colors"
        title="Copy message"
      >
        {isCopied ? (
          <Check className="w-3 h-3 sm:w-4 sm:h-4 text-green-500" />
        ) : (
          <Copy className="w-3 h-3 sm:w-4 sm:h-4 text-gray-500" />
        )}
      </button>

      {onReadAloud && (
        <button
          onClick={handleReadAloud}
          className="p-1 hover:bg-gray-100 rounded transition-colors"
          title={isReading ? "Stop reading" : "Read aloud"}
        >
          {isReading ? (
            <VolumeX className="w-3 h-3 sm:w-4 sm:h-4 text-red-500" />
          ) : (
            <Volume2 className="w-3 h-3 sm:w-4 sm:h-4 text-gray-500" />
          )}
        </button>
      )}

      {onLike && (
        <button
          onClick={() => onLike(messageId)}
          className="p-1 hover:bg-gray-100 rounded transition-colors"
          title="Like message"
        >
          <ThumbsUp className="w-3 h-3 sm:w-4 sm:h-4 text-gray-500" />
        </button>
      )}

      {onDislike && (
        <button
          onClick={() => onDislike(messageId)}
          className="p-1 hover:bg-gray-100 rounded transition-colors"
          title="Dislike message"
        >
          <ThumbsDown className="w-3 h-3 sm:w-4 sm:h-4 text-gray-500" />
        </button>
      )}
    </div>
  );
};

export const EnhancedMessage: React.FC<EnhancedMessageProps> = ({
  message,
  onCopy,
  onLike,
  onDislike,
  onReadAloud,
  onStopReading,
  isReading = false
}) => {
  const [isReasoningExpanded, setIsReasoningExpanded] = useState(false);

  const isUser = message.role === 'user';
  const hasReasoning = message.reasoning_steps && message.reasoning_steps.length > 0;

  // Debug logging for rendering decisions
  if (isUser) {
    // User message - green bubble, right aligned, responsive width
    return (
      <div className="flex justify-end mb-6 group">
        <div className="max-w-[85%] sm:max-w-lg">
          {/* File attachments */}
          {message.files && message.files.length > 0 && (
            <div className="mb-2">
              {message.files.map((file, index) => (
                <FileAttachment key={file.id || index} file={file} />
              ))}
            </div>
          )}
          
          <div className="bg-green-500 text-white px-3 sm:px-4 py-2 sm:py-3 rounded-2xl rounded-br-md text-sm">
            <div className="prose prose-sm max-w-none text-white prose-invert">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>
                {message.content}
              </ReactMarkdown>
            </div>
          </div>
          <UserMessageActions
            messageId={message.id}
            content={message.content}
            onCopy={onCopy}
            onEdit={undefined}
          />
        </div>
      </div>
    );
  }

  // AI message - transparent background, left aligned, responsive width
  return (
    <div className="flex justify-start mb-6 group">
      <div className="w-full max-w-full sm:max-w-4xl">
        {/* Reasoning Timeline (shown first, at the top) */}
        {hasReasoning && (
          <ReasoningTimeline
            steps={message.reasoning_steps!}
            isExpanded={isReasoningExpanded}
            onToggle={() => setIsReasoningExpanded(!isReasoningExpanded)}
            isThinking={message.is_thinking}
          />
        )}

        {/* Message Content */}
        <div className="px-2 sm:px-0">
          {message.is_thinking && !hasReasoning ? (
            <ThinkingAnimation />
          ) : message.is_streaming ? (
            <StreamingText content={message.content} />
          ) : (
            <div className="prose prose-sm sm:prose max-w-none">
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={{
                  code({ className, children, ...props }: any) {
                    const match = /language-(\w+)/.exec(className || '');
                    return match ? (
                      <pre className="bg-gray-100 p-2 sm:p-3 rounded-md overflow-x-auto text-xs sm:text-sm">
                        <code className={className} {...props}>
                          {children}
                        </code>
                      </pre>
                    ) : (
                      <code className="bg-gray-100 px-1 py-0.5 rounded text-xs sm:text-sm" {...props}>
                        {children}
                      </code>
                    );
                  },
                }}
              >
                {message.content}
              </ReactMarkdown>
            </div>
          )}

          {/* AI Message Actions - Only for completed messages */}
          {!message.is_thinking && !message.is_streaming && (
            <AIMessageActions
              messageId={message.id}
              content={message.content}
              onCopy={onCopy}
              onLike={onLike}
              onDislike={onDislike}
              onReadAloud={onReadAloud}
              onStopReading={onStopReading}
              isReading={isReading}
            />
          )}
        </div>
      </div>
    </div>
  );
};
