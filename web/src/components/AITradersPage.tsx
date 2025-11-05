import React, { useState, useEffect } from 'react';
import useSWR from 'swr';
import { api } from '../lib/api';
import type { TraderInfo, CreateTraderRequest, AIModel, Exchange } from '../types';
import { useLanguage } from '../contexts/LanguageContext';
import { t } from '../i18n/translations';
import { getExchangeIcon } from './ExchangeIcons';
import { getModelIcon } from './ModelIcons';

// 获取友好的AI模型名称
function getModelDisplayName(modelId: string): string {
  switch (modelId.toLowerCase()) {
    case 'deepseek':
      return 'DeepSeek';
    case 'qwen':
      return 'Qwen';
    case 'claude':
      return 'Claude';
    case 'gpt4':
    case 'gpt-4':
      return 'GPT-4';
    case 'gpt3.5':
    case 'gpt-3.5':
      return 'GPT-3.5';
    default:
      return modelId.toUpperCase();
  }
}

interface AITradersPageProps {
  onTraderSelect?: (traderId: string) => void;
}

export function AITradersPage({ onTraderSelect }: AITradersPageProps) {
  const { language } = useLanguage();
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showModelModal, setShowModelModal] = useState(false);
  const [showExchangeModal, setShowExchangeModal] = useState(false);
  const [editingModel, setEditingModel] = useState<string | null>(null);
  const [editingExchange, setEditingExchange] = useState<string | null>(null);
  const [allModels, setAllModels] = useState<AIModel[]>([]);
  const [allExchanges, setAllExchanges] = useState<Exchange[]>([]);
  const [supportedModels, setSupportedModels] = useState<AIModel[]>([]);
  const [supportedExchanges, setSupportedExchanges] = useState<Exchange[]>([]);

  const { data: traders, mutate: mutateTraders } = useSWR<TraderInfo[]>(
    'traders',
    api.getTraders,
    { refreshInterval: 5000 }
  );

  // 加载AI模型和交易所配置
  useEffect(() => {
    const loadConfigs = async () => {
      try {
        console.log('🔄 开始加载模型和交易所配置...');
        const [modelConfigs, exchangeConfigs, supportedModels, supportedExchanges] = await Promise.all([
          api.getModelConfigs(),
          api.getExchangeConfigs(),
          api.getSupportedModels(),
          api.getSupportedExchanges()
        ]);
        console.log('✅ 用户模型配置加载成功:', modelConfigs);
        console.log('✅ 用户交易所配置加载成功:', exchangeConfigs);
        console.log('✅ 支持的模型加载成功:', supportedModels);
        console.log('✅ 支持的交易所加载成功:', supportedExchanges);
        setAllModels(modelConfigs);
        setAllExchanges(exchangeConfigs);
        setSupportedModels(supportedModels);
        setSupportedExchanges(supportedExchanges);
      } catch (error) {
        console.error('❌ 加载配置失败:', error);
      }
    };
    loadConfigs();
  }, []);

  // 显示所有用户的模型和交易所配置（用于调试）
  const configuredModels = allModels || [];
  const configuredExchanges = allExchanges || [];
  
  // 只在创建交易员时使用已启用且配置完整的
  const enabledModels = allModels?.filter(m => m.enabled && m.apiKey) || [];
  const enabledExchanges = allExchanges?.filter(e => {
    if (!e.enabled || !e.apiKey) return false;
    // Hyperliquid 只需要私钥（作为apiKey），不需要secretKey
    if (e.id === 'hyperliquid') return true;
    // 其他交易所需要secretKey
    return e.secretKey && e.secretKey.trim() !== '';
  }) || [];

  // 检查模型是否正在被运行中的交易员使用
  const isModelInUse = (modelId: string) => {
    return traders?.some(t => t.ai_model_unique_id === modelId && t.is_running) || false;
  };

  // 检查交易所是否正在被运行中的交易员使用
  const isExchangeInUse = (exchangeId: string) => {
    return traders?.some(t => t.exchange_unique_id === exchangeId && t.is_running) || false;
  };

  // 根据unique_id获取模型信息
  const getModelByUniqueId = (uniqueId: string): AIModel | undefined => {
    return allModels?.find(m => m.unique_id === uniqueId);
  };

  // 根据unique_id获取交易所信息
  const getExchangeByUniqueId = (uniqueId: string): Exchange | undefined => {
    return allExchanges?.find(e => e.unique_id === uniqueId);
  };

  const handleCreateTrader = async (modelId: string, exchangeId: string, name: string, initialBalance: number, customPrompt?: string, overrideBase?: boolean) => {
    try {
      const model = allModels?.find(m => m.id === modelId);
      const exchange = allExchanges?.find(e => e.id === exchangeId);
      
      if (!model?.enabled) {
        alert(t('modelNotConfigured', language));
        return;
      }
      
      if (!exchange?.enabled) {
        alert(t('exchangeNotConfigured', language));
        return;
      }
      
      const request: CreateTraderRequest = {
        name,
        ai_model_unique_id: model?.unique_id || '',
        exchange_unique_id: exchange?.unique_id || '',
        initial_balance: initialBalance,
        custom_prompt: customPrompt,
        override_base_prompt: overrideBase
      };
      
      await api.createTrader(request);
      setShowCreateModal(false);
      mutateTraders();
    } catch (error) {
      console.error('Failed to create trader:', error);
      alert('创建交易员失败');
    }
  };

  const handleDeleteTrader = async (traderId: string) => {
    if (!confirm(t('confirmDeleteTrader', language))) return;
    
    try {
      await api.deleteTrader(traderId);
      mutateTraders();
    } catch (error) {
      console.error('Failed to delete trader:', error);
      alert('删除交易员失败');
    }
  };

  const handleToggleTrader = async (traderId: string, running: boolean) => {
    try {
      if (running) {
        await api.stopTrader(traderId);
      } else {
        await api.startTrader(traderId);
      }
      mutateTraders();
    } catch (error) {
      console.error('Failed to toggle trader:', error);
      alert('操作失败');
    }
  };

  const handleModelClick = (modelId: string) => {
    if (!isModelInUse(modelId)) {
      setEditingModel(modelId);
      setShowModelModal(true);
    }
  };

  const handleExchangeClick = (exchangeId: string) => {
    if (!isExchangeInUse(exchangeId)) {
      setEditingExchange(exchangeId);
      setShowExchangeModal(true);
    }
  };

  const handleDeleteModelConfig = async (modelId: string) => {
    if (!confirm('确定要删除此AI模型配置吗？')) return;

    try {
      const modelToDelete = allModels?.find(m => m.id === modelId);
      if (!modelToDelete?.unique_id) {
        alert('无法找到模型配置');
        return;
      }

      await api.deleteModelConfig(modelToDelete.unique_id);

      // 重新获取模型列表
      const refreshedModels = await api.getModelConfigs();
      setAllModels(refreshedModels);
      setShowModelModal(false);
      setEditingModel(null);
    } catch (error) {
      console.error('Failed to delete model config:', error);
      alert('删除配置失败');
    }
  };

  const handleSaveModelConfig = async (modelId: string, apiKey: string) => {
    try {
      // 查找现有配置
      const existingModel = allModels?.find(m => m.id === modelId);

      if (existingModel?.unique_id) {
        // 更新现有配置
        await api.updateModelConfig({
          unique_id: existingModel.unique_id,
          api_key: apiKey,
          enabled: true
        });
      } else {
        // 创建新配置
        await api.createModelConfig({
          model_id: modelId,
          api_key: apiKey
        });
      }

      // 重新获取用户配置以确保数据同步
      const refreshedModels = await api.getModelConfigs();
      setAllModels(refreshedModels);

      setShowModelModal(false);
      setEditingModel(null);
    } catch (error) {
      console.error('Failed to save model config:', error);
      alert('保存配置失败');
    }
  };

  const handleDeleteExchangeConfig = async (exchangeId: string) => {
    if (!confirm('确定要删除此交易所配置吗？')) return;

    try {
      const exchangeToDelete = allExchanges?.find(e => e.id === exchangeId);
      if (!exchangeToDelete?.unique_id) {
        alert('无法找到交易所配置');
        return;
      }

      await api.deleteExchangeConfig(exchangeToDelete.unique_id);

      // 重新获取交易所列表
      const refreshedExchanges = await api.getExchangeConfigs();
      setAllExchanges(refreshedExchanges);
      setShowExchangeModal(false);
      setEditingExchange(null);
    } catch (error) {
      console.error('Failed to delete exchange config:', error);
      alert('删除交易所配置失败');
    }
  };

  const handleSaveExchangeConfig = async (exchangeId: string, apiKey: string, secretKey?: string, testnet?: boolean, hyperliquidWalletAddr?: string, asterUser?: string, asterSigner?: string, asterPrivateKey?: string) => {
    try {
      // 查找现有配置
      const existingExchange = allExchanges?.find(e => e.id === exchangeId);

      if (existingExchange?.unique_id) {
        // 更新现有配置
        await api.updateExchangeConfig({
          unique_id: existingExchange.unique_id,
          api_key: apiKey,
          secret_key: secretKey,
          testnet: testnet,
          hyperliquid_wallet_addr: hyperliquidWalletAddr,
          aster_user: asterUser,
          aster_signer: asterSigner,
          aster_private_key: asterPrivateKey,
          enabled: true
        });
      } else {
        // 创建新配置
        await api.createExchangeConfig({
          exchange_id: exchangeId,
          api_key: apiKey,
          secret_key: secretKey,
          testnet: testnet,
          hyperliquid_wallet_addr: hyperliquidWalletAddr,
          aster_user: asterUser,
          aster_signer: asterSigner,
          aster_private_key: asterPrivateKey
        });
      }

      // 重新获取用户配置以确保数据同步
      const refreshedExchanges = await api.getExchangeConfigs();
      setAllExchanges(refreshedExchanges);

      setShowExchangeModal(false);
      setEditingExchange(null);
    } catch (error) {
      console.error('Failed to save exchange config:', error);
      alert('保存交易所配置失败');
    }
  };

  const handleAddModel = () => {
    setEditingModel(null);
    setShowModelModal(true);
  };

  const handleAddExchange = () => {
    setEditingExchange(null);
    setShowExchangeModal(true);
  };

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <div className="w-12 h-12 rounded-xl flex items-center justify-center text-2xl" style={{
            background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
            boxShadow: '0 4px 14px rgba(240, 185, 11, 0.4)'
          }}>
            🤖
          </div>
          <div>
            <h1 className="text-2xl font-bold flex items-center gap-2" style={{ color: '#EAECEF' }}>
              {t('aiTraders', language)}
              <span className="text-xs font-normal px-2 py-1 rounded" style={{ 
                background: 'rgba(240, 185, 11, 0.15)', 
                color: '#F0B90B' 
              }}>
                {traders?.length || 0} {t('active', language)}
              </span>
            </h1>
            <p className="text-xs" style={{ color: '#848E9C' }}>
              {t('manageAITraders', language)}
            </p>
          </div>
        </div>
        
        <div className="flex gap-3">
          <button
            onClick={handleAddModel}
            className="px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
            style={{ 
              background: '#2B3139', 
              color: '#EAECEF', 
              border: '1px solid #474D57' 
            }}
          >
            ➕ {t('aiModels', language)}
          </button>
          
          <button
            onClick={handleAddExchange}
            className="px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
            style={{ 
              background: '#2B3139', 
              color: '#EAECEF', 
              border: '1px solid #474D57' 
            }}
          >
            ➕ {t('exchanges', language)}
          </button>
          
          <button
            onClick={() => setShowCreateModal(true)}
            disabled={configuredModels.length === 0 || configuredExchanges.length === 0}
            className="px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
            style={{ 
              background: (configuredModels.length > 0 && configuredExchanges.length > 0) ? '#F0B90B' : '#2B3139', 
              color: (configuredModels.length > 0 && configuredExchanges.length > 0) ? '#000' : '#848E9C' 
            }}
          >
            ➕ {t('createTrader', language)}
          </button>
        </div>
      </div>

      {/* Configuration Status */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* AI Models */}
        <div className="binance-card p-4">
          <h3 className="text-lg font-semibold mb-3" style={{ color: '#EAECEF' }}>
            🧠 {t('aiModels', language)}
          </h3>
          <div className="space-y-3">
            {configuredModels.map(model => {
              const inUse = isModelInUse(model.id);
              return (
                <div 
                  key={model.id} 
                  className={`flex items-center justify-between p-3 rounded transition-all ${
                    inUse ? 'cursor-not-allowed' : 'cursor-pointer hover:bg-gray-700'
                  }`}
                  style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
                  onClick={() => handleModelClick(model.id)}
                >
                  <div className="flex items-center gap-3">
                    <div className="w-8 h-8 flex items-center justify-center">
                      {getModelIcon(model.provider || model.id, { width: 32, height: 32 }) || (
                        <div className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold"
                             style={{ 
                               background: model.id === 'deepseek' ? '#60a5fa' : '#c084fc',
                               color: '#fff'
                             }}>
                          {model.name[0]}
                        </div>
                      )}
                    </div>
                    <div>
                      <div className="font-semibold" style={{ color: '#EAECEF' }}>{model.name}</div>
                      <div className="text-xs" style={{ color: '#848E9C' }}>
                        {inUse ? '正在使用' : model.enabled ? '已启用' : '已配置'}
                      </div>
                    </div>
                  </div>
                  <div className={`w-3 h-3 rounded-full ${model.enabled && model.apiKey ? 'bg-green-400' : 'bg-gray-500'}`} />
                </div>
              );
            })}
            {configuredModels.length === 0 && (
              <div className="text-center py-8" style={{ color: '#848E9C' }}>
                <div className="text-2xl mb-2">🧠</div>
                <div className="text-sm">暂无已配置的AI模型</div>
              </div>
            )}
          </div>
        </div>

        {/* Exchanges */}
        <div className="binance-card p-4">
          <h3 className="text-lg font-semibold mb-3" style={{ color: '#EAECEF' }}>
            🏦 {t('exchanges', language)}
          </h3>
          <div className="space-y-3">
            {configuredExchanges.map(exchange => {
              const inUse = isExchangeInUse(exchange.id);
              return (
                <div 
                  key={exchange.id} 
                  className={`flex items-center justify-between p-3 rounded transition-all ${
                    inUse ? 'cursor-not-allowed' : 'cursor-pointer hover:bg-gray-700'
                  }`}
                  style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
                  onClick={() => handleExchangeClick(exchange.id)}
                >
                  <div className="flex items-center gap-3">
                    <div className="w-8 h-8 flex items-center justify-center">
                      {getExchangeIcon(exchange.id, { width: 32, height: 32 })}
                    </div>
                    <div>
                      <div className="font-semibold" style={{ color: '#EAECEF' }}>{exchange.name}</div>
                      <div className="text-xs" style={{ color: '#848E9C' }}>
                        {exchange.type.toUpperCase()} • {inUse ? '正在使用' : exchange.enabled ? '已启用' : '已配置'}
                      </div>
                    </div>
                  </div>
                  <div className={`w-3 h-3 rounded-full ${exchange.enabled && exchange.apiKey ? 'bg-green-400' : 'bg-gray-500'}`} />
                </div>
              );
            })}
            {configuredExchanges.length === 0 && (
              <div className="text-center py-8" style={{ color: '#848E9C' }}>
                <div className="text-2xl mb-2">🏦</div>
                <div className="text-sm">暂无已配置的交易所</div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Traders List */}
      <div className="binance-card p-6">
        <div className="flex items-center justify-between mb-5">
          <h2 className="text-xl font-bold flex items-center gap-2" style={{ color: '#EAECEF' }}>
            👥 {t('currentTraders', language)}
          </h2>
        </div>

        {traders && traders.length > 0 ? (
          <div className="space-y-4">
            {traders.map(trader => (
              <div key={trader.trader_id} 
                   className="flex items-center justify-between p-4 rounded transition-all hover:translate-y-[-1px]"
                   style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
                <div className="flex items-center gap-4">
                  <div className="w-12 h-12 rounded-full flex items-center justify-center text-xl"
                       style={{
                         background: getModelByUniqueId(trader.ai_model_unique_id)?.provider?.includes('deepseek') ? '#60a5fa' : '#c084fc',
                         color: '#fff'
                       }}>
                    🤖
                  </div>
                  <div>
                    <div className="font-bold text-lg" style={{ color: '#EAECEF' }}>
                      {trader.trader_name}
                    </div>
                    <div className="text-sm" style={{
                      color: getModelByUniqueId(trader.ai_model_unique_id)?.provider?.includes('deepseek') ? '#60a5fa' : '#c084fc'
                    }}>
                      {getModelDisplayName(getModelByUniqueId(trader.ai_model_unique_id)?.provider || 'unknown')} Model • {getExchangeByUniqueId(trader.exchange_unique_id)?.id?.toUpperCase() || 'UNKNOWN'}
                    </div>
                  </div>
                </div>

                <div className="flex items-center gap-4">
                  {/* Status */}
                  <div className="text-center">
                    <div className="text-xs mb-1" style={{ color: '#848E9C' }}>{t('status', language)}</div>
                    <div className={`px-3 py-1 rounded text-xs font-bold ${
                      trader.is_running ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
                    }`} style={trader.is_running 
                      ? { background: 'rgba(14, 203, 129, 0.1)', color: '#0ECB81' }
                      : { background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }
                    }>
                      {trader.is_running ? t('running', language) : t('stopped', language)}
                    </div>
                  </div>

                  {/* Actions */}
                  <div className="flex gap-2">
                    <button
                      onClick={() => onTraderSelect?.(trader.trader_id)}
                      className="px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                      style={{ background: 'rgba(99, 102, 241, 0.1)', color: '#6366F1' }}
                    >
                      📊 查看
                    </button>
                    
                    <button
                      onClick={() => handleToggleTrader(trader.trader_id, trader.is_running || false)}
                      className="px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                      style={trader.is_running 
                        ? { background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }
                        : { background: 'rgba(14, 203, 129, 0.1)', color: '#0ECB81' }
                      }
                    >
                      {trader.is_running ? t('stop', language) : t('start', language)}
                    </button>
                    
                    <button
                      onClick={() => handleDeleteTrader(trader.trader_id)}
                      className="px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                      style={{ background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }}
                    >
                      🗑️
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="text-center py-16" style={{ color: '#848E9C' }}>
            <div className="text-6xl mb-4 opacity-50">🤖</div>
            <div className="text-lg font-semibold mb-2">{t('noTraders', language)}</div>
            <div className="text-sm mb-4">{t('createFirstTrader', language)}</div>
            {(configuredModels.length === 0 || configuredExchanges.length === 0) && (
              <div className="text-sm text-yellow-500">
                {configuredModels.length === 0 && configuredExchanges.length === 0 
                  ? t('configureModelsAndExchangesFirst', language)
                  : configuredModels.length === 0 
                    ? t('configureModelsFirst', language)
                    : t('configureExchangesFirst', language)
                }
              </div>
            )}
          </div>
        )}
      </div>

      {/* Create Trader Modal */}
      {showCreateModal && (
        <CreateTraderModal
          enabledModels={enabledModels}
          enabledExchanges={enabledExchanges}
          onCreate={handleCreateTrader}
          onClose={() => setShowCreateModal(false)}
          language={language}
        />
      )}

      {/* Model Configuration Modal */}
      {showModelModal && (
        <ModelConfigModal
          allModels={supportedModels}
          editingModelId={editingModel}
          onSave={handleSaveModelConfig}
          onDelete={handleDeleteModelConfig}
          onClose={() => {
            setShowModelModal(false);
            setEditingModel(null);
          }}
          language={language}
        />
      )}

      {/* Exchange Configuration Modal */}
      {showExchangeModal && (
        <ExchangeConfigModal
          allExchanges={supportedExchanges}
          editingExchangeId={editingExchange}
          onSave={handleSaveExchangeConfig}
          onDelete={handleDeleteExchangeConfig}
          onClose={() => {
            setShowExchangeModal(false);
            setEditingExchange(null);
          }}
          language={language}
        />
      )}
    </div>
  );
}

// Create Trader Modal Component
function CreateTraderModal({ 
  enabledModels, 
  enabledExchanges,
  onCreate, 
  onClose, 
  language 
}: {
  enabledModels: AIModel[];
  enabledExchanges: Exchange[];
  onCreate: (modelId: string, exchangeId: string, name: string, initialBalance: number, customPrompt?: string, overrideBase?: boolean) => void;
  onClose: () => void;
  language: any;
}) {
  // 默认选择DeepSeek模型，如果没有启用则选择第一个
  const defaultModel = enabledModels.find(m => m.id === 'deepseek') || enabledModels[0];
  // 默认选择Binance交易所，如果没有启用则选择第一个
  const defaultExchange = enabledExchanges.find(e => e.id === 'binance') || enabledExchanges[0];
  
  const [selectedModel, setSelectedModel] = useState(defaultModel?.id || '');
  const [selectedExchange, setSelectedExchange] = useState(defaultExchange?.id || '');
  const [traderName, setTraderName] = useState('');
  const [initialBalance, setInitialBalance] = useState(1000);
  const [customPrompt, setCustomPrompt] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [overrideBase, setOverrideBase] = useState(false);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedModel || !selectedExchange || !traderName.trim()) return;
    
    onCreate(selectedModel, selectedExchange, traderName.trim(), initialBalance, customPrompt.trim() || undefined, overrideBase);
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div className="bg-gray-800 rounded-lg p-6 w-full max-w-lg max-h-[90vh] overflow-y-auto" style={{ background: '#1E2329' }}>
        <h3 className="text-xl font-bold mb-4" style={{ color: '#EAECEF' }}>
          {t('createNewTrader', language)}
        </h3>
        
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
              {t('selectAIModel', language)}
            </label>
            <select
              value={selectedModel}
              onChange={(e) => setSelectedModel(e.target.value)}
              className="w-full px-3 py-2 rounded"
              style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
              required
            >
              {enabledModels.map(model => (
                <option key={model.id} value={model.id}>
                  {model.name}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
              {t('traderName', language)}
            </label>
            <input
              type="text"
              value={traderName}
              onChange={(e) => setTraderName(e.target.value)}
              placeholder={t('enterTraderName', language)}
              className="w-full px-3 py-2 rounded"
              style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
              required
            />
          </div>

          <div>
            <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
              {t('selectExchange', language)}
            </label>
            <select
              value={selectedExchange}
              onChange={(e) => setSelectedExchange(e.target.value)}
              className="w-full px-3 py-2 rounded"
              style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
              required
            >
              {enabledExchanges.map(exchange => (
                <option key={exchange.id} value={exchange.id}>
                  {exchange.name} ({exchange.type.toUpperCase()})
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
              初始资金 (USDT)
            </label>
            <input
              type="number"
              value={initialBalance}
              onChange={(e) => setInitialBalance(Number(e.target.value))}
              min="100"
              max="100000"
              className="w-full px-3 py-2 rounded"
              style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
              required
            />
          </div>
          
          {/* Advanced Settings Toggle */}
          <div className="mt-4">
            <button
              type="button"
              onClick={() => setShowAdvanced(!showAdvanced)}
              className="flex items-center gap-2 text-sm font-semibold"
              style={{ color: '#F0B90B' }}
            >
              <span style={{ transform: showAdvanced ? 'rotate(90deg)' : 'rotate(0)', transition: 'transform 0.2s' }}>▶</span>
              高级设置
            </button>
          </div>
          
          {/* Custom Prompt Field - Show when advanced is toggled */}
          {showAdvanced && (
            <div className="mt-4">
              <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                自定义交易策略 (可选)
              </label>
              <textarea
                value={customPrompt}
                onChange={(e) => setCustomPrompt(e.target.value)}
                placeholder="例如：专注于主流币种BTC/ETH/SOL，避免MEME币。使用保守策略，单笔仓位不超过账户的30%..."
                rows={5}
                className="w-full px-3 py-2 rounded resize-none"
                style={{ 
                  background: '#0B0E11', 
                  border: '1px solid #2B3139', 
                  color: '#EAECEF',
                  fontSize: '14px'
                }}
              />
              <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                输入自定义的交易策略和规则，将作为AI交易员的额外指导。留空使用默认策略。
              </div>
              
              {/* Override Base Strategy Checkbox */}
              {customPrompt.trim() && (
                <div className="mt-3 p-3 rounded" style={{ background: 'rgba(246, 70, 93, 0.1)', border: '1px solid rgba(246, 70, 93, 0.2)' }}>
                  <label className="flex items-start gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={overrideBase}
                      onChange={(e) => setOverrideBase(e.target.checked)}
                      className="mt-1"
                      style={{ accentColor: '#F6465D' }}
                    />
                    <div>
                      <div className="text-sm font-semibold" style={{ color: '#F6465D' }}>
                        覆盖基础交易策略
                      </div>
                      <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                        ⚠️ 警告：勾选后将完全使用您的自定义策略，不再使用系统默认的风控和交易逻辑。
                        这可能导致交易风险增加。仅在您完全理解交易逻辑时使用此选项。
                      </div>
                    </div>
                  </label>
                </div>
              )}
            </div>
          )}

          <div className="flex gap-3 mt-6">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              {t('create', language)}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// Model Configuration Modal Component  
function ModelConfigModal({
  allModels,
  editingModelId,
  onSave,
  onDelete,
  onClose,
  language
}: {
  allModels: AIModel[];
  editingModelId: string | null;
  onSave: (modelId: string, apiKey: string) => void;
  onDelete: (modelId: string) => void;
  onClose: () => void;
  language: any;
}) {
  const [selectedModelId, setSelectedModelId] = useState(editingModelId || '');
  const [apiKey, setApiKey] = useState('');

  // 获取当前编辑的模型信息
  const selectedModel = allModels?.find(m => m.id === selectedModelId);

  // 如果是编辑现有模型，初始化API Key
  useEffect(() => {
    if (editingModelId && selectedModel) {
      setApiKey(selectedModel.apiKey || '');
    }
  }, [editingModelId, selectedModel]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedModelId || !apiKey.trim()) return;
    
    onSave(selectedModelId, apiKey.trim());
  };

  // 可选择的模型列表（所有支持的模型）
  const availableModels = allModels || [];

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div className="bg-gray-800 rounded-lg p-6 w-full max-w-lg relative" style={{ background: '#1E2329' }}>
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            {editingModelId ? '编辑AI模型' : '添加AI模型'}
          </h3>
          {editingModelId && (
            <button
              type="button"
              onClick={() => {
                if (confirm('确定要删除此AI模型配置吗？')) {
                  onDelete(editingModelId);
                }
              }}
              className="p-2 rounded hover:bg-red-100 transition-colors"
              style={{ background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }}
              title="删除配置"
            >
              🗑️
            </button>
          )}
        </div>
        
        <form onSubmit={handleSubmit} className="space-y-4">
          {!editingModelId && (
            <div>
              <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                选择AI模型
              </label>
              <select
                value={selectedModelId}
                onChange={(e) => setSelectedModelId(e.target.value)}
                className="w-full px-3 py-2 rounded"
                style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                required
              >
                <option value="">请选择模型</option>
                {availableModels.map(model => (
                  <option key={model.id} value={model.id}>
                    {model.name} ({model.provider})
                  </option>
                ))}
              </select>
            </div>
          )}

          {selectedModel && (
            <div className="p-4 rounded" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
              <div className="flex items-center gap-3 mb-3">
                <div className="w-8 h-8 flex items-center justify-center">
                  {getModelIcon(selectedModel.provider || selectedModel.id, { width: 32, height: 32 }) || (
                    <div className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold"
                         style={{ 
                           background: selectedModel.id === 'deepseek' ? '#60a5fa' : '#c084fc',
                           color: '#fff'
                         }}>
                      {selectedModel.name[0]}
                    </div>
                  )}
                </div>
                <div>
                  <div className="font-semibold" style={{ color: '#EAECEF' }}>{selectedModel.name}</div>
                  <div className="text-xs" style={{ color: '#848E9C' }}>{selectedModel.provider}</div>
                </div>
              </div>
              
              <div>
                <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                  API Key
                </label>
                <input
                  type="password"
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  placeholder={`请输入 ${selectedModel.name} API Key`}
                  className="w-full px-3 py-2 rounded"
                  style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                  required
                />
              </div>
            </div>
          )}

          <div className="flex gap-3 mt-6">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              disabled={!selectedModelId || !apiKey.trim()}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              {t('save', language)}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// Exchange Configuration Modal Component
function ExchangeConfigModal({
  allExchanges,
  editingExchangeId,
  onSave,
  onDelete,
  onClose,
  language
}: {
  allExchanges: Exchange[];
  editingExchangeId: string | null;
  onSave: (exchangeId: string, apiKey: string, secretKey?: string, testnet?: boolean, hyperliquidWalletAddr?: string, asterUser?: string, asterSigner?: string, asterPrivateKey?: string) => void;
  onDelete: (exchangeId: string) => void;
  onClose: () => void;
  language: any;
}) {
  const [selectedExchangeId, setSelectedExchangeId] = useState(editingExchangeId || '');
  const [apiKey, setApiKey] = useState('');
  const [secretKey, setSecretKey] = useState('');
  const [testnet, setTestnet] = useState(false);
  // Hyperliquid 特定字段
  const [hyperliquidWalletAddr, setHyperliquidWalletAddr] = useState('');
  // Aster 特定字段
  const [asterUser, setAsterUser] = useState('');
  const [asterSigner, setAsterSigner] = useState('');
  const [asterPrivateKey, setAsterPrivateKey] = useState('');

  // 获取当前编辑的交易所信息
  const selectedExchange = allExchanges?.find(e => e.id === selectedExchangeId);

  // 如果是编辑现有交易所，初始化表单数据
  useEffect(() => {
    if (editingExchangeId && selectedExchange) {
      setApiKey(selectedExchange.apiKey || '');
      setSecretKey(selectedExchange.secretKey || '');
      setTestnet(selectedExchange.testnet || false);
      setHyperliquidWalletAddr(selectedExchange.hyperliquidWalletAddr || '');
      setAsterUser(selectedExchange.asterUser || '');
      setAsterSigner(selectedExchange.asterSigner || '');
      setAsterPrivateKey(selectedExchange.asterPrivateKey || '');
    }
  }, [editingExchangeId, selectedExchange]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedExchangeId) return;
    
    // 根据交易所类型验证不同字段
    if (selectedExchange?.id === 'hyperliquid') {
      if (!apiKey.trim() || !hyperliquidWalletAddr.trim()) return;
    } else if (selectedExchange?.id === 'aster') {
      if (!asterUser.trim() || !asterSigner.trim() || !asterPrivateKey.trim()) return;
    } else {
      // Binance 等其他交易所
      if (!apiKey.trim() || !secretKey.trim()) return;
    }
    
    onSave(selectedExchangeId, apiKey.trim(), secretKey.trim(), testnet, 
           hyperliquidWalletAddr.trim(), asterUser.trim(), asterSigner.trim(), asterPrivateKey.trim());
  };

  // 可选择的交易所列表（所有支持的交易所）
  const availableExchanges = allExchanges || [];

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div className="bg-gray-800 rounded-lg p-6 w-full max-w-lg relative" style={{ background: '#1E2329' }}>
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            {editingExchangeId ? '编辑交易所' : '添加交易所'}
          </h3>
          {editingExchangeId && (
            <button
              type="button"
              onClick={() => {
                if (confirm('确定要删除此交易所配置吗？')) {
                  onDelete(editingExchangeId);
                }
              }}
              className="p-2 rounded hover:bg-red-100 transition-colors"
              style={{ background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }}
              title="删除配置"
            >
              🗑️
            </button>
          )}
        </div>
        
        <form onSubmit={handleSubmit} className="space-y-4">
          {!editingExchangeId && (
            <div>
              <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                选择交易所
              </label>
              <select
                value={selectedExchangeId}
                onChange={(e) => setSelectedExchangeId(e.target.value)}
                className="w-full px-3 py-2 rounded"
                style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                required
              >
                <option value="">请选择交易所</option>
                {availableExchanges.map(exchange => (
                  <option key={exchange.id} value={exchange.id}>
                    {exchange.name} ({exchange.type.toUpperCase()})
                  </option>
                ))}
              </select>
            </div>
          )}

          {selectedExchange && (
            <div className="p-4 rounded" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
              <div className="flex items-center gap-3 mb-3">
                <div className="w-8 h-8 flex items-center justify-center">
                  {getExchangeIcon(selectedExchange.id, { width: 32, height: 32 })}
                </div>
                <div>
                  <div className="font-semibold" style={{ color: '#EAECEF' }}>{selectedExchange.name}</div>
                  <div className="text-xs" style={{ color: '#848E9C' }}>{selectedExchange.type.toUpperCase()}</div>
                </div>
              </div>
              
              <div className="space-y-3">
                {/* Binance 配置 */}
                {selectedExchange.id === 'binance' && (
                  <>
                    <div>
                      <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                        API Key
                      </label>
                      <input
                        type="password"
                        value={apiKey}
                        onChange={(e) => setApiKey(e.target.value)}
                        placeholder="请输入 Binance API Key"
                        className="w-full px-3 py-2 rounded"
                        style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                        required
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                        Secret Key
                      </label>
                      <input
                        type="password"
                        value={secretKey}
                        onChange={(e) => setSecretKey(e.target.value)}
                        placeholder="请输入 Binance Secret Key"
                        className="w-full px-3 py-2 rounded"
                        style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                        required
                      />
                    </div>
                  </>
                )}

                {/* Hyperliquid 配置 */}
                {selectedExchange.id === 'hyperliquid' && (
                  <>
                    <div>
                      <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                        Private Key (无需0x前缀)
                      </label>
                      <input
                        type="password"
                        value={apiKey}
                        onChange={(e) => setApiKey(e.target.value)}
                        placeholder="请输入以太坊私钥"
                        className="w-full px-3 py-2 rounded"
                        style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                        required
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                        钱包地址
                      </label>
                      <input
                        type="text"
                        value={hyperliquidWalletAddr}
                        onChange={(e) => setHyperliquidWalletAddr(e.target.value)}
                        placeholder="请输入以太坊钱包地址"
                        className="w-full px-3 py-2 rounded"
                        style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                        required
                      />
                    </div>
                    <div className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        checked={testnet}
                        onChange={(e) => setTestnet(e.target.checked)}
                        className="w-4 h-4"
                      />
                      <label className="text-sm" style={{ color: '#EAECEF' }}>
                        使用测试网
                      </label>
                    </div>
                  </>
                )}

                {/* Aster 配置 */}
                {selectedExchange.id === 'aster' && (
                  <>
                    <div>
                      <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                        用户地址
                      </label>
                      <input
                        type="text"
                        value={asterUser}
                        onChange={(e) => setAsterUser(e.target.value)}
                        placeholder="请输入 Aster 用户地址"
                        className="w-full px-3 py-2 rounded"
                        style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                        required
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                        签名者地址
                      </label>
                      <input
                        type="text"
                        value={asterSigner}
                        onChange={(e) => setAsterSigner(e.target.value)}
                        placeholder="请输入 Aster 签名者地址"
                        className="w-full px-3 py-2 rounded"
                        style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                        required
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                        私钥
                      </label>
                      <input
                        type="password"
                        value={asterPrivateKey}
                        onChange={(e) => setAsterPrivateKey(e.target.value)}
                        placeholder="请输入 Aster 私钥"
                        className="w-full px-3 py-2 rounded"
                        style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                        required
                      />
                    </div>
                  </>
                )}
              </div>
            </div>
          )}

          <div className="flex gap-3 mt-6">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              disabled={
                !selectedExchangeId || 
                (selectedExchange?.id === 'binance' && (!apiKey.trim() || !secretKey.trim())) ||
                (selectedExchange?.id === 'hyperliquid' && (!apiKey.trim() || !hyperliquidWalletAddr.trim())) ||
                (selectedExchange?.id === 'aster' && (!asterUser.trim() || !asterSigner.trim() || !asterPrivateKey.trim()))
              }
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              {t('save', language)}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}