from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from decouple import config
import os
import logging
from sqlalchemy import text
from dotenv import load_dotenv

# Load environment variables from .env file
load_dotenv()

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

from app.db.database import engine, Base
from app.models import user, chat as chat_models, reasoning, fallback as fallback_models, file  # Import ALL models to create tables
from app.routers import auth, mobile_auth, chat, fallback  # Add fallback import

# Create FastAPI instance
app = FastAPI(
    title="SmartKrishi API",
    description="AI-powered farming assistant API",
    version="1.0.0",
)

# Production CORS settings - Include your Vercel URL
origins = [
    "http://localhost:3000",
    "http://localhost:5173", 
    "http://127.0.0.1:3000", 
    "http://127.0.0.1:5173",
    "https://smart-krishi-nine.vercel.app",  # Your Vercel frontend URL
    "https://smart-krishi-website.vercel.app",
    "https://*.vercel.app",  # For Vercel preview deployments
]

# Add environment-based frontend URL
frontend_url = os.getenv("FRONTEND_URL")
if frontend_url and frontend_url not in origins:
    origins.append(frontend_url)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=origins,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# API version prefix
API_V1_STR = config("API_V1_STR", default="/api/v1")

# Create database tables safely at startup
@app.on_event("startup")
async def startup_event():
    try:
        # Create tables
        Base.metadata.create_all(bind=engine)
        logger.info("✅ Database tables created successfully")
    except Exception as e:
        logger.error(f"❌ Database connection failed: {e}")
        # Don't crash the app, just log the error

# Include routers
app.include_router(auth.router, prefix=f"{API_V1_STR}/auth", tags=["authentication"])
app.include_router(mobile_auth.router, prefix=f"{API_V1_STR}/auth", tags=["mobile-authentication"])
app.include_router(chat.router, prefix=f"{API_V1_STR}/chat", tags=["chat"])  # Add chat router
app.include_router(fallback.router, prefix=f"{API_V1_STR}/fallback", tags=["fallback"])  # Add fallback router

@app.get("/")
def read_root():
    """Root endpoint."""
    return {
        "message": "Welcome to SmartKrishi API",
        "version": "1.0.0",
        "docs": "/docs",
        "redoc": "/redoc"
    }


@app.get("/health")
def health_check():
    """Health check endpoint with database connection test."""
    try:
        # Test database connection
        with engine.connect() as conn:
            conn.execute(text("SELECT 1"))
        return {
            "status": "healthy", 
            "service": "SmartKrishi API",
            "database": "connected",
            "version": "1.0.0"
        }
    except Exception as e:
        logger.error(f"Health check failed: {e}")
        return {
            "status": "unhealthy", 
            "service": "SmartKrishi API",
            "database": "disconnected",
            "error": str(e),
            "version": "1.0.0"
        }


if __name__ == "__main__":
    import uvicorn
    uvicorn.run("main:app", host="0.0.0.0", port=8000, reload=True)
