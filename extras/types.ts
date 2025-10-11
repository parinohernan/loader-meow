// Tipos para la base de datos Supabase
export interface DatabaseMessage {
  id?: number;
  telegram_message_id: number;
  user_id: number;
  username?: string | undefined;
  first_name?: string | undefined;
  last_name?: string | undefined;
  text: string;
  chat_id: number;
  chat_type: string;
  timestamp: string;
  created_at?: string;
  updated_at?: string;
}

export interface DatabaseAnalysis {
  id?: number;
  message_id: number; // FK a messages
  telegram_message_id: number;
  user_id: number;
  original_text: string;
  sentiment: string;
  confidence: number;
  emotions: string[];
  topics: string[];
  keywords: string[];
  summary: string;
  category: string;
  language: string;
  model_used: string;
  processing_time: number;
  analysis_timestamp: string;
  created_at?: string;
  updated_at?: string;
}

export interface SupabaseConfig {
  url: string | undefined;
  anonKey: string | undefined;
  serviceRoleKey?: string | undefined;
}

export interface DatabaseTables {
  messages: string;
  analysis: string;
}

// Tipos para consultas
export interface MessageQuery {
  userId?: number;
  chatId?: number;
  dateFrom?: Date;
  dateTo?: Date;
  limit?: number;
  offset?: number;
}

export interface AnalysisQuery {
  userId?: number;
  sentiment?: string;
  category?: string;
  dateFrom?: Date;
  dateTo?: Date;
  limit?: number;
  offset?: number;
}

// Tipos para estadísticas
export interface UserStats {
  userId: number;
  username?: string | undefined;
  totalMessages: number;
  sentimentDistribution: {
    positive: number;
    negative: number;
    neutral: number;
  };
  topCategories: Array<{
    category: string;
    count: number;
  }>;
  topEmotions: Array<{
    emotion: string;
    count: number;
  }>;
  avgConfidence: number;
  firstMessage: string;
  lastMessage: string;
}
