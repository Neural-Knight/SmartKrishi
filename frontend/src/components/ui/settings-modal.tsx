import { X, LogOut, User, Phone, Info, Sparkles, Leaf } from "lucide-react"
import { useState } from "react"
import { FallbackSettings } from "./fallback-settings"
import { NetworkStatus } from "./network-status"

interface SettingsModalProps {
  isOpen: boolean
  onClose: () => void
  userName: string
  onLogout: () => void
}

export default function SettingsModal({
  isOpen,
  onClose,
  userName,
  onLogout
}: SettingsModalProps) {
  const [activeTab, setActiveTab] = useState<'account' | 'sms'>('account');

  if (!isOpen) return null

  return (
    <>
      {/* Overlay */}
      <div
        className="fixed inset-0 bg-black/40 backdrop-blur-sm z-50 flex items-center justify-center p-2 sm:p-4"
        onClick={onClose}
      >
        {/* Modal */}
        <div
          className="bg-white/70 backdrop-blur-xl rounded-xl sm:rounded-2xl shadow-xl w-full max-w-2xl border border-white/20 max-h-[95vh] sm:max-h-[90vh] overflow-hidden flex flex-col"
          onClick={(e) => e.stopPropagation()}
        >
          {/* Header with Tabs */}
          <div className="flex-shrink-0">
            <div className="flex items-center justify-between p-4 sm:p-6 border-b border-white/20 bg-gradient-to-r from-emerald-500/10 to-teal-400/10 rounded-t-xl sm:rounded-t-2xl">
              <h2 className="text-base sm:text-lg font-semibold bg-gradient-to-r from-emerald-600 to-teal-600 bg-clip-text text-transparent">
                Settings
              </h2>
              <button
                onClick={onClose}
                className="p-1.5 rounded-lg hover:bg-white/50 transition-colors"
              >
                <X className="w-5 h-5 text-gray-600" />
              </button>
            </div>
            
            {/* Tab Navigation */}
            <div className="flex bg-white/30 border-b border-white/20">
              <button
                onClick={() => setActiveTab('account')}
                className={`flex-1 px-3 sm:px-6 py-3 text-xs sm:text-sm font-medium transition-all flex items-center justify-center gap-1 sm:gap-2 ${
                  activeTab === 'account'
                    ? 'text-emerald-700 border-b-2 border-emerald-500 bg-white/40'
                    : 'text-gray-600 hover:text-gray-800 hover:bg-white/20'
                }`}
              >
                <User className="w-3 h-3 sm:w-4 sm:h-4" />
                <span className="hidden sm:inline">Account</span>
                <span className="sm:hidden">Profile</span>
              </button>
              <button
                onClick={() => setActiveTab('sms')}
                className={`flex-1 px-3 sm:px-6 py-3 text-xs sm:text-sm font-medium transition-all flex items-center justify-center gap-1 sm:gap-2 ${
                  activeTab === 'sms'
                    ? 'text-orange-700 border-b-2 border-orange-500 bg-white/40'
                    : 'text-gray-600 hover:text-gray-800 hover:bg-white/20'
                }`}
              >
                <Phone className="w-3 h-3 sm:w-4 sm:h-4" />
                <span className="hidden sm:inline">SMS Fallback</span>
                <span className="sm:hidden">SMS</span>
              </button>
            </div>
          </div>

          {/* Tab Content */}
          <div className="flex-1 overflow-y-auto">
            {activeTab === 'account' ? (
              <div className="p-4 sm:p-6 space-y-6">
                {/* Enhanced User Profile Section */}
                <div className="relative">
                  {/* Background Gradient */}
                  <div className="absolute inset-0 bg-gradient-to-r from-emerald-500/10 via-teal-500/10 to-green-500/10 rounded-2xl"></div>
                  
                  {/* User Profile Card */}
                  <div className="relative p-6 sm:p-8 bg-white/60 backdrop-blur-sm border border-white/40 rounded-2xl shadow-lg">
                    <div className="flex flex-col items-center text-center space-y-4">
                      {/* Enhanced Avatar */}
                      <div className="relative">
                        <div className="w-20 h-20 sm:w-24 sm:h-24 bg-gradient-to-br from-emerald-500 via-teal-500 to-green-500 rounded-full flex items-center justify-center shadow-xl ring-4 ring-white/50">
                          <User className="w-10 h-10 sm:w-12 sm:h-12 text-white" />
                        </div>
                        {/* Online Status Indicator */}
                        <div className="absolute -bottom-1 -right-1 w-6 h-6 bg-green-500 rounded-full border-4 border-white shadow-lg">
                          <div className="w-full h-full bg-green-400 rounded-full animate-pulse"></div>
                        </div>
                      </div>
                      
                      {/* User Name and Role */}
                      <div className="space-y-2">
                        <h3 className="text-xl sm:text-2xl font-bold bg-gradient-to-r from-gray-800 to-gray-600 bg-clip-text text-transparent">
                          {userName}
                        </h3>
                        <div className="flex items-center justify-center space-x-2">
                          <div className="w-2 h-2 bg-emerald-500 rounded-full"></div>
                          <span className="text-sm sm:text-base text-emerald-700 font-medium">
                            SmartKrishi User
                          </span>
                          <div className="w-2 h-2 bg-emerald-500 rounded-full"></div>
                        </div>
                      </div>
                      
                      {/* Member Since */}
                      <div className="px-4 py-2 bg-gradient-to-r from-emerald-100/80 to-teal-100/80 rounded-full border border-emerald-200/50">
                        <p className="text-xs sm:text-sm text-emerald-700 font-medium">
                          🌱 Farming Smart Since 2024
                        </p>
                      </div>
                    </div>
                  </div>
                </div>

                {/* Account Stats */}
                <div className="grid grid-cols-2 gap-3 sm:gap-4">
                  <div className="bg-white/50 backdrop-blur-sm border border-white/30 rounded-xl p-4 text-center">
                    <div className="w-8 h-8 bg-blue-100 rounded-full flex items-center justify-center mx-auto mb-2">
                      <Sparkles className="w-4 h-4 text-blue-600" />
                    </div>
                    <p className="text-lg sm:text-xl font-bold text-gray-800">24/7</p>
                    <p className="text-xs sm:text-sm text-gray-600">AI Support</p>
                  </div>
                  <div className="bg-white/50 backdrop-blur-sm border border-white/30 rounded-xl p-4 text-center">
                    <div className="w-8 h-8 bg-green-100 rounded-full flex items-center justify-center mx-auto mb-2">
                      <Leaf className="w-4 h-4 text-green-600" />
                    </div>
                    <p className="text-lg sm:text-xl font-bold text-gray-800">∞</p>
                    <p className="text-xs sm:text-sm text-gray-600">Crop Insights</p>
                  </div>
                </div>

                {/* Quick Actions */}
                <div className="space-y-3">
                  <h4 className="text-sm font-semibold text-gray-700 flex items-center">
                    <Info className="w-4 h-4 mr-2 text-gray-500" />
                    Account Actions
                  </h4>
                  <button
                    onClick={onLogout}
                    className="w-full flex items-center justify-center space-x-3 py-3 px-6 rounded-2xl 
                               bg-gradient-to-r from-red-500 to-red-600 
                               hover:from-red-600 hover:to-red-700 
                               text-white font-medium 
                               shadow-lg hover:shadow-xl transition-all duration-300
                               transform hover:scale-[1.02] active:scale-[0.98]"
                  >
                    <LogOut className="w-5 h-5" />
                    <span>Sign Out</span>
                  </button>
                </div>

                {/* Enhanced App Information */}
                <div className="bg-gradient-to-r from-gray-50/80 to-blue-50/80 backdrop-blur-sm border border-gray-200/50 rounded-2xl p-6 space-y-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-3">
                      <div className="w-10 h-10 bg-gradient-to-br from-emerald-500 to-teal-500 rounded-xl flex items-center justify-center">
                        <span className="text-white font-bold text-lg">🌱</span>
                      </div>
                      <div>
                        <h4 className="font-semibold text-gray-800">SmartKrishi AI</h4>
                        <p className="text-sm text-gray-600">Version 1.0</p>
                      </div>
                    </div>
                    <div className="text-right">
                      <div className="px-3 py-1 bg-green-100 text-green-700 rounded-full text-xs font-medium">
                        Active
                      </div>
                    </div>
                  </div>
                  
                  <div className="space-y-2">
                    <p className="text-sm text-gray-700 leading-relaxed">
                      Your AI-powered farming assistant providing expert crop advice, 
                      market insights, and agricultural intelligence.
                    </p>
                    <div className="flex items-center space-x-4 text-xs text-gray-500">
                      <span>• Real-time AI Analysis</span>
                      <span>• Multi-language Support</span>
                      <span>• 24/7 Availability</span>
                    </div>
                  </div>
                </div>
              </div>
            ) : (
              <div className="p-4 sm:p-6 space-y-6">
                <NetworkStatus className="mb-6" />
                <FallbackSettings />
              </div>
            )}
          </div>
        </div>
      </div>
    </>
  )
}
