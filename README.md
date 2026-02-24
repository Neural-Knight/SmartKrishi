# SmartKrishi - AI-Powered Agricultural Assistant

A comprehensive full-stack platform providing intelligent agricultural guidance through AI-powered chat, real-time assistance, and advanced farming insights.

## 🌾 Overview

SmartKrishi is a modern agricultural assistant that combines cutting-edge AI technology with practical farming knowledge. The platform provides farmers with instant access to agricultural expertise, crop management advice, weather insights, and farming best practices through an intuitive chat interface.

## ✨ Key Features

- **🤖 AI-Powered Chat**: Advanced conversational AI with agricultural domain expertise
- **📱 Multi-Platform Access**: Web app, mobile-responsive design, and SMS fallback
- **📂 File Analysis**: Upload and analyze farm photos, soil reports, and documents  
- **🔄 Real-time Streaming**: Instant AI responses with step-by-step reasoning
- **🔐 Secure Authentication**: Firebase-based phone authentication
- **📊 Smart Insights**: Crop recommendations, disease identification, and yield optimization
- **🌐 Offline Support**: SMS/WhatsApp fallback for areas with limited internet
- **📈 Conversation History**: Persistent chat history and knowledge base

## 🏗️ System Architecture

```mermaid
graph TB
    subgraph "Frontend Applications"
        WEB[Web Application<br/>React + TypeScript]
        MOB[Mobile Web<br/>PWA Ready]
        SMS[SMS Interface<br/>WhatsApp Bot]
    end
    
    subgraph "API Gateway & Load Balancer"
        LB[Nginx/Cloudflare<br/>Load Balancer]
        CORS[CORS Handler]
        RATE[Rate Limiter]
    end
    
    subgraph "Backend Services"
        API[FastAPI Server<br/>Python 3.12]
        AUTH[Auth Service<br/>JWT + Firebase]
        CHAT[Chat Service<br/>Streaming WebSocket]
        FILE[File Service<br/>Upload & Analysis]
        SMS_SVC[SMS Service<br/>Twilio/WhatsApp]
    end
    
    subgraph "AI & External Services"
        GEMINI[Google Gemini<br/>AI Model]
        AGENT[Agent API<br/>Advanced Reasoning]
        FIREBASE[Firebase Auth<br/>Phone Verification]
        STORAGE[Cloud Storage<br/>File Storage]
    end
    
    subgraph "Data Layer"
        POSTGRES[(PostgreSQL<br/>Primary Database)]
        REDIS[(Redis<br/>Cache & Sessions)]
        FILES[(File Storage<br/>Images & Documents)]
    end
    
    subgraph "Infrastructure"
        DOCKER[Docker Containers]
        MONITOR[Monitoring<br/>Logs & Metrics]
        BACKUP[Backup Systems]
    end
    
    WEB --> LB
    MOB --> LB
    SMS --> LB
    
    LB --> CORS
    CORS --> RATE
    RATE --> API
    
    API --> AUTH
    API --> CHAT
    API --> FILE
    API --> SMS_SVC
    
    AUTH --> FIREBASE
    CHAT --> GEMINI
    CHAT --> AGENT
    FILE --> STORAGE
    
    API --> POSTGRES
    API --> REDIS
    FILE --> FILES
    
    API --> DOCKER
    DOCKER --> MONITOR
    MONITOR --> BACKUP
```

## 🛠️ Tech Stack

### Frontend
- **Framework**: React 19.1+ with TypeScript 5.8+
- **Build Tool**: Vite 7.1+ with hot module replacement
- **UI Library**: Radix UI + Tailwind CSS 3+
- **State Management**: React Context + Custom Hooks
- **Authentication**: Firebase Auth with phone verification
- **HTTP Client**: Axios with interceptors
- **Real-time**: Server-Sent Events (SSE)
- **PWA**: Progressive Web App capabilities

### Backend
- **Framework**: FastAPI 0.115+ with Python 3.12+
- **Database**: PostgreSQL 15+ with SQLAlchemy 2.0+
- **Authentication**: JWT + Firebase Admin SDK
- **API Documentation**: OpenAPI 3.0 with Swagger UI
- **File Processing**: Pillow + Multi-format support
- **Streaming**: WebSocket + Server-Sent Events
- **Deployment**: Gunicorn + Docker

### AI & External Services
- **AI Model**: Google Gemini Pro for agricultural intelligence
- **Agent API**: Advanced reasoning and tool usage
- **Phone Auth**: Firebase Authentication
- **SMS/WhatsApp**: Twilio integration
- **File Storage**: Cloud storage with CDN
- **Monitoring**: Application performance monitoring

### Infrastructure
- **Database**: PostgreSQL with connection pooling
- **Caching**: Redis for session and response caching
- **Containerization**: Docker with multi-stage builds
- **Load Balancing**: Nginx reverse proxy
- **SSL/TLS**: Automatic HTTPS with Let's Encrypt
- **Monitoring**: Structured logging and health checks


## 🚀 Quick Start

### Prerequisites

- **Node.js**: 18+ (for frontend)
- **Python**: 3.12+ (for backend)
- **PostgreSQL**: 15+ (database)
- **Firebase Project**: With phone authentication
- **Google AI**: Gemini API key

### 1. Clone Repository

```bash
git clone https://github.com/Neural-Knight/SmartKrishi.git
cd SmartKrishi
```

### 2. Backend Setup

