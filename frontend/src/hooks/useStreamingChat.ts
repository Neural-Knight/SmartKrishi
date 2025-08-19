import { useState, useCallback, useRef } from 'react';
import { chatService } from '../services/chatService';
import type { 
  StreamingEvent, 
  StreamingStatus, 
  ReasoningStep,
  ChatMessage
} from '../services/chatService';

interface UseStreamingChatProps {
  onMessageUpdate?: (message: ChatMessage) => void;
  onNewMessage?: (message: ChatMessage) => void;
  onError?: (error: string) => void;
  onChatCreated?: (chatId: string) => void;
}

interface MessageStreamingState {
  messageId: string;
  isThinking: boolean;
  isStreaming: boolean;
  accumulatedContent: string;
  reasoningSteps: ReasoningStep[];
}

interface StreamingState {
  isStreaming: boolean;
  currentStatus: StreamingStatus;
  streamingMessages: Map<string, MessageStreamingState>;
}

export const useStreamingChat = ({
  onMessageUpdate,
  onNewMessage,
  onError,
  onChatCreated
}: UseStreamingChatProps = {}) => {
  const [state, setState] = useState<StreamingState>({
    isStreaming: false,
    currentStatus: { status: 'complete' },
    streamingMessages: new Map()
  });

  const abortController = useRef<AbortController | null>(null);

  const sendMessageWithStreaming = useCallback(async (
    message: string,
    chatId?: string,
    options?: {
      model?: string;
      tools?: string[];
      include_logs?: boolean;
    }
  ) => {
    // Abort any ongoing stream
    if (abortController.current) {
      abortController.current.abort();
    }

    abortController.current = new AbortController();

    // Add user message immediately
    const userMessage: ChatMessage = {
      id: `user-${Date.now()}`,
      role: 'user',
      content: message,
      timestamp: new Date()
    };
    
    onNewMessage?.(userMessage);

    setState(prev => ({
      ...prev,
      isStreaming: true,
      currentStatus: { status: 'initializing' }
    }));

    try {
      const streamGenerator = chatService.sendMessageStream(message, chatId, options);

      for await (const event of streamGenerator) {
        if (abortController.current?.signal.aborted) {
          break;
        }

        await handleStreamingEvent(event);
      }

      setState(prev => ({
        ...prev,
        isStreaming: false,
        currentStatus: { status: 'complete' }
      }));

    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Streaming failed';
      
      setState(prev => ({
        ...prev,
        isStreaming: false,
        currentStatus: { status: 'error' }
      }));

      onError?.(errorMessage);
    }
  }, [onMessageUpdate, onNewMessage, onError, onChatCreated]);

  const uploadFileWithStreaming = useCallback(async (
    files: File[] | File,
    message: string,
    chatId?: string,
    skipUserMessage = false // New parameter to prevent duplicate user messages
  ) => {
    // Abort any ongoing stream
    if (abortController.current) {
      abortController.current.abort();
    }

    abortController.current = new AbortController();

    setState(prev => ({
      ...prev,
      isStreaming: true,
      currentStatus: { status: 'initializing' }
    }));

    try {
      const fileArray = Array.isArray(files) ? files : [files];
      
      if (fileArray.length === 0) {
        return sendMessageWithStreaming(message, chatId);
      }

      // For multiple files or new file upload flow
      if (fileArray.length > 1 || !chatService.uploadFileAndAnalyzeStream) {
        // Upload files first
        setState(prev => ({
          ...prev,
          isStreaming: true,
          currentStatus: { status: 'initializing' }
        }));

        const uploadResults = await chatService.uploadFiles(chatId || '', fileArray);
        
        // Create user message with files
        const userMessage: ChatMessage = {
          id: `user-${Date.now()}`,
          role: 'user',
          content: message,
          timestamp: new Date(),
          files: uploadResults.map(result => ({
            id: result.file_id,
            original_filename: result.original_filename,
            file_type: result.file_type,
            file_size: result.file_size,
            processing_status: result.processing_status,
            agent_file_id: result.agent_file_id,
            created_at: new Date().toISOString()
          }))
        };
        
        // Only add user message if not already added
        if (!skipUserMessage) {
          onNewMessage?.(userMessage);
        }

        // Then stream the response
        return sendMessageWithStreaming(message, chatId);
      }

      // Legacy single file streaming (if available)
      const streamGenerator = chatService.uploadFileAndAnalyzeStream(fileArray[0], message, chatId);

      for await (const event of streamGenerator) {
        if (abortController.current?.signal.aborted) {
          break;
        }

        await handleStreamingEvent(event);
      }

      setState(prev => ({
        ...prev,
        isStreaming: false,
        currentStatus: { status: 'complete' }
      }));

    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'File upload streaming failed';
      
      setState(prev => ({
        ...prev,
        isStreaming: false,
        currentStatus: { status: 'error' }
      }));

      onError?.(errorMessage);
    }
  }, [sendMessageWithStreaming, onNewMessage, onError]);

  // Helper function to update status from log stage
  const updateStatusFromLogStage = useCallback((stage: string) => {
    const statusMap: Record<string, StreamingStatus> = {
      'initialization': { status: 'initializing', current_step: 'initialization' },
      'planner_start': { status: 'planning', current_step: 'planner_start' },
      'planner_input': { status: 'planning', current_step: 'planner_input' },
      'planner_complete': { status: 'planning', current_step: 'planner_complete' },
      'tools_start': { status: 'thinking', current_step: 'tools_start' },
      'tool_executing': { status: 'thinking', current_step: 'tool_executing' },
      'tools_complete': { status: 'thinking', current_step: 'tools_complete' },
      'agent_start': { status: 'thinking', current_step: 'agent_start' },
      'file_check': { status: 'thinking', current_step: 'file_check' },
      'agent_prompt_ready': { status: 'thinking', current_step: 'agent_prompt_ready' },
      'complete': { status: 'complete', current_step: 'complete' }
    };
    
    const newStatus = statusMap[stage];
    if (newStatus) {
      setState(prev => ({
        ...prev,
        currentStatus: newStatus
      }));
    }
  }, []);

  // Helper function to add reasoning step with duplicate detection
  const addReasoningStep = useCallback((
    messageId: string,
    currentMessage: MessageStreamingState,
    step: Partial<ReasoningStep>
  ): boolean => {
    // Simple duplicate detection - check if same step type and content already exists
    const isDuplicate = currentMessage.reasoningSteps.some(existingStep => 
      existingStep.step_type === step.step_type && 
      existingStep.content === step.content &&
      Math.abs(existingStep.step_order - currentMessage.reasoningSteps.length) <= 1
    );
    
    if (!isDuplicate && step.content && step.content.trim() !== '') {
      const reasoningStep: ReasoningStep = {
        id: `${Date.now()}-${Math.random()}`,
        step_order: currentMessage.reasoningSteps.length,
        created_at: new Date().toISOString(),
        ...step
      } as ReasoningStep;

      currentMessage.reasoningSteps.push(reasoningStep);
      currentMessage.isThinking = true;
      
      // Update the thinking message
      const thinkingMessage: ChatMessage = {
        id: messageId,
        role: 'assistant',
        content: currentMessage.accumulatedContent,
        timestamp: new Date(),
        reasoning_steps: [...currentMessage.reasoningSteps],
        is_thinking: true,
        is_streaming: false
      };
      
      onMessageUpdate?.(thinkingMessage);
      return true;
    }
    
    return false;
  }, [onMessageUpdate]);

  const handleStreamingEvent = useCallback(async (event: StreamingEvent) => {
    // Agent API Event processing
    
    const messageId = event.message_id;
    if (!messageId) return;

    setState(prev => {
      const newStreamingMessages = new Map(prev.streamingMessages);
      const currentMessage = newStreamingMessages.get(messageId) || {
        messageId,
        isThinking: false,
        isStreaming: false,
        accumulatedContent: '',
        reasoningSteps: []
      };

      switch (event.type) {
        case 'log':
          // Use log events for status updates instead of skipping
          if (event.stage) {
            updateStatusFromLogStage(event.stage);
          }
          break;
          
        case 'plan':
          addReasoningStep(messageId, currentMessage, {
            step_type: 'plan',
            content: `Planning: ${event.plan?.primary_intent || 'Creating strategy'}`,
            plan: event.plan,
            raw_response: event.raw_response,
            stage: event.stage
          });
          break;
          
        case 'thinking':
          addReasoningStep(messageId, currentMessage, {
            step_type: 'thinking',
            content: event.content || '',
            stage: event.stage
          });
          break;
          
        case 'tool_call':
          addReasoningStep(messageId, currentMessage, {
            step_type: 'tool_call',
            content: `Using ${event.tool || 'tool'}`,
            tool_name: event.tool,
            tool: event.tool,
            tool_args: typeof event.args === 'string' ? event.args : JSON.stringify(event.args || ''),
            tool_result: event.result,
            stage: event.stage
          });
          break;
          
        case 'code_execution':
          if (event.stage === 'code') {
            addReasoningStep(messageId, currentMessage, {
              step_type: 'code_execution',
              content: `Executing: ${event.language || 'code'}`,
              code: event.code,
              language: event.language,
              stage: 'code'
            });
          } else if (event.stage === 'result') {
            addReasoningStep(messageId, currentMessage, {
              step_type: 'code_execution',
              content: `Result: ${event.outcome || 'success'}`,
              result: event.result,
              outcome: event.outcome || 'success',
              language: event.language, // Preserve language from code stage
              stage: 'result'
            });
          }
          break;
          
        case 'google_search_call':
          addReasoningStep(messageId, currentMessage, {
            step_type: 'google_search_call',
            content: `Searching: ${event.query || 'web search'}`,
            query: event.query,
            stage: event.stage
          });
          break;
          
        case 'google_search_response':
          addReasoningStep(messageId, currentMessage, {
            step_type: 'google_search_response',
            content: 'Search results received',
            results: event.results,
            stage: event.stage
          });
          break;
          
        case 'grounding_web_search_queries':
          addReasoningStep(messageId, currentMessage, {
            step_type: 'grounding_web_search_queries',
            content: `Web searches: ${event.queries?.join(', ') || 'web queries'}`,
            queries: event.queries,
            stage: event.stage
          });
          break;
          
        case 'grounding_chunks':
          addReasoningStep(messageId, currentMessage, {
            step_type: 'grounding_chunks',
            content: `Sources: ${event.sources?.length || 0} references`,
            sources: event.sources,
            stage: event.stage
          });
          break;
          
        // case 'grounding_supports':
        //   addReasoningStep(messageId, currentMessage, {
        //     step_type: 'grounding_supports',
        //     content: 'Citations linked',
        //     supports: event.supports,
        //     stage: event.stage
        //   });
        //   break;
          break;
       

        case 'response_chunk':
          // Start streaming response
          currentMessage.isThinking = false;
          currentMessage.isStreaming = true;
          currentMessage.accumulatedContent += event.content || '';

          const streamingMessage: ChatMessage = {
            id: messageId,
            role: 'assistant',
            content: currentMessage.accumulatedContent,
            timestamp: new Date(),
            reasoning_steps: [...currentMessage.reasoningSteps],
            is_thinking: false,
            is_streaming: true
          };
          
          onMessageUpdate?.(streamingMessage);
          break;

        case 'response':
          // Complete response received (alternative to response_chunk + end)
          currentMessage.isThinking = false;
          currentMessage.isStreaming = false;
          
          const completeMessage: ChatMessage = {
            id: messageId,
            role: 'assistant',
            content: event.response || event.content || currentMessage.accumulatedContent,
            timestamp: new Date(),
            reasoning_steps: [...currentMessage.reasoningSteps],
            is_thinking: false,
            is_streaming: false,
            grounding_metadata: event.grounding_metadata
          };
          
          onMessageUpdate?.(completeMessage);
          break;

        case 'error':
          // Add error as reasoning step and handle error
          addReasoningStep(messageId, currentMessage, {
            step_type: 'error',
            content: event.message || event.error || 'An error occurred',
            error: event.message || event.error
          });
          
          const errorMessage: ChatMessage = {
            id: messageId,
            role: 'assistant',
            content: 'I encountered an error while processing your request.',
            timestamp: new Date(),
            reasoning_steps: [...currentMessage.reasoningSteps],
            is_thinking: false,
            is_streaming: false
          };
          
          onMessageUpdate?.(errorMessage);
          onError?.(event.error || event.message || 'Unknown streaming error');
          break;

        case 'end':
          // Finalize message
          currentMessage.isThinking = false;
          currentMessage.isStreaming = false;
          
          const finalContent = event.final_content || event.response || currentMessage.accumulatedContent;
          
          const finalMessage: ChatMessage = {
            id: messageId,
            role: 'assistant',
            content: finalContent,
            timestamp: new Date(),
            reasoning_steps: [...currentMessage.reasoningSteps],
            is_thinking: false,
            is_streaming: false
          };
          
          // Update existing message instead of creating a new one to avoid duplicates
          onMessageUpdate?.(finalMessage);
          
          // Notify about chat creation if this was a new chat
          if (event.chat_id && onChatCreated) {
            // End event received with chat_id
            onChatCreated(event.chat_id);
          }
          
          newStreamingMessages.delete(messageId); // Clean up
          break;

        default:
          // Handle any unknown event types by logging them but not processing
          // Unknown event type received
          break;
      }

      newStreamingMessages.set(messageId, currentMessage);

      return {
        ...prev,
        streamingMessages: newStreamingMessages
      };
    });
  }, [onMessageUpdate, onError, onChatCreated, updateStatusFromLogStage, addReasoningStep]);

  const stopStreaming = useCallback(() => {
    if (abortController.current) {
      abortController.current.abort();
      setState(prev => ({
        ...prev,
        isStreaming: false,
        currentStatus: { status: 'complete' }
      }));
    }
  }, []);

  return {
    ...state,
    sendMessageWithStreaming,
    uploadFileWithStreaming,
    stopStreaming
  };
};
