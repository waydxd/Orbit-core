-- Migration: 002_chat_and_pending_actions.sql
-- Description: Create chat conversations, messages, and pending actions schema

-- Create conversations table
CREATE TABLE IF NOT EXISTS conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    correlation_id UUID NOT NULL UNIQUE,
    status VARCHAR(20) CHECK (status IN ('active', 'closed', 'archived')) DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_conversations_user_id ON conversations(user_id);
CREATE INDEX IF NOT EXISTS idx_conversations_correlation_id ON conversations(correlation_id);
CREATE INDEX IF NOT EXISTS idx_conversations_status ON conversations(status);

-- Create chat_messages table
CREATE TABLE IF NOT EXISTS chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) CHECK (role IN ('user', 'assistant', 'system')) NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_conversation_id ON chat_messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_user_id ON chat_messages(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_created_at ON chat_messages(created_at);

-- Create pending_actions table
CREATE TABLE IF NOT EXISTS pending_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    action_id VARCHAR(255) UNIQUE NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    proposed_action JSONB NOT NULL,
    action_type VARCHAR(100) NOT NULL,
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,
    status VARCHAR(20) CHECK (status IN ('pending', 'confirmed', 'cancelled', 'expired', 'failed')) DEFAULT 'pending',
    version INT DEFAULT 1,
    correlation_id UUID NOT NULL,
    agent_metadata JSONB,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_pending_actions_action_id ON pending_actions(action_id);
CREATE INDEX IF NOT EXISTS idx_pending_actions_user_id ON pending_actions(user_id);
CREATE INDEX IF NOT EXISTS idx_pending_actions_conversation_id ON pending_actions(conversation_id);
CREATE INDEX IF NOT EXISTS idx_pending_actions_status ON pending_actions(status);
CREATE INDEX IF NOT EXISTS idx_pending_actions_expires_at ON pending_actions(expires_at);
CREATE INDEX IF NOT EXISTS idx_pending_actions_idempotency_key ON pending_actions(idempotency_key);

-- Create agent_tool_logs table for audit
CREATE TABLE IF NOT EXISTS agent_tool_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pending_action_id UUID REFERENCES pending_actions(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tool_name VARCHAR(100) NOT NULL,
    tool_input JSONB NOT NULL,
    tool_output JSONB,
    status VARCHAR(20) CHECK (status IN ('started', 'success', 'failed')) NOT NULL,
    error_message TEXT,
    correlation_id UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agent_tool_logs_pending_action_id ON agent_tool_logs(pending_action_id);
CREATE INDEX IF NOT EXISTS idx_agent_tool_logs_conversation_id ON agent_tool_logs(conversation_id);
CREATE INDEX IF NOT EXISTS idx_agent_tool_logs_user_id ON agent_tool_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_agent_tool_logs_created_at ON agent_tool_logs(created_at);

-- Create triggers for updated_at columns
DROP TRIGGER IF EXISTS update_conversations_updated_at ON conversations;
CREATE TRIGGER update_conversations_updated_at BEFORE UPDATE ON conversations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_pending_actions_updated_at ON pending_actions;
CREATE TRIGGER update_pending_actions_updated_at BEFORE UPDATE ON pending_actions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