```bash
cd backend

# Create virtual environment
python -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate

# Install dependencies
pip install -r requirements.txt

# Setup environment
cp .env.example .env
# Edit .env with your configuration

# Run database migrations
alembic upgrade head

# Start backend server
uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
```

### 3. Frontend Setup

```bash
cd frontend

# Install dependencies
pnpm install  # or npm install

# Setup environment
cp .env.example .env
# Edit .env with your configuration

# Start development server
pnpm dev  # or npm run dev
```

### 4. Access Application

- **Frontend**: http://localhost:5173
- **Backend API**: http://localhost:8000
- **API Documentation**: http://localhost:8000/docs

## 🔧 Configuration

### Environment Variables

#### Backend (.env)
```bash
DATABASE_URL=postgresql://user:pass@localhost:5432/smartkrishi
SECRET_KEY=your-jwt-secret-key
GEMINI_API_KEY=your-google-gemini-api-key
FIREBASE_CREDENTIALS=/path/to/serviceAccountKey.json
```

#### Frontend (.env)
```bash
VITE_API_BASE_URL=http://localhost:8000
VITE_FIREBASE_API_KEY=your-firebase-api-key
VITE_FIREBASE_PROJECT_ID=your-project-id
```

### Database Setup

```bash
# Create PostgreSQL database
createdb smartkrishi

# Run migrations
cd backend
alembic upgrade head

# Optional: Add sample data
python -c "from app.db.init_db import init_db; init_db()"
```

## 📱 Features Deep Dive

### AI-Powered Agricultural Assistant

- **Crop Management**: Personalized advice for different crops and seasons
- **Disease Identification**: Photo-based plant disease diagnosis
- **Weather Integration**: Weather-aware farming recommendations
- **Soil Analysis**: Soil report interpretation and improvement suggestions
- **Pest Control**: Integrated pest management strategies
- **Market Insights**: Crop pricing and market trend analysis

### Multi-Channel Access

- **Web Application**: Full-featured browser-based interface
- **Mobile Web**: Responsive design optimized for smartphones
- **SMS Fallback**: Text-message based interaction for offline areas
- **WhatsApp Bot**: Integration with popular messaging platform
- **Voice Input**: Speech-to-text for hands-free operation

### Advanced AI Features

- **Reasoning Visualization**: Step-by-step AI decision process
- **Context Awareness**: Remembers previous conversations and farm details
- **Multi-modal Input**: Text, images, documents, and voice
- **Real-time Streaming**: Instant responses with typing indicators
- **Confidence Scoring**: AI confidence levels for recommendations

## 🧪 Testing

### Backend Tests

```bash
cd backend

# Run unit tests
pytest

# Run with coverage
pytest --cov=app --cov-report=html

# Run integration tests
pytest tests/test_integrated/
```

### Frontend Tests

```bash
cd frontend

# Run unit tests
pnpm test

# Run E2E tests
pnpm test:e2e

# Type checking
pnpm type-check
```

## 🚀 Deployment

### Production Deployment Options

#### 1. Cloud Platform Deployment

**Render** (Recommended):
- Connect GitHub repository
- Automatic deployments on push
- Managed PostgreSQL database
- SSL certificates included

**Railway**:
- One-click deployment
- Database provisioning
- Custom domain support

**Vercel + Render**:
- Frontend on Vercel
- Backend on Render
- Optimal performance

#### 2. Docker Deployment

```bash
# Build and run with Docker Compose
docker-compose up -d --build

# Scale services
docker-compose up -d --scale backend=3

# Production build
docker-compose -f docker-compose.prod.yml up -d
```

#### 3. Traditional VPS Deployment

```bash
# Setup reverse proxy (Nginx)
# Configure SSL (Let's Encrypt)
# Process management (PM2/Systemd)
# Database backup automation
```

### Environment-Specific Configuration

#### Development
- Local PostgreSQL database
- Hot module replacement
- Detailed error messages
- Debug logging enabled

#### Staging
- Cloud database connection
- Production-like environment
- Integration testing
- Performance monitoring

#### Production
- Optimized builds
- Error tracking
- Performance monitoring
- Automated backups
- CDN for static assets

## 🔒 Security Features

- **Authentication**: Multi-factor authentication with phone verification
- **Authorization**: Role-based access control
- **Data Protection**: Encryption at rest and in transit
- **Input Validation**: Comprehensive request validation
- **Rate Limiting**: API abuse prevention
- **CORS Configuration**: Secure cross-origin requests
- **SQL Injection Prevention**: Parameterized queries
- **XSS Protection**: Content Security Policy headers

## 📊 Monitoring & Analytics

- **Application Performance**: Response time and error tracking
- **User Analytics**: Usage patterns and feature adoption
- **Health Monitoring**: Service uptime and availability
- **Error Tracking**: Automatic error capture and alerts
- **Database Performance**: Query optimization and monitoring
- **Resource Utilization**: CPU, memory, and storage monitoring

## 🤝 Contributing

We welcome contributions! Please see our contributing guidelines:

1. **Fork the repository**
2. **Create feature branch** (`git checkout -b feature/amazing-feature`)
3. **Follow code style** guidelines
4. **Add tests** for new features
5. **Update documentation**
6. **Commit changes** (`git commit -m 'Add amazing feature'`)
7. **Push to branch** (`git push origin feature/amazing-feature`)
8. **Open Pull Request**
