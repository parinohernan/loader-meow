import { createClient, SupabaseClient } from '@supabase/supabase-js';
import { logger } from '../../utils/logger';
import { SupabaseError } from '../../utils/errors';
import { 
  DatabaseMessage, 
  DatabaseAnalysis, 
  SupabaseConfig,
  DatabaseTables,
  MessageQuery,
  AnalysisQuery,
  UserStats
} from './types';
import { TelegramMessage } from '../telegram/types';
import { AnalysisResult } from '../ai-analyzer/types';

export class SupabaseService {
  private client: SupabaseClient;
  private tables: DatabaseTables;

  constructor(config: SupabaseConfig, tables: DatabaseTables) {
    // Verificar que la configuración esté completa
    if (!config.url || !config.anonKey) {
      throw new Error('Supabase configuration is incomplete');
    }
    
    // Usar service role key si está disponible, sino usar anon key
    const key = config.serviceRoleKey || config.anonKey;
    
    this.client = createClient(config.url, key);
    this.tables = tables;
  }

  // Guardar mensaje original
  public async saveMessage(message: TelegramMessage): Promise<number> {
    try {
      const dbMessage: Omit<DatabaseMessage, 'id' | 'created_at' | 'updated_at'> = {
        telegram_message_id: message.messageId,
        user_id: message.userId,
        username: message.username || undefined,
        first_name: message.firstName || undefined,
        last_name: message.lastName || undefined,
        text: message.text,
        chat_id: message.chatId,
        chat_type: message.chatType,
        timestamp: message.timestamp.toISOString()
      };

      const { data, error } = await this.client
        .from(this.tables.messages)
        .insert(dbMessage)
        .select('id')
        .single();

      if (error) {
        throw new SupabaseError(`Failed to save message: ${error.message}`);
      }

      if (!data?.id) {
        throw new SupabaseError('No ID returned from message insert');
      }

      logger.info(`Message saved to database with ID: ${data.id}`);
      return data.id;

    } catch (error) {
      logger.error('Error saving message to database:', error);
      if (error instanceof SupabaseError) {
        throw error;
      }
      throw new SupabaseError(`Database error: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  // Guardar análisis (ahora datos extraídos de cargas)
  public async saveAnalysis(analysis: AnalysisResult, messageDbId: number): Promise<number> {
    try {
      // Crear un resumen de los datos extraídos
      const cargas = analysis.extractedData.cargas;
      const resumen = cargas.length > 0 
        ? `Se extrajeron ${cargas.length} carga(s): ${cargas.map(c => `${c.material} de ${c.localidadCarga} a ${c.localidadDescarga}`).join(', ')}`
        : 'No se encontraron datos de carga válidos';

      const dbAnalysis: Omit<DatabaseAnalysis, 'id' | 'created_at' | 'updated_at'> = {
        message_id: messageDbId,
        telegram_message_id: analysis.messageId,
        user_id: analysis.userId,
        original_text: analysis.originalText,
        sentiment: analysis.extractedData.extractionSuccess ? 'positive' : 'negative',
        confidence: analysis.extractedData.confidence,
        emotions: analysis.extractedData.extractionSuccess ? ['satisfacción'] : ['confusión'],
        topics: cargas.map(c => c.material),
        keywords: cargas.flatMap(c => [c.material, c.presentacion, c.tipoEquipo, c.localidadCarga, c.localidadDescarga]),
        summary: resumen,
        category: 'carga_transporte',
        language: 'es',
        model_used: analysis.metadata.modelUsed,
        processing_time: analysis.metadata.processingTime,
        analysis_timestamp: analysis.metadata.timestamp.toISOString()
      };

      const { data, error } = await this.client
        .from(this.tables.analysis)
        .insert(dbAnalysis)
        .select('id')
        .single();

      if (error) {
        throw new SupabaseError(`Failed to save analysis: ${error.message}`);
      }

      if (!data?.id) {
        throw new SupabaseError('No ID returned from analysis insert');
      }

      logger.info(`Analysis saved to database with ID: ${data.id}`);
      return data.id;

    } catch (error) {
      logger.error('Error saving analysis to database:', error);
      if (error instanceof SupabaseError) {
        throw error;
      }
      throw new SupabaseError(`Database error: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  // Obtener mensajes
  public async getMessages(query: MessageQuery = {}): Promise<DatabaseMessage[]> {
    try {
      let supabaseQuery = this.client
        .from(this.tables.messages)
        .select('*');

      if (query.userId) {
        supabaseQuery = supabaseQuery.eq('user_id', query.userId);
      }

      if (query.chatId) {
        supabaseQuery = supabaseQuery.eq('chat_id', query.chatId);
      }

      if (query.dateFrom) {
        supabaseQuery = supabaseQuery.gte('timestamp', query.dateFrom.toISOString());
      }

      if (query.dateTo) {
        supabaseQuery = supabaseQuery.lte('timestamp', query.dateTo.toISOString());
      }

      if (query.limit) {
        supabaseQuery = supabaseQuery.limit(query.limit);
      }

      if (query.offset) {
        supabaseQuery = supabaseQuery.range(query.offset, query.offset + (query.limit || 10) - 1);
      }

      supabaseQuery = supabaseQuery.order('timestamp', { ascending: false });

      const { data, error } = await supabaseQuery;

      if (error) {
        throw new SupabaseError(`Failed to fetch messages: ${error.message}`);
      }

      return data || [];

    } catch (error) {
      logger.error('Error fetching messages:', error);
      if (error instanceof SupabaseError) {
        throw error;
      }
      throw new SupabaseError(`Database error: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  // Obtener análisis
  public async getAnalyses(query: AnalysisQuery = {}): Promise<DatabaseAnalysis[]> {
    try {
      let supabaseQuery = this.client
        .from(this.tables.analysis)
        .select('*');

      if (query.userId) {
        supabaseQuery = supabaseQuery.eq('user_id', query.userId);
      }

      if (query.sentiment) {
        supabaseQuery = supabaseQuery.eq('sentiment', query.sentiment);
      }

      if (query.category) {
        supabaseQuery = supabaseQuery.eq('category', query.category);
      }

      if (query.dateFrom) {
        supabaseQuery = supabaseQuery.gte('analysis_timestamp', query.dateFrom.toISOString());
      }

      if (query.dateTo) {
        supabaseQuery = supabaseQuery.lte('analysis_timestamp', query.dateTo.toISOString());
      }

      if (query.limit) {
        supabaseQuery = supabaseQuery.limit(query.limit);
      }

      if (query.offset) {
        supabaseQuery = supabaseQuery.range(query.offset, query.offset + (query.limit || 10) - 1);
      }

      supabaseQuery = supabaseQuery.order('analysis_timestamp', { ascending: false });

      const { data, error } = await supabaseQuery;

      if (error) {
        throw new SupabaseError(`Failed to fetch analyses: ${error.message}`);
      }

      return data || [];

    } catch (error) {
      logger.error('Error fetching analyses:', error);
      if (error instanceof SupabaseError) {
        throw error;
      }
      throw new SupabaseError(`Database error: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  // Obtener estadísticas de usuario
  public async getUserStats(userId: number): Promise<UserStats | null> {
    try {
      // Esta consulta es más compleja, podríamos usar una función de Supabase
      // Por ahora, haremos múltiples consultas
      
      const messages = await this.getMessages({ userId });
      const analyses = await this.getAnalyses({ userId });

      if (messages.length === 0) {
        return null;
      }

      // Calcular estadísticas
      const sentimentCounts = analyses.reduce((acc, analysis) => {
        acc[analysis.sentiment] = (acc[analysis.sentiment] || 0) + 1;
        return acc;
      }, {} as Record<string, number>);

      const categoryStats = analyses.reduce((acc, analysis) => {
        const existing = acc.find(item => item.category === analysis.category);
        if (existing) {
          existing.count++;
        } else {
          acc.push({ category: analysis.category, count: 1 });
        }
        return acc;
      }, [] as Array<{ category: string; count: number }>);

      const emotionStats = analyses.reduce((acc, analysis) => {
        analysis.emotions.forEach(emotion => {
          const existing = acc.find(item => item.emotion === emotion);
          if (existing) {
            existing.count++;
          } else {
            acc.push({ emotion, count: 1 });
          }
        });
        return acc;
      }, [] as Array<{ emotion: string; count: number }>);

      const avgConfidence = analyses.length > 0 
        ? analyses.reduce((sum, a) => sum + a.confidence, 0) / analyses.length 
        : 0;

      return {
        userId,
        username: messages[0]?.username || undefined,
        totalMessages: messages.length,
        sentimentDistribution: {
          positive: sentimentCounts.positive || 0,
          negative: sentimentCounts.negative || 0,
          neutral: sentimentCounts.neutral || 0
        },
        topCategories: categoryStats.sort((a, b) => b.count - a.count).slice(0, 5),
        topEmotions: emotionStats.sort((a, b) => b.count - a.count).slice(0, 5),
        avgConfidence,
        firstMessage: messages[messages.length - 1]?.timestamp || '',
        lastMessage: messages[0]?.timestamp || ''
      };

    } catch (error) {
      logger.error('Error getting user stats:', error);
      throw new SupabaseError(`Failed to get user stats: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  // Probar conexión
  public async testConnection(): Promise<boolean> {
    try {
      const { error } = await this.client
        .from(this.tables.messages)
        .select('count', { count: 'exact', head: true });

      return !error;
    } catch (error) {
      logger.error('Supabase connection test failed:', error);
      return false;
    }
  }
}

export * from './types';
