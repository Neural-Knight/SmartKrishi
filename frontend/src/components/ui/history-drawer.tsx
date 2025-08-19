import { useState, useEffect, useRef } from "react"
import { X, Search, Plus, MoreVertical, Edit2, Trash2, MessageCircle, Phone } from "lucide-react"
import { chatService, type ChatSummary } from "@/services/chatService"

interface HistoryDrawerProps {
  isOpen: boolean
  onClose: () => void
  onChatSelect: (chatId: string) => void
  onNewChat: () => void
  currentChatId?: string
}

export default function HistoryDrawer({
  isOpen,
  onClose,
  onChatSelect,
  onNewChat,
  currentChatId
}: HistoryDrawerProps) {
  const [searchQuery, setSearchQuery] = useState("")
  const [chatHistory, setChatHistory] = useState<ChatSummary[]>([])
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editingName, setEditingName] = useState("")
  const [openMenuId, setOpenMenuId] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [selectedCategory, setSelectedCategory] = useState<'all' | 'normal' | 'fallback'>('all')

  // Map of refs for each chat menu
  const menuRefs = useRef<Record<string, HTMLDivElement | null>>({})

  useEffect(() => {
    if (isOpen) {
      loadChatHistory()
    } else {
      setOpenMenuId(null)
    }
  }, [isOpen])

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (!openMenuId) return
      const menuEl = menuRefs.current[openMenuId]
      if (menuEl && !menuEl.contains(e.target as Node)) {
        setOpenMenuId(null)
      }
    }
    document.addEventListener("mousedown", handleClickOutside)
    return () => document.removeEventListener("mousedown", handleClickOutside)
  }, [openMenuId])

  const loadChatHistory = async () => {
    try {
      setLoading(true)
      const chats = await chatService.getUserChats()
      setChatHistory(chats)
    } catch (error) {
      // Error handling
    } finally {
      setLoading(false)
    }
  }

  const filteredHistory = chatHistory.filter(chat => {
    // Search filter
    const matchesSearch = chat.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
      chat.last_message.toLowerCase().includes(searchQuery.toLowerCase());
    
    // Category filter (for now, treat all chats as normal since we don't have fallback flag in the current schema)
    // This can be extended when the backend supports is_fallback_chat flag
    const matchesCategory = selectedCategory === 'all' || 
      (selectedCategory === 'normal' && true) || // All current chats are normal
      (selectedCategory === 'fallback' && false); // No fallback chats yet
    
    return matchesSearch && matchesCategory;
  });

  const handleRename = (chat: ChatSummary) => {
    setEditingId(chat.id)
    setEditingName(chat.title)
    setOpenMenuId(null)
  }

  const handleSaveRename = async () => {
    if (editingId && editingName.trim()) {
      try {
        await chatService.updateChat(editingId, editingName.trim())
        setChatHistory(prev =>
          prev.map(chat =>
            chat.id === editingId ? { ...chat, title: editingName.trim() } : chat
          )
        )
      } catch (error) {
        // Error handling
      }
    }
    setEditingId(null)
    setEditingName("")
  }

  const handleDelete = async (chatId: string) => {
    try {
      await chatService.deleteChat(chatId)
      setChatHistory(prev => prev.filter(chat => chat.id !== chatId))
      setOpenMenuId(null)
    } catch (error) {
      // Error handling
    }
  }

  const handleCreateNewChat = async () => {
    try {
      const newChat = await chatService.createChat("New Chat")
      const newChatSummary: ChatSummary = {
        id: newChat.id,
        title: newChat.title,
        created_at: newChat.created_at,
        updated_at: newChat.updated_at,
        last_message: "",
        message_count: 0
      }
      setChatHistory(prev => [newChatSummary, ...prev])
      onChatSelect(newChat.id)
      onClose()
    } catch (error) {
      // Error handling
      onNewChat()
      onClose()
    }
  }

  const formatDate = (dateString: string) => {
    const date = new Date(dateString)
    const now = new Date()
    const diffDays = Math.floor((now.getTime() - date.getTime()) / (1000 * 60 * 60 * 24))
    if (diffDays === 0) return "Today"
    if (diffDays === 1) return "Yesterday"
    if (diffDays < 7) return `${diffDays} days ago`
    return date.toLocaleDateString()
  }

  if (!isOpen) return null

  return (
    <>
      {/* Overlay */}
      <div
        className="fixed inset-0 bg-black/40 backdrop-blur-sm z-40"
        onClick={() => {
          setOpenMenuId(null)
          onClose()
        }}
      />

      {/* Drawer */}
      <div className="fixed right-0 top-0 h-full w-full sm:w-96 lg:w-80 bg-white/70 backdrop-blur-lg border-l border-white/20 shadow-xl z-50 flex flex-col rounded-l-xl overflow-hidden">
        
        {/* Header */}
        <div className="p-4 border-b border-white/20 bg-gradient-to-r from-emerald-500/10 to-teal-400/10">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold bg-gradient-to-r from-emerald-600 to-teal-600 bg-clip-text text-transparent">
              Chat History
            </h2>
            <button
              onClick={() => {
                setOpenMenuId(null)
                onClose()
              }}
              className="p-1 rounded-lg hover:bg-white/40 text-gray-600 hover:text-gray-900 transition-colors"
            >
              <X className="w-5 h-5" />
            </button>
          </div>

          {/* Search */}
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-emerald-600" />
            <input
              type="text"
              placeholder="Search chats..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full pl-10 pr-4 py-2 rounded-full bg-white/60 border border-white/30 focus:ring-2 focus:ring-emerald-400 text-sm placeholder-gray-500"
            />
          </div>
          
          {/* Category Tabs */}
          <div className="flex bg-white/40 rounded-full p-1 mt-3">
            <button
              onClick={() => setSelectedCategory('all')}
              className={`flex-1 px-3 py-1.5 rounded-full text-xs font-medium transition-all ${
                selectedCategory === 'all' 
                  ? 'bg-emerald-500 text-white shadow-sm' 
                  : 'text-emerald-700 hover:bg-white/60'
              }`}
            >
              All ({chatHistory.length})
            </button>
            <button
              onClick={() => setSelectedCategory('normal')}
              className={`flex-1 px-3 py-1.5 rounded-full text-xs font-medium transition-all flex items-center justify-center gap-1 ${
                selectedCategory === 'normal' 
                  ? 'bg-blue-500 text-white shadow-sm' 
                  : 'text-blue-700 hover:bg-white/60'
              }`}
            >
              <MessageCircle className="w-3 h-3" />
              Normal ({chatHistory.length})
            </button>
            <button
              onClick={() => setSelectedCategory('fallback')}
              className={`flex-1 px-3 py-1.5 rounded-full text-xs font-medium transition-all flex items-center justify-center gap-1 ${
                selectedCategory === 'fallback' 
                  ? 'bg-orange-500 text-white shadow-sm' 
                  : 'text-orange-700 hover:bg-white/60'
              }`}
            >
              <Phone className="w-3 h-3" />
              SMS (0)
            </button>
          </div>
        </div>

        {/* New Chat Button */}
        <div className="p-4 border-b border-white/20">
          <button
            onClick={handleCreateNewChat}
            className="w-full flex items-center justify-center space-x-2 p-3 rounded-full bg-gradient-to-r from-emerald-500 to-teal-500 hover:from-emerald-600 hover:to-teal-600 text-white shadow-md hover:shadow-lg transition-all duration-300"
          >
            <Plus className="w-4 h-4" />
            <span>New Chat</span>
          </button>
        </div>

        {/* Chat List */}
        <div className="flex-1 overflow-y-auto p-2">
          {loading ? (
            <div className="p-4 text-center text-gray-500">Loading chats...</div>
          ) : filteredHistory.length === 0 ? (
            <div className="p-4 text-center text-gray-500">
              {searchQuery 
                ? "No chats found" 
                : selectedCategory === 'fallback' 
                  ? "No SMS fallback chats yet" 
                  : "No chat history yet"}
            </div>
          ) : (
            filteredHistory.map((chat) => (
              <div
                key={chat.id}
                className={`relative group mb-2 p-3 rounded-lg cursor-pointer border transition-all duration-300 ${
                  currentChatId === chat.id
                    ? "bg-gradient-to-r from-emerald-50 to-teal-50 border-emerald-200"
                    : "bg-white/50 hover:bg-white/70 border-transparent hover:border-white/30"
                }`}
                onClick={() => {
                  onChatSelect(chat.id)
                  onClose()
                }}
              >
                <div className="flex items-start justify-between">
                  <div className="flex items-start gap-2 flex-1 min-w-0">
                    {/* Chat Type Icon */}
                    <div className="flex-shrink-0 mt-0.5">
                      <MessageCircle className="w-3.5 h-3.5 text-blue-500" />
                      {/* Future: <Phone className="w-3.5 h-3.5 text-orange-500" /> for SMS chats */}
                    </div>
                    
                    <div className="flex-1 min-w-0">
                      {editingId === chat.id ? (
                        <input
                          type="text"
                          value={editingName}
                          onChange={(e) => setEditingName(e.target.value)}
                          onBlur={handleSaveRename}
                          onKeyDown={(e) => e.key === "Enter" && handleSaveRename()}
                          className="w-full text-sm font-medium text-gray-900 bg-white border border-emerald-500 rounded px-2 py-1"
                          autoFocus
                          onClick={(e) => e.stopPropagation()}
                        />
                      ) : (
                        <h3 className="text-sm font-medium text-gray-900 truncate">
                          {chat.title}
                        </h3>
                      )}
                      <p className="text-xs text-gray-500 truncate mt-1">
                        {chat.last_message || "No messages yet"}
                      </p>
                      <div className="flex items-center justify-between mt-1">
                        <p className="text-xs text-gray-400">
                          {formatDate(chat.updated_at)}
                        </p>
                        <div className="flex items-center gap-2">
                          {chat.message_count > 0 && (
                            <span className="text-xs text-gray-400">
                              {chat.message_count} messages
                            </span>
                          )}
                          {/* Future: SMS indicator badge */}
                        </div>
                      </div>
                    </div>
                  </div>

                  {/* Menu */}
                  <div
                    className="relative"
                    ref={(el) => { menuRefs.current[chat.id] = el }}
                  >
                    <button
                      onClick={(e) => {
                        e.stopPropagation()
                        setOpenMenuId(openMenuId === chat.id ? null : chat.id)
                      }}
                      className="p-1 rounded hover:bg-white/50 opacity-100 lg:opacity-0 lg:group-hover:opacity-100 transition-opacity"
                    >
                      <MoreVertical className="w-4 h-4 text-gray-500" />
                    </button>

                    {openMenuId === chat.id && (
                      <div className="absolute right-0 top-8 bg-white/80 backdrop-blur-md border border-white/30 rounded-lg shadow-lg py-1 z-10 w-36">
                        <button
                          onClick={(e) => {
                            e.stopPropagation()
                            handleRename(chat)
                          }}
                          className="w-full px-4 py-2 text-left text-sm text-gray-700 hover:bg-emerald-50 flex items-center space-x-2 transition-colors"
                        >
                          <Edit2 className="w-4 h-4" />
                          <span>Rename</span>
                        </button>
                        <button
                          onClick={(e) => {
                            e.stopPropagation()
                            handleDelete(chat.id)
                          }}
                          className="w-full px-4 py-2 text-left text-sm text-red-600 hover:bg-red-50 flex items-center space-x-2 transition-colors"
                        >
                          <Trash2 className="w-4 h-4" />
                          <span>Delete</span>
                        </button>
                      </div>
                    )}
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      </div>
    </>
  )
}
