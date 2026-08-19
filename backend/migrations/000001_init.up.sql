CREATE TYPE auth_provider AS ENUM ('email', 'mobile');

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR NOT NULL,
    email VARCHAR UNIQUE,
    phone_number VARCHAR UNIQUE,
    hashed_password VARCHAR,
    auth_provider auth_provider NOT NULL DEFAULT 'email',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ,
    auto_fallback_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    fallback_mode VARCHAR(20) NOT NULL DEFAULT 'manual',
    fallback_active BOOLEAN NOT NULL DEFAULT FALSE,
    fallback_phone VARCHAR,
    fallback_phone_verified BOOLEAN NOT NULL DEFAULT FALSE,
    whatsapp_user_id VARCHAR UNIQUE
);

CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_users_phone_number ON users (phone_number);

CREATE TABLE chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id INTEGER NOT NULL REFERENCES users(id),
    title VARCHAR(255) NOT NULL,
    agent_chat_id VARCHAR(255),
    is_fallback_chat BOOLEAN NOT NULL DEFAULT FALSE,
    fallback_phone_number VARCHAR(20),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id),
    role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    message_type VARCHAR(20) NOT NULL DEFAULT 'text',
    file_url VARCHAR(500),
    is_edited BOOLEAN NOT NULL DEFAULT FALSE,
    original_content TEXT,
    fallback_type VARCHAR(20),
    fallback_phone_number VARCHAR(20),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    edited_at TIMESTAMPTZ
);

CREATE TABLE uploaded_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id INTEGER NOT NULL REFERENCES users(id),
    chat_id UUID NOT NULL REFERENCES chats(id),
    message_id UUID REFERENCES chat_messages(id),
    original_filename VARCHAR(255) NOT NULL,
    file_type VARCHAR(50) NOT NULL,
    file_size BIGINT NOT NULL,
    mime_type VARCHAR(100),
    agent_file_id VARCHAR(255),
    processing_status VARCHAR(50) NOT NULL DEFAULT 'uploaded',
    summary TEXT,
    file_metadata TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE reasoning_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
    chat_id UUID NOT NULL REFERENCES chats(id),
    user_id INTEGER NOT NULL REFERENCES users(id),
    step_type VARCHAR(50) NOT NULL,
    step_order INTEGER NOT NULL,
    stage VARCHAR(50),
    content TEXT,
    tool_name VARCHAR(100),
    tool_args TEXT,
    tool_result JSONB,
    step_metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE agent_api_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id INTEGER NOT NULL UNIQUE REFERENCES users(id),
    preferred_model VARCHAR(100) NOT NULL DEFAULT 'gemini-2.5-flash',
    default_tools JSONB,
    include_logs BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE fallback_sessions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    chat_id UUID REFERENCES chats(id),
    phone_number VARCHAR(20) NOT NULL,
    fallback_type VARCHAR(20) NOT NULL DEFAULT 'sms',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    activation_trigger VARCHAR(20) NOT NULL,
    activated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deactivated_at TIMESTAMPTZ
);

CREATE TABLE fallback_messages (
    id SERIAL PRIMARY KEY,
    session_id INTEGER NOT NULL REFERENCES fallback_sessions(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id),
    phone_number VARCHAR(20) NOT NULL,
    message_type VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    fallback_type VARCHAR(20) NOT NULL DEFAULT 'sms',
    sms_id VARCHAR,
    is_delivered BOOLEAN NOT NULL DEFAULT FALSE,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
