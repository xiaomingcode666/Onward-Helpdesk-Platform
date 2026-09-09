import { createContext, useContext, type ReactNode } from 'react';

export type RailopsLocale = 'zh-CN' | 'en-US' | 'es-ES';

export type RailopsLocaleText = {
  applyFilters: string;
  cancel: string;
  collapseSidebar: string;
  confirm: string;
  emptyData: string;
  expandSidebar: string;
  filter: string;
  filterOptions: string;
  filtered: string;
  forbidden: string;
  forbiddenDescription: string;
  loading: string;
  loadFailed: string;
  loadFailedDescription: string;
  moduleTabs: string;
  paginationTotal: string;
  product: string;
  reset: string;
  resetAll: string;
  retry: string;
  searchProduct: string;
  submit: string;
  unitDevice: string;
  yes: string;
};

const localeText: Record<RailopsLocale, RailopsLocaleText> = {
  'zh-CN': {
    applyFilters: '应用筛选',
    cancel: '取消',
    collapseSidebar: '折叠侧边栏',
    confirm: '确认',
    emptyData: '暂无数据',
    expandSidebar: '展开侧边栏',
    filter: '筛选',
    filterOptions: '筛选选项',
    filtered: '已筛选',
    forbidden: '无权访问',
    forbiddenDescription: '暂无访问权限',
    loading: '加载中...',
    loadFailed: '加载失败',
    loadFailedDescription: '数据加载失败，请稍后重试',
    moduleTabs: '模块内容选项',
    paginationTotal: '共 {total} 条',
    product: '产品',
    reset: '重置',
    resetAll: '重置全部',
    retry: '重新加载',
    searchProduct: '搜索产品...',
    submit: '提交',
    unitDevice: '{count} 台',
    yes: '是',
  },
  'en-US': {
    applyFilters: 'Apply filters',
    cancel: 'Cancel',
    collapseSidebar: 'Collapse sidebar',
    confirm: 'Confirm',
    emptyData: 'No data',
    expandSidebar: 'Expand sidebar',
    filter: 'Filter',
    filterOptions: 'Filter options',
    filtered: 'Filtered',
    forbidden: 'Access denied',
    forbiddenDescription: 'You do not have access.',
    loading: 'Loading...',
    loadFailed: 'Load failed',
    loadFailedDescription: 'Data could not be loaded. Please try again later.',
    moduleTabs: 'Module tabs',
    paginationTotal: '{total} records',
    product: 'Products',
    reset: 'Reset',
    resetAll: 'Reset all',
    retry: 'Reload',
    searchProduct: 'Search products...',
    submit: 'Submit',
    unitDevice: '{count} devices',
    yes: 'Yes',
  },
  'es-ES': {
    applyFilters: 'Aplicar filtros',
    cancel: 'Cancelar',
    collapseSidebar: 'Contraer barra lateral',
    confirm: 'Confirmar',
    emptyData: 'Sin datos',
    expandSidebar: 'Expandir barra lateral',
    filter: 'Filtrar',
    filterOptions: 'Opciones de filtro',
    filtered: 'Filtrado',
    forbidden: 'Acceso denegado',
    forbiddenDescription: 'No tienes acceso.',
    loading: 'Cargando...',
    loadFailed: 'Error al cargar',
    loadFailedDescription: 'No se pudieron cargar los datos. Intentalo mas tarde.',
    moduleTabs: 'Pestanas del modulo',
    paginationTotal: '{total} registros',
    product: 'Productos',
    reset: 'Restablecer',
    resetAll: 'Restablecer todo',
    retry: 'Recargar',
    searchProduct: 'Buscar productos...',
    submit: 'Enviar',
    unitDevice: '{count} dispositivos',
    yes: 'Si',
  },
};

const RailopsLocaleContext = createContext<RailopsLocaleText | null>(null);

export function RailopsLocaleProvider({
  children,
  locale,
  text,
}: {
  children: ReactNode;
  locale: string;
  text?: Partial<RailopsLocaleText>;
}) {
  const baseText = localeText[normalizeRailopsLocale(locale)];
  return (
    <RailopsLocaleContext.Provider value={text ? { ...baseText, ...text } : baseText}>
      {children}
    </RailopsLocaleContext.Provider>
  );
}

export function useRailopsLocaleText(text?: Partial<RailopsLocaleText>) {
  const contextText = useContext(RailopsLocaleContext);
  const baseText = contextText ?? localeText[readDocumentLocale()];
  return text ? { ...baseText, ...text } : baseText;
}

export function formatRailopsText(
  template: string,
  values: Record<string, string | number>,
) {
  return template.replace(/\{(\w+)\}/g, (match, key) =>
    Object.prototype.hasOwnProperty.call(values, key) ? String(values[key]) : match,
  );
}

function normalizeRailopsLocale(locale: string): RailopsLocale {
  if (locale === 'en-US') return 'en-US';
  if (locale === 'es-ES') return 'es-ES';
  return 'zh-CN';
}

function readDocumentLocale(): RailopsLocale {
  if (typeof document === 'undefined') {
    return 'zh-CN';
  }
  return normalizeRailopsLocale(document.documentElement.lang);
}
